// Package esiproc implements functions for processing documents using ESI.
package esiproc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/nussjustin/esi"
)

type UnexpectedElementError struct {
	// Element is the element for which the error was reported.
	Element esi.Element
}

// Error returns a human-readable error message.
func (e *UnexpectedElementError) Error() string {
	start, end := e.Element.Pos()
	return fmt.Sprintf("unexpected element %s at position %d:%d", e.Element.Name(), start, end)
}

// Is checks if the given error matches the receiver.
func (e *UnexpectedElementError) Is(err error) bool {
	var o *UnexpectedElementError
	return errors.As(err, &o) && o.Error() == e.Error()
}

type UnsupportedElementError struct {
	// Element is the element for which the error was reported.
	Element esi.Element
}

// Error returns a human-readable error message.
func (e *UnsupportedElementError) Error() string {
	start, end := e.Element.Pos()
	return fmt.Sprintf("unsupported element %s at position %d:%d", e.Element.Name(), start, end)
}

// Is checks if the given error matches the receiver.
func (e *UnsupportedElementError) Is(err error) bool {
	var o *UnsupportedElementError
	return errors.As(err, &o) && o.Error() == e.Error()
}

// Unwrap returns [errors.ErrUnsupported].
func (e *UnsupportedElementError) Unwrap() error {
	return errors.ErrUnsupported
}

// FetchFunc defines the signature for functions used fetch data for <esi:include/> elements.
type FetchFunc func(ctx context.Context, urlStr string) ([]byte, error)

// TestFunc defines the signature for functions used to evaluate <esi:when test="..."/> expressions.
type TestFunc func(ctx context.Context, expr string) (bool, error)

// ProcessorOpt is the type for functions that can be used to customize the behaviour of a [Processor].
type ProcessorOpt func(*processorOptions)

type processorOptions struct {
	fetchConcurrency int
	fetchFunc        FetchFunc
	testFunc         TestFunc
}

// WithFetchConcurrency configures a [Processor] to make at most n concurrent fetches at a time.
//
// If n is < 1, WithFetchConcurrency panics.
func WithFetchConcurrency(n int) ProcessorOpt {
	if n < 1 {
		panic("WithFetchConcurrency called with n < 1")
	}

	return func(p *processorOptions) {
		p.fetchConcurrency = n
	}
}

// WithFetchFunc specifies the function used to resolve <esi:include/> elements.
//
// If f is nil, <esi:include/> elements will be unsupported, leading to [UnsupportedElementError] when one is found.
func WithFetchFunc(f FetchFunc) ProcessorOpt {
	return func(p *processorOptions) {
		p.fetchFunc = f
	}
}

// WithTestFunc specifies the function used to check <esi:when test="..."/> expressions.
//
// If f is nil, <esi:when/> elements will be unsupported, leading to [UnsupportedElementError] when one is found.
func WithTestFunc(f TestFunc) ProcessorOpt {
	return func(p *processorOptions) {
		p.testFunc = f
	}
}

// Processor implements the handling of ESI elements.
//
// The following elements are supported:
// - esi:attempt
// - esi:choose
// - esi:comment
// - esi:except
// - esi:include (see [WithFetchFunc], including alt and onerror)
// - esi:otherwise
// - esi:remove
// - esi:try
// - esi:when (see [WithTestFunc])
//
// Other elements are not supported and will result in an error when trying to process them.
//
// Processor is safe for concurrent use.
type Processor struct {
	opts processorOptions

	workerCtx    context.Context //nolint:containedctx
	workerCancel context.CancelFunc
	workerWg     sync.WaitGroup

	queue *queue[*fetchQueueItem]
}

type fetchQueueItem struct {
	ctx context.Context //nolint:containedctx
	inc *esi.IncludeElement

	data []byte
	err  error

	done chan struct{}
}

// New creates a new Processor and applies the given options.
//
// The default is equivalent to: New(WithFetchConcurrency(1), WithFetchFunc(nil), WithTestFunc(nil)).
func New(opts ...ProcessorOpt) *Processor {
	p := &Processor{queue: newQueue[*fetchQueueItem]()}
	p.opts.fetchConcurrency = 1

	for _, opt := range opts {
		opt(&p.opts)
	}

	p.workerCtx, p.workerCancel = context.WithCancel(context.Background())

	for range p.opts.fetchConcurrency {
		p.workerWg.Add(1)
		go func() {
			defer p.workerWg.Done()
			p.worker()
		}()
	}

	return p
}

