// Package esiproc implements functions for processing documents using ESI.
package esiproc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/nussjustin/esi"
	"github.com/nussjustin/esi/esiexpr/ast"
	"github.com/nussjustin/esi/esixml"
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

// Env implements methods for processing ESI expressions and variables.
type Env interface {
	// Eval evaluates the given ESI expression and returns the boolean result.
	Eval(ctx context.Context, expr string) (any, error)

	// Interpolate replaces variables inside the given string with their actual or default value.
	Interpolate(ctx context.Context, s string) (string, error)
}

// IncludeFunc defines the signature for functions used to include data for <esi:include/> elements.
type IncludeFunc func(ctx context.Context, urlStr string, meta map[string]string) ([]byte, error)

// ProcessorOpt is the type for functions that can be used to customize the behaviour of a [Processor].
type ProcessorOpt func(*processorOptions)

type processorOptions struct {
	env            Env
	incConcurrency int
	incFunc        IncludeFunc
}

// WithEnv specifies the environment to use for processing.
//
// If env is nil, <esi:when/> elements will be unsupported, leading to [UnsupportedElementError] when one is found.
func WithEnv(env Env) ProcessorOpt {
	return func(p *processorOptions) {
		p.env = env
	}
}

// WithIncludeConcurrency configures a [Processor] to make at most n concurrent calls to the configured [IncludeFunc]
// at a time.
//
// If n is < 1, WithIncludeConcurrency panics.
func WithIncludeConcurrency(n int) ProcessorOpt {
	if n < 1 {
		panic("WithIncludeConcurrency called with n < 1")
	}

	return func(p *processorOptions) {
		p.incConcurrency = n
	}
}

// WithIncludeFunc specifies the function used to resolve <esi:include/> elements.
//
// If f is nil, <esi:include/> elements will be unsupported, leading to [UnsupportedElementError] when one is found.
func WithIncludeFunc(f IncludeFunc) ProcessorOpt {
	return func(p *processorOptions) {
		p.incFunc = f
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
// The default is equivalent to: New(WithIncludeConcurrency(1), WithIncludeFunc(nil), WithTestFunc(nil)).
func New(opts ...ProcessorOpt) *Processor {
	p := &Processor{}
	p.opts.incConcurrency = 1

	for _, opt := range opts {
		opt(&p.opts)
	}

	p.incSema = make(chan struct{}, p.opts.incConcurrency)

	return p
}

// Process processes the given data and writes the result to w.
//
// When encountering an unsupported element, [errors.ErrUnsupported] is returned.
//
// If Process is called after Release, an error is returned.
func (p *Processor) Process(ctx context.Context, w io.Writer, nodes iter.Seq2[esi.Node, error]) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resC := make(chan processedNode, 32)

	go func() {
		defer close(resC)
		p.processNodesIter(ctx, resC, nodes)
	}()

	var n int

	for {
		var res processedNode
		var ok bool

		select {
		case <-ctx.Done():
			return n, ctx.Err()
		case res, ok = <-resC:
			if !ok {
				return n, nil
			}

			data, err := res.wait(ctx)
			if err != nil {
				return 0, err
			}

			n1, err := w.Write(data)
			if err != nil {
				return 0, err
			}

			n += n1
		}
	}
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
		if p.opts.env == nil {
			send(nil, nil, &UnsupportedElementError{Element: v})
			return
		}

		for _, w := range v.When {
			result, err := p.opts.env.Eval(ctx, w.Test)
			if err != nil {
				send(nil, nil, err)
				return
			}

			resultBool, ok := result.(bool)
			if !ok {
				send(nil, nil, &InvalidExpressionResultError{Element: w, Expr: w.Test, Result: result})
				return
			}

			if !resultBool {
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
		if p.opts.incFunc == nil {
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

func attrsToMap(attrs []esixml.Attr) map[string]string {
	if len(attrs) == 0 {
		return nil
	}

	m := make(map[string]string, len(attrs))

	for _, attr := range attrs {
		m[attr.Name.String()] = attr.Value
	}

	return m
}

func (p *Processor) include(ctx context.Context, ele *esi.IncludeElement) (*include, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p.incSema <- struct{}{}:
	}

	inc := &include{done: make(chan struct{})}

	go func() {
		defer close(inc.done)
		defer func() {
			<-p.incSema
		}()

		meta := attrsToMap(ele.Attr)

		inc.data, inc.err = p.includeURL(ctx, ele.Source, meta)

		if inc.err != nil && ele.Alt != "" {
			inc.data, inc.err = p.includeURL(ctx, ele.Alt, meta)
		}

		if inc.err != nil && ele.OnError == esi.ErrorBehaviourContinue {
			inc.err = nil
		}
	}()

	return inc, nil
}

func (p *Processor) includeURL(ctx context.Context, urlStr string, meta map[string]string) ([]byte, error) {
	if p.opts.env == nil {
		return p.opts.incFunc(ctx, urlStr, meta)
	}

	urlStr, err := p.opts.env.Interpolate(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	return p.opts.incFunc(ctx, urlStr, meta)
}
