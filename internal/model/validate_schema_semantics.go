package model

import (
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// validateSchemaSemantics checks schema combinations that are structurally
// representable but unsupported by Core v1 semantics.
//
// Units and dimensions belong to the numeric domain. Non-numeric properties
// therefore cannot carry Units, and non-numeric PropertyContracts cannot
// require a Dimension.
func (d *Draft) validateSchemaSemantics(
	report *ValidationReport,
) {
	for _, contract := range d.propertyContracts {
		if !contract.IsValid() {
			continue
		}

		_, hasDimension := contract.Dimension()
		if hasDimension && !valueTypeSupportsUnit(contract.ValueType()) {
			report.append(
				newSubjectValidationIssue(
					ValidationIssueUnsupportedSchemaCombination,
					contract.ID(),
					"non-numeric property contract cannot require a dimension",
				),
			)
		}
	}

	for _, definition := range d.propertyDefinitions {
		if !definition.IsValid() {
			continue
		}

		_, hasUnit := definition.Unit()
		if hasUnit && !valueTypeSupportsUnit(definition.ValueType()) {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueUnsupportedSchemaCombination,
					definition.ID(),
					definition.Owner().ID(),
					"non-numeric property definition cannot carry a unit",
				),
			)
		}
	}
}

func valueTypeSupportsUnit(t value.Type) bool {
	switch t {
	case value.Integer, value.Decimal:
		return true
	default:
		return false
	}
}
