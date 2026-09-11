package model

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
)

// validateOwnership verifies the bidirectional ownership relationship
// between schema owners and PropertyDefinitions.
//
// A PropertyDefinition belongs to exactly one PropertyOwner. The same
// definition must therefore be listed by that owner and must not be listed
// by any other owner.
//
// Missing references are handled by validateReferences. This pass only
// reasons about PropertyDefinitions that can actually be resolved.
func (d *Draft) validateOwnership(
	report *ValidationReport,
) {
	definitions := make(
		map[identity.UUID]schema.PropertyDefinition,
		len(d.propertyDefinitions),
	)

	for _, definition := range d.propertyDefinitions {
		if definition.ID().IsZero() {
			continue
		}

		// Duplicate definition IDs are handled by the duplicate-ID pass.
		//
		// Do not overwrite the first occurrence here. Ownership validation
		// must remain deterministic even for an already-invalid draft.
		if _, exists := definitions[definition.ID()]; exists {
			continue
		}

		definitions[definition.ID()] = definition
	}

	d.validateEntityTypeOwnership(
		report,
		definitions,
	)

	d.validateRelationTypeOwnership(
		report,
		definitions,
	)

	d.validateDefinitionListedByOwner(
		report,
	)
}

func (d *Draft) validateEntityTypeOwnership(
	report *ValidationReport,
	definitions map[identity.UUID]schema.PropertyDefinition,
) {
	for _, entityType := range d.entityTypes {
		if entityType.ID().IsZero() {
			continue
		}

		expectedOwner :=
			schema.EntityTypeOwner(entityType.ID())

		for _, definitionID := range entityType.PropertyDefinitions() {
			definition, exists :=
				definitions[definitionID]

			// UnknownPropertyDefinition is already reported by
			// validateReferences.
			if !exists {
				continue
			}

			if definition.Owner() == expectedOwner {
				continue
			}

			report.append(
				newRelatedValidationIssue(
					ValidationIssuePropertyOwnerMismatch,
					entityType.ID(),
					definitionID,
					fmt.Sprintf(
						"entity type lists property definition owned by %s",
						describePropertyOwner(
							definition.Owner(),
						),
					),
				),
			)
		}
	}
}

func (d *Draft) validateRelationTypeOwnership(
	report *ValidationReport,
	definitions map[identity.UUID]schema.PropertyDefinition,
) {
	for _, relationType := range d.relationTypes {
		if relationType.ID().IsZero() {
			continue
		}

		expectedOwner :=
			schema.RelationTypeOwner(
				relationType.ID(),
			)

		for _, definitionID := range relationType.PropertyDefinitions() {
			definition, exists :=
				definitions[definitionID]

			if !exists {
				continue
			}

			if definition.Owner() == expectedOwner {
				continue
			}

			report.append(
				newRelatedValidationIssue(
					ValidationIssuePropertyOwnerMismatch,
					relationType.ID(),
					definitionID,
					fmt.Sprintf(
						"relation type lists property definition owned by %s",
						describePropertyOwner(
							definition.Owner(),
						),
					),
				),
			)
		}
	}
}

// validateDefinitionListedByOwner checks the opposite direction.
//
// It is not enough that:
//
//	definition.Owner() == EntityType(Pallet)
//
// Pallet itself must also list that definition in PropertyDefinitions().
//
// Otherwise the model contains an orphaned PropertyDefinition that claims
// ownership which the owner does not acknowledge.
func (d *Draft) validateDefinitionListedByOwner(
	report *ValidationReport,
) {
	entityDefinitions :=
		make(map[identity.UUID]map[identity.UUID]struct{})

	relationDefinitions :=
		make(map[identity.UUID]map[identity.UUID]struct{})

	for _, entityType := range d.entityTypes {
		set := make(
			map[identity.UUID]struct{},
			len(entityType.PropertyDefinitions()),
		)

		for _, definitionID := range entityType.PropertyDefinitions() {
			set[definitionID] = struct{}{}
		}

		// Duplicate EntityType IDs are already an independent validation
		// issue. Preserve the first one to keep this pass deterministic.
		if _, exists :=
			entityDefinitions[entityType.ID()]; !exists {

			entityDefinitions[entityType.ID()] = set
		}
	}

	for _, relationType := range d.relationTypes {
		set := make(
			map[identity.UUID]struct{},
			len(relationType.PropertyDefinitions()),
		)

		for _, definitionID := range relationType.PropertyDefinitions() {
			set[definitionID] = struct{}{}
		}

		if _, exists :=
			relationDefinitions[relationType.ID()]; !exists {

			relationDefinitions[relationType.ID()] = set
		}
	}

	for _, definition := range d.propertyDefinitions {
		owner := definition.Owner()

		// Invalid/missing owners belong to structural/reference validation.
		if !owner.IsValid() {
			continue
		}

		var listed bool

		switch owner.Kind() {
		case schema.PropertyOwnerEntityType:
			definitions, ownerExists :=
				entityDefinitions[owner.ID()]

			if !ownerExists {
				// validateReferences reports PropertyOwnerNotFound.
				continue
			}

			_, listed =
				definitions[definition.ID()]

		case schema.PropertyOwnerRelationType:
			definitions, ownerExists :=
				relationDefinitions[owner.ID()]

			if !ownerExists {
				continue
			}

			_, listed =
				definitions[definition.ID()]

		default:
			// Structural validation owns invalid PropertyOwnerKind.
			continue
		}

		if listed {
			continue
		}

		report.append(
			newRelatedValidationIssue(
				ValidationIssuePropertyOwnerMismatch,
				definition.ID(),
				owner.ID(),
				fmt.Sprintf(
					"property definition claims %s as owner but is not listed by that owner",
					describePropertyOwner(owner),
				),
			),
		)
	}
}

func describePropertyOwner(
	owner schema.PropertyOwner,
) string {
	if !owner.IsValid() {
		return "invalid owner"
	}

	switch owner.Kind() {
	case schema.PropertyOwnerEntityType:
		return fmt.Sprintf(
			"entity type %s",
			owner.ID(),
		)

	case schema.PropertyOwnerRelationType:
		return fmt.Sprintf(
			"relation type %s",
			owner.ID(),
		)

	default:
		return "invalid owner"
	}
}
