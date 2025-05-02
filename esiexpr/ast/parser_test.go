package ast_test

import (
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/esi/esiexpr/ast"
	"github.com/nussjustin/esi/esiexpr/internal/text"
	"github.com/nussjustin/esi/esiexpr/token"
)

func pos(start, end int) token.Position {
	return token.Position{Start: start, End: end}
}

func ptr[T any](v T) *T {
	return &v
}

func tok(start, end int, typ token.Type) token.Token {
	return token.Token{Position: pos(start, end), Type: typ}
}

func unexpected(start, end int, typ token.Type) error {
	return &ast.UnexpectedTokenError{Token: tok(start, end, typ)}
}

func TestParse(t *testing.T) {
	testCases := []struct {
		Name     string
		Input    string
		Expected ast.Node
		Error    error
	}{
		{
			Name:     "bool false",
			Input:    `false`,
			Expected: &ast.ValueNode{Position: pos(0, 5), Value: false},
		},
		{
			Name:  "broken bool false",
			Input: `fals`,
			Error: unexpected(0, 4, token.TypeSimpleString),
		},
		{
			Name:  "invalid bool false",
			Input: `falSe`,
			Error: unexpected(0, 5, token.TypeSimpleString),
		},
		{
			Name:  "uppercase bool false",
			Input: `FALSE`,
			Error: unexpected(0, 5, token.TypeSimpleString),
		},
		{
			Name:     "bool true",
			Input:    `true`,
			Expected: &ast.ValueNode{Position: pos(0, 4), Value: true},
		},
		{
			Name:  "broken bool true",
			Input: `tru`,
			Error: unexpected(0, 3, token.TypeSimpleString),
		},
		{
			Name:  "invalid bool true",
			Input: `trUe`,
			Error: unexpected(0, 4, token.TypeSimpleString),
		},
		{
			Name:  "uppercase bool true",
			Input: `TRUE`,
			Error: unexpected(0, 4, token.TypeSimpleString),
		},
		{
			Name:     "float",
			Input:    `12.34`,
			Expected: &ast.ValueNode{Position: pos(0, 5), Value: 12.34},
		},
		{
			Name:     "float ending in decimal point",
			Input:    `12.`,
			Expected: &ast.ValueNode{Position: pos(0, 3), Value: 12.0},
		},
		{
			Name:  "float with non-digit",
			Input: `12.3a4`,
			Error: unexpected(0, 6, token.TypeSimpleString),
		},
		{
			Name:  "float with two decimal points",
			Input: `12.3.4`,
			Error: unexpected(0, 6, token.TypeSimpleString),
		},
		{
			Name:     "negative float",
			Input:    `-12.34`,
			Expected: &ast.ValueNode{Position: pos(0, 6), Value: -12.34},
		},
		{
			Name:     "int",
			Input:    `1234`,
			Expected: &ast.ValueNode{Position: pos(0, 4), Value: 1234},
		},
		{
			Name:  "int with non-digit",
			Input: `12a34`,
			Error: unexpected(0, 5, token.TypeSimpleString),
		},
		{
			Name:     "negative int",
			Input:    `-1234`,
			Expected: &ast.ValueNode{Position: pos(0, 5), Value: -1234},
		},
		{
			Name:     "null",
			Input:    `null`,
			Expected: &ast.ValueNode{Position: pos(0, 4), Value: nil},
		},
		{
			Name:  "broken null",
			Input: `nul`,
			Error: unexpected(0, 3, token.TypeSimpleString),
		},
		{
			Name:  "invalid null",
			Input: `nuLl`,
			Error: unexpected(0, 4, token.TypeSimpleString),
		},
		{
			Name:  "uppercase null",
			Input: `NULL`,
			Error: unexpected(0, 4, token.TypeSimpleString),
		},
		{
			Name:     "empty",
			Input:    `''`,
			Expected: &ast.ValueNode{Position: pos(0, 2), Value: ""},
		},
		{
			Name:     "string",
			Input:    `'hello world'`,
			Expected: &ast.ValueNode{Position: pos(0, 13), Value: "hello world"},
		},
		{
			Name:  "string with quote",
			Input: `'hello \'world\''`,
			Error: unexpected(9, 14, token.TypeSimpleString),
		},
		{
			Name:  "string without closing quote",
			Input: `'hello world`,
			Error: &ast.Error{
				Offset: 12,
				Underlying: &text.UnexpectedEndOfInput{
					At:       12,
					Expected: '\'',
				},
			},
		},
		{
			Name:     "sub expression",
			Input:    `(true)`,
			Expected: &ast.ValueNode{Position: pos(1, 5), Value: true},
		},
		{
			Name:  "empty sub expression",
			Input: `()`,
			Error: unexpected(1, 2, token.TypeClosingParenthesis),
		},
		{
			Name:  "non-closed sub expression",
			Input: `(true`,
			Error: &ast.Error{Offset: 5, Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "lone closing parenthesis",
			Input: `(true))`,
			Error: unexpected(6, 7, token.TypeClosingParenthesis),
		},
		{
			Name:     "variable",
			Input:    `$(TEST)`,
			Expected: &ast.VariableNode{Position: pos(0, 7), Name: "TEST"},
		},
		{
			Name:     "variable with underscore",
			Input:    `$(TEST_IT)`,
			Expected: &ast.VariableNode{Position: pos(0, 10), Name: "TEST_IT"},
		},
		{
			Name:     "variable with empty key",
			Input:    `$(TEST{})`,
			Expected: &ast.VariableNode{Position: pos(0, 9), Name: "TEST", Key: ptr("")},
		},
		{
			Name:     "variable with key",
			Input:    `$(TEST{it})`,
			Expected: &ast.VariableNode{Position: pos(0, 11), Name: "TEST", Key: ptr("it")},
		},
		{
			Name:  "variable with space before key",
			Input: `$(TEST{ it})`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(7, 8)},
		},
		{
			Name:  "variable with space in key",
			Input: `$(TEST{i t})`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(8, 9)},
		},
		{
			Name:  "variable with space after key",
			Input: `$(TEST{it })`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(9, 10)},
		},
		{
			Name:  "variable with default",
			Input: `$(TEST|default)`,
			Expected: &ast.VariableNode{
				Position: pos(0, 15),
				Name:     "TEST",
				Default: &ast.ValueNode{
					Position: pos(7, 14),
					Value:    "default",
				},
			},
		},
		{
			Name:  "variable with empty default",
			Input: `$(TEST|)`,
			Expected: &ast.VariableNode{
				Position: pos(0, 8),
				Name:     "TEST",
				Default: &ast.ValueNode{
					Position: pos(7, 7),
					Value:    "",
				},
			},
		},
		{
			Name:  "variable with quoted default",
			Input: `$(TEST|'quoted default')`,
			Expected: &ast.VariableNode{
				Position: pos(0, 24),
				Name:     "TEST",
				Default: &ast.ValueNode{
					Position: pos(7, 23),
					Value:    "quoted default",
				},
			},
		},
		{
			Name:  "variable with key and default",
			Input: `$(TEST{it}|default)`,
			Expected: &ast.VariableNode{
				Position: pos(0, 19),
				Name:     "TEST",
				Key:      ptr("it"),
				Default: &ast.ValueNode{
					Position: pos(11, 18),
					Value:    "default",
				},
			},
		},
		{
			Name:  "variable with key and empty default",
			Input: `$(TEST{it}|)`,
			Expected: &ast.VariableNode{
				Position: pos(0, 12),
				Name:     "TEST",
				Key:      ptr("it"),
				Default: &ast.ValueNode{
					Position: pos(11, 11),
					Value:    "",
				},
			},
		},
		{
			Name:  "variable with key and quoted default",
			Input: `$(TEST{it}|'quoted default')`,
			Expected: &ast.VariableNode{
				Position: pos(0, 28),
				Name:     "TEST",
				Key:      ptr("it"),
				Default: &ast.ValueNode{
					Position: pos(11, 27),
					Value:    "quoted default",
				},
			},
		},
		{
			Name:  "variable with variable default",
			Input: `$(TEST|$(DEFAULT{test}|none))`,
			Expected: &ast.VariableNode{
				Position: pos(0, 29),
				Name:     "TEST",
				Default: &ast.VariableNode{
					Position: pos(7, 28),
					Name:     "DEFAULT",
					Key:      ptr("test"),
					Default: &ast.ValueNode{
						Position: pos(23, 27),
						Value:    "none",
					},
				},
			},
		},
		{
			Name:  "variable with invalid character after (",
			Input: `$(!TEST)`,
			Error: unexpected(2, 3, token.TypeNegation),
		},
		{
			Name:  "variable with space after (",
			Input: `$( TEST)`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(2, 3)},
		},
		{
			Name:  "variable with invalid character before )",
			Input: `$(TEST!)`,
			Error: unexpected(6, 7, token.TypeNegation),
		},
		{
			Name:  "variable with space before )",
			Input: `$(TEST )`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(6, 7)},
		},
		{
			Name:  "variable with invalid identifier",
			Input: `$(WITH.DOT)`,
			Expected: &ast.VariableNode{
				Position: pos(0, 11),
				Name:     "WITH.DOT",
			},
		},
		{
			Name:  "variable with space before default delimiter",
			Input: `$(TEST |default)`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(6, 7)},
		},
		{
			Name:  "variable with space after default delimiter",
			Input: `$(TEST| default)`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(7, 8)},
		},
		{
			Name:  "variable with default string false",
			Input: `$(TEST|false)`,
			Expected: &ast.VariableNode{
				Position: pos(0, 13),
				Name:     "TEST",
				Default: &ast.ValueNode{
					Position: pos(7, 12),
					Value:    "false",
				},
			},
		},
		{
			Name:  "variable with default string null",
			Input: `$(TEST|null)`,
			Expected: &ast.VariableNode{
				Position: pos(0, 12),
				Name:     "TEST",
				Default: &ast.ValueNode{
					Position: pos(7, 11),
					Value:    "null",
				},
			},
		},
		{
			Name:  "variable with default string true",
			Input: `$(TEST|true)`,
			Expected: &ast.VariableNode{
				Position: pos(0, 12),
				Name:     "TEST",
				Default: &ast.ValueNode{
					Position: pos(7, 11),
					Value:    "true",
				},
			},
		},
		{
			Name:  "negated bool false",
			Input: `!false`,
			Expected: &ast.NegateNode{
				Position: pos(0, 6),
				Expr: &ast.ValueNode{
					Position: pos(1, 6),
					Value:    false,
				},
			},
		},
		{
			Name:  "negated bool true",
			Input: `!true`,
			Expected: &ast.NegateNode{
				Position: pos(0, 5),
				Expr: &ast.ValueNode{
					Position: pos(1, 5),
					Value:    true,
				},
			},
		},
		{
			Name:  "negated float",
			Input: `!12.34`,
			Expected: &ast.NegateNode{
				Position: pos(0, 6),
				Expr: &ast.ValueNode{
					Position: pos(1, 6),
					Value:    12.34,
				},
			},
		},
		{
			Name:  "negated int",
			Input: `!1234`,
			Expected: &ast.NegateNode{
				Position: pos(0, 5),
				Expr: &ast.ValueNode{
					Position: pos(1, 5),
					Value:    1234,
				},
			},
		},
		{
			Name:  "negated null",
			Input: `!null`,
			Expected: &ast.NegateNode{
				Position: pos(0, 5),
				Expr: &ast.ValueNode{
					Position: pos(1, 5),
					Value:    nil,
				},
			},
		},
		{
			Name:  "negated string",
			Input: `!'test'`,
			Expected: &ast.NegateNode{
				Position: pos(0, 7),
				Expr: &ast.ValueNode{
					Position: pos(1, 7),
					Value:    "test",
				},
			},
		},
		{
			Name:  "negated sub expression",
			Input: `!(true & false)`,
			Expected: &ast.NegateNode{
				Position: pos(0, 15),
				Expr: &ast.AndNode{
					Position: pos(2, 14),
					Left: &ast.ValueNode{
						Position: pos(2, 6),
						Value:    true,
					},
					Right: &ast.ValueNode{
						Position: pos(9, 14),
						Value:    false,
					},
				},
			},
		},
		{
			Name:  "negated variable",
			Input: `!$(TEST)`,
			Expected: &ast.NegateNode{
				Position: pos(0, 8),
				Expr: &ast.VariableNode{
					Position: pos(1, 8),
					Name:     "TEST",
				},
			},
		},

		{
			Name:  "equals",
			Input: `12 == 34`,
			Expected: &ast.ComparisonNode{
				Position: pos(0, 8),
				Operator: ast.ComparisonOperatorEquals,
				Left:     &ast.ValueNode{Position: pos(0, 2), Value: 12},
				Right:    &ast.ValueNode{Position: pos(6, 8), Value: 34},
			},
		},
		{
			Name:  "equals without right-side",
			Input: `12 == `,
			Error: &ast.MissingOperandError{Offset: 5},
		},
		{
			Name:  "broken equals",
			Input: `12 = 34`,
			Error: &text.UnexpectedCharacterError{At: 4, Got: ' ', Expected: '='},
		},
		{
			Name:  "not equals",
			Input: `12 != 34`,
			Expected: &ast.ComparisonNode{
				Position: pos(0, 8),
				Operator: ast.ComparisonOperatorNotEquals,
				Left:     &ast.ValueNode{Position: pos(0, 2), Value: 12},
				Right:    &ast.ValueNode{Position: pos(6, 8), Value: 34},
			},
		},
		{
			Name:  "not equals without right-side",
			Input: `12 != `,
			Error: &ast.MissingOperandError{Offset: 5},
		},
		{
			Name:  "broken not equals",
			Input: `12 ! 34`,
			Error: unexpected(3, 4, token.TypeNegation),
		},
		{
			Name:  "greater than",
			Input: `12 > 34`,
			Expected: &ast.ComparisonNode{
				Position: pos(0, 7),
				Operator: ast.ComparisonOperatorGreaterThan,
				Left:     &ast.ValueNode{Position: pos(0, 2), Value: 12},
				Right:    &ast.ValueNode{Position: pos(5, 7), Value: 34},
			},
		},
		{
			Name:  "greater than without right-side",
			Input: `12 > `,
			Error: &ast.MissingOperandError{Offset: 4},
		},
		{
			Name:  "greater than equals",
			Input: `12 >= 34`,
			Expected: &ast.ComparisonNode{
				Position: pos(0, 8),
				Operator: ast.ComparisonOperatorGreaterThanEquals,
				Left:     &ast.ValueNode{Position: pos(0, 2), Value: 12},
				Right:    &ast.ValueNode{Position: pos(6, 8), Value: 34},
			},
		},
		{
			Name:  "greater than equals without right-side",
			Input: `12 >= `,
			Error: &ast.MissingOperandError{Offset: 5},
		},
		{
			Name:  "less than",
			Input: `12 < 34`,
			Expected: &ast.ComparisonNode{
				Position: pos(0, 7),
				Operator: ast.ComparisonOperatorLessThan,
				Left:     &ast.ValueNode{Position: pos(0, 2), Value: 12},
				Right:    &ast.ValueNode{Position: pos(5, 7), Value: 34},
			},
		},
		{
			Name:  "less than without right-side",
			Input: `12 < `,
			Error: &ast.MissingOperandError{Offset: 4},
		},
		{
			Name:  "less than equals",
			Input: `12 <= 34`,
			Expected: &ast.ComparisonNode{
				Position: pos(0, 8),
				Operator: ast.ComparisonOperatorLessThanEquals,
				Left:     &ast.ValueNode{Position: pos(0, 2), Value: 12},
				Right:    &ast.ValueNode{Position: pos(6, 8), Value: 34},
			},
		},
		{
			Name:  "less than equals without right-side",
			Input: `12 <= `,
			Error: &ast.MissingOperandError{Offset: 5},
		},
		{
			Name:  "and",
			Input: `true & false`,
			Expected: &ast.AndNode{
				Position: pos(0, 12),
				Left: &ast.ValueNode{
					Position: pos(0, 4),
					Value:    true,
				},
				Right: &ast.ValueNode{
					Position: pos(7, 12),
					Value:    false,
				},
			},
		},
		{
			Name:  "and without right-side",
			Input: `true & `,
			Error: &ast.MissingOperandError{Offset: 6},
		},
		{
			Name:  "multiple and",
			Input: `true & false & null`,
			Expected: &ast.AndNode{
				Position: pos(0, 19),
				Left: &ast.AndNode{
					Position: pos(0, 12),
					Left: &ast.ValueNode{
						Position: pos(0, 4),
						Value:    true,
					},
					Right: &ast.ValueNode{
						Position: pos(7, 12),
						Value:    false,
					},
				},
				Right: &ast.ValueNode{
					Position: pos(15, 19),
					Value:    nil,
				},
			},
		},
		{
			Name:  "or",
			Input: `false | true`,
			Expected: &ast.OrNode{
				Position: pos(0, 12),
				Left: &ast.ValueNode{
					Position: pos(0, 5),
					Value:    false,
				},
				Right: &ast.ValueNode{
					Position: pos(8, 12),
					Value:    true,
				},
			},
		},
		{
			Name:  "or without right-side",
			Input: `false | `,
			Error: &ast.MissingOperandError{Offset: 7},
		},
		{
			Name:  "multiple or",
			Input: `false | true | null`,
			Expected: &ast.OrNode{
				Position: pos(0, 19),
				Left: &ast.OrNode{
					Position: pos(0, 12),
					Left: &ast.ValueNode{
						Position: pos(0, 5),
						Value:    false,
					},
					Right: &ast.ValueNode{
						Position: pos(8, 12),
						Value:    true,
					},
				},
				Right: &ast.ValueNode{
					Position: pos(15, 19),
					Value:    nil,
				},
			},
		},
		{
			Name:  "mixed and-or",
			Input: `false & true | null`,
			Expected: &ast.OrNode{
				Position: pos(0, 19),
				Left: &ast.AndNode{
					Position: pos(0, 12),
					Left: &ast.ValueNode{
						Position: pos(0, 5),
						Value:    false,
					},
					Right: &ast.ValueNode{
						Position: pos(8, 12),
						Value:    true,
					},
				},
				Right: &ast.ValueNode{
					Position: pos(15, 19),
					Value:    nil,
				},
			},
		},

		{
			Name:  "variable equals value",
			Input: "$(TEST) == 12.34",
			Expected: &ast.ComparisonNode{
				Position: pos(0, 16),
				Operator: ast.ComparisonOperatorEquals,
				Left: &ast.VariableNode{
					Position: pos(0, 7),
					Name:     "TEST",
				},
				Right: &ast.ValueNode{
					Position: pos(11, 16),
					Value:    12.34,
				},
			},
		},
		{
			Name:  "value equals variable",
			Input: "1234 == $(TEST{it})",
			Expected: &ast.ComparisonNode{
				Position: pos(0, 19),
				Operator: ast.ComparisonOperatorEquals,
				Left: &ast.ValueNode{
					Position: pos(0, 4),
					Value:    1234,
				},
				Right: &ast.VariableNode{
					Position: pos(8, 19),
					Name:     "TEST",
					Key:      ptr("it"),
				},
			},
		},
		{
			Name:  "variable equals variable",
			Input: "$(VAR1) == $(VAR2|vars2)",
			Expected: &ast.ComparisonNode{
				Position: pos(0, 24),
				Operator: ast.ComparisonOperatorEquals,
				Left: &ast.VariableNode{
					Position: pos(0, 7),
					Name:     "VAR1",
				},
				Right: &ast.VariableNode{
					Position: pos(11, 24),
					Name:     "VAR2",
					Default: &ast.ValueNode{
						Position: pos(18, 23),
						Value:    "vars2",
					},
				},
			},
		},

		{
			Name:  "extra data",
			Input: `true data`,
			Error: unexpected(5, 9, token.TypeSimpleString),
		},
		{
			Name:  "extra data bool",
			Input: `true false`,
			Error: unexpected(5, 10, token.TypeSimpleString),
		},

		{
			Name:  "left to right",
			Input: `1 == 2 | 3 & 4`,
			Expected: &ast.AndNode{
				Position: pos(0, 14),
				Left: &ast.OrNode{
					Position: pos(0, 10),
					Left: &ast.ComparisonNode{
						Position: pos(0, 6),
						Operator: "==",
						Left: &ast.ValueNode{
							Position: pos(0, 1),
							Value:    1,
						},
						Right: &ast.ValueNode{
							Position: pos(5, 6),
							Value:    2,
						},
					},
					Right: &ast.ValueNode{Position: pos(9, 10), Value: 3},
				},
				Right: &ast.ValueNode{Position: pos(13, 14), Value: 4},
			},
		},

		{
			Name: "complex expression",
			Input: `$(VAR1{key1}|$(DEFAULT{key2}|none)) < -12.34 & ((
				false == $(VAR2)) | 	true & !(null == null) ) != 'quoted'`,
			Expected: &ast.AndNode{
				Position: pos(0, 111),
				Left: &ast.ComparisonNode{
					Position: pos(0, 44),
					Operator: "<",
					Left: &ast.VariableNode{
						Position: pos(0, 35),
						Name:     "VAR1",
						Key:      ptr("key1"),
						Default: &ast.VariableNode{
							Position: pos(13, 34),
							Name:     "DEFAULT",
							Key:      ptr("key2"),
							Default: &ast.ValueNode{
								Position: pos(29, 33),
								Value:    "none",
							},
						},
					},
					Right: &ast.ValueNode{Position: pos(38, 44), Value: -12.34},
				},
				Right: &ast.ComparisonNode{
					Position: pos(54, 111),
					Operator: "!=",
					Left: &ast.AndNode{
						Position: pos(54, 97),
						Left: &ast.OrNode{
							Position: pos(54, 79),
							Left: &ast.ComparisonNode{
								Position: pos(54, 70),
								Operator: ast.ComparisonOperatorEquals,
								Left: &ast.ValueNode{
									Position: pos(54, 59),
									Value:    false,
								},
								Right: &ast.VariableNode{
									Position: pos(63, 70),
									Name:     "VAR2",
								},
							},
							Right: &ast.ValueNode{
								Position: pos(75, 79),
								Value:    true,
							},
						},
						Right: &ast.NegateNode{
							Position: pos(82, 97),
							Expr: &ast.ComparisonNode{
								Position: pos(84, 96),
								Operator: ast.ComparisonOperatorEquals,
								Left:     &ast.ValueNode{Position: pos(84, 88)},
								Right:    &ast.ValueNode{Position: pos(92, 96)},
							},
						},
					},
					Right: &ast.ValueNode{Position: pos(103, 111), Value: "quoted"},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			expr, err := ast.NewParser[string](testCase.Input).Parse()

			if got, want := err, testCase.Error; !errors.Is(got, want) {
				t.Errorf("got error %v, want %v", got, want)
			}

			if diff := cmp.Diff(testCase.Expected, expr); diff != "" {
				t.Errorf("Parse() mismatch (-want, +got) = %v", diff)
			}
		})
	}
}

