package evaluator

import (
	"math/big"

	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	enginevalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

func (s *evaluationState) evaluateArithmetic(
	operator ast.ArithmeticOperator,
	left enginevalue.Resolved,
	right enginevalue.Resolved,
) (enginevalue.Resolved, error) {
	if left.IsUndefined() ||
		right.IsUndefined() {

		return dependencyUndefined(),
			nil
	}

	if left.IsNull() ||
		right.IsNull() {

		return enginevalue.Null(),
			nil
	}

	leftValue :=
		left.Value()

	rightValue :=
		right.Value()

	if !typecheck.IsNumeric(
		leftValue.Type(),
	) ||
		!typecheck.IsNumeric(
			rightValue.Type(),
		) {

		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"committed arithmetic expression produced non-numeric operand",
				nil,
			)
	}

	switch operator {
	case ast.ArithmeticAdd,
		ast.ArithmeticSubtract:

		return s.evaluateAdditive(
			operator,
			leftValue,
			rightValue,
		)

	case ast.ArithmeticMultiply:
		return s.evaluateMultiply(
			leftValue,
			rightValue,
		)

	case ast.ArithmeticDivide:
		return s.evaluateDivide(
			leftValue,
			rightValue,
		)

	default:
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"committed arithmetic expression has invalid operator",
				nil,
			)
	}
}

func (s *evaluationState) evaluateAdditive(
	operator ast.ArithmeticOperator,
	left enginevalue.Typed,
	right enginevalue.Typed,
) (enginevalue.Resolved, error) {
	leftUnit,
		leftHasUnit :=
		optionalUnit(
			left,
		)

	rightUnit,
		rightHasUnit :=
		optionalUnit(
			right,
		)

	if leftHasUnit !=
		rightHasUnit {

		return enginevalue.Resolved{},
			numericInvariantError(
				"additive operands disagree on unit presence",
			)
	}

	resultHasUnit :=
		leftHasUnit

	resultUnit :=
		leftUnit

	leftType :=
		left.Type()

	rightType :=
		right.Type()

	resultType, ok :=
		typecheck.PromoteAdditive(
			leftType,
			rightType,
		)

	if !ok {
		return enginevalue.Resolved{},
			numericInvariantError(
				"additive operands have invalid numeric types",
			)
	}

	if resultHasUnit {
		if !leftUnit.
			CanConvertTo(
				rightUnit,
			) {

			return enginevalue.Resolved{},
				numericInvariantError(
					"additive operand units are not automatically convertible",
				)
		}

		if resultType ==
			enginevalue.Integer &&
			!leftUnit.Equal(
				rightUnit,
			) {

			resultType =
				enginevalue.Decimal
		}
	}

	if resultType ==
		enginevalue.Integer {

		leftInteger :=
			left.
				Raw().
				Integer()

		rightInteger :=
			right.
				Raw().
				Integer()

		var result big.Int

		if operator ==
			ast.ArithmeticAdd {

			result.Add(
				leftInteger,
				rightInteger,
			)
		} else {
			result.Sub(
				leftInteger,
				rightInteger,
			)
		}

		return enginevalue.Defined(
			newNumericTyped(
				enginevalue.NewInteger(
					&result,
				),
				resultUnit,
				resultHasUnit,
			),
		), nil
	}

	leftDecimal :=
		numericDecimal(
			left,
		)

	rightDecimal :=
		numericDecimal(
			right,
		)

	if resultHasUnit &&
		!leftUnit.Equal(
			rightUnit,
		) {

		converted, err :=
			unit.Convert(
				rightDecimal,
				rightUnit,
				leftUnit,
				s.version.
					PrecisionPolicy(),
			)

		if err != nil {
			return enginevalue.Resolved{},
				numericInvariantError(
					"validated additive unit conversion failed: " +
						err.Error(),
				)
		}

		rightDecimal =
			converted
	}

	var result decimal.Decimal

	if operator ==
		ast.ArithmeticAdd {

		result =
			leftDecimal.Add(
				rightDecimal,
			)
	} else {
		result =
			leftDecimal.Sub(
				rightDecimal,
			)
	}

	return enginevalue.Defined(
		newNumericTyped(
			enginevalue.NewDecimal(
				result,
			),
			resultUnit,
			resultHasUnit,
		),
	), nil
}

