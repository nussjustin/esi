package esiexpr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/nussjustin/esi/esiexpr/ast"
)

// ComparisonUnsupportedError is returned by [Env.Eval] if comparison should be made but [Env.CompareValues] is nil.
type ComparisonUnsupportedError struct {
	// Operator is the requested operation.
	Operator ast.ComparisonOperator
}

// Error returns a human-readable message.
func (c *ComparisonUnsupportedError) Error() string {
	return "comparison " + string(c.Operator) + " not supported"
}

// Is checks if the given error matches the receiver.
func (c *ComparisonUnsupportedError) Is(target error) bool {
	if errors.Is(target, errors.ErrUnsupported) {
		return true
	}

	var o *ComparisonUnsupportedError
	return errors.As(target, &o) && o.Operator == c.Operator
}

// NonBoolValueError is returned by [Env.Eval] if a non-bool value is encountered in a context that requires a bool.
type NonBoolValueError struct {
	// Value is the offending value.
	Value ast.Value
}

// Error returns a human-readable message.
func (n *NonBoolValueError) Error() string {
	return "value is not a boolean"
}

// Is checks if the given error matches the receiver.
func (n *NonBoolValueError) Is(target error) bool {
	if errors.Is(target, errors.ErrUnsupported) {
		return true
	}

	var o *NonBoolValueError
	return errors.As(target, &o) && n.Value == o.Value
}

// Env implements methods for evaluating ESI expressions and interpolating variables in strings.
type Env struct {
	// CompareValues is called by [Eval] when comparing values.
	//
	// The result must be a value < 0 if a compares less than b, > 0 if a compares greater than b or 0 if they compare
	// equal.
	//
	// If CompareValues is nil, an error is returned when a comparison is required.
	CompareValues func(a, b ast.Value) (int, error)

	// LookupVar is called by [Env.Eval] and [Env.Interpolate] to get the value for a variable.
	LookupVar func(ctx context.Context, name string, key *string) (ast.Value, error)

	// ValueToBool is called when trying to convert a non-bool value into a bool.
	//
	// If ValueToBool is nil, an error is returned when encountering a non-bool value in a bool context.
	ValueToBool func(v ast.Value) (bool, error)
}

var parserPool = sync.Pool{
	New: func() any {
		return &ast.Parser[string]{}
	},
}

func getParser(data string) *ast.Parser[string] {
	p := parserPool.Get().(*ast.Parser[string])
	p.Reset(data)
	return p
}

func poolParser(p *ast.Parser[string]) {
	p.Reset("")
	parserPool.Put(p)
}

// Eval evaluates the given expression and returns the result.
//
// It implements the [esiproc.EvalFunc] signature.
func (e *Env) Eval(ctx context.Context, data string) (any, error) {
	p := getParser(data)
	defer poolParser(p)

	node, err := p.Parse()
	if err != nil {
		return nil, err
	}

	return e.eval(ctx, node)
}

// Interpolate replaces all ESI variables in the given string.
//
// It implements the [esiproc.InterpolateFunc] signature.
func (e *Env) Interpolate(ctx context.Context, s string) (string, error) {
	p := getParser("")
	defer poolParser(p)

	var b strings.Builder

	for s != "" {
		index := strings.Index(s, "$(")
		if index == -1 {
			break
		}

		_, _ = b.WriteString(s[:index])

		p.Reset(s[index:])

		v, err := p.ParseVariable()
		if err != nil {
			return "", err
		}

		val, err := e.evalVariable(ctx, v)
		if err != nil {
			return "", err
		}

		if val != nil {
			_, _ = fmt.Fprintf(&b, "%v", val)
		}

		s = s[index+v.Position.End:]
	}

	// Optimization: If we have no variables at all, return the original string.
	if b.Len() == 0 {
		return s, nil
	}

	if s != "" {
		b.WriteString(s)
	}

	return b.String(), nil
}

var (
	falseVal = ast.Value(false)
	trueVal  = ast.Value(true)
)

