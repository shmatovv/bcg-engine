package typecheck

import (
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// UnifyErrorKind identifies why two static types cannot be unified.
type UnifyErrorKind uint8

const (
	UnifyErrorInvalid UnifyErrorKind = iota
	UnifyErrorIncompatibleTypes
	UnifyErrorIncompatibleUnits
	UnifyErrorIncompatibleTemporalModes
)

// IsValid reports whether k is a valid unify error kind.
func (k UnifyErrorKind) IsValid() bool {
	switch k {
	case UnifyErrorIncompatibleTypes,
		UnifyErrorIncompatibleUnits,
		UnifyErrorIncompatibleTemporalModes:
		return true
	default:
		return false
	}
}

// String returns the canonical name of k.
func (k UnifyErrorKind) String() string {
	switch k {
	case UnifyErrorIncompatibleTypes:
		return "IncompatibleTypes"
	case UnifyErrorIncompatibleUnits:
		return "IncompatibleUnits"
	case UnifyErrorIncompatibleTemporalModes:
		return "IncompatibleTemporalModes"
	default:
		return "Invalid"
	}
}

// UnifyError describes a static type unification failure.
type UnifyError struct {
	kind UnifyErrorKind
}

// Kind returns the unification error kind.
func (e UnifyError) Kind() UnifyErrorKind {
	return e.kind
}

// Error implements error.
func (e UnifyError) Error() string {
	switch e.kind {
	case UnifyErrorIncompatibleTypes:
		return "typecheck: types cannot be unified"
	case UnifyErrorIncompatibleUnits:
		return "typecheck: types have incompatible units"
	case UnifyErrorIncompatibleTemporalModes:
		return "typecheck: types have incompatible temporal modes"
	default:
		return "typecheck: invalid unification error"
	}
}

// Unify determines one common static type capable of representing a value
// produced by either left or right.
//
// Unification operates on the complete static type:
//
//   - ValueType;
//   - Unit;
//   - TemporalMode;
//   - Nullability.
//
// Temporal modes must match exactly. Core v1 performs no implicit
// Scalar <-> Series conversion.
//
// Result nullability is the union of possible branch values:
//
//	nullable(result) = nullable(left) OR nullable(right)
//
// Numeric types support promotion:
//
//	Integer + Integer -> Integer
//	Integer + Decimal -> Decimal
//	Decimal + Integer -> Decimal
//	Decimal + Decimal -> Decimal
//
// Compatible numeric Units are unified using the left operand's Unit:
//
//	Integer<kg> + Decimal<g> -> Decimal<kg>
//
// Integer operands with different convertible Units are widened to Decimal.
//
// This is required because normalizing the right value into the left Unit may
// produce a fractional result:
//
//	Integer<meter> + Integer<centimeter>
//	    -> Decimal<meter>
//
// Unit compatibility requires context-free automatic convertibility.
// Equal Dimension alone is intentionally insufficient.
//
// Therefore:
//
//	Integer<RUB> + Decimal<kopeck>
//	    -> Decimal<RUB>
//
// but:
//
//	Integer<RUB> + Decimal<EUR>
//	    -> error
//
// Choosing the left Unit is deterministic when Unify is applied to an
// ordered AST construct such as IF or COALESCE. Model-wide contract
// unification must define its own canonical representation and must not
// depend on unordered iteration.
func Unify(
	left typing.Type,
	right typing.Type,
) (typing.Type, error) {
	if !left.IsValid() || !right.IsValid() {
		return typing.Type{}, UnifyError{
			kind: UnifyErrorIncompatibleTypes,
		}
	}

	if left.TemporalMode() != right.TemporalMode() {
		return typing.Type{}, UnifyError{
			kind: UnifyErrorIncompatibleTemporalModes,
		}
	}

	leftValueType := left.ValueType()
	rightValueType := right.ValueType()

	var result typing.Type
	var err error

	if IsNumeric(leftValueType) &&
		IsNumeric(rightValueType) {
		result, err = unifyNumeric(left, right)
		if err != nil {
			return typing.Type{}, err
		}
	} else {
		result, err = unifyNonNumeric(left, right)
		if err != nil {
			return typing.Type{}, err
		}
	}

	return result.
		WithTemporalMode(left.TemporalMode()).
		WithNullable(
			left.Nullable() || right.Nullable(),
		), nil
}

func unifyNumeric(
	left typing.Type,
	right typing.Type,
) (typing.Type, error) {
	resultValueType, ok := PromoteAdditive(
		left.ValueType(),
		right.ValueType(),
	)
	if !ok {
		return typing.Type{}, UnifyError{
			kind: UnifyErrorIncompatibleTypes,
		}
	}

	leftUnit, leftHasUnit := left.Unit()
	rightUnit, rightHasUnit := right.Unit()

	if !leftHasUnit && !rightHasUnit {
		return typing.New(resultValueType), nil
	}

	if leftHasUnit != rightHasUnit {
		return typing.Type{}, UnifyError{
			kind: UnifyErrorIncompatibleUnits,
		}
	}

	if !leftUnit.CanConvertTo(rightUnit) {
		return typing.Type{}, UnifyError{
			kind: UnifyErrorIncompatibleUnits,
		}
	}

	if resultValueType == value.Integer &&
		!leftUnit.Equal(rightUnit) {
		resultValueType = value.Decimal
	}

	return typing.NewWithUnit(
		resultValueType,
		leftUnit,
	), nil
}

func unifyNonNumeric(
	left typing.Type,
	right typing.Type,
) (typing.Type, error) {
	if left.ValueType() != right.ValueType() {
		return typing.Type{}, UnifyError{
			kind: UnifyErrorIncompatibleTypes,
		}
	}

	if left.HasUnit() || right.HasUnit() {
		return typing.Type{}, UnifyError{
			kind: UnifyErrorIncompatibleUnits,
		}
	}

	switch left.ValueType() {
	case value.Bool,
		value.String,
		value.Date,
		value.DateTime,
		value.Duration:
		return typing.New(left.ValueType()), nil

	default:
		return typing.Type{}, UnifyError{
			kind: UnifyErrorIncompatibleTypes,
		}
	}
}
