package esiexpr_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/esi/esiexpr"
	"github.com/nussjustin/esi/esiexpr/ast"
	"github.com/nussjustin/esi/esiexpr/token"
)

func pos(start, end int) token.Position {
	return token.Position{Start: start, End: end}
}

var (
	errComparison      = errors.New("comparison not supported")
	errInvalidVar      = errors.New("invalid var")
	errMismatchedType  = errors.New("mismatched type for comparison")
	errUnsupportedType = errors.New("unsupported type for comparison")
)

func compareValues(a, b ast.Value) (int, error) {
	switch av := a.(type) {
	case float64:
		bv, ok := b.(float64)
		if !ok {
			return 0, errMismatchedType
		}
		return int(av - bv), nil
	case int:
		bv, ok := b.(int)
		if !ok {
			return 0, errMismatchedType
		}
		return av - bv, nil
	case string:
		bv, ok := b.(string)
		if !ok {
			return 0, errMismatchedType
		}
		return strings.Compare(av, bv), nil
	default:
		return 0, errUnsupportedType
	}
}

func valueToBool(val ast.Value) (bool, error) {
	switch v := val.(type) {
	case bool:
		return v, nil
	case float64:
		return v != 0, nil
	case int:
		return v != 0, nil
	case nil:
		return false, nil
	case string:
		return v != "", nil
	default:
		panic("unreachable")
	}
}

var testEnv = &esiexpr.Env{
	LookupVar: func(_ context.Context, name string, key *string) (ast.Value, error) {
		switch name {
		case "BOOL":
			return true, nil
		case "DICT":
			switch *key {
			case "bool":
				return false, nil
			case "error":
				return nil, errInvalidVar
			case "float":
				return -23.45, nil
			case "int":
				return -2345, nil
			case "nil":
				return nil, nil
			case "string":
				return "STRING", nil
			default:
				panic("unreachable")
			}
		case "ERROR":
			return nil, errInvalidVar
		case "FLOAT":
			return 12.34, nil
		case "INT":
			return 1234, nil
		case "NIL":
			return nil, nil
		case "STRING":
			return "string", nil
		default:
			panic("unreachable")
		}
	},
}

