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
	"github.com/nussjustin/esi/esiexpr"
)

// InvalidExpressionResultError is returned when the result of an expression has the wrong type.
type InvalidExpressionResultError struct {
	// Element is the element for which the error was reported.
	Element esi.Element

	// Expr is the raw evaluated expression.
	Expr string

	// Result is the result of the expression.
	Result esiexpr.Value
}

// Error returns a human-readable error message.
func (e *InvalidExpressionResultError) Error() string {
	return fmt.Sprintf("invalid expression result %q", fmt.Sprint(e.Result))
}

// Is checks if the given error matches the receiver.
func (e *InvalidExpressionResultError) Is(err error) bool {
	var o *InvalidExpressionResultError
	return errors.As(err, &o) && o.Error() == e.Error()
}

// UnexpectedElementError is returned when encountering an element that is not expected in the given context.
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

// UnsupportedElementError is returned when encountering an element that is not supported, either because it is not
// implemented or because the configuration of the [Processor] is not configured to handle it.
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

// Env implements methods for processing ESI expressions and variables.
type Env interface {
	// Eval evaluates the given ESI expression and returns the boolean result.
	Eval(ctx context.Context, expr string) (esiexpr.Value, error)

	// Interpolate replaces variables inside the given string with their actual or default value.
	Interpolate(ctx context.Context, s string) (string, error)
}

// FetchFunc defines the signature for functions used to fetch data for <esi:include/> elements.
type FetchFunc func(ctx context.Context, urlStr string) ([]byte, error)

// ProcessorOpt is the type for functions that can be used to customize the behaviour of a [Processor].
type ProcessorOpt func(*processorOptions)

type processorOptions struct {
	env              Env
	fetchConcurrency int
	fetchFunc        FetchFunc
}

// WithEnv specifies the environment to use for processing.
//
// If env is nil, <esi:when/> elements will be unsupported, leading to [UnsupportedElementError] when one is found.
func WithEnv(env Env) ProcessorOpt {
	return func(p *processorOptions) {
		p.env = env
	}
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
// If a non-nil [Env] is specified, using [WithEnv], both the src and alt attributes of the esi:include element will
// have any variables inside replaced via [Env.Interpolate].
//
// Other elements are not supported and will result in an error when trying to process them.
//
// Processor is safe for concurrent use.
type Processor struct {
	opts processorOptions

	ctx    context.Context //nolint:containedctx
	cancel context.CancelFunc

	wg sync.WaitGroup
	ch chan *fetchPromise
}

type fetchPromise struct {
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
	p := &Processor{ch: make(chan *fetchPromise)}
	p.opts.fetchConcurrency = 1

	for _, opt := range opts {
		opt(&p.opts)
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())

	// No need for any workers if we don't support fetching
	if p.opts.fetchFunc == nil {
		return p
	}

	for range p.opts.fetchConcurrency {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.handleFetchPromises()
		}()
	}

	return p
}

// Process processes the given data and writes the result to dst.
//
// When encountering an unsupported element, [errors.ErrUnsupported] is returned.
//
// If Process is called after Release, an error is returned.
func (p *Processor) Process(ctx context.Context, dst io.Writer, nodes []esi.Node) error {
	if err := p.ctx.Err(); err != nil {
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
	p.cancel()
	p.wg.Wait()
}

func (p *Processor) processNode(
	ctx context.Context,
	dst io.Writer,
	node esi.Node,
	promise *fetchPromise,
) error {
	switch v := node.(type) {
	case *esi.AttemptElement:
		return &UnexpectedElementError{Element: v}
	case *esi.CommentElement:
		return nil
	case *esi.ChooseElement:
		for _, w := range v.When {
			if p.opts.env == nil {
				return &UnsupportedElementError{Element: w}
			}

			result, err := p.opts.env.Eval(ctx, w.Test)
			if err != nil {
				return err
			}

			resultBool, ok := result.(bool)
			if !ok {
				return &InvalidExpressionResultError{Element: w, Expr: w.Test, Result: result}
			}

			if !resultBool {
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

		if promise == nil {
			if promise, err = p.queueFetch(ctx, v); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-promise.done:
			data, err = promise.data, promise.err
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

func (p *Processor) processNodes(ctx context.Context, dst io.Writer, nodes []esi.Node) error {
	promises := p.tryQueueFetches(ctx, nodes)

	for _, node := range nodes {
		if err := p.processNode(ctx, dst, node, promises[node]); err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) fetch(ctx context.Context, inc *esi.IncludeElement) ([]byte, error) {
	data, err := p.fetchURLWithVars(ctx, inc.Source)

	if err != nil && inc.Alt != "" {
		data, err = p.fetchURLWithVars(ctx, inc.Alt)
	}

	return data, err
}

func (p *Processor) fetchURLWithVars(ctx context.Context, urlWithVars string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if p.opts.env == nil {
		return p.opts.fetchFunc(ctx, urlWithVars)
	}

	url, err := p.opts.env.Interpolate(ctx, urlWithVars)
	if err != nil {
		return nil, err
	}

	return p.opts.fetchFunc(ctx, url)
}

func (p *Processor) handleFetchPromise(item *fetchPromise) {
	defer close(item.done)
	item.data, item.err = p.fetch(item.ctx, item.inc)
}

func (p *Processor) handleFetchPromises() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case item := <-p.ch:
			p.handleFetchPromise(item)
		}
	}
}

func (p *Processor) queueFetch(ctx context.Context, inc *esi.IncludeElement) (*fetchPromise, error) {
	if p.opts.fetchFunc == nil {
		return nil, &UnsupportedElementError{Element: inc}
	}

	item := &fetchPromise{
		ctx:  ctx,
		inc:  inc,
		done: make(chan struct{}),
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p.ch <- item:
		return item, nil
	}
}

func (p *Processor) tryQueueFetch(ctx context.Context, inc *esi.IncludeElement) *fetchPromise {
	if p.opts.fetchFunc == nil {
		return nil
	}

	f := &fetchPromise{
		ctx:  ctx,
		inc:  inc,
		done: make(chan struct{}),
	}

	select {
	case p.ch <- f:
		return f
	default:
		return nil
	}
}

func (p *Processor) tryQueueFetches(ctx context.Context, nodes []esi.Node) map[esi.Node]*fetchPromise {
	var m map[esi.Node]*fetchPromise

	for _, node := range nodes {
		inc, ok := node.(*esi.IncludeElement)
		if !ok {
			continue
		}

		f := p.tryQueueFetch(ctx, inc)
		if f == nil {
			continue
		}

		if m == nil {
			m = make(map[esi.Node]*fetchPromise)
		}

		m[inc] = f
	}

	return m
}