func (s *evaluationState) evaluateMultiply(
	left enginevalue.Typed,
	right enginevalue.Typed,
) (enginevalue.Resolved, error) {
	resultUnit,
		resultHasUnit,
		ok :=
		typecheck.MultiplicativeUnit(
			unitOrZero(left),
			left.HasUnit(),
			unitOrZero(right),
			right.HasUnit(),
		)

	if !ok {
		return enginevalue.Resolved{},
			numericInvariantError(
				"validated multiplicative unit inference failed",
			)
	}

	resultType, ok :=
		typecheck.PromoteMultiplicative(
			left.Type(),
			right.Type(),
		)

	if !ok {
		return enginevalue.Resolved{},
			numericInvariantError(
				"multiplicative operands have invalid numeric types",
			)
	}

	if resultType ==
		enginevalue.Integer {

		var result big.Int

		result.Mul(
			left.
				Raw().
				Integer(),
			right.
				Raw().
				Integer(),
		)

		return enginevalue.Defined(
			newNumericTyped(
				enginevalue.NewInteger(
					&result,
				),
				resultUnit,
				resultHasUnit,
			),
		), nil
	}

	result, err :=
		numericDecimal(
			left,
		).Mul(
			numericDecimal(
				right,
			),
		)

	if err != nil {
		return undefined(
			enginevalue.
				UndefinedInvalidOperation,
		), nil
	}

	return enginevalue.Defined(
		newNumericTyped(
			enginevalue.NewDecimal(
				result,
			),
			resultUnit,
			resultHasUnit,
		),
	), nil
}

func (s *evaluationState) evaluateDivide(
	left enginevalue.Typed,
	right enginevalue.Typed,
) (enginevalue.Resolved, error) {
	resultUnit,
		resultHasUnit,
		ok :=
		typecheck.DivisionUnit(
			unitOrZero(left),
			left.HasUnit(),
			unitOrZero(right),
			right.HasUnit(),
		)

	if !ok {
		return enginevalue.Resolved{},
			numericInvariantError(
				"validated division unit inference failed",
			)
	}

	denominator :=
		numericDecimal(
			right,
		)

	if denominator.IsZero() {
		return undefined(
			enginevalue.
				UndefinedDivisionByZero,
		), nil
	}

	result, err :=
		numericDecimal(
			left,
		).Div(
			denominator,
			s.version.
				PrecisionPolicy(),
		)

	if err != nil {
		return undefined(
			enginevalue.
				UndefinedInvalidOperation,
		), nil
	}

	return enginevalue.Defined(
		newNumericTyped(
			enginevalue.NewDecimal(
				result,
			),
			resultUnit,
			resultHasUnit,
		),
	), nil
}

func evaluateNegate(
	operand enginevalue.Resolved,
) (enginevalue.Resolved, error) {
	if operand.IsUndefined() {
		return dependencyUndefined(),
			nil
	}

	if operand.IsNull() {
		return enginevalue.Null(),
			nil
	}

	typed :=
		operand.Value()

	var raw enginevalue.Raw

	switch typed.Type() {
	case enginevalue.Integer:
		integer :=
			typed.
				Raw().
				Integer()

		integer.Neg(
			integer,
		)

		raw =
			enginevalue.NewInteger(
				integer,
			)

	case enginevalue.Decimal:
		raw =
			enginevalue.NewDecimal(
				typed.
					Raw().
					Decimal().
					Neg(),
			)

	default:
		return enginevalue.Resolved{},
			numericInvariantError(
				"validated Negate operand is not numeric",
			)
	}

	return enginevalue.Defined(
		newNumericTyped(
			raw,
			unitOrZero(
				typed,
			),
			typed.HasUnit(),
		),
	), nil
}

func evaluateComparison(
	operator ast.ComparisonOperator,
	left enginevalue.Resolved,
	right enginevalue.Resolved,
) (enginevalue.Resolved, error) {
	if left.IsUndefined() ||
		right.IsUndefined() {

		return dependencyUndefined(),
			nil
	}

	if left.IsNull() ||
		right.IsNull() {

		return enginevalue.Null(),
			nil
	}

	leftValue :=
		left.Value()

	rightValue :=
		right.Value()

	comparison, err :=
		compareDefined(
			leftValue,
			rightValue,
		)

	if err != nil {
		return enginevalue.Resolved{},
			err
	}

	var result bool

	switch operator {
	case ast.ComparisonEqual:
		result =
			comparison == 0

	case ast.ComparisonNotEqual:
		result =
			comparison != 0

	case ast.ComparisonLess:
		result =
			comparison < 0

	case ast.ComparisonLessOrEqual:
		result =
			comparison <= 0

	case ast.ComparisonGreater:
		result =
			comparison > 0

	case ast.ComparisonGreaterOrEqual:
		result =
			comparison >= 0

	default:
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"committed comparison expression has invalid operator",
				nil,
			)
	}

	return boolResolved(
		result,
	), nil
}

