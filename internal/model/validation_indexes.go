package model

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
)

type validationIndexes struct {
	entityTypeByID     map[identity.UUID]schema.EntityType
	enumDefinitionByID map[identity.UUID]schema.EnumDefinition

	relationTypeByID map[identity.UUID]schema.RelationType

	traitByID map[identity.UUID]schema.Trait

	propertyContractByID map[identity.UUID]schema.PropertyContract

	propertyDefinitionByID map[identity.UUID]schema.PropertyDefinition

	scopeDefinitionByID map[identity.UUID]scope.Definition

	entityByID map[identity.UUID]semantic.Entity

	relationByID map[identity.UUID]semantic.Relation

	constraintByID map[identity.UUID]constraint.Constraint
}

func buildValidationIndexes(
	d *Draft,
) validationIndexes {
	d.requireNonNil()

	result := validationIndexes{
		entityTypeByID: make(
			map[identity.UUID]schema.EntityType,
			len(d.entityTypes),
		),
		enumDefinitionByID: make(map[identity.UUID]schema.EnumDefinition, len(d.enumDefinitions)),
		relationTypeByID: make(
			map[identity.UUID]schema.RelationType,
			len(d.relationTypes),
		),
		traitByID: make(
			map[identity.UUID]schema.Trait,
			len(d.traits),
		),
		propertyContractByID: make(
			map[identity.UUID]schema.PropertyContract,
			len(d.propertyContracts),
		),
		propertyDefinitionByID: make(
			map[identity.UUID]schema.PropertyDefinition,
			len(d.propertyDefinitions),
		),
		scopeDefinitionByID: make(
			map[identity.UUID]scope.Definition,
			len(d.scopeDefinitions),
		),
		entityByID: make(
			map[identity.UUID]semantic.Entity,
			len(d.entities),
		),
		relationByID: make(
			map[identity.UUID]semantic.Relation,
			len(d.relations),
		),
		constraintByID: make(
			map[identity.UUID]constraint.Constraint,
			len(d.constraints),
		),
	}

	for _, entityType := range d.entityTypes {
		insertFirst(
			result.entityTypeByID,
			entityType.ID(),
			entityType,
		)
	}
	for _, definition := range d.enumDefinitions {
		insertFirst(result.enumDefinitionByID, definition.ID(), definition)
	}

	for _, relationType := range d.relationTypes {
		insertFirst(
			result.relationTypeByID,
			relationType.ID(),
			relationType,
		)
	}

	for _, trait := range d.traits {
		insertFirst(
			result.traitByID,
			trait.ID(),
			trait,
		)
	}

	for _, contract := range d.propertyContracts {
		insertFirst(
			result.propertyContractByID,
			contract.ID(),
			contract,
		)
	}

	for _, definition := range d.propertyDefinitions {
		insertFirst(
			result.propertyDefinitionByID,
			definition.ID(),
			definition,
		)
	}

	for _, definition := range d.scopeDefinitions {
		insertFirst(
			result.scopeDefinitionByID,
			definition.ID(),
			definition,
		)
	}

	for _, entity := range d.entities {
		insertFirst(
			result.entityByID,
			entity.ID(),
			entity,
		)
	}

	for _, relation := range d.relations {
		insertFirst(
			result.relationByID,
			relation.ID(),
			relation,
		)
	}

	for _, modelConstraint := range d.constraints {
		insertFirst(
			result.constraintByID,
			modelConstraint.ID(),
			modelConstraint,
		)
	}

	return result
}

func insertFirst[T any](
	target map[identity.UUID]T,
	id identity.UUID,
	value T,
) {
	if id.IsZero() {
		return
	}

	if _, exists := target[id]; exists {
		return
	}

	target[id] = value
}
