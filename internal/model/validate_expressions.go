package model

import (
	"math/big"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// validateExpressions statically validates every expression stored directly
// in the committed model layer.
func (d *Draft) validateExpressions(
	report *ValidationReport,
	indexes validationIndexes,
) {
	resolver := newValidationInferenceResolver(indexes)

	d.validateDerivedExpressions(
		report,
		resolver,
		indexes,
	)

	d.validateRelationApplicabilityExpressions(
		report,
		resolver,
		indexes,
	)
}

func (d *Draft) validateDerivedExpressions(
	report *ValidationReport,
	resolver validationInferenceResolver,
	indexes validationIndexes,
) {
	for _, definition := range d.propertyDefinitions {
		if !definition.IsValid() ||
			definition.Computation().Kind() != schema.ComputationDerived {
			continue
		}

		expression, ok := definition.Computation().Expression()
		if !ok || expression == nil {
			continue
		}

		// A missing contract is already a reference issue. Without the
		// contract we also cannot know target nullability.
		if contractID, hasContract := definition.ContractID(); hasContract {
			if _, exists := indexes.propertyContractByID[contractID]; !exists {
				continue
			}
		}

		inferred, err := inference.Infer(
			expression,
			inference.NewContext(definition.Owner()),
			resolver,
		)
		if err != nil {
			report.append(
				newSubjectValidationIssue(
					ValidationIssueInvalidDerivedExpression,
					definition.ID(),
					"derived property expression cannot be statically inferred: "+err.Error(),
				),
			)
			continue
		}

		if !derivedResultMatchesDefinition(
			inferred,
			definition,
			indexes,
		) {
			report.append(
				newSubjectValidationIssue(
					ValidationIssueInvalidDerivedExpression,
					definition.ID(),
					"derived property expression result does not satisfy the property definition static type",
				),
			)
		}
	}
}

// derivedResultMatchesDefinition verifies that the statically inferred
// expression result can be represented by the PropertyDefinition.
//
// PropertyDefinition.Unit is the canonical Unit of the resolved property.
// Therefore a derived expression is not required to infer that exact Unit.
// It may infer another Unit when the result can be normalized to the
// definition Unit using an exact, context-free conversion.
//
// Examples:
//
//	Decimal<m*cm> -> Decimal<m^2> allowed
//	Decimal<cm>   -> Decimal<m>   allowed
//	Decimal<RUB>  -> Decimal<EUR> rejected
//
// Integer properties need one additional restriction. A conversion is valid
// only when every possible Integer source value remains an Integer in the
// target Unit. This prevents a committed definition from promising Integer
// while normalization can require a fractional value:
//
//	Integer<m>  -> Integer<cm> allowed  (x100)
//	Integer<cm> -> Integer<m>  rejected (x1/100)
//
// The evaluator must normalize a successfully evaluated Defined value to the
// PropertyDefinition Unit before exposing the resolved property value.
func derivedResultMatchesDefinition(
	inferred typing.Type,
	definition schema.PropertyDefinition,
	indexes validationIndexes,
) bool {
	if !inferred.IsValid() || !definition.IsValid() {
		return false
	}

	if inferred.ValueType() != definition.ValueType() {
		return false
	}

	expectedTemporal, ok := expressionTemporalMode(
		definition.TemporalMode(),
	)
	if !ok || inferred.TemporalMode() != expectedTemporal {
		return false
	}

	if !derivedUnitMatchesDefinition(
		inferred,
		definition,
	) {
		return false
	}

	expectedNullable := false

	if contractID, hasContract := definition.ContractID(); hasContract {
		contract, exists := indexes.propertyContractByID[contractID]
		if !exists {
			return false
		}

		expectedNullable = contract.Nullable()
	}

	// A non-nullable expression is safe for a nullable property contract.
	// The opposite direction is not safe.
	if inferred.Nullable() && !expectedNullable {
		return false
	}

	return true
}

func derivedUnitMatchesDefinition(
	inferred typing.Type,
	definition schema.PropertyDefinition,
) bool {
	expectedUnit, expectedHasUnit := definition.Unit()
	inferredUnit, inferredHasUnit := inferred.Unit()

	if expectedHasUnit != inferredHasUnit {
		return false
	}

	if !expectedHasUnit {
		return true
	}

	if inferredUnit.Equal(expectedUnit) {
		return true
	}

	if !inferredUnit.CanConvertTo(expectedUnit) {
		return false
	}

	if definition.ValueType() != value.Integer {
		return true
	}

	return integerUnitConversionPreservesInteger(
		inferredUnit,
		expectedUnit,
	)
}

// integerUnitConversionPreservesInteger reports whether converting any
// Integer value from source to target can be performed without introducing a
// fractional component.
//
// Unit conversion is:
//
//	result = value * source.scale / target.scale
//
// For an arbitrary integer value, the result is guaranteed to remain integral
// exactly when the reduced conversion factor has denominator 1.
func integerUnitConversionPreservesInteger(
	source unit.Unit,
	target unit.Unit,
) bool {
	if !source.CanConvertTo(target) {
		return false
	}

	factor, err := source.Scale().Div(target.Scale())
	if err != nil {
		return false
	}

	return factor.Denominator().Cmp(big.NewInt(1)) == 0
}

func (d *Draft) validateRelationApplicabilityExpressions(
	report *ValidationReport,
	resolver validationInferenceResolver,
	indexes validationIndexes,
) {
	for _, relation := range d.relations {
		if !relation.IsValid() {
			continue
		}

		expression, hasApplicability := relation.Applicability()
		if !hasApplicability {
			continue
		}

		relationType, exists := indexes.relationTypeByID[relation.TypeID()]
		if !exists || !relationType.IsValid() {
			// Semantic reference validation owns the missing RelationType.
			continue
		}

		inferred, err := inference.Infer(
			expression,
			inference.NewContext(
				schema.RelationTypeOwner(relationType.ID()),
			),
			resolver,
		)
		if err != nil {
			report.append(
				newSubjectValidationIssue(
					ValidationIssueInvalidRelationApplicability,
					relation.ID(),
					"relation applicability expression cannot be statically inferred: "+err.Error(),
				),
			)
			continue
		}

		if inferred.ValueType() != value.Bool ||
			inferred.TemporalMode() != typing.TemporalScalar ||
			inferred.HasUnit() {
			report.append(
				newSubjectValidationIssue(
					ValidationIssueInvalidRelationApplicability,
					relation.ID(),
					"relation applicability must be a unitless Scalar Bool expression",
				),
			)
		}
	}
}

func expressionTemporalMode(
	mode schema.TemporalMode,
) (typing.TemporalMode, bool) {
	switch mode {
	case schema.TemporalScalar:
		return typing.TemporalScalar, true

	case schema.TemporalSeries:
		return typing.TemporalSeries, true

	default:
		return typing.TemporalInvalid, false
	}
}
