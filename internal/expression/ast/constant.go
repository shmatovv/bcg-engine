package ast

import "github.com/shmatovv/bcg-engine/internal/types/value"

// Constant represents a literal typed value in an expression.
//
// Examples:
//
//	10
//	3.14
//	true
//	"hello"
//	50 RUB
//
// Constant values are immutable.
type Constant struct {
	value value.Typed
}

// NewConstant constructs a Constant expression.
//
// NewConstant panics if v is invalid.
func NewConstant(v value.Typed) Constant {
	if !v.IsValid() {
		panic("ast: constant value must be valid")
	}

	return Constant{
		value: v,
	}
}

// Value returns the constant's typed value.
func (c Constant) Value() value.Typed {
	return c.value
}

// IsValid reports whether c contains a valid typed value.
func (c Constant) IsValid() bool {
	return c.value.IsValid()
}

// expressionNode marks Constant as an Expression node.
func (Constant) expressionNode() {}
