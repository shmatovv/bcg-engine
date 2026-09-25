package typecheck

import (
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
)

// UnaryErrorKind identifies why unary type inference failed.
type UnaryErrorKind uint8

const (
	UnaryErrorInvalid UnaryErrorKind = iota
	UnaryErrorNonNumericOperand
)

// IsValid reports whether k is a valid unary error kind.
func (k UnaryErrorKind) IsValid() bool {
	return k == UnaryErrorNonNumericOperand
}

// String returns the canonical name of k.
func (k UnaryErrorKind) String() string {
	switch k {
	case UnaryErrorNonNumericOperand:
		return "NonNumericOperand"
	default:
		return "Invalid"
	}
}

// UnaryError describes a static unary typing failure.
type UnaryError struct {
	kind UnaryErrorKind
}

// Kind returns the unary error kind.
func (e UnaryError) Kind() UnaryErrorKind {
	return e.kind
}

// Error implements the error interface.
func (e UnaryError) Error() string {
	switch e.kind {
	case UnaryErrorNonNumericOperand:
		return "typecheck: unary numeric operand must be numeric"
	default:
		return "typecheck: invalid unary error"
	}
}

// InferNegate determines the static result type of numeric negation.
//
// Negation preserves both the ValueType and Unit:
//
//	Integer       -> Integer
//	Decimal       -> Decimal
//	Integer<kg>   -> Integer<kg>
//	Decimal<RUB>  -> Decimal<RUB>
//
// Non-numeric operands are rejected.
func InferNegate(
	operand typing.Type,
) (typing.Type, error) {
	if !operand.IsValid() {
		return typing.Type{}, UnaryError{
			kind: UnaryErrorNonNumericOperand,
		}
	}

	if !IsNumeric(operand.ValueType()) {
		return typing.Type{}, UnaryError{
			kind: UnaryErrorNonNumericOperand,
		}
	}

	return operand, nil
}
