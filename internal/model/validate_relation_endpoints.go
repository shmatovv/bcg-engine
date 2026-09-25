package model

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
)

// validateRelationEndpoints verifies that concrete semantic Relation
// endpoints satisfy the endpoint constraints declared by their RelationType.
func (d *Draft) validateRelationEndpoints(
	report *ValidationReport,
	indexes validationIndexes,
) {
	for _, relation := range d.relations {
		if !relation.IsValid() {
			continue
		}

		relationType, relationTypeExists :=
			indexes.relationTypeByID[relation.TypeID()]

		if !relationTypeExists {
			continue
		}

		fromEntity, fromExists :=
			indexes.entityByID[relation.From()]
		toEntity, toExists :=
			indexes.entityByID[relation.To()]

		if fromExists {
			if constraint, constrained :=
				relationType.From(); constrained {
				validateRelationEndpoint(
					report,
					relation.ID(),
					"from",
					fromEntity,
					constraint,
					indexes.entityTypeByID,
				)
			}
		}

		if toExists {
			if constraint, constrained :=
				relationType.To(); constrained {
				validateRelationEndpoint(
					report,
					relation.ID(),
					"to",
					toEntity,
					constraint,
					indexes.entityTypeByID,
				)
			}
		}
	}
}

func validateRelationEndpoint(
	report *ValidationReport,
	relationID identity.UUID,
	endpointName string,
	entity semantic.Entity,
	constraint schema.TypeConstraint,
	entityTypes map[identity.UUID]schema.EntityType,
) {
	if !constraint.IsValid() || !entity.IsValid() {
		return
	}

	if constraint.Kind() == schema.TypeConstraintAnyEntity {
		return
	}

	if _, exists := entityTypes[entity.TypeID()]; !exists {
		return
	}

	if entityMatchesTypeConstraint(
		entity,
		constraint,
		entityTypes,
	) {
		return
	}

	report.append(
		newRelatedValidationIssue(
			ValidationIssueInvalidRelationEndpoint,
			relationID,
			entity.ID(),
			fmt.Sprintf(
				"relation %s endpoint does not satisfy relation type constraint %s",
				endpointName,
				describeTypeConstraint(constraint),
			),
		),
	)
}

func entityMatchesTypeConstraint(
	entity semantic.Entity,
	constraint schema.TypeConstraint,
	entityTypes map[identity.UUID]schema.EntityType,
) bool {
	if !entity.IsValid() || !constraint.IsValid() {
		return false
	}

	switch constraint.Kind() {
	case schema.TypeConstraintAnyEntity:
		return true

	case schema.TypeConstraintEntityType:
		requiredTypeID, ok := constraint.ID()
		if !ok {
			return false
		}
		return entity.TypeID() == requiredTypeID

	case schema.TypeConstraintTrait:
		requiredTraitID, ok := constraint.ID()
		if !ok {
			return false
		}

		entityType, exists := entityTypes[entity.TypeID()]
		if !exists {
			return false
		}

		return entityTypeHasTrait(
			entityType,
			requiredTraitID,
		)

	default:
		return false
	}
}

func entityTypeHasTrait(
	entityType schema.EntityType,
	traitID identity.UUID,
) bool {
	for _, candidate := range entityType.Traits() {
		if candidate == traitID {
			return true
		}
	}

	return false
}

func describeTypeConstraint(
	constraint schema.TypeConstraint,
) string {
	if !constraint.IsValid() {
		return "Invalid"
	}

	switch constraint.Kind() {
	case schema.TypeConstraintAnyEntity:
		return "AnyEntity"

	case schema.TypeConstraintEntityType:
		id, ok := constraint.ID()
		if !ok {
			return "EntityType(Invalid)"
		}
		return fmt.Sprintf("EntityType(%s)", id)

	case schema.TypeConstraintTrait:
		id, ok := constraint.ID()
		if !ok {
			return "Trait(Invalid)"
		}
		return fmt.Sprintf("Trait(%s)", id)

	default:
		return "Invalid"
	}
}