func compareDefined(
	left enginevalue.Typed,
	right enginevalue.Typed,
) (int, error) {
	if typecheck.IsNumeric(
		left.Type(),
	) &&
		typecheck.IsNumeric(
			right.Type(),
		) {

		return compareNumeric(
			left,
			right,
		)
	}

	if left.HasUnit() ||
		right.HasUnit() ||
		left.Type() !=
			right.Type() {

		return 0,
			newError(
				ErrorInvariantViolation,
				"validated comparison operands are incompatible",
				nil,
			)
	}

	switch left.Type() {
	case enginevalue.Bool:
		leftBool :=
			left.
				Raw().
				Bool()

		rightBool :=
			right.
				Raw().
				Bool()

		switch {
		case leftBool ==
			rightBool:

			return 0, nil

		case !leftBool &&
			rightBool:

			return -1, nil

		default:
			return 1, nil
		}

	case enginevalue.String:
		leftString :=
			left.
				Raw().
				StringValue()

		rightString :=
			right.
				Raw().
				StringValue()

		switch {
		case leftString <
			rightString:

			return -1, nil

		case leftString >
			rightString:

			return 1, nil

		default:
			return 0, nil
		}

	case enginevalue.Date:
		return left.
			Raw().
			Date().
			Cmp(
				right.
					Raw().
					Date(),
			), nil

	case enginevalue.DateTime:
		return left.
			Raw().
			DateTime().
			Cmp(
				right.
					Raw().
					DateTime(),
			), nil

	case enginevalue.Duration:
		return left.
			Raw().
			Duration().
			Cmp(
				right.
					Raw().
					Duration(),
			), nil

	default:
		return 0,
			newError(
				ErrorInvariantViolation,
				"validated comparison uses unsupported operand type",
				nil,
			)
	}
}

func compareNumeric(
	left enginevalue.Typed,
	right enginevalue.Typed,
) (int, error) {
	if left.HasUnit() !=
		right.HasUnit() {

		return 0,
			numericInvariantError(
				"numeric comparison operands disagree on unit presence",
			)
	}

	leftRational :=
		rational.FromDecimal(
			numericDecimal(
				left,
			),
		)

	rightRational :=
		rational.FromDecimal(
			numericDecimal(
				right,
			),
		)

	if left.HasUnit() {
		leftUnit :=
			left.Unit()

		rightUnit :=
			right.Unit()

		if !leftUnit.
			CanConvertTo(
				rightUnit,
			) {

			return 0,
				numericInvariantError(
					"numeric comparison units are not automatically convertible",
				)
		}

		leftRational =
			leftRational.Mul(
				leftUnit.Scale(),
			)

		rightRational =
			rightRational.Mul(
				rightUnit.Scale(),
			)
	}

	return leftRational.Cmp(
		rightRational,
	), nil
}

func logicalShortCircuit(
	operator ast.LogicalOperator,
	left enginevalue.Resolved,
) (
	enginevalue.Resolved,
	bool,
	error,
) {
	if !left.IsDefined() {
		return enginevalue.Resolved{},
			false,
			nil
	}

	typed :=
		left.Value()

	if typed.Type() !=
		enginevalue.Bool ||
		typed.HasUnit() {

		return enginevalue.Resolved{},
			false,
			newError(
				ErrorInvariantViolation,
				"validated logical operand is not unitless Bool",
				nil,
			)
	}

	boolean :=
		typed.
			Raw().
			Bool()

	switch operator {
	case ast.LogicalAnd:
		if !boolean {
			return boolResolved(
				false,
			), true, nil
		}

	case ast.LogicalOr:
		if boolean {
			return boolResolved(
				true,
			), true, nil
		}

	default:
		return enginevalue.Resolved{},
			false,
			newError(
				ErrorInvariantViolation,
				"committed logical expression has invalid operator",
				nil,
			)
	}

	return enginevalue.Resolved{},
		false,
		nil
}