func TestEnv_Eval(t *testing.T) {
	testsCases := []struct {
		Name          string
		Input         string
		CompareValues func(a, b ast.Value) (int, error)
		ValueToBool   func(v ast.Value) (bool, error)
		Result        ast.Value
		Error         error
	}{
		{
			Name:   "bool var",
			Input:  `$(BOOL)`,
			Result: true,
		},
		{
			Name:   "float var",
			Input:  `$(FLOAT)`,
			Result: 12.34,
		},
		{
			Name:   "int var",
			Input:  `$(INT)`,
			Result: 1234,
		},
		{
			Name:   "nil var",
			Input:  `$(NIL)`,
			Result: nil,
		},
		{
			Name:   "string",
			Input:  `'hello world'`,
			Result: `hello world`,
		},
		{
			Name:   "string var",
			Input:  `$(STRING)`,
			Result: `string`,
		},
		{
			Name:   "dict with unused default",
			Input:  `$(DICT{string}|default)`,
			Result: `STRING`,
		},
		{
			Name:   "dict with used default",
			Input:  `$(DICT{nil}|default)`,
			Result: `default`,
		},
		{
			Name:  "error",
			Input: `$(ERROR)`,
			Error: errInvalidVar,
		},
		{
			Name:  "error with default",
			Input: `$(ERROR|default)`,
			Error: errInvalidVar,
		},
		{
			Name:  "invalid syntax",
			Input: `$( STRING)`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(2, 3)},
		},
		{
			Name:   "nested vars",
			Input:  `$(DICT{nil}|$(NIL|$(DICT{int})))`,
			Result: -2345,
		},

		{
			Name:   "false and false",
			Input:  `false & false`,
			Result: false,
		},
		{
			Name:   "false and true",
			Input:  `false & true`,
			Result: false,
		},
		{
			Name:   "true and false",
			Input:  `true & false`,
			Result: false,
		},
		{
			Name:   "true and true",
			Input:  `true & true`,
			Result: true,
		},
		{
			Name:  "true and non-bool true",
			Input: `true & 1234`,
			Error: &esiexpr.NonBoolValueError{Value: 1234},
		},
		{
			Name:        "true and non-bool true converted",
			ValueToBool: valueToBool,
			Input:       `true & 0`,
			Result:      false,
		},
		{
			Name:        "true and non-bool true with failed conversion",
			ValueToBool: func(ast.Value) (bool, error) { return false, errUnsupportedType },
			Input:       `true & 0`,
			Error:       errUnsupportedType,
		},
		{
			Name:        "non short-circuiting and",
			ValueToBool: func(ast.Value) (bool, error) { return false, errUnsupportedType },
			Input:       `false & 0`,
			Error:       errUnsupportedType,
		},
		{
			Name:   "false or false",
			Input:  `false | false`,
			Result: false,
		},
		{
			Name:   "false or true",
			Input:  `false | true`,
			Result: true,
		},
		{
			Name:   "true or false",
			Input:  `true | false`,
			Result: true,
		},
		{
			Name:   "true or true",
			Input:  `true | true`,
			Result: true,
		},
		{
			Name:  "false or non-bool true",
			Input: `false | 1234`,
			Error: &esiexpr.NonBoolValueError{Value: 1234},
		},
		{
			Name:        "false or non-bool true converted",
			ValueToBool: valueToBool,
			Input:       `false | 0`,
			Result:      false,
		},
		{
			Name:        "false or non-bool true with failed conversion",
			ValueToBool: func(ast.Value) (bool, error) { return false, errUnsupportedType },
			Input:       `false | 0`,
			Error:       errUnsupportedType,
		},
		{
			Name:        "non short-circuiting or",
			ValueToBool: func(ast.Value) (bool, error) { return false, errUnsupportedType },
			Input:       `true | 0`,
			Error:       errUnsupportedType,
		},
		{
			Name:   "negated false",
			Input:  `!false`,
			Result: true,
		},
		{
			Name:   "negated true",
			Input:  `!true`,
			Result: false,
		},
		{
			Name:   "negated sub expression)",
			Input:  `!(false | true)`,
			Result: false,
		},
		{
			Name:  "negated non-bool",
			Input: `!'string'`,
			Error: &esiexpr.NonBoolValueError{Value: "string"},
		},
		{
			Name:        "negated non-bool with conversion",
			ValueToBool: valueToBool,
			Input:       `!'string'`,
			Result:      false,
		},
		{
			Name:        "negated non-bool with failed conversion",
			ValueToBool: func(ast.Value) (bool, error) { return false, errUnsupportedType },
			Input:       `!'string'`,
			Error:       errUnsupportedType,
		},

		{
			Name:          "$(INT) equals 1234",
			Input:         `$(INT) == 1234`,
			CompareValues: compareValues,
			Result:        true,
		},
		{
			Name:          "$(INT) equals 2345",
			Input:         `$(INT) == 2345`,
			CompareValues: compareValues,
			Result:        false,
		},
		{
			Name:  "equals without compare",
			Input: `1234 == 2345`,
			Error: &esiexpr.ComparisonUnsupportedError{Operator: ast.ComparisonOperatorEquals},
		},
		{
			Name:          "equals error",
			Input:         `1234 == 2345`,
			CompareValues: func(ast.Value, ast.Value) (int, error) { return 0, errComparison },
			Error:         errComparison,
		},
		{
			Name:          "$(INT) not equals 1234",
			Input:         `$(INT) != 1234`,
			CompareValues: compareValues,
			Result:        false,
		},
		{
			Name:          "$(INT) not equals 2345",
			Input:         `$(INT) != 2345`,
			CompareValues: compareValues,
			Result:        true,
		},
		{
			Name:  "not equals without compare",
			Input: `1234 != 2345`,
			Error: &esiexpr.ComparisonUnsupportedError{Operator: ast.ComparisonOperatorNotEquals},
		},
		{
			Name:          "not equals error",
			Input:         `1234 != 2345`,
			CompareValues: func(ast.Value, ast.Value) (int, error) { return 0, errComparison },
			Error:         errComparison,
		},
		{
			Name:          "$(INT) less than 1234",
			Input:         `$(INT) < 1234`,
			CompareValues: compareValues,
			Result:        false,
		},
		{
			Name:  "less than without compare",
			Input: `1234 < 2345`,
			Error: &esiexpr.ComparisonUnsupportedError{Operator: ast.ComparisonOperatorLessThan},
		},
		{
			Name:          "less than error",
			Input:         `1234 < 2345`,
			CompareValues: func(ast.Value, ast.Value) (int, error) { return 0, errComparison },
			Error:         errComparison,
		},
		{
			Name:          "$(INT) less than equal 1234",
			Input:         `$(INT) <= 1234`,
			CompareValues: compareValues,
			Result:        true,
		},
		{
			Name:  "less than equal without compare",
			Input: `1234 <= 2345`,
			Error: &esiexpr.ComparisonUnsupportedError{Operator: ast.ComparisonOperatorLessThanEquals},
		},
		{
			Name:          "less than equal error",
			Input:         `1234 <= 2345`,
			CompareValues: func(ast.Value, ast.Value) (int, error) { return 0, errComparison },
			Error:         errComparison,
		},
		{
			Name:          "$(INT) greater than 1234",
			Input:         `$(INT) > 1234`,
			CompareValues: compareValues,
			Result:        false,
		},
		{
			Name:  "greater than equal without compare",
			Input: `1234 > 2345`,
			Error: &esiexpr.ComparisonUnsupportedError{Operator: ast.ComparisonOperatorGreaterThan},
		},
		{
			Name:          "greater than error",
			Input:         `1234 > 2345`,
			CompareValues: func(ast.Value, ast.Value) (int, error) { return 0, errComparison },
			Error:         errComparison,
		},
		{
			Name:          "$(INT) greater than equal 1234",
			Input:         `$(INT) >= 1234`,
			CompareValues: compareValues,
			Result:        true,
		},
		{
			Name:  "greater than equal without compare",
			Input: `1234 >= 2345`,
			Error: &esiexpr.ComparisonUnsupportedError{Operator: ast.ComparisonOperatorGreaterThanEquals},
		},
		{
			Name:          "greater than equal error",
			Input:         `1234 >= 2345`,
			CompareValues: func(ast.Value, ast.Value) (int, error) { return 0, errComparison },
			Error:         errComparison,
		},

		{
			Name:          "complex",
			CompareValues: compareValues,
			ValueToBool:   valueToBool,
			Input:         `($(INT) < 10000 & ($(NIL|default) == $(STRING) | $(FLOAT) < $(DICT{float}))) | $(DICT{bool})`,
			Result:        false,
		},
	}

	for _, testCase := range testsCases {
		t.Run(testCase.Name, func(t *testing.T) {
			env := *testEnv
			env.CompareValues = testCase.CompareValues
			env.ValueToBool = testCase.ValueToBool

			got, err := env.Eval(t.Context(), testCase.Input)
			if !errors.Is(err, testCase.Error) {
				t.Errorf("got error %v, want %v", err, testCase.Error)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(testCase.Result, got); diff != "" {
				t.Errorf("got: %v, want: %v", got, testCase.Result)
			}
		})
	}
}

