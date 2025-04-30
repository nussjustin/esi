package ast

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nussjustin/esi/esiexpr/token"
)

// MissingOperandError is returned when the second operand of an comparison or and/or condition is missing.
type MissingOperandError struct {
	// Offset is the position in the input where the error occurred.
	Offset int
}

// Error returns a human-readable error message.
func (m *MissingOperandError) Error() string {
	return fmt.Sprintf("missing operand at %d", m.Offset)
}

// Is checks if the given error matches the receiver.
func (m *MissingOperandError) Is(err error) bool {
	var o *MissingOperandError
	return errors.As(err, &o) && *o == *m
}

// Error is a generic type for errors occurring during expression parsing.
type Error struct {
	// Offset is the position in the input where the error occurred.
	Offset int

	// Message may contain a custom message that describes the error
	Message string

	// Underlying optionally contains the underlying error that lead to this error.
	Underlying error
}

// Error returns a human-readable error message.
func (s *Error) Error() string {
	if s.Message == "" {
		return fmt.Sprintf("invalid syntax at offset %d", s.Offset)
	}

	return fmt.Sprintf("invalid syntax at offset %d: %s", s.Offset, s.Message)
}

// Is checks if the given error matches the receiver.
func (s *Error) Is(err error) bool {
	var o *Error
	return errors.As(err, &o) && o.Error() == s.Error() && errors.Is(o.Underlying, s.Underlying)
}

// Unwrap returns s.Underlying.
func (s *Error) Unwrap() error {
	return s.Underlying
}

// UnexpectedTokenError is returned when parsing encounters a token that was not expected.
type UnexpectedTokenError struct {
	// Token is the parsed token.
	Token token.Token
}

// Error returns a human-readable error message.
func (u *UnexpectedTokenError) Error() string {
	return fmt.Sprintf("unexpected token %s at position %s", u.Token.Type, u.Token.Position)
}

// Is checks if the given error matches the receiver.
func (u *UnexpectedTokenError) Is(err error) bool {
	var o *UnexpectedTokenError
	return errors.As(err, &o) && *o == *u
}

// UnexpectedWhiteSpaceError is returned when whitespace is encountered in a variable.
type UnexpectedWhiteSpaceError struct {
	// Position contains the start and end index of the whitespace.
	Position token.Position
}

// Error returns a human-readable error message.
func (u *UnexpectedWhiteSpaceError) Error() string {
	return fmt.Sprintf("unexpected whitespace at position %s", u.Position)
}

// Is checks if the given error matches the receiver.
func (u *UnexpectedWhiteSpaceError) Is(err error) bool {
	var o *UnexpectedWhiteSpaceError
	return errors.As(err, &o) && *o == *u
}

// Parser implements parsing of ESI expressions from a string or []byte.
//
// The main reason this is a type and not just a function is to that users can better manage allocations, be re-using
// Parser instances.
type Parser[T []byte | string] struct {
	sc   token.Scanner[T]
	data T

	err error

	bufferedToken    token.Token
	bufferedTokenErr error

	lastToken token.Token
}

// NewParser is a shorthand for creating a new *Parser and calling [Parser.Reset] on it.
func NewParser[T []byte | string](data T) *Parser[T] {
	p := &Parser[T]{}
	p.Reset(data)
	return p
}

// Parse parses the given ESI expression into a tree of nodes.
func (p *Parser[T]) Parse() (Node, error) {
	if err := p.err; err != nil {
		return nil, err
	}

	node, err := p.parse(false)
	if err != nil {
		p.err = err
		return nil, err
	}

	return node, nil
}

// ParseVariable parses a single variable.
func (p *Parser[T]) ParseVariable() (*VariableNode, error) {
	if err := p.err; err != nil {
		return nil, err
	}

	node, err := p.parseVariable()
	if err != nil {
		p.err = err
		return nil, err
	}

	return node.(*VariableNode), nil
}

