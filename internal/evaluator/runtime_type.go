package evaluator

import (
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// normalizeResolvedToStaticType converts one already-evaluated sample value to
// the exact deterministic representation inferred statically for the
// expression that consumes it.
//
// This is required primarily by polymorphic BoundContract references where
// different concrete implementations may use automatically-convertible units.
func (s *evaluationState) normalizeResolvedToStaticType(
	resolved enginevalue.Resolved,
	expected typing.Type,
) (enginevalue.Resolved, error) {
	if !resolved.IsValid() || !expected.IsValid() {
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"cannot normalize invalid runtime/static value",
				nil,
			)
	}

	if expected.TemporalMode() != typing.TemporalScalar &&
		expected.TemporalMode() != typing.TemporalSeries {
		return enginevalue.Resolved{},
			newError(
				ErrorUnsupportedSeries,
				"runtime normalization received an invalid temporal expression type",
				nil,
			)
	}

	if !resolved.IsDefined() {
		return resolved, nil
	}

	actual := resolved.Value()

	expectedUnit,
		expectedHasUnit := expected.Unit()

	if actual.HasUnit() != expectedHasUnit {
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"runtime value unit presence disagrees with static expression type",
				nil,
			)
	}

	if actual.Type() == expected.ValueType() {
		if !expectedHasUnit ||
			actual.Unit().Equal(expectedUnit) {
			return resolved, nil
		}

		if !actual.Unit().CanConvertTo(expectedUnit) {
			return enginevalue.Resolved{},
				newError(
					ErrorInvariantViolation,
					"runtime value unit cannot be converted to static expression unit",
					nil,
				)
		}

		switch actual.Type() {
		case enginevalue.Decimal:
			converted, err := unit.Convert(
				actual.Raw().Decimal(),
				actual.Unit(),
				expectedUnit,
				s.version.PrecisionPolicy(),
			)
			if err != nil {
				return enginevalue.Resolved{},
					newError(
						ErrorInvariantViolation,
						"validated Decimal runtime unit normalization failed",
						err,
					)
			}

			return enginevalue.Defined(
				enginevalue.NewTypedWithUnit(
					enginevalue.NewDecimal(converted),
					expectedUnit,
				),
			), nil

		case enginevalue.Integer:
			converted, err := convertIntegerUnitExact(
				actual.Raw().Integer(),
				actual.Unit(),
				expectedUnit,
			)
			if err != nil {
				return enginevalue.Resolved{}, err
			}

			return enginevalue.Defined(
				enginevalue.NewTypedWithUnit(
					enginevalue.NewInteger(converted),
					expectedUnit,
				),
			), nil

		default:
			return enginevalue.Resolved{},
				newError(
					ErrorInvariantViolation,
					"non-numeric runtime value unexpectedly requires unit conversion",
					nil,
				)
		}
	}

	// Integer may be widened to Decimal by deterministic static unification
	// when compatible units require a representation that can hold fractional
	// conversion results.
	if actual.Type() == enginevalue.Integer &&
		expected.ValueType() == enginevalue.Decimal {
		decimalValue := decimal.New(
			actual.Raw().Integer(),
			0,
		)

		if !expectedHasUnit {
			return enginevalue.Defined(
				enginevalue.NewTyped(
					enginevalue.NewDecimal(decimalValue),
				),
			), nil
		}

		if actual.Unit().Equal(expectedUnit) {
			return enginevalue.Defined(
				enginevalue.NewTypedWithUnit(
					enginevalue.NewDecimal(decimalValue),
					expectedUnit,
				),
			), nil
		}

		if !actual.Unit().CanConvertTo(expectedUnit) {
			return enginevalue.Resolved{},
				newError(
					ErrorInvariantViolation,
					"Integer runtime value unit cannot be converted to widened Decimal static unit",
					nil,
				)
		}

		converted, err := unit.Convert(
			decimalValue,
			actual.Unit(),
			expectedUnit,
			s.version.PrecisionPolicy(),
		)
		if err != nil {
			return enginevalue.Resolved{},
				newError(
					ErrorInvariantViolation,
					"validated widened Decimal runtime unit normalization failed",
					err,
				)
		}

		return enginevalue.Defined(
			enginevalue.NewTypedWithUnit(
				enginevalue.NewDecimal(converted),
				expectedUnit,
			),
		), nil
	}

	return enginevalue.Resolved{},
		newError(
			ErrorInvariantViolation,
			"runtime value type disagrees with static expression type",
			nil,
		)
}

func zeroResolvedForStaticType(
	typeInfo typing.Type,
) (enginevalue.Resolved, error) {
	if !typeInfo.IsValid() ||
		typeInfo.TemporalMode() != typing.TemporalScalar {
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"cannot construct aggregate zero for invalid/non-scalar static type",
				nil,
			)
	}

	var raw enginevalue.Raw

	switch typeInfo.ValueType() {
	case enginevalue.Integer:
		raw = enginevalue.NewInt64(0)

	case enginevalue.Decimal:
		raw = enginevalue.NewDecimal(decimal.Zero())

	default:
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"Sum empty result requires numeric static type",
				nil,
			)
	}

	if expectedUnit, hasUnit := typeInfo.Unit(); hasUnit {
		return enginevalue.Defined(
			enginevalue.NewTypedWithUnit(
				raw,
				expectedUnit,
			),
		), nil
	}

	return enginevalue.Defined(
		enginevalue.NewTyped(raw),
	), nil
}