func evaluateLogicalPair(
	operator ast.LogicalOperator,
	left enginevalue.Resolved,
	right enginevalue.Resolved,
) (enginevalue.Resolved, error) {
	leftState, err :=
		logicalState(
			left,
		)

	if err != nil {
		return enginevalue.Resolved{},
			err
	}

	rightState, err :=
		logicalState(
			right,
		)

	if err != nil {
		return enginevalue.Resolved{},
			err
	}

	switch operator {
	case ast.LogicalAnd:
		if leftState ==
			logicalFalse ||
			rightState ==
				logicalFalse {

			return boolResolved(
				false,
			), nil
		}

		if leftState ==
			logicalUndefined ||
			rightState ==
				logicalUndefined {

			return dependencyUndefined(),
				nil
		}

		if leftState ==
			logicalNull ||
			rightState ==
				logicalNull {

			return enginevalue.Null(),
				nil
		}

		return boolResolved(
			true,
		), nil

	case ast.LogicalOr:
		if leftState ==
			logicalTrue ||
			rightState ==
				logicalTrue {

			return boolResolved(
				true,
			), nil
		}

		if leftState ==
			logicalUndefined ||
			rightState ==
				logicalUndefined {

			return dependencyUndefined(),
				nil
		}

		if leftState ==
			logicalNull ||
			rightState ==
				logicalNull {

			return enginevalue.Null(),
				nil
		}

		return boolResolved(
			false,
		), nil

	default:
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"committed logical expression has invalid operator",
				nil,
			)
	}
}

type logicalValueState uint8

const (
	logicalInvalid logicalValueState = iota
	logicalFalse
	logicalTrue
	logicalNull
	logicalUndefined
)

func logicalState(
	resolved enginevalue.Resolved,
) (logicalValueState, error) {
	switch resolved.Kind() {
	case enginevalue.ResolvedNull:
		return logicalNull,
			nil

	case enginevalue.ResolvedUndefined:
		return logicalUndefined,
			nil

	case enginevalue.ResolvedDefined:
		typed :=
			resolved.Value()

		if typed.Type() !=
			enginevalue.Bool ||
			typed.HasUnit() {

			return logicalInvalid,
				newError(
					ErrorInvariantViolation,
					"validated logical operand is not unitless Bool",
					nil,
				)
		}

		if typed.
			Raw().
			Bool() {

			return logicalTrue,
				nil
		}

		return logicalFalse,
			nil

	default:
		return logicalInvalid,
			newError(
				ErrorInvariantViolation,
				"logical operation received invalid resolved value",
				nil,
			)
	}
}

func evaluateNot(
	operand enginevalue.Resolved,
) (enginevalue.Resolved, error) {
	if operand.IsUndefined() {
		return dependencyUndefined(),
			nil
	}

	if operand.IsNull() {
		return enginevalue.Null(),
			nil
	}

	typed :=
		operand.Value()

	if typed.Type() !=
		enginevalue.Bool ||
		typed.HasUnit() {

		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"validated NOT operand is not unitless Bool",
				nil,
			)
	}

	return boolResolved(
		!typed.
			Raw().
			Bool(),
	), nil
}

func evaluateIsNull(
	operand enginevalue.Resolved,
) enginevalue.Resolved {
	if operand.IsUndefined() {
		return dependencyUndefined()
	}

	return boolResolved(
		operand.IsNull(),
	)
}

func boolResolved(
	boolean bool,
) enginevalue.Resolved {
	return enginevalue.Defined(
		enginevalue.NewTyped(
			enginevalue.NewBool(
				boolean,
			),
		),
	)
}

func numericDecimal(
	typed enginevalue.Typed,
) decimal.Decimal {
	switch typed.Type() {
	case enginevalue.Integer:
		return decimal.New(
			typed.
				Raw().
				Integer(),
			0,
		)

	case enginevalue.Decimal:
		return typed.
			Raw().
			Decimal()

	default:
		panic(
			"evaluator: numericDecimal called for non-numeric value",
		)
	}
}

func newNumericTyped(
	raw enginevalue.Raw,
	u unit.Unit,
	hasUnit bool,
) enginevalue.Typed {
	if hasUnit {
		return enginevalue.NewTypedWithUnit(
			raw,
			u,
		)
	}

	return enginevalue.NewTyped(
		raw,
	)
}

func optionalUnit(
	typed enginevalue.Typed,
) (unit.Unit, bool) {
	if !typed.HasUnit() {
		return unit.Unit{},
			false
	}

	return typed.Unit(),
		true
}

func unitOrZero(
	typed enginevalue.Typed,
) unit.Unit {
	if !typed.HasUnit() {
		return unit.Unit{}
	}

	return typed.Unit()
}

func numericInvariantError(
	message string,
) error {
	return newError(
		ErrorInvariantViolation,
		message,
		nil,
	)
}
