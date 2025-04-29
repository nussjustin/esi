package esiexpr

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

// SyntaxError is returned by [Parse] and [ParseVariable] when encountering unexpected or invalid data.
type SyntaxError struct {
	// Offset is the position in the input where the error occurred.
	Offset int

	// Message may contain a custom message that describes the error
	Message string

	// Underlying optionally contains the underlying error that lead to this error.
	Underlying error
}

// Error returns a human-readable error message.
func (s *SyntaxError) Error() string {
	if s.Message == "" {
		return fmt.Sprintf("invalid syntax at offset %d", s.Offset)
	}

	return fmt.Sprintf("invalid syntax at offset %d: %s", s.Offset, s.Message)
}

// Is checks if the given error matches the receiver.
func (s *SyntaxError) Is(err error) bool {
	var o *SyntaxError
	return errors.As(err, &o) && o.Error() == s.Error() && errors.Is(o.Underlying, s.Underlying)
}

// Unwrap returns s.Underlying.
func (s *SyntaxError) Unwrap() error {
	return s.Underlying
}

// Node is the interface implemented by all possible types of parsed nodes.
type Node interface {
	node()
}

// Position specifies a start and end position in a parsed expression.
type Position struct {
	// Start is the inclusive start index.
	Start int

	// End is the exclusive end index.
	End int
}

// Pos returns the start and end position of the [Node].
func (p Position) Pos() (start, end int) {
	return p.Start, p.End
}

// String implements the [fmt.GoStringer] interface.
func (p Position) String() string {
	return strconv.Itoa(p.Start) + ":" + strconv.Itoa(p.End)
}

// AndNode represents two sub-expressions combined with the and operator (&).
type AndNode struct {
	// Position specifies the position of the node inside the expression.
	Position Position

	// Left contains the expression to the left of the operator.
	Left Node

	// Right contains the expression to the right of the operator.
	Right Node
}

func (*AndNode) node() {}

// ComparisonNode represents a comparison between two values using one of the supported comparison operators.
type ComparisonNode struct {
	// Position specifies the position of the node inside the expression.
	Position Position

	// Operator contains the parsed operator.
	Operator ComparisonOperator

	// Left contains the expression to the left of the operator.
	Left Node

	// Right contains the expression to the right of the operator.
	Right Node
}

func (*ComparisonNode) node() {}

// ComparisonOperator is an enum of supported comparison operators.
type ComparisonOperator string

const (
	// ComparisonOperatorEquals is the type for comparisons using the "==" operator.
	ComparisonOperatorEquals ComparisonOperator = "=="

	// ComparisonOperatorGreaterThan is the type for comparisons using the ">" operator.
	ComparisonOperatorGreaterThan ComparisonOperator = ">"

	// ComparisonOperatorGreaterThanEquals is the type for comparisons using the ">=" operator.
	ComparisonOperatorGreaterThanEquals ComparisonOperator = ">="

	// ComparisonOperatorLessThan is the type for comparisons using the "<=>=" operator.
	ComparisonOperatorLessThan ComparisonOperator = "<"

	// ComparisonOperatorLessThanEquals is the type for comparisons using the "<=>=" operator.
	ComparisonOperatorLessThanEquals ComparisonOperator = "<="

	// ComparisonOperatorNotEquals is the type for comparisons using the "!=" operator.
	ComparisonOperatorNotEquals ComparisonOperator = "!="
)

// NegateNode represents a sub-expression negated using the unary negation operator (!).
type NegateNode struct {
	// Position specifies the position of the node inside the expression.
	Position Position

	// Expr is the negated sub-expression.
	Expr Node
}

func (*NegateNode) node() {}

// OrNode represents two sub-expressions combined with the or operator (|).
type OrNode struct {
	// Position specifies the position of the node inside the expression.
	Position Position

	// Left contains the expression to the left of the operator.
	Left Node

	// Right contains the expression to the right of the operator.
	Right Node
}

func (*OrNode) node() {}

// Value is a value of type bool, float64, int, string or nil.
type Value any

// ValueNode represents a parsed value.
type ValueNode struct {
	// Position specifies the position of the node inside the expression.
	Position Position

	// Value contains the parsed value.
	Value Value
}

func (*ValueNode) node() {}

// VariableNode represents a variable reference including its default value, if any.
type VariableNode struct {
	// Position specifies the position of the node inside the expression.
	Position Position

	// Name contains the parsed variable name.
	Name string

	// Key is the name of the key inside the referenced dictionary or list.
	Key *string

	// Default contains the default value, if any.
	Default Node
}

func (*VariableNode) node() {}

var parserPool = sync.Pool{
	New: func() any {
		return &parser{}
	},
}

func getParser(data string) *parser {
	p, _ := parserPool.Get().(*parser)
	p.reset(data)
	return p
}

func putParser(p *parser) {
	p.reset("")
	parserPool.Put(p)
}