func TestParseVariable(t *testing.T) {
	testCases := []struct {
		Name     string
		Input    string
		Expected ast.VariableNode
		Error    error
	}{
		{
			Name:  "simple",
			Input: `$(SOME_VAR)`,
			Expected: ast.VariableNode{
				Position: pos(0, 11),
				Name:     "SOME_VAR",
			},
		},
		{
			Name:  "with key",
			Input: `$(SOME_DICT{key})`,
			Expected: ast.VariableNode{
				Position: pos(0, 17),
				Name:     "SOME_DICT",
				Key:      ptr("key"),
			},
		},
		{
			Name:  "with key and default",
			Input: `$(SOME_DICT{key}|default)`,
			Expected: ast.VariableNode{
				Position: pos(0, 25),
				Name:     "SOME_DICT",
				Key:      ptr("key"),
				Default: &ast.ValueNode{
					Position: pos(17, 24),
					Value:    "default",
				},
			},
		},
		{
			Name:  "with default",
			Input: `$(SOME_VAR|default)`,
			Expected: ast.VariableNode{
				Position: pos(0, 19),
				Name:     "SOME_VAR",
				Default: &ast.ValueNode{
					Position: pos(11, 18),
					Value:    "default",
				},
			},
		},
		{
			Name:  "data after variable",
			Input: `$(SOME_DICT{key}|default) suffix`,
			Expected: ast.VariableNode{
				Position: pos(0, 25),
				Name:     "SOME_DICT",
				Key:      ptr("key"),
				Default: &ast.ValueNode{
					Position: pos(17, 24),
					Value:    "default",
				},
			},
		},
		{
			Name:  "data before variable",
			Input: `prefix $(SOME_DICT{key}|default)`,
			Error: unexpected(0, 6, token.TypeSimpleString),
		},
		{
			Name:  "invalid variable",
			Input: `$(SOME DICT{key}|default)`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(6, 7)},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			v, err := ast.NewParser[string](testCase.Input).ParseVariable()

			if got, want := err, testCase.Error; !errors.Is(got, want) {
				t.Errorf("got error %v, want %v", got, want)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(testCase.Expected, *v); diff != "" {
				t.Errorf("Parse() mismatch (-want, +got) = %v", diff)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	const expr = `$(VAR1{key1}|$(DEFAULT{key2}|none)) < -12.34 & ((
				false == $(VAR2)) | 	true & !(null == null) ) != 'quoted'`

	b.ReportAllocs()
	b.SetBytes(int64(len(expr)))

	var p ast.Parser[string]

	for b.Loop() {
		p.Reset(expr)

		if _, err := p.Parse(); err != nil {
			b.Fatal(err)
		}
	}
}
