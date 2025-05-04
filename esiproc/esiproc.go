// Package esiproc implements functions for processing documents using ESI.
package esiproc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"sync"

	"github.com/nussjustin/esi"
	"github.com/nussjustin/esi/esiexpr/ast"
)

// InvalidExpressionResultError is returned when the result of an expression has the wrong type.
type InvalidExpressionResultError struct {
	// Element is the element for which the error was reported.
	Element esi.Element

	// Expr is the raw evaluated expression.
	Expr string

	// Result is the result of the expression.
	Result ast.Value
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

// Client defines methods used for fetching URLs for the processing of <esi:include/> elements.
type Client interface {
	// Do is called with the URL that should be included (either the src or alt attribute) and should return the
	// data to include.
	Do(ctx context.Context, proc *Processor, urlStr string, extra map[string]string) ([]byte, error)
}

// ClientFunc implements a [Client] by calling itself.
type ClientFunc func(ctx context.Context, proc *Processor, urlStr string, extra map[string]string) ([]byte, error)

// Do calls c and returns the result.
func (c ClientFunc) Do(ctx context.Context, proc *Processor, urlStr string, extra map[string]string) ([]byte, error) {
	return c(ctx, proc, urlStr, extra)
}

// EvalFunc defines the signature for functions used to evaluate bool-producing ESI expressions.
type EvalFunc func(ctx context.Context, expr string) (any, error)

// InterpolateFunc defines the signature for functions used to interpolate variables in a given string.
type InterpolateFunc func(ctx context.Context, s string) (string, error)

// ProcessorOpt is the type for functions that can be used to customize the behaviour of a [Processor].
type ProcessorOpt func(*processorOptions)

type processorOptions struct {
	client            Client
	clientConcurrency int
	evalFunc          EvalFunc
	interpolateFunc   InterpolateFunc
}

// WithClient specifies the client used to process <esi:include/> elements.
//
// If c is nil, <esi:include/> elements will be unsupported.
func WithClient(c Client) ProcessorOpt {
	return func(p *processorOptions) {
		p.client = c
	}
}

// WithClientConcurrency configures a [Processor] to make at most n concurrent calls to the configured [Client].
//
// If n is < 1, WithClientConcurrency panics.
func WithClientConcurrency(n int) ProcessorOpt {
	if n < 1 {
		panic("WithClientConcurrency called with n < 1")
	}

	return func(p *processorOptions) {
		p.clientConcurrency = n
	}
}

// WithEvalFunc specifies the function used to evaluate expressions for <esi:when> elements.
//
// If not given or if the last given function is nil, <esi:choose> elements will be unsupported.
func WithEvalFunc(f EvalFunc) ProcessorOpt {
	return func(p *processorOptions) {
		p.evalFunc = f
	}
}

// WithInterpolateFunc specifies the function used to interpolate variables into URLs for <esi:include> elements.
//
// If not given or if the last given function is nil, no interpolation is performance.
func WithInterpolateFunc(f InterpolateFunc) ProcessorOpt {
	return func(p *processorOptions) {
		p.interpolateFunc = f
	}
}

// Processor implements the handling of ESI elements.
//
// The following elements are supported:
//
//   - esi:attempt
//   - esi:choose
//   - esi:comment
//   - esi:except
//   - esi:include (see [WithIncludeFunc], including alt and onerror)
//   - esi:otherwise
//   - esi:remove
//   - esi:try
//   - esi:when (see [WithEnv])
//
// If a non-nil [Env] is specified, using [WithEnv], both the src and alt attributes of the esi:include element will
// have any variables inside replaced via [Env.Interpolate].
//
// Other elements are not supported and will result in an error when trying to process them.
//
// Processor is safe for concurrent use.
type Processor struct {
	opts    processorOptions
	incSema chan struct{}
}

type include struct {
	done chan struct{}
	data []byte
	err  error
}

type processedNode struct {
	inc  *include
	data []byte
	err  error
}

func (p *processedNode) wait(ctx context.Context) ([]byte, error) {
	if p.err != nil || p.inc == nil {
		return p.data, p.err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.inc.done:
	}

	return p.inc.data, p.inc.err
}

// New creates a new Processor and applies the given options.
//
// The default is equivalent to: New(WithClientConcurrency(1)).
func New(opts ...ProcessorOpt) *Processor {
	p := &Processor{}
	p.opts.clientConcurrency = 1

	for _, opt := range opts {
		opt(&p.opts)
	}

	p.incSema = make(chan struct{}, p.opts.clientConcurrency)

	return p
}

// Process processes the given data and writes the result to w.
//
// When encountering an unsupported element, [errors.ErrUnsupported] is returned.
//
// If Process is called after Release, an error is returned.
func (p *Processor) Process(ctx context.Context, w io.Writer, nodes iter.Seq2[esi.Node, error]) (int, error) {
	ctx, cancel := context.WithCancel(ctx)

	resC := make(chan processedNode, 32)

	var wg sync.WaitGroup
	wg.Add(2)

	var firstErr error
	var totalWritten int

	go func() {
		defer wg.Done()
		defer close(resC)

		p.processNodesIter(ctx, resC, nodes)
	}()

	go func() {
		defer wg.Done()
		defer cancel()

		for {
			var res processedNode
			var ok bool

			select {
			case <-ctx.Done():
				firstErr = ctx.Err()
				return
			case res, ok = <-resC:
				if !ok {
					return
				}

				data, err := res.wait(ctx)
				if err != nil {
					firstErr = err
					return
				}

				n1, err := w.Write(data)
				if err != nil {
					firstErr = err
					return
				}

				totalWritten += n1
			}
		}
	}()

	// Ensure we are completely finished with reading from nodes to avoid data races when re-using parsers.
	wg.Wait()

	if firstErr != nil {
		return 0, firstErr
	}

	return totalWritten, nil
}

func (p *Processor) eval(ctx context.Context, choose *esi.ChooseElement, when *esi.WhenElement) (bool, error) {
	if p.opts.evalFunc == nil {
		return false, &UnsupportedElementError{Element: choose}
	}

	result, err := p.opts.evalFunc(ctx, when.Test)
	if err != nil {
		return false, err
	}

	resultBool, ok := result.(bool)
	if !ok {
		return false, &InvalidExpressionResultError{Element: when, Expr: when.Test, Result: result}
	}

	return resultBool, nil
}

func (p *Processor) interpolate(ctx context.Context, s string) (string, error) {
	if p.opts.interpolateFunc == nil {
		return s, nil
	}

	return p.opts.interpolateFunc(ctx, s)
}

func (p *Processor) processNode(ctx context.Context, resC chan<- processedNode, node esi.Node) {
	send := func(data []byte, inc *include, err error) {
		select {
		case <-ctx.Done():
		case resC <- processedNode{inc: inc, data: data, err: err}:
		}
	}

	switch v := node.(type) {
	case *esi.AttemptElement:
		send(nil, nil, &UnexpectedElementError{Element: v})
	case *esi.CommentElement:
	case *esi.ChooseElement:
		for _, w := range v.When {
			result, err := p.eval(ctx, v, w)
			if err != nil {
				send(nil, nil, err)
				return
			}

			if !result {
				continue
			}

			p.processNodes(ctx, resC, w.Nodes)
			return
		}

		if v.Otherwise == nil {
			return
		}

		p.processNodes(ctx, resC, v.Otherwise.Nodes)
	case *esi.ExceptElement:
		send(nil, nil, &UnexpectedElementError{Element: v})
	case *esi.IncludeElement:
		if p.opts.client == nil {
			send(nil, nil, &UnsupportedElementError{Element: v})
			return
		}

		inc, err := p.include(ctx, v)

		send(nil, inc, err)
	case *esi.InlineElement:
		send(nil, nil, &UnsupportedElementError{Element: v})
	case *esi.OtherwiseElement:
		send(nil, nil, &UnexpectedElementError{Element: v})
	case *esi.RemoveElement:
	case *esi.RawData:
		send(v.Bytes, nil, nil)
	case *esi.TryElement:
		attemptCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		attemptC := make(chan processedNode, 32)

		go func() {
			defer close(attemptC)
			p.processNodes(attemptCtx, attemptC, v.Attempt.Nodes)
		}()

		var allData [][]byte

		for attempt := range attemptC {
			data, err := attempt.wait(ctx)
			if err != nil {
				p.processNodes(ctx, resC, v.Except.Nodes)
				return
			}
			allData = append(allData, data)
		}

		for _, data := range allData {
			send(data, nil, nil)
		}
	case *esi.VarsElement:
		send(nil, nil, &UnsupportedElementError{Element: v})
	case *esi.WhenElement:
		send(nil, nil, &UnexpectedElementError{Element: v})
	default:
		panic("unreachable")
	}
}

func (p *Processor) processNodes(ctx context.Context, resC chan<- processedNode, nodes []esi.Node) {
	for _, node := range nodes {
		p.processNode(ctx, resC, node)
	}
}

func (p *Processor) processNodesIter(ctx context.Context, resC chan<- processedNode, nodes iter.Seq2[esi.Node, error]) {
	for node, err := range nodes {
		if err != nil {
			select {
			case <-ctx.Done():
			case resC <- processedNode{err: err}:
			}
			return
		}

		p.processNode(ctx, resC, node)
	}
}

func (p *Processor) include(ctx context.Context, ele *esi.IncludeElement) (*include, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	inc := &include{done: make(chan struct{})}

	go func() {
		defer close(inc.done)

		var extra map[string]string

		if len(ele.Attr) != 0 {
			extra = make(map[string]string, len(ele.Attr))
			for _, attr := range ele.Attr {
				extra[attr.Name.String()] = attr.Value
			}
		}

		inc.data, inc.err = p.doInclude(ctx, ele.Source, extra)

		if inc.err != nil && ele.Alt != "" {
			inc.data, inc.err = p.doInclude(ctx, ele.Alt, extra)
		}

		if inc.err != nil && ele.OnError == esi.ErrorBehaviourContinue {
			inc.err = nil
		}
	}()

	return inc, nil
}

func (p *Processor) doInclude(ctx context.Context, urlStr string, extra map[string]string) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p.incSema <- struct{}{}:
	}

	defer func() {
		<-p.incSema
	}()

	interpolatedURL, err := p.interpolate(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// TODO: Test extr
	return p.opts.client.Do(ctx, p, interpolatedURL, extra)
}
