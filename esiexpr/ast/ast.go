package ast

import (
	"github.com/nussjustin/esi/esiexpr/token"
)

// Node is the interface implemented by all possible types of parsed nodes.
type Node interface {
	Pos() token.Position

	node()
}

// AndNode represents two sub-expressions combined with the and operator (&).
type AndNode struct {
	// Position specifies the position of the node inside the expression.
	Position token.Position

	// Left contains the expression to the left of the operator.
	Left Node

	// Right contains the expression to the right of the operator.
	Right Node
}

// Pos returns the position of the node.
func (n *AndNode) Pos() token.Position {
	return n.Position
}

func (*AndNode) node() {}

// ComparisonNode represents a comparison between two values using one of the supported comparison operators.
type ComparisonNode struct {
	// Position specifies the position of the node inside the expression.
	Position token.Position

	// Operator contains the parsed operator.
	Operator ComparisonOperator

	// Left contains the expression to the left of the operator.
	Left Node

	// Right contains the expression to the right of the operator.
	Right Node
}

// Pos returns the position of the node.
func (n *ComparisonNode) Pos() token.Position {
	return n.Position
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
	Position token.Position

	// Expr is the negated sub-expression.
	Expr Node
}

// Pos returns the position of the node.
func (n *NegateNode) Pos() token.Position {
	return n.Position
}

func (*NegateNode) node() {}

// OrNode represents two sub-expressions combined with the or operator (|).
type OrNode struct {
	// Position specifies the position of the node inside the expression.
	Position token.Position

	// Left contains the expression to the left of the operator.
	Left Node

	// Right contains the expression to the right of the operator.
	Right Node
}

// Pos returns the position of the node.
func (n *OrNode) Pos() token.Position {
	return n.Position
}

func (*OrNode) node() {}

// Value is a value of type bool, float64, int, string or nil.
type Value any

// ValueNode represents a parsed value.
type ValueNode struct {
	// Position specifies the position of the node inside the expression.
	Position token.Position

	// Value contains the parsed value.
	Value Value
}

// Pos returns the position of the node.
func (n *ValueNode) Pos() token.Position {
	return n.Position
}

func (*ValueNode) node() {}

// VariableNode represents a variable reference including its default value, if any.
type VariableNode struct {
	// Position specifies the position of the node inside the expression.
	Position token.Position

	// Name contains the parsed variable name.
	Name string

	// Key is the name of the key inside the referenced dictionary or list.
	Key *string

	// Default contains the default value, if any.
	Default Node
}

// Pos returns the position of the node.
func (n *VariableNode) Pos() token.Position {
	return n.Position
}

func (*VariableNode) node() {}