func (e *Env) eval(ctx context.Context, node ast.Node) (ast.Value, error) {
	switch v := node.(type) {
	case *ast.AndNode:
		return e.evalAnd(ctx, v)
	case *ast.ComparisonNode:
		return e.evalComparison(ctx, v)
	case *ast.NegateNode:
		return e.evalNot(ctx, v)
	case *ast.OrNode:
		return e.evalOr(ctx, v)
	case *ast.ValueNode:
		return v.Value, nil
	case *ast.VariableNode:
		return e.evalVariable(ctx, v)
	default:
		panic("unreachable")
	}
}

func (e *Env) evalAnd(ctx context.Context, node *ast.AndNode) (ast.Value, error) {
	leftVal, err := e.eval(ctx, node.Left)
	if err != nil {
		return nil, err
	}

	left, err := e.valueToBool(leftVal)
	if err != nil {
		return nil, err
	}

	rightVal, err := e.eval(ctx, node.Right)
	if err != nil {
		return nil, err
	}

	right, err := e.valueToBool(rightVal)
	if err != nil {
		return nil, err
	}

	if left && right {
		return trueVal, nil
	}

	return falseVal, nil
}

func (e *Env) evalComparison(ctx context.Context, node *ast.ComparisonNode) (ast.Value, error) {
	if e.CompareValues == nil {
		return nil, &ComparisonUnsupportedError{Operator: node.Operator}
	}

	leftVal, err := e.eval(ctx, node.Left)
	if err != nil {
		return nil, err
	}

	rightVal, err := e.eval(ctx, node.Right)
	if err != nil {
		return nil, err
	}

	diff, err := e.CompareValues(leftVal, rightVal)
	if err != nil {
		return nil, err
	}

	toVal := func(b bool) ast.Value {
		if b {
			return trueVal
		}
		return falseVal
	}

	switch node.Operator {
	case ast.ComparisonOperatorEquals:
		return toVal(diff == 0), nil
	case ast.ComparisonOperatorGreaterThan:
		return toVal(diff > 0), nil
	case ast.ComparisonOperatorGreaterThanEquals:
		return toVal(diff >= 0), nil
	case ast.ComparisonOperatorLessThan:
		return toVal(diff < 0), nil
	case ast.ComparisonOperatorLessThanEquals:
		return toVal(diff <= 0), nil
	case ast.ComparisonOperatorNotEquals:
		return toVal(diff != 0), nil
	default:
		panic("unreachable")
	}
}

func (e *Env) evalNot(ctx context.Context, node *ast.NegateNode) (ast.Value, error) {
	exprVal, err := e.eval(ctx, node.Expr)
	if err != nil {
		return nil, err
	}

	val, err := e.valueToBool(exprVal)
	if err != nil {
		return nil, err
	}

	if val {
		return falseVal, nil
	}

	return trueVal, nil
}

func (e *Env) evalOr(ctx context.Context, node *ast.OrNode) (ast.Value, error) {
	leftVal, err := e.eval(ctx, node.Left)
	if err != nil {
		return nil, err
	}

	left, err := e.valueToBool(leftVal)
	if err != nil {
		return nil, err
	}

	rightVal, err := e.eval(ctx, node.Right)
	if err != nil {
		return nil, err
	}

	right, err := e.valueToBool(rightVal)
	if err != nil {
		return nil, err
	}

	if left || right {
		return trueVal, nil
	}

	return falseVal, nil
}

func (e *Env) evalVariable(ctx context.Context, node *ast.VariableNode) (ast.Value, error) {
	val, err := e.LookupVar(ctx, node.Name, node.Key)
	if err != nil {
		return nil, err
	}

	if val != nil {
		return val, nil
	}

	if node.Default == nil {
		return val, nil
	}

	switch v := node.Default.(type) {
	case *ast.VariableNode:
		return e.evalVariable(ctx, v)
	case *ast.ValueNode:
		return v.Value, nil
	default:
		panic("unreachable")
	}
}

func (e *Env) valueToBool(val ast.Value) (bool, error) {
	if b, ok := val.(bool); ok {
		return b, nil
	}

	if e.ValueToBool == nil {
		return false, &NonBoolValueError{Value: val}
	}

	return e.ValueToBool(val)
}
