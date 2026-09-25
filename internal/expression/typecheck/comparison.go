package typecheck

import (
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

type ComparisonErrorKind uint8

const (
	ComparisonErrorInvalid ComparisonErrorKind = iota
	ComparisonErrorInvalidOperator
	ComparisonErrorIncompatibleOperands
	ComparisonErrorIncompatibleUnits
	ComparisonErrorIncompatibleTemporalModes
)

func (k ComparisonErrorKind) IsValid() bool {
	switch k {
	case ComparisonErrorInvalidOperator,
		ComparisonErrorIncompatibleOperands,
		ComparisonErrorIncompatibleUnits,
		ComparisonErrorIncompatibleTemporalModes:
		return true
	default:
		return false
	}
}

func (k ComparisonErrorKind) String() string {
	switch k {
	case ComparisonErrorInvalidOperator:
		return "InvalidOperator"
	case ComparisonErrorIncompatibleOperands:
		return "IncompatibleOperands"
	case ComparisonErrorIncompatibleUnits:
		return "IncompatibleUnits"
	case ComparisonErrorIncompatibleTemporalModes:
		return "IncompatibleTemporalModes"
	default:
		return "Invalid"
	}
}

type ComparisonError struct {
	kind ComparisonErrorKind
}

func (e ComparisonError) Kind() ComparisonErrorKind {
	return e.kind
}

func (e ComparisonError) Error() string {
	switch e.kind {
	case ComparisonErrorInvalidOperator:
		return "typecheck: invalid comparison operator"
	case ComparisonErrorIncompatibleOperands:
		return "typecheck: incompatible comparison operands"
	case ComparisonErrorIncompatibleUnits:
		return "typecheck: comparison operands have incompatible units"
	case ComparisonErrorIncompatibleTemporalModes:
		return "typecheck: comparison operands have incompatible temporal modes"
	default:
		return "typecheck: invalid comparison error"
	}
}

// InferComparison determines the static result type of a comparison.
//
// Comparison is Null-strict:
//
//	Null == 5 -> Null
//
// Therefore comparison result is nullable whenever either operand is
// nullable.
//
// Core v1 requires equal temporal modes and performs no implicit temporal
// conversion.
func InferComparison(
	operator ast.ComparisonOperator,
	left typing.Type,
	right typing.Type,
) (typing.Type, error) {
	if !operator.IsValid() {
		return typing.Type{}, ComparisonError{
			kind: ComparisonErrorInvalidOperator,
		}
	}

	if !left.IsValid() || !right.IsValid() {
		return typing.Type{}, ComparisonError{
			kind: ComparisonErrorIncompatibleOperands,
		}
	}

	if left.TemporalMode() != right.TemporalMode() {
		return typing.Type{}, ComparisonError{
			kind: ComparisonErrorIncompatibleTemporalModes,
		}
	}

	if IsNumeric(left.ValueType()) &&
		IsNumeric(right.ValueType()) {

		if !comparisonUnitsCompatible(left, right) {
			return typing.Type{}, ComparisonError{
				kind: ComparisonErrorIncompatibleUnits,
			}
		}

		return comparisonResult(left, right)
	}

	switch operator {
	case ast.ComparisonEqual,
		ast.ComparisonNotEqual:
		if !equalityComparable(
			left.ValueType(),
			right.ValueType(),
		) {
			return typing.Type{}, ComparisonError{
				kind: ComparisonErrorIncompatibleOperands,
			}
		}
		if left.ValueType() == value.Enum {
			leftID, leftOK := left.EnumDefinitionID()
			rightID, rightOK := right.EnumDefinitionID()
			if !leftOK || !rightOK || leftID != rightID {
				return typing.Type{}, ComparisonError{kind: ComparisonErrorIncompatibleOperands}
			}
		}

	case ast.ComparisonLess,
		ast.ComparisonLessOrEqual,
		ast.ComparisonGreater,
		ast.ComparisonGreaterOrEqual:
		if !orderedComparable(
			left.ValueType(),
			right.ValueType(),
		) {
			return typing.Type{}, ComparisonError{
				kind: ComparisonErrorIncompatibleOperands,
			}
		}

	default:
		return typing.Type{}, ComparisonError{
			kind: ComparisonErrorInvalidOperator,
		}
	}

	if left.HasUnit() || right.HasUnit() {
		return typing.Type{}, ComparisonError{
			kind: ComparisonErrorIncompatibleUnits,
		}
	}

	return comparisonResult(left, right)
}

func comparisonResult(
	left typing.Type,
	right typing.Type,
) (typing.Type, error) {
	result := typing.New(value.Bool)

	qualified, err := propagateStrictBinaryQualifiers(
		result,
		left,
		right,
	)
	if err != nil {
		return typing.Type{}, ComparisonError{
			kind: ComparisonErrorIncompatibleTemporalModes,
		}
	}

	return qualified, nil
}

func comparisonUnitsCompatible(
	left typing.Type,
	right typing.Type,
) bool {
	leftUnit, leftHasUnit := left.Unit()
	rightUnit, rightHasUnit := right.Unit()

	if !leftHasUnit && !rightHasUnit {
		return true
	}

	if leftHasUnit != rightHasUnit {
		return false
	}

	return leftUnit.CanConvertTo(rightUnit)
}

func equalityComparable(
	left value.Type,
	right value.Type,
) bool {
	if left != right {
		return false
	}

	switch left {
	case value.Bool,
		value.String,
		value.Date,
		value.DateTime,
		value.Duration:
		return true
	default:
		return false
	}
}

func orderedComparable(
	left value.Type,
	right value.Type,
) bool {
	if left != right {
		return false
	}

	switch left {
	case value.Date,
		value.DateTime,
		value.Duration:
		return true
	default:
		return false
	}
}