// Reset resets the internal state of the parse and switches it to parse data.
func (p *Parser[T]) Reset(data T) {
	p.sc.Reset(data)
	p.data = data

	p.err = nil

	p.bufferedToken = token.Token{}
	p.bufferedTokenErr = nil

	p.lastToken = token.Token{}
}

func (p *Parser[T]) next() (token.Token, error) {
	var tok token.Token
	var err error

	if p.bufferedToken.Type != token.TypeInvalid || p.bufferedTokenErr != nil {
		tok, err = p.bufferedToken, p.bufferedTokenErr

		p.bufferedToken, p.bufferedTokenErr = token.Token{}, nil
	} else {
		tok, err = p.sc.Next()
	}

	if err != nil {
		offset := p.sc.Offset()

		if o, ok := err.(interface{ Offset() int }); ok {
			offset = o.Offset()
		}

		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}

		return token.Token{}, &Error{Offset: offset, Underlying: err}
	}

	p.lastToken = tok

	return tok, nil
}

func (p *Parser[T]) nextOfType(t token.Type) (token.Token, error) {
	tok, err := p.next()
	if err != nil {
		return token.Token{}, err
	}

	if tok.Type != t {
		return token.Token{}, &UnexpectedTokenError{Token: tok}
	}

	return tok, nil
}

func (p *Parser[T]) peek() (token.Token, error) {
	if p.bufferedToken.Type != token.TypeInvalid || p.bufferedTokenErr != nil {
		return p.bufferedToken, p.bufferedTokenErr
	}

	tok, err := p.sc.Next()
	p.bufferedToken = tok
	p.bufferedTokenErr = err
	return tok, err
}

func (p *Parser[T]) peekType() token.Type {
	tok, err := p.peek()
	if err != nil {
		return token.TypeInvalid
	}
	return tok.Type
}

func (p *Parser[T]) checkNoWhitespace() error {
	if p.lastToken.Type == token.TypeInvalid {
		return nil
	}

	tok, err := p.peek()
	if err != nil {
		return nil //nolint:nilerr
	}

	if tok.Position.Start == p.lastToken.Position.End {
		return nil
	}

	return &UnexpectedWhiteSpaceError{
		Position: token.Position{
			Start: p.lastToken.Position.End,
			End:   tok.Position.Start,
		},
	}
}

func (p *Parser[T]) readString() (string, token.Token, error) {
	tok, err := p.peek()
	if err != nil {
		return "", token.Token{}, err
	}

	if tok.Type == token.TypeQuotedString {
		return p.readQuotedString()
	}

	return p.readSimpleString()
}

func (p *Parser[T]) readQuotedString() (string, token.Token, error) {
	tok, err := p.nextOfType(token.TypeQuotedString)
	if err != nil {
		return "", token.Token{}, err
	}
	return string(p.data[tok.Position.Start+1 : tok.Position.End-1]), tok, nil
}

func (p *Parser[T]) readSimpleString() (string, token.Token, error) {
	tok, err := p.nextOfType(token.TypeSimpleString)
	if err != nil {
		return "", token.Token{}, err
	}
	return string(p.data[tok.Position.Start:tok.Position.End]), tok, nil
}

func (p *Parser[T]) parse(sub bool) (Node, error) {
	node, err := p.parseSingleOrComparisons()
	if err != nil {
		return nil, err
	}

	for {
		tok, err := p.peek()
		if err != nil {
			if !sub && errors.Is(err, io.EOF) {
				return node, nil
			}
			return nil, err
		}

		if sub && tok.Type == token.TypeClosingParenthesis {
			return node, nil
		}

		switch tok.Type { //nolint:exhaustive
		case token.TypeAnd:
			node, err = p.parseAnd(node)
			if err != nil {
				return nil, err
			}
		case token.TypeOr:
			node, err = p.parseOr(node)
			if err != nil {
				return nil, err
			}
		default:
			return p.unexpected(tok)
		}
	}
}