// Parse parses the given ESI expression into a tree of nodes.
func Parse(data string) (Node, error) {
	p := getParser(data)
	defer putParser(p)

	node, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	p.discardSpaces()

	if p.offset < len(p.data) {
		return nil, &SyntaxError{Offset: p.offset, Message: "unexpected data after expression"}
	}

	return node, nil
}

type parser struct {
	data   string
	offset int
}

func (p *parser) consume(c byte) error {
	c1, ok := p.peek()
	if !ok {
		return &SyntaxError{
			Offset:     p.offset,
			Message:    "end of input, '" + string(rune(c)) + "' expected",
			Underlying: io.ErrUnexpectedEOF,
		}
	}

	if c1 != c {
		return &SyntaxError{
			Offset:  p.offset,
			Message: "unexpected character '" + string(rune(c1)) + "', '" + string(rune(c)) + "' expected",
		}
	}

	p.offset++

	return nil
}

func (p *parser) tryConsume(c byte) bool {
	if p.offset >= len(p.data) || p.data[p.offset] != c {
		return false
	}
	p.offset++
	return true
}

func (p *parser) discardSpaces() {
	for p.offset < len(p.data) {
		switch p.data[p.offset] {
		case ' ', '\r', '\n', '\t':
			p.offset++
		default:
			return
		}
	}
}

func (p *parser) peek() (byte, bool) {
	if p.offset >= len(p.data) {
		return 0, false
	}
	return p.data[p.offset], true
}

func (p *parser) readQuotedString() (string, error) {
	if err := p.consume('\''); err != nil {
		return "", err
	}

	start := p.offset

	for {
		c, ok := p.peek()
		if !ok {
			return "", &SyntaxError{Offset: p.offset, Message: "missing closing quote"}
		}

		p.offset++

		if c == '\'' {
			break
		}
	}

	return p.data[start : p.offset-1], nil
}

func (p *parser) readString(typ string) (string, error) {
	start := p.offset

loop:
	for {
		c, ok := p.peek()
		if !ok {
			break
		}

		switch {
		case c == '|' || c == '(' || c == ')' || c == '{' || c == '}':
			break loop
		case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
			p.offset++
		case c == ' ', c == '\r', c == '\n', c == '\t':
			return "", &SyntaxError{Offset: p.offset, Message: "unexpected space in " + typ}
		default:
			return "", &SyntaxError{Offset: p.offset, Message: "unexpected character '" + string(rune(c)) + "' in " + typ}
		}
	}

	return p.data[start:p.offset], nil
}

func (p *parser) readStringMaybeQuoted(typ string) (string, error) {
	if c, _ := p.peek(); c == '\'' {
		return p.readQuotedString()
	}
	return p.readString(typ)
}

func (p *parser) parseBoolFalse() (Node, error) {
	start := p.offset

	for _, c := range [...]byte{'f', 'a', 'l', 's', 'e'} {
		if err := p.consume(c); err != nil {
			return nil, err
		}
	}

	return &ValueNode{
		Position: Position{Start: start, End: p.offset},
		Value:    false,
	}, nil
}

func (p *parser) parseBoolTrue() (Node, error) {
	start := p.offset

	for _, c := range [...]byte{'t', 'r', 'u', 'e'} {
		if err := p.consume(c); err != nil {
			return nil, err
		}
	}

	return &ValueNode{
		Position: Position{Start: start, End: p.offset},
		Value:    true,
	}, nil
}

func (p *parser) parseExpr() (Node, error) {
	p.discardSpaces()

	start := p.offset

	node, err := p.parseSingleExpr()
	if err != nil {
		return nil, err
	}

	p.discardSpaces()

	switch c, _ := p.peek(); c {
	case '!':
		_ = p.consume('!')

		if err := p.consume('='); err != nil {
			return nil, err
		}

		node1, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		return &ComparisonNode{
			Position: Position{Start: start, End: p.offset},
			Operator: ComparisonOperatorNotEquals,
			Left:     node,
			Right:    node1,
		}, nil
	case '=':
		_ = p.consume('=')

		if err := p.consume('='); err != nil {
			return nil, err
		}

		node1, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		return &ComparisonNode{
			Position: Position{Start: start, End: p.offset},
			Operator: ComparisonOperatorEquals,
			Left:     node,
			Right:    node1,
		}, nil
	case '<':
		_ = p.consume('<')

		op := ComparisonOperatorLessThan

		if p.tryConsume('=') {
			op = ComparisonOperatorLessThanEquals
		}

		node1, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		return &ComparisonNode{
			Position: Position{Start: start, End: p.offset},
			Operator: op,
			Left:     node,
			Right:    node1,
		}, nil
	case '>':
		_ = p.consume('>')

		op := ComparisonOperatorGreaterThan

		if p.tryConsume('=') {
			op = ComparisonOperatorGreaterThanEquals
		}

		node1, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		return &ComparisonNode{
			Position: Position{Start: start, End: p.offset},
			Operator: op,
			Left:     node,
			Right:    node1,
		}, nil
	case '|':
		_ = p.consume('|')

		node1, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		return &OrNode{
			Position: Position{Start: start, End: p.offset},
			Left:     node,
			Right:    node1,
		}, nil
	case '&':
		_ = p.consume('&')

		node1, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		return &AndNode{
			Position: Position{Start: start, End: p.offset},
			Left:     node,
			Right:    node1,
		}, nil
	default:
		return node, nil
	}
}

