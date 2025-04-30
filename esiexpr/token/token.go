package token

import (
	"strconv"
)

// Position specifies a start and end position in a specific input.
type Position struct {
	// Start is the inclusive start index.
	Start int

	// End is the exclusive end index.
	End int
}

// String implements the [fmt.Stringer] interface.
func (p Position) String() string {
	return strconv.Itoa(p.Start) + ":" + strconv.Itoa(p.End)
}

// Token is a single token extracted from a given input string or []byte.
type Token struct {
	// Position contains the start and end indices of the token.
	//
	// For tokens of type [TypeSimpleString] or [TypeQuotedString] this can be used to get the string from the input.
	Position Position

	// Type describes the type of the token.
	Type Type
}

// Pos returns the position of the token.
func (t Token) Pos() Position {
	return t.Position
}

// Type is an enum of all supported token types.
type Type uint8

const (
	// TypeInvalid is the zero value for TokenType and is not a valid type.
	TypeInvalid Type = iota

	// TypeSimpleString represents a simple, unquoted string.
	TypeSimpleString

	// TypeQuotedString represents a string quoted using single quotes.
	TypeQuotedString

	// TypeDollarOpeningParenthesis represents the combination of $ and ( at the start of a variable.
	TypeDollarOpeningParenthesis

	// TypeOpeningParenthesis represents as single (.
	TypeOpeningParenthesis

	// TypeClosingParenthesis represents as single ).
	TypeClosingParenthesis

	// TypeOpeningBracket represents as single {.
	TypeOpeningBracket

	// TypeClosingBracket represents as single }.
	TypeClosingBracket

	// TypeOr represents as single |.
	TypeOr

	// TypeAnd represents as single &.
	TypeAnd

	// TypeNegation represents as single !.
	TypeNegation

	// TypeEquals represents the == operator.
	TypeEquals

	// TypeNotEquals represents the != operator.
	TypeNotEquals

	// TypeGreaterThan represents the > operator.
	TypeGreaterThan

	// TypeGreaterThanEquals represents the >= operator.
	TypeGreaterThanEquals

	// TypeLessThan represents the < operator.
	TypeLessThan

	// TypeLessThanEqual represents the <= operator.
	TypeLessThanEqual
)

// String implements the [fmt.Stringer] interface.
func (t Type) String() string {
	switch t {
	case TypeInvalid:
		return "invalid"
	case TypeQuotedString:
		return "quoted string"
	case TypeSimpleString:
		return "simple string"
	case TypeDollarOpeningParenthesis:
		return "$("
	case TypeOpeningParenthesis:
		return "("
	case TypeClosingParenthesis:
		return ")"
	case TypeOpeningBracket:
		return "{"
	case TypeClosingBracket:
		return "}"
	case TypeOr:
		return "|"
	case TypeAnd:
		return "&"
	case TypeNegation:
		return "!"
	case TypeEquals:
		return "=="
	case TypeNotEquals:
		return "!="
	case TypeGreaterThan:
		return ">"
	case TypeGreaterThanEquals:
		return ">="
	case TypeLessThan:
		return "<"
	case TypeLessThanEqual:
		return "<="
	default:
		panic("invalid token type")
	}
}
