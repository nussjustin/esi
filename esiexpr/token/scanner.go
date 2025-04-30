package token

import (
	"io"

	"github.com/nussjustin/esi/esiexpr/internal/text"
)

// Scanner implements scanning of ESI expressions into tokens.
type Scanner[T []byte | string] struct {
	in  text.Scanner[T]
	err error
}

// NewScanner is a shorthand for creating a new *Scanner and calling [Scanner.Reset] on it.
func NewScanner[T []byte | string](data T) *Scanner[T] {
	s := &Scanner[T]{}
	s.Reset(data)
	return s
}

// Next returns the next token from the input, or an error.
//
// When reaching the end [io.EOF] is returned.
func (s *Scanner[T]) Next() (Token, error) {
	s.in.SkipSpaces()

	c, ok := s.in.Peek()
	if !ok {
		s.err = io.EOF
		return Token{Type: TypeInvalid}, io.EOF
	}

	start := s.in.Offset()

	var tok Token
	var err error

	switch c {
	case '$':
		tok, err = s.scanDollar()
	case '(':
		tok, err = s.scan('(', TypeOpeningParenthesis)
	case ')':
		tok, err = s.scan(')', TypeClosingParenthesis)
	case '{':
		tok, err = s.scan('{', TypeOpeningBracket)
	case '}':
		tok, err = s.scan('}', TypeClosingBracket)
	case '|':
		tok, err = s.scan('|', TypeOr)
	case '&':
		tok, err = s.scan('&', TypeAnd)
	case '=':
		tok, err = s.scanEquals()
	case '>':
		tok, err = s.scanGreaterThan()
	case '<':
		tok, err = s.scanLessThan()
	case '!':
		tok, err = s.scanNotEqualsOrUnaryNegation()
	case '\'':
		tok, err = s.scanQuotedString()
	default:
		tok, err = s.scanString()
	}

	if err != nil {
		s.err = err
		return Token{Type: TypeInvalid}, err
	}

	tok.Position.Start = start
	tok.Position.End = s.in.Offset()
	return tok, nil
}

// Offset returns the current offset in the input.
func (s *Scanner[T]) Offset() int {
	return s.in.Offset()
}

// Reset resets the internal state of the Scanner and changes it to read from data.
func (s *Scanner[T]) Reset(data T) {
	s.in.Reset(data)
	s.err = nil
}

func (s *Scanner[T]) scan(c byte, typ Type) (Token, error) {
	offset := s.in.Offset()

	_ = s.in.Consume(c)

	return Token{Position: Position{Start: offset, End: offset + 1}, Type: typ}, nil
}

func (s *Scanner[T]) scanDollar() (Token, error) {
	offset := s.in.Offset()

	_ = s.in.Consume('$')

	if s.in.Consume('(') {
		return Token{
			Position: Position{Start: offset, End: offset + 1},
			Type:     TypeDollarOpeningParenthesis,
		}, nil
	}

	return s.scanString()
}

func (s *Scanner[T]) scanEquals() (Token, error) {
	_ = s.in.Consume('=')

	if err := s.in.ConsumeOrError('='); err != nil {
		return Token{Type: TypeInvalid}, err
	}

	return Token{Type: TypeEquals}, nil
}

func (s *Scanner[T]) scanGreaterThan() (Token, error) {
	_ = s.in.Consume('>')

	if s.in.Consume('=') {
		return Token{Type: TypeGreaterThanEquals}, nil
	}

	return Token{Type: TypeGreaterThan}, nil
}

func (s *Scanner[T]) scanLessThan() (Token, error) {
	_ = s.in.Consume('<')

	if s.in.Consume('=') {
		return Token{Type: TypeLessThanEqual}, nil
	}

	return Token{Type: TypeLessThan}, nil
}

func (s *Scanner[T]) scanNotEqualsOrUnaryNegation() (Token, error) {
	_ = s.in.Consume('!')

	if s.in.Consume('=') {
		return Token{Type: TypeNotEquals}, nil
	}

	return Token{Type: TypeNegation}, nil
}

func (s *Scanner[T]) scanQuotedString() (Token, error) {
	_ = s.in.Consume('\'')

	s.in.SkipWhile(func(c byte) bool {
		return c != '\''
	})

	if err := s.in.ConsumeOrError('\''); err != nil {
		return Token{Type: TypeInvalid}, err
	}

	return Token{Type: TypeQuotedString}, nil
}

func (s *Scanner[T]) scanString() (Token, error) {
	for {
		s.in.SkipWhile(func(c byte) bool {
			switch {
			case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_', c == '.', c == '-':
				return true
			default:
				return false
			}
		})

		// If the next character is a $, check if it is followed by a ( in which case it is means the start of a
		// variable. Otherwise treat it as part of the string.
		if !s.in.Consume('$') {
			break
		}

		if !s.in.Consume('(') {
			continue
		}

		s.in.Unread()
		s.in.Unread()
		break
	}

	return Token{Type: TypeSimpleString}, nil
}
