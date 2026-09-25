package typecheck

import (
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

type LogicalErrorKind uint8

const (
	LogicalErrorInvalid LogicalErrorKind = iota
	LogicalErrorNonBooleanOperand
	LogicalErrorIncompatibleTemporalModes
)

func (k LogicalErrorKind) IsValid() bool {
	switch k {
	case LogicalErrorNonBooleanOperand,
		LogicalErrorIncompatibleTemporalModes:
		return true
	default:
		return false
	}
}

func (k LogicalErrorKind) String() string {
	switch k {
	case LogicalErrorNonBooleanOperand:
		return "NonBooleanOperand"
	case LogicalErrorIncompatibleTemporalModes:
		return "IncompatibleTemporalModes"
	default:
		return "Invalid"
	}
}

type LogicalError struct {
	kind LogicalErrorKind
}

func (e LogicalError) Kind() LogicalErrorKind {
	return e.kind
}

func (e LogicalError) Error() string {
	switch e.kind {
	case LogicalErrorNonBooleanOperand:
		return "typecheck: logical operand must be boolean"
	case LogicalErrorIncompatibleTemporalModes:
		return "typecheck: logical operands have incompatible temporal modes"
	default:
		return "typecheck: invalid logical error"
	}
}

// InferLogical determines the static result type of AND/OR.
//
// Runtime uses strong three-valued logic. Therefore a nullable operand makes
// the static result potentially nullable even though short-circuit evaluation
// may still produce a defined Bool for a particular runtime value.
//
// Core v1 requires equal temporal modes.
func InferLogical(
	left typing.Type,
	right typing.Type,
) (typing.Type, error) {
	if !isBoolean(left) || !isBoolean(right) {
		return typing.Type{}, LogicalError{
			kind: LogicalErrorNonBooleanOperand,
		}
	}

	mode, err := requireSameTemporalMode(left, right)
	if err != nil {
		return typing.Type{}, LogicalError{
			kind: LogicalErrorIncompatibleTemporalModes,
		}
	}

	return typing.New(value.Bool).
		WithTemporalMode(mode).
		WithNullable(
			left.Nullable() || right.Nullable(),
		), nil
}

// InferNot determines the static result type of logical negation.
//
// NOT preserves temporal mode and nullability:
//
//	NOT Null -> Null
func InferNot(
	operand typing.Type,
) (typing.Type, error) {
	if !isBoolean(operand) {
		return typing.Type{}, LogicalError{
			kind: LogicalErrorNonBooleanOperand,
		}
	}

	result := typing.New(value.Bool)

	qualified, err := propagateStrictUnaryQualifiers(
		result,
		operand,
	)
	if err != nil {
		return typing.Type{}, LogicalError{
			kind: LogicalErrorNonBooleanOperand,
		}
	}

	return qualified, nil
}

func isBoolean(t typing.Type) bool {
	return t.IsValid() &&
		t.ValueType() == value.Bool &&
		!t.HasUnit()
}
