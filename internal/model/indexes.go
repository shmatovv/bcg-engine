package model

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
)

type indexes struct {
	valueBindingsByTarget map[semantic.ValueTarget][]semantic.ValueBinding

	entityTypeByID map[identity.UUID]schema.EntityType

	relationTypeByID map[identity.UUID]schema.RelationType

	traitByID map[identity.UUID]schema.Trait

	propertyContractByID map[identity.UUID]schema.PropertyContract

	propertyDefinitionByID map[identity.UUID]schema.PropertyDefinition

	scopeDefinitionByID map[identity.UUID]scope.Definition

	entityByID map[identity.UUID]semantic.Entity

	relationByID map[identity.UUID]semantic.Relation

	constraintByID map[identity.UUID]constraint.Constraint

	constraintDependenciesByID map[identity.UUID]ConstraintDependencies

	constraintIDsByDependency map[address.PropertyAddress][]identity.UUID

	ruleByID map[identity.UUID]rule.Rule

	entityTypesByTrait map[identity.UUID][]schema.EntityType

	contractImplementationByOwner map[schema.PropertyOwner]map[identity.UUID]schema.PropertyDefinition
}

func buildIndexes(
	entityTypes []schema.EntityType,
	relationTypes []schema.RelationType,
	traits []schema.Trait,
	propertyContracts []schema.PropertyContract,
	propertyDefinitions []schema.PropertyDefinition,
	scopeDefinitions []scope.Definition,
	entities []semantic.Entity,
	relations []semantic.Relation,
	constraints []constraint.Constraint,
	rules []rule.Rule,
	valueBindings []semantic.ValueBinding,
) indexes {
	result := indexes{
		valueBindingsByTarget: make(map[semantic.ValueTarget][]semantic.ValueBinding),
		entityTypeByID: make(
			map[identity.UUID]schema.EntityType,
			len(entityTypes),
		),

		relationTypeByID: make(
			map[identity.UUID]schema.RelationType,
			len(relationTypes),
		),

		traitByID: make(
			map[identity.UUID]schema.Trait,
			len(traits),
		),

		propertyContractByID: make(
			map[identity.UUID]schema.PropertyContract,
			len(propertyContracts),
		),

		propertyDefinitionByID: make(
			map[identity.UUID]schema.PropertyDefinition,
			len(propertyDefinitions),
		),

		scopeDefinitionByID: make(
			map[identity.UUID]scope.Definition,
			len(scopeDefinitions),
		),

		entityByID: make(
			map[identity.UUID]semantic.Entity,
			len(entities),
		),

		relationByID: make(
			map[identity.UUID]semantic.Relation,
			len(relations),
		),

		constraintByID: make(
			map[identity.UUID]constraint.Constraint,
			len(constraints),
		),

		constraintDependenciesByID: make(
			map[identity.UUID]ConstraintDependencies,
			len(constraints),
		),

		constraintIDsByDependency: make(
			map[address.PropertyAddress][]identity.UUID,
		),

		ruleByID: make(
			map[identity.UUID]rule.Rule,
			len(rules),
		),

		entityTypesByTrait: make(
			map[identity.UUID][]schema.EntityType,
		),

		contractImplementationByOwner: make(
			map[schema.PropertyOwner]map[identity.UUID]schema.PropertyDefinition,
		),
	}

	for _, entityType := range entityTypes {
		result.entityTypeByID[entityType.ID()] = entityType

		for _, traitID := range entityType.Traits() {
			result.entityTypesByTrait[traitID] =
				append(
					result.entityTypesByTrait[traitID],
					entityType,
				)
		}
	}

	for _, relationType := range relationTypes {
		result.relationTypeByID[relationType.ID()] =
			relationType
	}

	for _, trait := range traits {
		result.traitByID[trait.ID()] =
			trait
	}

	for _, contract := range propertyContracts {
		result.propertyContractByID[contract.ID()] =
			contract
	}

	for _, definition := range propertyDefinitions {
		result.propertyDefinitionByID[definition.ID()] =
			definition

		contractID, hasContract :=
			definition.ContractID()

		if !hasContract {
			continue
		}

		owner := definition.Owner()

		implementations, ok :=
			result.contractImplementationByOwner[owner]

		if !ok {
			implementations = make(
				map[identity.UUID]schema.PropertyDefinition,
			)

			result.contractImplementationByOwner[owner] =
				implementations
		}

		implementations[contractID] =
			definition
	}

	for _, definition := range scopeDefinitions {
		result.scopeDefinitionByID[definition.ID()] =
			definition
	}

	for _, entity := range entities {
		result.entityByID[entity.ID()] =
			entity
	}

	for _, relation := range relations {
		result.relationByID[relation.ID()] =
			relation
	}

	for _, modelConstraint := range constraints {
		result.constraintByID[modelConstraint.ID()] =
			modelConstraint
	}

	for _, modelRule := range rules {
		result.ruleByID[modelRule.ID()] =
			modelRule
	}

	for traitID := range result.entityTypesByTrait {
		entityTypes :=
			result.entityTypesByTrait[traitID]

		sort.Slice(
			entityTypes,
			func(i, j int) bool {
				return entityTypes[i].ID().String() <
					entityTypes[j].ID().String()
			},
		)

		result.entityTypesByTrait[traitID] =
			entityTypes
	}

	for _, binding := range valueBindings {
		target := binding.Target()
		result.valueBindingsByTarget[target] = append(result.valueBindingsByTarget[target], binding)
	}
	return result
}
