package model

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
)

func (d *Draft) validateReferences(
	report *ValidationReport,
) {
	entityTypes := make(
		map[identity.UUID]struct{},
		len(d.entityTypes),
	)

	relationTypes := make(
		map[identity.UUID]struct{},
		len(d.relationTypes),
	)

	traits := make(
		map[identity.UUID]struct{},
		len(d.traits),
	)

	contracts := make(
		map[identity.UUID]struct{},
		len(d.propertyContracts),
	)

	definitions := make(
		map[identity.UUID]struct{},
		len(d.propertyDefinitions),
	)

	for _, v := range d.entityTypes {
		if !v.ID().IsZero() {
			entityTypes[v.ID()] = struct{}{}
		}
	}

	for _, v := range d.relationTypes {
		if !v.ID().IsZero() {
			relationTypes[v.ID()] = struct{}{}
		}
	}

	for _, v := range d.traits {
		if !v.ID().IsZero() {
			traits[v.ID()] = struct{}{}
		}
	}

	for _, v := range d.propertyContracts {
		if !v.ID().IsZero() {
			contracts[v.ID()] = struct{}{}
		}
	}

	for _, v := range d.propertyDefinitions {
		if !v.ID().IsZero() {
			definitions[v.ID()] = struct{}{}
		}
	}

	// EntityType references.
	for _, entityType := range d.entityTypes {
		for _, traitID := range entityType.Traits() {
			if _, ok := traits[traitID]; !ok {
				report.append(
					newRelatedValidationIssue(
						ValidationIssueUnknownTrait,
						entityType.ID(),
						traitID,
						"entity type references unknown trait",
					),
				)
			}
		}

		for _, definitionID := range entityType.PropertyDefinitions() {

			if _, ok := definitions[definitionID]; !ok {
				report.append(
					newRelatedValidationIssue(
						ValidationIssueUnknownPropertyDefinition,
						entityType.ID(),
						definitionID,
						"entity type references unknown property definition",
					),
				)
			}
		}
	}

	// Trait references.
	for _, trait := range d.traits {
		for _, contractID := range trait.RequiredPropertyContracts() {

			if _, ok := contracts[contractID]; !ok {
				report.append(
					newRelatedValidationIssue(
						ValidationIssueUnknownPropertyContract,
						trait.ID(),
						contractID,
						"trait references unknown property contract",
					),
				)
			}
		}
	}

	// PropertyDefinition references.
	for _, definition := range d.propertyDefinitions {
		owner := definition.Owner()

		if owner.IsValid() {
			switch owner.Kind() {
			case schema.PropertyOwnerEntityType:
				if _, ok := entityTypes[owner.ID()]; !ok {
					report.append(
						newRelatedValidationIssue(
							ValidationIssuePropertyOwnerNotFound,
							definition.ID(),
							owner.ID(),
							"property definition owner entity type does not exist",
						),
					)
				}

			case schema.PropertyOwnerRelationType:
				if _, ok := relationTypes[owner.ID()]; !ok {
					report.append(
						newRelatedValidationIssue(
							ValidationIssuePropertyOwnerNotFound,
							definition.ID(),
							owner.ID(),
							"property definition owner relation type does not exist",
						),
					)
				}
			}
		}

		if contractID, ok := definition.ContractID(); ok {
			if _, exists := contracts[contractID]; !exists {
				report.append(
					newRelatedValidationIssue(
						ValidationIssueUnknownPropertyContract,
						definition.ID(),
						contractID,
						"property definition references unknown property contract",
					),
				)
			}
		}
	}

	// RelationType references.
	for _, relationType := range d.relationTypes {
		if constraint, ok := relationType.From(); ok {
			validateTypeConstraintReference(
				report,
				relationType.ID(),
				"from",
				constraint,
				entityTypes,
				traits,
			)
		}

		if constraint, ok := relationType.To(); ok {
			validateTypeConstraintReference(
				report,
				relationType.ID(),
				"to",
				constraint,
				entityTypes,
				traits,
			)
		}

		for _, definitionID := range relationType.PropertyDefinitions() {

			if _, ok := definitions[definitionID]; !ok {
				report.append(
					newRelatedValidationIssue(
						ValidationIssueUnknownPropertyDefinition,
						relationType.ID(),
						definitionID,
						"relation type references unknown property definition",
					),
				)
			}
		}
	}
}

func validateTypeConstraintReference(
	report *ValidationReport,
	relationTypeID identity.UUID,
	endpoint string,
	constraint schema.TypeConstraint,
	entityTypes map[identity.UUID]struct{},
	traits map[identity.UUID]struct{},
) {
	// Structural validation is responsible for malformed constraints.
	// Reference validation only resolves structurally valid references.
	if !constraint.IsValid() {
		return
	}

	switch constraint.Kind() {
	case schema.TypeConstraintEntityType:
		id, ok := constraint.ID()
		if !ok {
			return
		}

		if _, exists := entityTypes[id]; !exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueUnknownEntityType,
					relationTypeID,
					id,
					fmt.Sprintf(
						"relation type %s endpoint references unknown entity type",
						endpoint,
					),
				),
			)
		}

	case schema.TypeConstraintTrait:
		id, ok := constraint.ID()
		if !ok {
			return
		}

		if _, exists := traits[id]; !exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueUnknownTrait,
					relationTypeID,
					id,
					fmt.Sprintf(
						"relation type %s endpoint references unknown trait",
						endpoint,
					),
				),
			)
		}

	case schema.TypeConstraintAnyEntity:
		// AnyEntity contains no reference that needs resolution.
		return
	}
}