func (p *Parser[T]) parseAnd(left Node) (Node, error) {
	tok, err := p.nextOfType(token.TypeAnd)
	if err != nil {
		return nil, err
	}

	right, err := p.parseSingleOrComparisons()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &MissingOperandError{Offset: tok.Position.End}
		}
		return nil, err
	}

	return &AndNode{
		Position: token.Position{
			Start: left.Pos().Start,
			End:   right.Pos().End,
		},
		Left:  left,
		Right: right,
	}, nil
}

func (p *Parser[T]) parseOperator(left Node) (Node, error) {
	tok, err := p.next()
	if err != nil {
		return nil, err
	}

	var op ComparisonOperator

	switch tok.Type { //nolint:exhaustive
	case token.TypeEquals:
		op = ComparisonOperatorEquals
	case token.TypeNotEquals:
		op = ComparisonOperatorNotEquals
	case token.TypeGreaterThan:
		op = ComparisonOperatorGreaterThan
	case token.TypeGreaterThanEquals:
		op = ComparisonOperatorGreaterThanEquals
	case token.TypeLessThan:
		op = ComparisonOperatorLessThan
	case token.TypeLessThanEqual:
		op = ComparisonOperatorLessThanEquals
	default:
		return nil, &UnexpectedTokenError{Token: tok}
	}

	right, err := p.parseSingle()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &MissingOperandError{Offset: tok.Position.End}
		}
		return nil, err
	}

	return &ComparisonNode{
		Position: token.Position{
			Start: left.Pos().Start,
			End:   right.Pos().End,
		},
		Operator: op,
		Left:     left,
		Right:    right,
	}, nil
}

func (p *Parser[T]) parseOr(left Node) (Node, error) {
	tok, err := p.nextOfType(token.TypeOr)
	if err != nil {
		return nil, err
	}

	right, err := p.parseSingleOrComparisons()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &MissingOperandError{Offset: tok.Position.End}
		}
		return nil, err
	}

	return &OrNode{
		Position: token.Position{
			Start: left.Pos().Start,
			End:   right.Pos().End,
		},
		Left:  left,
		Right: right,
	}, nil
}

func (p *Parser[T]) parseNegation() (Node, error) {
	tok, err := p.nextOfType(token.TypeNegation)
	if err != nil {
		return nil, err
	}

	node, err := p.parseSingle()
	if err != nil {
		return nil, err
	}

	return &NegateNode{
		Position: token.Position{
			Start: tok.Position.Start,
			// Use last token so that we handle closing parenthesis
			End: p.lastToken.Position.End,
		},
		Expr: node,
	}, nil
}

func (p *Parser[T]) parseQuotedString() (Node, error) {
	tok, err := p.nextOfType(token.TypeQuotedString)
	if err != nil {
		return nil, err
	}

	s := p.data[tok.Position.Start:tok.Position.End]

	return &ValueNode{
		Position: tok.Position,
		Value:    s[1 : len(s)-1],
	}, nil
}

func (p *Parser[T]) parseSingle() (Node, error) {
	tok, err := p.peek()
	if err != nil {
		return nil, err
	}

	switch tok.Type { //nolint:exhaustive
	case token.TypeDollarOpeningParenthesis:
		return p.parseVariable()
	case token.TypeNegation:
		return p.parseNegation()
	case token.TypeOpeningParenthesis:
		return p.parseSubExpr()
	case token.TypeSimpleString:
		return p.parseStringAsScalar()
	case token.TypeQuotedString:
		return p.parseQuotedString()
	default:
		return p.unexpected(tok)
	}
}

func (p *Parser[T]) parseSingleOrComparisons() (Node, error) {
	node, err := p.parseSingle()
	if err != nil {
		return nil, err
	}

	switch p.peekType() { //nolint:exhaustive
	case token.TypeEquals:
		return p.parseOperator(node)
	case token.TypeGreaterThan:
		return p.parseOperator(node)
	case token.TypeGreaterThanEquals:
		return p.parseOperator(node)
	case token.TypeLessThan:
		return p.parseOperator(node)
	case token.TypeLessThanEqual:
		return p.parseOperator(node)
	case token.TypeNotEquals:
		return p.parseOperator(node)
	default:
		return node, nil
	}
}