func (p *parser) parseNegation() (Node, error) {
	start := p.offset

	if err := p.consume('!'); err != nil {
		return nil, err
	}

	expr, err := p.parseSingleExpr()
	if err != nil {
		return nil, err
	}

	return &NegateNode{
		Position: Position{Start: start, End: p.offset},
		Expr:     expr,
	}, nil
}

func (p *parser) parseNull() (Node, error) {
	start := p.offset

	for _, c := range [...]byte{'n', 'u', 'l', 'l'} {
		if err := p.consume(c); err != nil {
			return nil, err
		}
	}

	return &ValueNode{
		Position: Position{Start: start, End: p.offset},
		Value:    nil,
	}, nil
}

func (p *parser) parseNumber() (Node, error) {
	start := p.offset

	_ = p.tryConsume('-')

intLoop:
	for {
		c, _ := p.peek()

		switch {
		case '0' <= c && c <= '9' || c == '-':
			p.offset++
		default:
			break intLoop
		}
	}

	var isFloat bool

	if p.tryConsume('.') {
		isFloat = true

	decLoop:
		for {
			c, _ := p.peek()

			switch {
			case '0' <= c && c <= '9' || c == '-':
				p.offset++
			default:
				break decLoop
			}
		}
	}

	if isFloat {
		f, err := strconv.ParseFloat(p.data[start:p.offset], 64)
		if err != nil {
			return nil, &SyntaxError{Offset: start, Message: "invalid number", Underlying: err}
		}

		return &ValueNode{
			Position: Position{Start: start, End: p.offset},
			Value:    f,
		}, nil
	}

	n, err := strconv.ParseInt(p.data[start:p.offset], 10, 64)
	if err != nil {
		return nil, &SyntaxError{Offset: start, Message: "invalid number", Underlying: err}
	}

	return &ValueNode{
		Position: Position{Start: start, End: p.offset},
		Value:    int(n),
	}, nil
}

func (p *parser) parseQuotedString() (Node, error) {
	start := p.offset

	s, err := p.readQuotedString()
	if err != nil {
		return nil, err
	}

	return &ValueNode{
		Position: Position{Start: start, End: p.offset},
		Value:    s,
	}, nil
}

func (p *parser) parseSingleExpr() (Node, error) {
	p.discardSpaces()

	c, ok := p.peek()

	switch {
	case !ok:
		return nil, &SyntaxError{
			Offset:     p.offset,
			Message:    "expression expected",
			Underlying: io.ErrUnexpectedEOF,
		}
	case c == 'f':
		return p.parseBoolFalse()
	case c == 'n':
		return p.parseNull()
	case c == 't':
		return p.parseBoolTrue()
	case c == '\'':
		return p.parseQuotedString()
	case c == '$':
		return p.parseVar()
	case c == '!':
		return p.parseNegation()
	case c == '(':
		return p.parseSubExpr()
	case c >= '0' && c <= '9' || c == '-':
		return p.parseNumber()
	default:
		return nil, &SyntaxError{Offset: p.offset, Message: "unexpected character '" + string(c) + "'"}
	}
}

func (p *parser) parseSubExpr() (Node, error) {
	if err := p.consume('('); err != nil {
		return nil, err
	}

	p.discardSpaces()

	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	p.discardSpaces()

	if err := p.consume(')'); err != nil {
		return nil, err
	}

	return expr, nil
}

func (p *parser) parseVar() (Node, error) {
	start := p.offset

	if err := p.consume('$'); err != nil {
		return nil, err
	}

	if err := p.consume('('); err != nil {
		return nil, err
	}

	name, err := p.readString("variable name")
	if err != nil {
		return nil, err
	}

	var key *string

	if p.tryConsume('{') {
		key1, err := p.readStringMaybeQuoted("key")
		if err != nil {
			return nil, err
		}

		if err := p.consume('}'); err != nil {
			return nil, err
		}

		key = &key1
	}

	var defaultExpr Node

	if p.tryConsume('|') {
		defaultExpr, err = p.parseVarDefault()
		if err != nil {
			return nil, err
		}
	}

	if err := p.consume(')'); err != nil {
		return nil, err
	}

	return &VariableNode{
		Position: Position{Start: start, End: p.offset},
		Name:     name,
		Key:      key,
		Default:  defaultExpr,
	}, nil
}

func (p *parser) parseVarDefault() (Node, error) {
	switch c, _ := p.peek(); c {
	case '\'':
		return p.parseQuotedString()
	case '$':
		return p.parseVar()
	default:
		start := p.offset

		s, err := p.readString("default value")
		if err != nil {
			return nil, err
		}

		return &ValueNode{
			Position: Position{Start: start, End: p.offset},
			Value:    s,
		}, nil
	}
}

func (p *parser) reset(data string) {
	*p = parser{data: data}
}
