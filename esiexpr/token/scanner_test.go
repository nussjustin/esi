package token_test

import (
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/esi/esiexpr/internal/text"
	"github.com/nussjustin/esi/esiexpr/token"
)

func pos(start, end int) token.Position {
	return token.Position{Start: start, End: end}
}

func TestScanner(t *testing.T) {
	testCases := []struct {
		Name  string
		Input string
		Token []token.Token
		Error error
	}{
		{
			Name:  "empty",
			Input: ``,
		},
		{
			Name:  "opening parenthesis",
			Input: `(`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeOpeningParenthesis},
			},
		},
		{
			Name:  "closing parenthesis",
			Input: `)`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeClosingParenthesis},
			},
		},
		{
			Name:  "opening bracket",
			Input: `{`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeOpeningBracket},
			},
		},
		{
			Name:  "closing bracket",
			Input: `}`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeClosingBracket},
			},
		},
		{
			Name:  "dollar",
			Input: `$`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "dollar followed by opening parenthesis",
			Input: `$(`,
			Token: []token.Token{
				{Position: pos(0, 2), Type: token.TypeDollarOpeningParenthesis},
			},
		},
		{
			Name:  "and",
			Input: `&`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeAnd},
			},
		},
		{
			Name:  "or",
			Input: `|`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeOr},
			},
		},
		{
			Name:  "negation",
			Input: `!`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeNegation},
			},
		},
		{
			Name:  "equals",
			Input: `==`,
			Token: []token.Token{
				{Position: pos(0, 2), Type: token.TypeEquals},
			},
		},
		{
			Name:  "broken equals",
			Input: `=A`,
			Error: &text.UnexpectedCharacterError{At: 1, Got: 'A', Expected: '='},
		},
		{
			Name:  "incomplete equals",
			Input: `=`,
			Error: &text.UnexpectedEndOfInput{At: 1, Expected: '='},
		},
		{
			Name:  "not equals",
			Input: `!=`,
			Token: []token.Token{
				{Position: pos(0, 2), Type: token.TypeNotEquals},
			},
		},
		{
			Name:  "greater than",
			Input: `>`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeGreaterThan},
			},
		},
		{
			Name:  "greater than equals",
			Input: `>=`,
			Token: []token.Token{
				{Position: pos(0, 2), Type: token.TypeGreaterThanEquals},
			},
		},
		{
			Name:  "less than",
			Input: `<`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeLessThan},
			},
		},
		{
			Name:  "less than equals",
			Input: `<=`,
			Token: []token.Token{
				{Position: pos(0, 2), Type: token.TypeLessThanEqual},
			},
		},
		{
			Name:  "integer",
			Input: `1234`,
			Token: []token.Token{
				{Position: pos(0, 4), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "negative integer",
			Input: `-1234`,
			Token: []token.Token{
				{Position: pos(0, 5), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "float",
			Input: `12.34`,
			Token: []token.Token{
				{Position: pos(0, 5), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "negative float",
			Input: `-12.34`,
			Token: []token.Token{
				{Position: pos(0, 6), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "string",
			Input: `literal`,
			Token: []token.Token{
				{Position: pos(0, 7), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "string starting with dollar",
			Input: `$literal`,
			Token: []token.Token{
				{Position: pos(0, 8), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "string containing dollar",
			Input: `awe$ome`,
			Token: []token.Token{
				{Position: pos(0, 7), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "string containing variable",
			Input: `awe$(VAR)ome`,
			Token: []token.Token{
				{Position: pos(0, 3), Type: token.TypeSimpleString},
				{Position: pos(3, 5), Type: token.TypeDollarOpeningParenthesis},
				{Position: pos(5, 8), Type: token.TypeSimpleString},
				{Position: pos(8, 9), Type: token.TypeClosingParenthesis},
				{Position: pos(9, 12), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "string ending with dollar",
			Input: `a lot of $`,
			Token: []token.Token{
				{Position: pos(0, 1), Type: token.TypeSimpleString},
				{Position: pos(2, 5), Type: token.TypeSimpleString},
				{Position: pos(6, 8), Type: token.TypeSimpleString},
				{Position: pos(9, 10), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "number like string",
			Input: `12.34.56`,
			Token: []token.Token{
				{Position: pos(0, 8), Type: token.TypeSimpleString},
			},
		},
		{
			Name:  "quoted string",
			Input: `'quoted string'`,
			Token: []token.Token{
				{Position: pos(0, 15), Type: token.TypeQuotedString},
			},
		},
		{
			Name:  "variable",
			Input: `$(VARIABLE{key}|default)`,
			Token: []token.Token{
				{
					Position: pos(0, 2),
					Type:     token.TypeDollarOpeningParenthesis,
				},
				{
					Position: pos(2, 10),
					Type:     token.TypeSimpleString,
				},
				{
					Position: pos(10, 11),
					Type:     token.TypeOpeningBracket,
				},
				{
					Position: pos(11, 14),
					Type:     token.TypeSimpleString,
				},
				{
					Position: pos(14, 15),
					Type:     token.TypeClosingBracket,
				},
				{
					Position: pos(15, 16),
					Type:     token.TypeOr,
				},
				{
					Position: pos(16, 23),
					Type:     token.TypeSimpleString,
				},
				{
					Position: pos(23, 24),
					Type:     token.TypeClosingParenthesis,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			s := token.NewScanner[string](testCase.Input)

			var got []token.Token
			var gotErr error

			for {
				tok, err := s.Next()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					gotErr = err
					break
				}
				got = append(got, tok)
			}

			if !errors.Is(testCase.Error, gotErr) {
				t.Errorf("got error %v, want %v", gotErr, testCase.Error)
			}

			if diff := cmp.Diff(testCase.Token, got); diff != "" {
				t.Errorf("tokens mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
