package typecheck

import (
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

type ConditionalErrorKind uint8

const (
	ConditionalErrorInvalid ConditionalErrorKind = iota
	ConditionalErrorNonBooleanCondition
	ConditionalErrorIncompatibleBranches
	ConditionalErrorInvalidOperand
	ConditionalErrorNonScalarCondition
	ConditionalErrorIncompatibleTemporalModes
)

func (k ConditionalErrorKind) IsValid() bool {
	switch k {
	case ConditionalErrorNonBooleanCondition,
		ConditionalErrorIncompatibleBranches,
		ConditionalErrorInvalidOperand,
		ConditionalErrorNonScalarCondition,
		ConditionalErrorIncompatibleTemporalModes:
		return true
	default:
		return false
	}
}

func (k ConditionalErrorKind) String() string {
	switch k {
	case ConditionalErrorNonBooleanCondition:
		return "NonBooleanCondition"
	case ConditionalErrorIncompatibleBranches:
		return "IncompatibleBranches"
	case ConditionalErrorInvalidOperand:
		return "InvalidOperand"
	case ConditionalErrorNonScalarCondition:
		return "NonScalarCondition"
	case ConditionalErrorIncompatibleTemporalModes:
		return "IncompatibleTemporalModes"
	default:
		return "Invalid"
	}
}

type ConditionalError struct {
	kind  ConditionalErrorKind
	cause error
}

func (e ConditionalError) Kind() ConditionalErrorKind {
	return e.kind
}

func (e ConditionalError) Unwrap() error {
	return e.cause
}

func (e ConditionalError) Error() string {
	switch e.kind {
	case ConditionalErrorNonBooleanCondition:
		return "typecheck: IF condition must be boolean"
	case ConditionalErrorIncompatibleBranches:
		return "typecheck: conditional branches cannot be unified"
	case ConditionalErrorInvalidOperand:
		return "typecheck: conditional operand is invalid"
	case ConditionalErrorNonScalarCondition:
		return "typecheck: IF condition must be scalar"
	case ConditionalErrorIncompatibleTemporalModes:
		return "typecheck: conditional operands have incompatible temporal modes"
	default:
		return "typecheck: invalid conditional error"
	}
}

// InferIf determines the static result type of an IF expression.
//
// IF condition is a Bool. A Scalar condition chooses one whole branch. A
// Series condition selects between two Series branches sample-by-sample.
//
// Branches must have compatible ValueType, Unit and TemporalMode. A Series
// condition with Scalar branches is rejected because it would require an
// implicit Scalar-to-Series broadcast.
//
// Result nullability is determined by the possible branch values. Nullable
// condition is not automatically converted into a nullable result type;
// condition-state handling belongs to evaluation semantics.
func InferIf(
	condition typing.Type,
	thenType typing.Type,
	elseType typing.Type,
) (typing.Type, error) {
	if !isBoolean(condition) {
		return typing.Type{}, ConditionalError{
			kind: ConditionalErrorNonBooleanCondition,
		}
	}

	if condition.TemporalMode() == typing.TemporalSeries &&
		thenType.TemporalMode() != typing.TemporalSeries {
		return typing.Type{}, ConditionalError{
			kind: ConditionalErrorIncompatibleTemporalModes,
		}
	}

	if thenType.TemporalMode() != elseType.TemporalMode() {
		return typing.Type{}, ConditionalError{
			kind: ConditionalErrorIncompatibleTemporalModes,
		}
	}

	result, err := Unify(
		thenType,
		elseType,
	)
	if err != nil {
		return typing.Type{}, ConditionalError{
			kind:  ConditionalErrorIncompatibleBranches,
			cause: err,
		}
	}

	return result, nil
}

// InferCoalesce determines the static result type of COALESCE.
//
// COALESCE uses fallback only for Null.
//
// Therefore:
//
//	COALESCE(nullable A, non-null B) -> non-null Result
//	COALESCE(nullable A, nullable B) -> nullable Result
//
// If primary is statically non-nullable, fallback cannot be selected because
// of Null and the result is also non-nullable.
//
// Both operands must have the same temporal mode in Core v1.
func InferCoalesce(
	primary typing.Type,
	fallback typing.Type,
) (typing.Type, error) {
	if !primary.IsValid() || !fallback.IsValid() {
		return typing.Type{}, ConditionalError{
			kind: ConditionalErrorInvalidOperand,
		}
	}

	if primary.TemporalMode() != fallback.TemporalMode() {
		return typing.Type{}, ConditionalError{
			kind: ConditionalErrorIncompatibleTemporalModes,
		}
	}

	result, err := Unify(
		primary,
		fallback,
	)
	if err != nil {
		return typing.Type{}, ConditionalError{
			kind:  ConditionalErrorIncompatibleBranches,
			cause: err,
		}
	}

	return result.
		WithTemporalMode(primary.TemporalMode()).
		WithNullable(
			primary.Nullable() &&
				fallback.Nullable(),
		), nil
}

// InferIsNull determines the static result type of IS NULL.
//
// IS NULL explicitly observes the Null state and therefore always produces a
// non-nullable Bool.
//
// Temporal shape is preserved: testing a Series value for Null produces a
// Series<Bool>. Scalar-only restrictions belong to places that require one
// predicate, such as IF condition and aggregate filter.
func InferIsNull(
	operand typing.Type,
) (typing.Type, error) {
	if !operand.IsValid() {
		return typing.Type{}, ConditionalError{
			kind: ConditionalErrorInvalidOperand,
		}
	}

	return typing.New(value.Bool).
		WithTemporalMode(operand.TemporalMode()).
		WithNullable(false), nil
}
