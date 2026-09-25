package model

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func (d *Draft) validateContractsAndTraits(
	report *ValidationReport,
) {
	contracts := make(
		map[identity.UUID]schema.PropertyContract,
		len(d.propertyContracts),
	)

	for _, contract := range d.propertyContracts {
		if contract.ID().IsZero() {
			continue
		}

		if _, exists := contracts[contract.ID()]; exists {
			continue
		}

		contracts[contract.ID()] = contract
	}

	implementations := d.buildCompatibleContractImplementations(
		report,
		contracts,
	)

	d.validateTraitRequirements(
		report,
		contracts,
		implementations,
	)
}

type contractImplementationState struct {
	first      schema.PropertyDefinition
	count      int
	compatible bool
}

// buildCompatibleContractImplementations validates contract implementations
// and returns only implementations that are unambiguous and compatible.
//
// A Trait requirement is satisfied only when the owner provides exactly one
// compatible PropertyDefinition for the required PropertyContract.
func (d *Draft) buildCompatibleContractImplementations(
	report *ValidationReport,
	contracts map[identity.UUID]schema.PropertyContract,
) map[schema.PropertyOwner]map[identity.UUID]schema.PropertyDefinition {
	states := make(
		map[schema.PropertyOwner]map[identity.UUID]*contractImplementationState,
	)

	for _, definition := range d.propertyDefinitions {
		contractID, hasContract := definition.ContractID()
		if !hasContract {
			continue
		}

		contract, contractExists := contracts[contractID]
		if !contractExists {
			// Reference validation owns the missing contract.
			continue
		}

		owner := definition.Owner()
		if !owner.IsValid() {
			continue
		}

		byContract, exists := states[owner]
		if !exists {
			byContract = make(
				map[identity.UUID]*contractImplementationState,
			)
			states[owner] = byContract
		}

		state, exists := byContract[contractID]
		if !exists {
			state = &contractImplementationState{
				first: definition,
			}
			byContract[contractID] = state
		}

		state.count++

		if state.count > 1 {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueDuplicateContractImplementation,
					definition.ID(),
					state.first.ID(),
					"owner contains more than one implementation of the same property contract",
				),
			)
		}

		compatible := validateContractImplementation(
			report,
			definition,
			contract,
		)

		if state.count == 1 {
			state.compatible = compatible
		} else {
			// Any duplicate makes resolution ambiguous, regardless of whether
			// one of the definitions is otherwise compatible.
			state.compatible = false
		}
	}

	result := make(
		map[schema.PropertyOwner]map[identity.UUID]schema.PropertyDefinition,
	)

	for owner, byContract := range states {
		for contractID, state := range byContract {
			if state.count != 1 || !state.compatible {
				continue
			}

			implementations, exists := result[owner]
			if !exists {
				implementations = make(
					map[identity.UUID]schema.PropertyDefinition,
				)
				result[owner] = implementations
			}

			implementations[contractID] = state.first
		}
	}

	return result
}

// validateContractImplementation reports all locally detectable contract
// mismatches and returns true only when the definition is compatible.
func validateContractImplementation(
	report *ValidationReport,
	definition schema.PropertyDefinition,
	contract schema.PropertyContract,
) bool {
	compatible := true

	if definition.ValueType() != contract.ValueType() || (definition.ValueType() == value.Enum && !sameEnumDefinition(definition, contract)) {
		report.append(
			newRelatedValidationIssue(
				ValidationIssueContractValueTypeMismatch,
				definition.ID(),
				contract.ID(),
				"property definition value type does not satisfy property contract",
			),
		)
		compatible = false
	}

	if definition.TemporalMode() != contract.TemporalMode() {
		report.append(
			newRelatedValidationIssue(
				ValidationIssueContractTemporalModeMismatch,
				definition.ID(),
				contract.ID(),
				"property definition temporal mode does not satisfy property contract",
			),
		)
		compatible = false
	}

	if !validateContractDimension(
		report,
		definition,
		contract,
	) {
		compatible = false
	}

	return compatible
}

func sameEnumDefinition(definition schema.PropertyDefinition, contract schema.PropertyContract) bool {
	left, leftOK := definition.EnumDefinitionID()
	right, rightOK := contract.EnumDefinitionID()
	return leftOK && rightOK && left == right
}

func validateContractDimension(
	report *ValidationReport,
	definition schema.PropertyDefinition,
	contract schema.PropertyContract,
) bool {
	requiredDimension, contractHasDimension := contract.Dimension()

	// Absence of Dimension means the contract does not constrain dimensional
	// semantics. It does NOT mean that the implementation must be unitless.
	if !contractHasDimension {
		return true
	}

	definitionUnit, definitionHasUnit := definition.Unit()
	if !definitionHasUnit {
		report.append(
			newRelatedValidationIssue(
				ValidationIssueContractDimensionMismatch,
				definition.ID(),
				contract.ID(),
				"property definition has no unit but property contract requires a dimension",
			),
		)
		return false
	}

	if !definitionUnit.Dimension().Equal(requiredDimension) {
		report.append(
			newRelatedValidationIssue(
				ValidationIssueContractDimensionMismatch,
				definition.ID(),
				contract.ID(),
				"property definition unit dimension does not satisfy property contract",
			),
		)
		return false
	}

	return true
}

func (d *Draft) validateTraitRequirements(
	report *ValidationReport,
	contracts map[identity.UUID]schema.PropertyContract,
	implementations map[schema.PropertyOwner]map[identity.UUID]schema.PropertyDefinition,
) {
	traits := make(
		map[identity.UUID]schema.Trait,
		len(d.traits),
	)

	for _, trait := range d.traits {
		if trait.ID().IsZero() {
			continue
		}

		if _, exists := traits[trait.ID()]; exists {
			continue
		}

		traits[trait.ID()] = trait
	}

	for _, entityType := range d.entityTypes {
		// Draft is allowed to contain structurally invalid objects.
		//
		// EntityTypeOwner intentionally panics for zero UUID because it is a
		// strict domain constructor. Validation must never feed malformed
		// draft data into strict constructors.
		if entityType.ID().IsZero() {
			continue
		}

		owner := schema.EntityTypeOwner(entityType.ID())
		ownerImplementations := implementations[owner]

		for _, traitID := range entityType.Traits() {
			trait, exists := traits[traitID]
			if !exists {
				continue
			}

			for _, contractID := range trait.RequiredPropertyContracts() {
				if _, exists := contracts[contractID]; !exists {
					continue
				}

				if _, implemented := ownerImplementations[contractID]; implemented {
					continue
				}

				report.append(
					newRelatedValidationIssue(
						ValidationIssueTraitRequirementNotSatisfied,
						entityType.ID(),
						contractID,
						"entity type does not provide exactly one compatible property definition required by its trait",
					),
				)
			}
		}
	}
}