// Process processes the given data and writes the result to dst.
//
// When encountering an unsupported element, [errors.ErrUnsupported] is returned.
//
// If Process is called after Release, an error is returned.
func (p *Processor) Process(ctx context.Context, dst io.Writer, nodes esi.Nodes) error {
	if err := p.workerCtx.Err(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return p.processNodes(ctx, dst, nodes)
}

// Release releases all resources associated with the Processor.
//
// If called multiple times, all but the first call will be no-ops.
func (p *Processor) Release() {
	p.workerCancel()
	p.workerWg.Wait()
}

func (p *Processor) processNode(
	ctx context.Context,
	dst io.Writer,
	node esi.Node,
	item *fetchQueueItem,
) error {
	switch v := node.(type) {
	case *esi.AttemptElement:
		return &UnexpectedElementError{Element: v}
	case *esi.CommentElement:
		return nil
	case *esi.ChooseElement:
		for _, w := range v.When {
			if p.opts.testFunc == nil {
				return &UnsupportedElementError{Element: w}
			}

			ok, err := p.opts.testFunc(ctx, w.Test)
			if err != nil {
				return err
			}

			if !ok {
				continue
			}

			return p.processNodes(ctx, dst, w.Nodes)
		}

		if v.Otherwise == nil {
			return nil
		}

		return p.processNodes(ctx, dst, v.Otherwise.Nodes)
	case *esi.ExceptElement:
		return &UnexpectedElementError{Element: v}
	case *esi.IncludeElement:
		var data []byte
		var err error

		if item != nil {
			select {
			case <-ctx.Done():
				err = ctx.Err()
			case <-item.done:
				data, err = item.data, item.err
			}
		} else {
			data, err = p.fetch(ctx, v)
		}

		if err != nil {
			if v.OnError == esi.ErrorBehaviourContinue {
				return nil
			}

			return err
		}

		_, err = dst.Write(data)
		return err
	case *esi.InlineElement:
		return &UnsupportedElementError{Element: v}
	case *esi.OtherwiseElement:
		return &UnexpectedElementError{Element: v}
	case *esi.RemoveElement:
		return nil
	case *esi.RawData:
		_, err := dst.Write(v.Bytes)
		return err
	case *esi.TryElement:
		var buf bytes.Buffer

		if err := p.processNodes(ctx, &buf, v.Attempt.Nodes); err == nil {
			_, err := buf.WriteTo(dst)
			return err
		}

		return p.processNodes(ctx, dst, v.Except.Nodes)
	case *esi.VarsElement:
		return &UnsupportedElementError{Element: v}
	case *esi.WhenElement:
		return &UnexpectedElementError{Element: v}
	default:
		panic("unreachable")
	}
}

func (p *Processor) processNodes(ctx context.Context, dst io.Writer, nodes esi.Nodes) error {
	queuedFetches := p.queueFetchesFromNodes(ctx, nodes)

	for _, node := range nodes {
		if err := p.processNode(ctx, dst, node, queuedFetches[node]); err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) fetch(ctx context.Context, inc *esi.IncludeElement) ([]byte, error) {
	item := p.queueFetch(ctx, inc)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-item.done:
	}

	return item.data, item.err
}

func (p *Processor) fetchNow(ctx context.Context, inc *esi.IncludeElement) ([]byte, error) {
	if p.opts.fetchFunc == nil {
		return nil, &UnsupportedElementError{Element: inc}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := p.opts.fetchFunc(ctx, inc.Source)

	if err == nil {
		return data, nil
	}

	if inc.Alt == "" {
		return nil, err
	}

	return p.opts.fetchFunc(ctx, inc.Alt)
}

func (p *Processor) queueFetch(ctx context.Context, inc *esi.IncludeElement) *fetchQueueItem {
	item := &fetchQueueItem{
		ctx:  ctx,
		inc:  inc,
		done: make(chan struct{}),
	}

	p.queue.push(item)

	return item
}

func (p *Processor) queueFetchesFromNodes(ctx context.Context, nodes esi.Nodes) map[esi.Node]*fetchQueueItem {
	var m map[esi.Node]*fetchQueueItem

	for _, node := range nodes {
		include, ok := node.(*esi.IncludeElement)
		if !ok {
			continue
		}

		if m == nil {
			m = make(map[esi.Node]*fetchQueueItem)
		}

		m[include] = p.queueFetch(ctx, include)
	}

	return m
}

func (p *Processor) process(item *fetchQueueItem) {
	defer close(item.done)
	item.data, item.err = p.fetchNow(item.ctx, item.inc)
}

func (p *Processor) worker() {
	for {
		item, ok := p.queue.pop(p.workerCtx.Done())
		if !ok {
			return
		}
		p.process(item)
	}
}