func TestEnv_Interpolate(t *testing.T) {
	testsCases := []struct {
		Name   string
		Input  string
		Result string
		Error  error
	}{
		{
			Name:   "simple bool",
			Input:  `$(BOOL)`,
			Result: "true",
		},
		{
			Name:   "simple float",
			Input:  `$(FLOAT)`,
			Result: `12.34`,
		},
		{
			Name:   "simple int",
			Input:  `$(INT)`,
			Result: "1234",
		},
		{
			Name:   "simple nil",
			Input:  `$(NIL)`,
			Result: "",
		},
		{
			Name:   "simple string",
			Input:  `$(STRING)`,
			Result: `string`,
		},
		{
			Name:   "dict",
			Input:  `$(DICT{string})`,
			Result: `STRING`,
		},
		{
			Name:   "simple with unused default",
			Input:  `$(INT|default)`,
			Result: "1234",
		},
		{
			Name:   "simple with used default",
			Input:  `$(NIL|default)`,
			Result: `default`,
		},
		{
			Name:   "dict with unused default",
			Input:  `$(DICT{string}|default)`,
			Result: `STRING`,
		},
		{
			Name:   "dict with used default",
			Input:  `$(DICT{nil}|default)`,
			Result: `default`,
		},
		{
			Name:  "error",
			Input: `$(ERROR)`,
			Error: errInvalidVar,
		},
		{
			Name:  "error with default",
			Input: `$(ERROR|default)`,
			Error: errInvalidVar,
		},
		{
			Name:  "invalid syntax",
			Input: `$( STRING)`,
			Error: &ast.UnexpectedWhiteSpaceError{Position: pos(2, 3)},
		},
		{
			Name:   "multiple",
			Input:  `before-$(BOOL)-$(DICT{float})-$(FLOAT)-$(INT)-$(STRING)-after`,
			Result: `before-true--23.45-12.34-1234-string-after`,
		},
		{
			Name:   "nested vars",
			Input:  `$(DICT{nil}|$(NIL|$(DICT{int})))`,
			Result: "-2345",
		},
	}

	for _, testCase := range testsCases {
		t.Run(testCase.Name, func(t *testing.T) {
			got, err := testEnv.Interpolate(t.Context(), testCase.Input)
			if !errors.Is(err, testCase.Error) {
				t.Errorf("got error %v, want %v", err, testCase.Error)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(testCase.Result, got); diff != "" {
				t.Errorf("got: %v, want: %v", got, testCase.Result)
			}
		})
	}
}