func (p *Parser[T]) parseString() (Node, error) {
	s, tok, err := p.readString()
	if err != nil {
		return nil, err
	}

	return &ValueNode{Position: tok.Position, Value: s}, nil
}

var (
	emptyStringVal any = ""
	falseVal       any = false
	trueVal        any = true
)

func (p *Parser[T]) parseStringAsScalar() (Node, error) {
	s, tok, err := p.readSimpleString()
	if err != nil {
		return nil, err
	}

	switch s {
	case "":
		return &ValueNode{Position: tok.Position, Value: emptyStringVal}, nil
	case "false":
		return &ValueNode{Position: tok.Position, Value: falseVal}, nil
	case "null":
		return &ValueNode{Position: tok.Position, Value: nil}, nil
	case "true":
		return &ValueNode{Position: tok.Position, Value: trueVal}, nil
	default:
	}

	if strings.IndexByte(s, '.') == -1 {
		i, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			return &ValueNode{Position: tok.Position, Value: int(i)}, nil
		}
	} else {
		f, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return &ValueNode{Position: tok.Position, Value: f}, nil
		}
	}

	return p.unexpected(tok)
}

func (p *Parser[T]) parseSubExpr() (Node, error) {
	if _, err := p.nextOfType(token.TypeOpeningParenthesis); err != nil {
		return nil, err
	}

	node, err := p.parse(true)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &Error{Offset: p.sc.Offset(), Underlying: io.ErrUnexpectedEOF}
		}
		return nil, err
	}

	if _, err := p.nextOfType(token.TypeClosingParenthesis); err != nil {
		return nil, err
	}

	return node, nil
}

func (p *Parser[T]) parseVariable() (Node, error) {
	start, err := p.nextOfType(token.TypeDollarOpeningParenthesis)
	if err != nil {
		return nil, err
	}

	if err := p.checkNoWhitespace(); err != nil {
		return nil, err
	}

	name, _, err := p.readSimpleString()
	if err != nil {
		return nil, err
	}

	v := &VariableNode{Name: name}

	if err := p.checkNoWhitespace(); err != nil {
		return nil, err
	}

	if p.peekType() == token.TypeOpeningBracket {
		_, _ = p.nextOfType(token.TypeOpeningBracket)

		if err := p.checkNoWhitespace(); err != nil {
			return nil, err
		}

		var key string

		// Handle empty key
		if p.peekType() != token.TypeClosingBracket {
			if key, _, err = p.readString(); err != nil {
				return nil, err
			}
		}

		v.Key = &key

		if err := p.checkNoWhitespace(); err != nil {
			return nil, err
		}

		if _, err := p.nextOfType(token.TypeClosingBracket); err != nil {
			return nil, err
		}
	}

	if p.peekType() == token.TypeOr {
		tok, _ := p.nextOfType(token.TypeOr)

		if err := p.checkNoWhitespace(); err != nil {
			return nil, err
		}

		switch p.peekType() { //nolint:exhaustive
		case token.TypeClosingParenthesis:
			v.Default = &ValueNode{
				Position: token.Position{
					Start: tok.Position.Start + 1,
					End:   tok.Position.Start + 1,
				},
				Value: "",
			}
		case token.TypeDollarOpeningParenthesis:
			v.Default, err = p.parseVariable()
		default:
			v.Default, err = p.parseString()
		}

		if err != nil {
			return nil, err
		}
	}

	if err := p.checkNoWhitespace(); err != nil {
		return nil, err
	}

	end, err := p.nextOfType(token.TypeClosingParenthesis)
	if err != nil {
		return nil, err
	}

	v.Position.Start = start.Position.Start
	v.Position.End = end.Position.End

	return v, nil
}

func (p *Parser[T]) unexpected(tok token.Token) (Node, error) {
	return nil, &UnexpectedTokenError{Token: tok}
}
