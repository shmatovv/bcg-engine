package typecheck

import (
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// ArithmeticErrorKind identifies why arithmetic type inference failed.
type ArithmeticErrorKind uint8

const (
	ArithmeticErrorInvalid ArithmeticErrorKind = iota
	ArithmeticErrorInvalidOperator
	ArithmeticErrorNonNumericOperand
	ArithmeticErrorIncompatibleUnits
	ArithmeticErrorIncompatibleTemporalModes
)

func (k ArithmeticErrorKind) IsValid() bool {
	switch k {
	case ArithmeticErrorInvalidOperator,
		ArithmeticErrorNonNumericOperand,
		ArithmeticErrorIncompatibleUnits,
		ArithmeticErrorIncompatibleTemporalModes:
		return true
	default:
		return false
	}
}

func (k ArithmeticErrorKind) String() string {
	switch k {
	case ArithmeticErrorInvalidOperator:
		return "InvalidOperator"
	case ArithmeticErrorNonNumericOperand:
		return "NonNumericOperand"
	case ArithmeticErrorIncompatibleUnits:
		return "IncompatibleUnits"
	case ArithmeticErrorIncompatibleTemporalModes:
		return "IncompatibleTemporalModes"
	default:
		return "Invalid"
	}
}

type ArithmeticError struct {
	kind ArithmeticErrorKind
}

func (e ArithmeticError) Kind() ArithmeticErrorKind {
	return e.kind
}

func (e ArithmeticError) Error() string {
	switch e.kind {
	case ArithmeticErrorInvalidOperator:
		return "typecheck: invalid arithmetic operator"
	case ArithmeticErrorNonNumericOperand:
		return "typecheck: arithmetic operands must be numeric"
	case ArithmeticErrorIncompatibleUnits:
		return "typecheck: arithmetic operands have incompatible units"
	case ArithmeticErrorIncompatibleTemporalModes:
		return "typecheck: arithmetic operands have incompatible temporal modes"
	default:
		return "typecheck: invalid arithmetic error"
	}
}

// InferArithmetic determines the static result type of a binary arithmetic
// operation.
//
// Arithmetic is strict with respect to Null:
//
//	Nullable<A> op B -> Nullable<Result>
//
// Core v1 performs no implicit temporal conversion:
//
//	Scalar op Scalar -> Scalar
//	Series op Series -> Series
//	Scalar op Series -> error
//
// ValueType promotion and Unit algebra are independent from these qualifiers.
func InferArithmetic(
	operator ast.ArithmeticOperator,
	left typing.Type,
	right typing.Type,
) (typing.Type, error) {
	if !operator.IsValid() {
		return typing.Type{}, ArithmeticError{
			kind: ArithmeticErrorInvalidOperator,
		}
	}

	if !left.IsValid() || !right.IsValid() {
		return typing.Type{}, ArithmeticError{
			kind: ArithmeticErrorNonNumericOperand,
		}
	}

	if left.TemporalMode() != right.TemporalMode() {
		return typing.Type{}, ArithmeticError{
			kind: ArithmeticErrorIncompatibleTemporalModes,
		}
	}

	leftValueType := left.ValueType()
	rightValueType := right.ValueType()

	if !IsNumeric(leftValueType) || !IsNumeric(rightValueType) {
		return typing.Type{}, ArithmeticError{
			kind: ArithmeticErrorNonNumericOperand,
		}
	}

	resultValueType, ok := inferArithmeticValueType(
		operator,
		leftValueType,
		rightValueType,
	)
	if !ok {
		return typing.Type{}, ArithmeticError{
			kind: ArithmeticErrorInvalidOperator,
		}
	}

	resultUnit, hasResultUnit, ok := inferArithmeticUnit(
		operator,
		left,
		right,
	)
	if !ok {
		return typing.Type{}, ArithmeticError{
			kind: ArithmeticErrorIncompatibleUnits,
		}
	}

	resultValueType = promoteAdditiveUnitConversionResult(
		operator,
		resultValueType,
		left,
		right,
	)

	var result typing.Type

	if hasResultUnit {
		result = typing.NewWithUnit(
			resultValueType,
			resultUnit,
		)
	} else {
		result = typing.New(resultValueType)
	}

	qualified, err := propagateStrictBinaryQualifiers(
		result,
		left,
		right,
	)
	if err != nil {
		return typing.Type{}, ArithmeticError{
			kind: ArithmeticErrorIncompatibleTemporalModes,
		}
	}

	return qualified, nil
}

func inferArithmeticValueType(
	operator ast.ArithmeticOperator,
	left value.Type,
	right value.Type,
) (value.Type, bool) {
	switch operator {
	case ast.ArithmeticAdd,
		ast.ArithmeticSubtract:
		return PromoteAdditive(left, right)

	case ast.ArithmeticMultiply:
		return PromoteMultiplicative(left, right)

	case ast.ArithmeticDivide:
		return PromoteDivision(left, right)

	default:
		return value.Invalid, false
	}
}

func inferArithmeticUnit(
	operator ast.ArithmeticOperator,
	left typing.Type,
	right typing.Type,
) (unit.Unit, bool, bool) {
	leftUnit, leftHasUnit := left.Unit()
	rightUnit, rightHasUnit := right.Unit()

	switch operator {
	case ast.ArithmeticAdd,
		ast.ArithmeticSubtract:
		return AdditiveUnit(
			leftUnit,
			leftHasUnit,
			rightUnit,
			rightHasUnit,
		)

	case ast.ArithmeticMultiply:
		return MultiplicativeUnit(
			leftUnit,
			leftHasUnit,
			rightUnit,
			rightHasUnit,
		)

	case ast.ArithmeticDivide:
		return DivisionUnit(
			leftUnit,
			leftHasUnit,
			rightUnit,
			rightHasUnit,
		)

	default:
		return unit.Unit{}, false, false
	}
}

// promoteAdditiveUnitConversionResult widens an Integer additive result to
// Decimal when evaluation must normalize one operand into another Unit.
//
// Without this promotion the static type could promise Integer even though
// an exact context-free conversion produces a fractional value:
//
//	1 meter + 50 centimeter = 1.5 meter
//
// Equal Units require no conversion and preserve Integer. Multiplication and
// division use their own numeric promotion rules and are unaffected here.
func promoteAdditiveUnitConversionResult(
	operator ast.ArithmeticOperator,
	resultValueType value.Type,
	left typing.Type,
	right typing.Type,
) value.Type {
	if operator != ast.ArithmeticAdd &&
		operator != ast.ArithmeticSubtract {
		return resultValueType
	}

	if resultValueType != value.Integer {
		return resultValueType
	}

	leftUnit, leftHasUnit := left.Unit()
	rightUnit, rightHasUnit := right.Unit()

	if !leftHasUnit || !rightHasUnit {
		return resultValueType
	}

	if leftUnit.Equal(rightUnit) {
		return resultValueType
	}

	return value.Decimal
}
