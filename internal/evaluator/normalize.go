package evaluator

import (
	"math/big"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/types/rational"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func (s *evaluationState) normalizeDerivedResult(
	resolved enginevalue.Resolved,
	definition schema.PropertyDefinition,
) (enginevalue.Resolved, error) {
	if !resolved.IsDefined() {
		return resolved, nil
	}

	typed :=
		resolved.Value()

	if typed.Type() !=
		definition.ValueType() {

		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"derived runtime value type does not match committed PropertyDefinition",
				nil,
			)
	}

	expectedUnit,
		expectedHasUnit :=
		definition.Unit()

	if typed.HasUnit() !=
		expectedHasUnit {

		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"derived runtime unit presence does not match committed PropertyDefinition",
				nil,
			)
	}

	if !expectedHasUnit ||
		typed.
			Unit().
			Equal(
				expectedUnit,
			) {

		return resolved, nil
	}

	if !typed.
		Unit().
		CanConvertTo(
			expectedUnit,
		) {

		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"derived runtime unit cannot be normalized to PropertyDefinition unit",
				nil,
			)
	}

	switch typed.Type() {
	case enginevalue.Decimal:
		converted, err :=
			unit.Convert(
				typed.
					Raw().
					Decimal(),
				typed.Unit(),
				expectedUnit,
				s.version.
					PrecisionPolicy(),
			)

		if err != nil {
			return enginevalue.Resolved{},
				newError(
					ErrorInvariantViolation,
					"validated derived unit normalization failed",
					err,
				)
		}

		return enginevalue.Defined(
			enginevalue.NewTypedWithUnit(
				enginevalue.NewDecimal(
					converted,
				),
				expectedUnit,
			),
		), nil

	case enginevalue.Integer:
		converted, err :=
			convertIntegerUnitExact(
				typed.
					Raw().
					Integer(),
				typed.Unit(),
				expectedUnit,
			)

		if err != nil {
			return enginevalue.Resolved{},
				err
		}

		return enginevalue.Defined(
			enginevalue.NewTypedWithUnit(
				enginevalue.NewInteger(
					converted,
				),
				expectedUnit,
			),
		), nil

	default:
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"non-numeric derived value unexpectedly requires unit normalization",
				nil,
			)
	}
}

func convertIntegerUnitExact(
	integer *big.Int,
	from unit.Unit,
	to unit.Unit,
) (*big.Int, error) {
	if integer == nil ||
		!from.IsValid() ||
		!to.IsValid() ||
		!from.CanConvertTo(to) {

		return nil,
			newError(
				ErrorInvariantViolation,
				"invalid exact Integer unit conversion",
				nil,
			)
	}

	factor, err :=
		from.
			Scale().
			Div(
				to.Scale(),
			)

	if err != nil {
		return nil,
			newError(
				ErrorInvariantViolation,
				"validated Integer unit scale division failed",
				err,
			)
	}

	converted :=
		rational.New(
			integer,
			big.NewInt(1),
		).Mul(
			factor,
		)

	if converted.
		Denominator().
		Cmp(
			big.NewInt(1),
		) != 0 {

		return nil,
			newError(
				ErrorInvariantViolation,
				"validated Integer unit normalization produced a fractional value",
				nil,
			)
	}

	return converted.Numerator(),
		nil
}
