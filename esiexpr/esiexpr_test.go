package esiexpr_test

import (
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/esi/esiexpr"
)

func position(start, end int) esiexpr.Position {
	return esiexpr.Position{Start: start, End: end}
}

func ptr[T any](v T) *T {
	return &v
}

func TestParse(t *testing.T) {
	testCases := []struct {
		Name     string
		Input    string
		Expected esiexpr.Node
		Error    error
	}{
		{
			Name:     "bool false",
			Input:    `false`,
			Expected: &esiexpr.ValueNode{Position: position(0, 5), Value: false},
		},
		{
			Name:  "broken bool false",
			Input: `fals`,
			Error: &esiexpr.SyntaxError{Offset: 4, Message: "end of input, 'e' expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "invalid bool false",
			Input: `falSe`,
			Error: &esiexpr.SyntaxError{Offset: 3, Message: "unexpected character 'S', 's' expected"},
		},
		{
			Name:  "uppercase bool false",
			Input: `FALSE`,
			Error: &esiexpr.SyntaxError{Message: "unexpected character 'F'"},
		},
		{
			Name:     "bool true",
			Input:    `true`,
			Expected: &esiexpr.ValueNode{Position: position(0, 4), Value: true},
		},
		{
			Name:  "broken bool true",
			Input: `tru`,
			Error: &esiexpr.SyntaxError{Offset: 3, Message: "end of input, 'e' expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "invalid bool true",
			Input: `trUe`,
			Error: &esiexpr.SyntaxError{Offset: 2, Message: "unexpected character 'U', 'u' expected"},
		},
		{
			Name:  "uppercase bool true",
			Input: `TRUE`,
			Error: &esiexpr.SyntaxError{Message: "unexpected character 'T'"},
		},
		{
			Name:     "float",
			Input:    `12.34`,
			Expected: &esiexpr.ValueNode{Position: position(0, 5), Value: 12.34},
		},
		{
			Name:     "float ending in decimal point",
			Input:    `12.`,
			Expected: &esiexpr.ValueNode{Position: position(0, 3), Value: 12.0},
		},
		{
			Name:  "float with non-digit",
			Input: `12.3a4`,
			Error: &esiexpr.SyntaxError{Offset: 4, Message: "unexpected data after expression"},
		},
		{
			Name:  "float with two decimal points",
			Input: `12.3.4`,
			Error: &esiexpr.SyntaxError{Offset: 4, Message: "unexpected data after expression"},
		},
		{
			Name:     "negative float",
			Input:    `-12.34`,
			Expected: &esiexpr.ValueNode{Position: position(0, 6), Value: -12.34},
		},
		{
			Name:     "int",
			Input:    `1234`,
			Expected: &esiexpr.ValueNode{Position: position(0, 4), Value: 1234},
		},
		{
			Name:  "int with non-digit",
			Input: `12a34`,
			Error: &esiexpr.SyntaxError{Offset: 2, Message: "unexpected data after expression"},
		},
		{
			Name:     "negative int",
			Input:    `-1234`,
			Expected: &esiexpr.ValueNode{Position: position(0, 5), Value: -1234},
		},
		{
			Name:     "null",
			Input:    `null`,
			Expected: &esiexpr.ValueNode{Position: position(0, 4), Value: nil},
		},
		{
			Name:  "broken null",
			Input: `nul`,
			Error: &esiexpr.SyntaxError{Offset: 3, Message: "end of input, 'l' expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "invalid null",
			Input: `nuLl`,
			Error: &esiexpr.SyntaxError{Offset: 2, Message: "unexpected character 'L', 'l' expected"},
		},
		{
			Name:  "uppercase null",
			Input: `NULL`,
			Error: &esiexpr.SyntaxError{Message: "unexpected character 'N'"},
		},
		{
			Name:     "empty",
			Input:    `''`,
			Expected: &esiexpr.ValueNode{Position: position(0, 2), Value: ""},
		},
		{
			Name:     "string",
			Input:    `'hello world'`,
			Expected: &esiexpr.ValueNode{Position: position(0, 13), Value: "hello world"},
		},
		{
			Name:  "string with quote",
			Input: `'hello \'world\''`,
			Error: &esiexpr.SyntaxError{Offset: 9, Message: "unexpected data after expression"},
		},
		{
			Name:  "string without closing quote",
			Input: `'hello world`,
			Error: &esiexpr.SyntaxError{Offset: 12, Message: "missing closing quote"},
		},
		{
			Name:     "sub expression",
			Input:    `(true)`,
			Expected: &esiexpr.ValueNode{Position: position(1, 5), Value: true},
		},
		{
			Name:  "empty sub expression",
			Input: `()`,
			Error: &esiexpr.SyntaxError{Offset: 1, Message: "unexpected character ')'"},
		},
		{
			Name:  "non-closed sub expression",
			Input: `(true`,
			Error: &esiexpr.SyntaxError{Offset: 5, Message: "end of input, ')' expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "lone closing parenthesis",
			Input: `(true))`,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "unexpected data after expression"},
		},
		{
			Name:     "variable",
			Input:    `$(TEST)`,
			Expected: &esiexpr.VariableNode{Position: position(0, 7), Name: "TEST"},
		},
		{
			Name:     "variable with underscore",
			Input:    `$(TEST_IT)`,
			Expected: &esiexpr.VariableNode{Position: position(0, 10), Name: "TEST_IT"},
		},
		{
			Name:     "variable with empty key",
			Input:    `$(TEST{})`,
			Expected: &esiexpr.VariableNode{Position: position(0, 9), Name: "TEST", Key: ptr("")},
		},
		{
			Name:     "variable with key",
			Input:    `$(TEST{it})`,
			Expected: &esiexpr.VariableNode{Position: position(0, 11), Name: "TEST", Key: ptr("it")},
		},
		{
			Name:  "variable with space before key",
			Input: `$(TEST{ it})`,
			Error: &esiexpr.SyntaxError{Offset: 7, Message: "unexpected space in key"},
		},
		{
			Name:  "variable with space in key",
			Input: `$(TEST{i t})`,
			Error: &esiexpr.SyntaxError{Offset: 8, Message: "unexpected space in key"},
		},
		{
			Name:  "variable with space after key",
			Input: `$(TEST{it })`,
			Error: &esiexpr.SyntaxError{Offset: 9, Message: "unexpected space in key"},
		},
		{
			Name:  "variable with default",
			Input: `$(TEST|default)`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 15),
				Name:     "TEST",
				Default: &esiexpr.ValueNode{
					Position: position(7, 14),
					Value:    "default",
				},
			},
		},
		{
			Name:  "variable with empty default",
			Input: `$(TEST|)`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 8),
				Name:     "TEST",
				Default: &esiexpr.ValueNode{
					Position: position(7, 7),
					Value:    "",
				},
			},
		},
		{
			Name:  "variable with quoted default",
			Input: `$(TEST|'quoted default')`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 24),
				Name:     "TEST",
				Default: &esiexpr.ValueNode{
					Position: position(7, 23),
					Value:    "quoted default",
				},
			},
		},
		{
			Name:  "variable with key and default",
			Input: `$(TEST{it}|default)`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 19),
				Name:     "TEST",
				Key:      ptr("it"),
				Default: &esiexpr.ValueNode{
					Position: position(11, 18),
					Value:    "default",
				},
			},
		},
		{
			Name:  "variable with key and empty default",
			Input: `$(TEST{it}|)`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 12),
				Name:     "TEST",
				Key:      ptr("it"),
				Default: &esiexpr.ValueNode{
					Position: position(11, 11),
					Value:    "",
				},
			},
		},
		{
			Name:  "variable with key and quoted default",
			Input: `$(TEST{it}|'quoted default')`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 28),
				Name:     "TEST",
				Key:      ptr("it"),
				Default: &esiexpr.ValueNode{
					Position: position(11, 27),
					Value:    "quoted default",
				},
			},
		},
		{
			Name:  "variable with variable default",
			Input: `$(TEST|$(DEFAULT{test}|none))`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 29),
				Name:     "TEST",
				Default: &esiexpr.VariableNode{
					Position: position(7, 28),
					Name:     "DEFAULT",
					Key:      ptr("test"),
					Default: &esiexpr.ValueNode{
						Position: position(23, 27),
						Value:    "none",
					},
				},
			},
		},
		{
			Name:  "variable with invalid character after (",
			Input: `$(!TEST)`,
			Error: &esiexpr.SyntaxError{Offset: 2, Message: "unexpected character '!' in variable name"},
		},
		{
			Name:  "variable with space after (",
			Input: `$( TEST)`,
			Error: &esiexpr.SyntaxError{Offset: 2, Message: "unexpected space in variable name"},
		},
		{
			Name:  "variable with invalid character before )",
			Input: `$(TEST!)`,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "unexpected character '!' in variable name"},
		},
		{
			Name:  "variable with space before )",
			Input: `$(TEST )`,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "unexpected space in variable name"},
		},
		{
			Name:  "variable with invalid identifier",
			Input: `$(IN.VALID)`,
			Error: &esiexpr.SyntaxError{Offset: 4, Message: "unexpected character '.' in variable name"},
		},
		{
			Name:  "variable with space before default delimiter",
			Input: `$(TEST |default)`,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "unexpected space in variable name"},
		},
		{
			Name:  "variable with space after default delimiter",
			Input: `$(TEST| default)`,
			Error: &esiexpr.SyntaxError{Offset: 7, Message: "unexpected space in default value"},
		},
		{
			Name:  "variable with default string false",
			Input: `$(TEST|false)`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 13),
				Name:     "TEST",
				Default: &esiexpr.ValueNode{
					Position: position(7, 12),
					Value:    "false",
				},
			},
		},
		{
			Name:  "variable with default string null",
			Input: `$(TEST|null)`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 12),
				Name:     "TEST",
				Default: &esiexpr.ValueNode{
					Position: position(7, 11),
					Value:    "null",
				},
			},
		},
		{
			Name:  "variable with default string true",
			Input: `$(TEST|true)`,
			Expected: &esiexpr.VariableNode{
				Position: position(0, 12),
				Name:     "TEST",
				Default: &esiexpr.ValueNode{
					Position: position(7, 11),
					Value:    "true",
				},
			},
		},
		{
			Name:  "negated bool false",
			Input: `!false`,
			Expected: &esiexpr.NegateNode{
				Position: position(0, 6),
				Expr: &esiexpr.ValueNode{
					Position: position(1, 6),
					Value:    false,
				},
			},
		},
		{
			Name:  "negated bool true",
			Input: `!true`,
			Expected: &esiexpr.NegateNode{
				Position: position(0, 5),
				Expr: &esiexpr.ValueNode{
					Position: position(1, 5),
					Value:    true,
				},
			},
		},
		{
			Name:  "negated float",
			Input: `!12.34`,
			Expected: &esiexpr.NegateNode{
				Position: position(0, 6),
				Expr: &esiexpr.ValueNode{
					Position: position(1, 6),
					Value:    12.34,
				},
			},
		},
		{
			Name:  "negated int",
			Input: `!1234`,
			Expected: &esiexpr.NegateNode{
				Position: position(0, 5),
				Expr: &esiexpr.ValueNode{
					Position: position(1, 5),
					Value:    1234,
				},
			},
		},
		{
			Name:  "negated null",
			Input: `!null`,
			Expected: &esiexpr.NegateNode{
				Position: position(0, 5),
				Expr: &esiexpr.ValueNode{
					Position: position(1, 5),
					Value:    nil,
				},
			},
		},
		{
			Name:  "negated string",
			Input: `!'test'`,
			Expected: &esiexpr.NegateNode{
				Position: position(0, 7),
				Expr: &esiexpr.ValueNode{
					Position: position(1, 7),
					Value:    "test",
				},
			},
		},
		{
			Name:  "negated sub expression",
			Input: `!(true & false)`,
			Expected: &esiexpr.NegateNode{
				Position: position(0, 15),
				Expr: &esiexpr.AndNode{
					Position: position(2, 14),
					Left: &esiexpr.ValueNode{
						Position: position(2, 6),
						Value:    true,
					},
					Right: &esiexpr.ValueNode{
						Position: position(9, 14),
						Value:    false,
					},
				},
			},
		},
		{
			Name:  "negated variable",
			Input: `!$(TEST)`,
			Expected: &esiexpr.NegateNode{
				Position: position(0, 8),
				Expr: &esiexpr.VariableNode{
					Position: position(1, 8),
					Name:     "TEST",
				},
			},
		},

		{
			Name:  "equals",
			Input: `12 == 34`,
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 8),
				Operator: esiexpr.ComparisonOperatorEquals,
				Left:     &esiexpr.ValueNode{Position: position(0, 2), Value: 12},
				Right:    &esiexpr.ValueNode{Position: position(6, 8), Value: 34},
			},
		},
		{
			Name:  "equals without right-side",
			Input: `12 == `,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "expression expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "broken equals",
			Input: `12 = 34`,
			Error: &esiexpr.SyntaxError{Offset: 4, Message: "unexpected character ' ', '=' expected"},
		},
		{
			Name:  "not equals",
			Input: `12 != 34`,
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 8),
				Operator: esiexpr.ComparisonOperatorNotEquals,
				Left:     &esiexpr.ValueNode{Position: position(0, 2), Value: 12},
				Right:    &esiexpr.ValueNode{Position: position(6, 8), Value: 34},
			},
		},
		{
			Name:  "not equals without right-side",
			Input: `12 != `,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "expression expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "broken not equals",
			Input: `12 ! 34`,
			Error: &esiexpr.SyntaxError{Offset: 4, Message: "unexpected character ' ', '=' expected"},
		},
		{
			Name:  "greater than",
			Input: `12 > 34`,
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 7),
				Operator: esiexpr.ComparisonOperatorGreaterThan,
				Left:     &esiexpr.ValueNode{Position: position(0, 2), Value: 12},
				Right:    &esiexpr.ValueNode{Position: position(5, 7), Value: 34},
			},
		},
		{
			Name:  "greater than without right-side",
			Input: `12 > `,
			Error: &esiexpr.SyntaxError{Offset: 5, Message: "expression expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "greater than equals",
			Input: `12 >= 34`,
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 8),
				Operator: esiexpr.ComparisonOperatorGreaterThanEquals,
				Left:     &esiexpr.ValueNode{Position: position(0, 2), Value: 12},
				Right:    &esiexpr.ValueNode{Position: position(6, 8), Value: 34},
			},
		},
		{
			Name:  "greater than equals without right-side",
			Input: `12 >= `,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "expression expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "less than",
			Input: `12 < 34`,
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 7),
				Operator: esiexpr.ComparisonOperatorLessThan,
				Left:     &esiexpr.ValueNode{Position: position(0, 2), Value: 12},
				Right:    &esiexpr.ValueNode{Position: position(5, 7), Value: 34},
			},
		},
		{
			Name:  "less than without right-side",
			Input: `12 < `,
			Error: &esiexpr.SyntaxError{Offset: 5, Message: "expression expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "less than equals",
			Input: `12 <= 34`,
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 8),
				Operator: esiexpr.ComparisonOperatorLessThanEquals,
				Left:     &esiexpr.ValueNode{Position: position(0, 2), Value: 12},
				Right:    &esiexpr.ValueNode{Position: position(6, 8), Value: 34},
			},
		},
		{
			Name:  "less than equals without right-side",
			Input: `12 <= `,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "expression expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "and",
			Input: `true & false`,
			Expected: &esiexpr.AndNode{
				Position: position(0, 12),
				Left: &esiexpr.ValueNode{
					Position: position(0, 4),
					Value:    true,
				},
				Right: &esiexpr.ValueNode{
					Position: position(7, 12),
					Value:    false,
				},
			},
		},
		{
			Name:  "and without right-side",
			Input: `true & `,
			Error: &esiexpr.SyntaxError{Offset: 7, Message: "expression expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "multiple and",
			Input: `true & false & null`,
			Expected: &esiexpr.AndNode{
				Position: position(0, 19),
				Left: &esiexpr.ValueNode{
					Position: position(0, 4),
					Value:    true,
				},
				Right: &esiexpr.AndNode{
					Position: position(7, 19),
					Left: &esiexpr.ValueNode{
						Position: position(7, 12),
						Value:    false,
					},
					Right: &esiexpr.ValueNode{
						Position: position(15, 19),
						Value:    nil,
					},
				},
			},
		},
		{
			Name:  "or",
			Input: `false | true`,
			Expected: &esiexpr.OrNode{
				Position: position(0, 12),
				Left: &esiexpr.ValueNode{
					Position: position(0, 5),
					Value:    false,
				},
				Right: &esiexpr.ValueNode{
					Position: position(8, 12),
					Value:    true,
				},
			},
		},
		{
			Name:  "or without right-side",
			Input: `false | `,
			Error: &esiexpr.SyntaxError{Offset: 8, Message: "expression expected", Underlying: io.ErrUnexpectedEOF},
		},
		{
			Name:  "multiple or",
			Input: `false | true | null`,
			Expected: &esiexpr.OrNode{
				Position: position(0, 19),
				Left: &esiexpr.ValueNode{
					Position: position(0, 5),
					Value:    false,
				},
				Right: &esiexpr.OrNode{
					Position: position(8, 19),
					Left: &esiexpr.ValueNode{
						Position: position(8, 12),
						Value:    true,
					},
					Right: &esiexpr.ValueNode{
						Position: position(15, 19),
						Value:    nil,
					},
				},
			},
		},
		{
			Name:  "mixed and-or",
			Input: `false & true | null`,
			Expected: &esiexpr.AndNode{
				Position: position(0, 19),
				Left: &esiexpr.ValueNode{
					Position: position(0, 5),
					Value:    false,
				},
				Right: &esiexpr.OrNode{
					Position: position(8, 19),
					Left: &esiexpr.ValueNode{
						Position: position(8, 12),
						Value:    true,
					},
					Right: &esiexpr.ValueNode{
						Position: position(15, 19),
						Value:    nil,
					},
				},
			},
		},

		{
			Name:  "variable equals value",
			Input: "$(TEST) == 12.34",
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 16),
				Operator: esiexpr.ComparisonOperatorEquals,
				Left: &esiexpr.VariableNode{
					Position: position(0, 7),
					Name:     "TEST",
				},
				Right: &esiexpr.ValueNode{
					Position: position(11, 16),
					Value:    12.34,
				},
			},
		},
		{
			Name:  "value equals variable",
			Input: "1234 == $(TEST{it})",
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 19),
				Operator: esiexpr.ComparisonOperatorEquals,
				Left: &esiexpr.ValueNode{
					Position: position(0, 4),
					Value:    1234,
				},
				Right: &esiexpr.VariableNode{
					Position: position(8, 19),
					Name:     "TEST",
					Key:      ptr("it"),
				},
			},
		},
		{
			Name:  "variable equals variable",
			Input: "$(VAR1) == $(VAR2|vars2)",
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 24),
				Operator: esiexpr.ComparisonOperatorEquals,
				Left: &esiexpr.VariableNode{
					Position: position(0, 7),
					Name:     "VAR1",
				},
				Right: &esiexpr.VariableNode{
					Position: position(11, 24),
					Name:     "VAR2",
					Default: &esiexpr.ValueNode{
						Position: position(18, 23),
						Value:    "vars2",
					},
				},
			},
		},

		{
			Name:  "extra data",
			Input: `true data`,
			Error: &esiexpr.SyntaxError{Offset: 5, Message: "unexpected data after expression"},
		},
		{
			Name:  "extra data that is a valid expression",
			Input: `true false`,
			Error: &esiexpr.SyntaxError{Offset: 5, Message: "unexpected data after expression"},
		},

		{
			Name: "complex expression",
			Input: `$(VAR1{key1}|$(DEFAULT{key2}|none)) < -12.34 & ((
				false == $(VAR2)) | 	true & !(null == null) ) == true != 'quoted'`,
			Expected: &esiexpr.ComparisonNode{
				Position: position(0, 119),
				Operator: "<",
				Left: &esiexpr.VariableNode{
					Position: position(0, 35),
					Name:     "VAR1",
					Key:      ptr("key1"),
					Default: &esiexpr.VariableNode{
						Position: position(13, 34),
						Name:     "DEFAULT",
						Key:      ptr("key2"),
						Default: &esiexpr.ValueNode{
							Position: position(29, 33),
							Value:    "none",
						},
					},
				},
				Right: &esiexpr.AndNode{
					Position: position(38, 119),
					Left:     &esiexpr.ValueNode{Position: position(38, 44), Value: -12.34},
					Right: &esiexpr.ComparisonNode{
						Position: position(47, 119),
						Operator: "==",
						Left: &esiexpr.OrNode{
							Position: esiexpr.Position{
								Start: 48,
								End:   98,
							},
							Left: &esiexpr.ComparisonNode{
								Position: position(54, 70),
								Operator: esiexpr.ComparisonOperatorEquals,
								Left:     &esiexpr.ValueNode{Position: position(54, 59), Value: false},
								Right:    &esiexpr.VariableNode{Position: position(63, 70), Name: "VAR2"},
							},
							Right: &esiexpr.AndNode{
								Position: position(75, 98),
								Left:     &esiexpr.ValueNode{Position: position(75, 79), Value: true},
								Right: &esiexpr.NegateNode{
									Position: position(82, 97),
									Expr: &esiexpr.ComparisonNode{
										Position: position(84, 96),
										Operator: esiexpr.ComparisonOperatorEquals,
										Left: &esiexpr.ValueNode{
											Position: position(84, 88),
										},
										Right: &esiexpr.ValueNode{
											Position: position(92, 96),
										},
									},
								},
							},
						},
						Right: &esiexpr.ComparisonNode{
							Position: esiexpr.Position{
								Start: 103,
								End:   119,
							},
							Operator: esiexpr.ComparisonOperatorNotEquals,
							Left:     &esiexpr.ValueNode{Position: position(103, 107), Value: true},
							Right:    &esiexpr.ValueNode{Position: position(111, 119), Value: "quoted"},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			expr, err := esiexpr.Parse(testCase.Input)

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
		Expected esiexpr.VariableNode
		Error    error
	}{
		{
			Name:  "simple",
			Input: `$(SOME_VAR)`,
			Expected: esiexpr.VariableNode{
				Position: position(0, 11),
				Name:     "SOME_VAR",
			},
		},
		{
			Name:  "with key",
			Input: `$(SOME_DICT{key})`,
			Expected: esiexpr.VariableNode{
				Position: position(0, 17),
				Name:     "SOME_DICT",
				Key:      ptr("key"),
			},
		},
		{
			Name:  "with key and default",
			Input: `$(SOME_DICT{key}|default)`,
			Expected: esiexpr.VariableNode{
				Position: position(0, 25),
				Name:     "SOME_DICT",
				Key:      ptr("key"),
				Default: &esiexpr.ValueNode{
					Position: position(17, 24),
					Value:    "default",
				},
			},
		},
		{
			Name:  "with default",
			Input: `$(SOME_VAR|default)`,
			Expected: esiexpr.VariableNode{
				Position: position(0, 19),
				Name:     "SOME_VAR",
				Default: &esiexpr.ValueNode{
					Position: position(11, 18),
					Value:    "default",
				},
			},
		},
		{
			Name:  "data after variable",
			Input: `$(SOME_DICT{key}|default) suffix`,
			Expected: esiexpr.VariableNode{
				Position: position(0, 25),
				Name:     "SOME_DICT",
				Key:      ptr("key"),
				Default: &esiexpr.ValueNode{
					Position: position(17, 24),
					Value:    "default",
				},
			},
		},
		{
			Name:  "data before variable",
			Input: `prefix $(SOME_DICT{key}|default)`,
			Error: &esiexpr.SyntaxError{Offset: 0, Message: "unexpected character 'p', '$' expected"},
		},
		{
			Name:  "invalid variable",
			Input: `$(SOME DICT{key}|default)`,
			Error: &esiexpr.SyntaxError{Offset: 6, Message: "unexpected space in variable name"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			v, err := esiexpr.ParseVariable(testCase.Input)

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
		false == $(VAR2)) | 	true & !(null == null) ) == true != 'quoted'`

	for b.Loop() {
		if _, err := esiexpr.Parse(expr); err != nil {
			b.Fatal(err)
		}
	}
}
