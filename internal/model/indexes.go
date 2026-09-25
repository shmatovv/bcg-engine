package model

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
)

type indexes struct {
	valueBindingsByTarget map[semantic.ValueTarget][]semantic.ValueBinding

	entityTypeByID          map[identity.UUID]schema.EntityType
	enumDefinitionByID      map[identity.UUID]schema.EnumDefinition
	entityKeyDefinitionByID map[identity.UUID]schema.EntityKeyDefinition

	relationTypeByID map[identity.UUID]schema.RelationType

	traitByID map[identity.UUID]schema.Trait

	propertyContractByID map[identity.UUID]schema.PropertyContract

	propertyDefinitionByID map[identity.UUID]schema.PropertyDefinition

	scopeDefinitionByID map[identity.UUID]scope.Definition

	entityByID map[identity.UUID]semantic.Entity

	relationByID map[identity.UUID]semantic.Relation

	// Semantic traversal indexes are distinct from the value dependency
	// graph. They answer one-hop relation membership in canonical Relation
	// UUID order; evaluating an Aggregate is what turns traversal into
	// concrete property dependencies.
	outgoingRelationsByEndpoint map[relationTraversalKey][]semantic.Relation
	incomingRelationsByEndpoint map[relationTraversalKey][]semantic.Relation

	constraintByID map[identity.UUID]constraint.Constraint

	constraintDependenciesByID map[identity.UUID]ConstraintDependencies

	constraintIDsByDependency map[address.PropertyAddress][]identity.UUID

	ruleByID      map[identity.UUID]rule.Rule
	invariantByID map[identity.UUID]invariant.Invariant

	ruleDependenciesByID map[identity.UUID]RuleDependencies

	ruleIDsByDependency map[address.PropertyAddress][]identity.UUID

	entityTypesByTrait map[identity.UUID][]schema.EntityType

	contractImplementationByOwner map[schema.PropertyOwner]map[identity.UUID]schema.PropertyDefinition
}

type relationTraversalKey struct {
	entityID       identity.UUID
	relationTypeID identity.UUID
}

func buildIndexes(
	entityTypes []schema.EntityType,
	enumDefinitions []schema.EnumDefinition,
	entityKeyDefinitions []schema.EntityKeyDefinition,
	relationTypes []schema.RelationType,
	traits []schema.Trait,
	propertyContracts []schema.PropertyContract,
	propertyDefinitions []schema.PropertyDefinition,
	scopeDefinitions []scope.Definition,
	entities []semantic.Entity,
	relations []semantic.Relation,
	constraints []constraint.Constraint,
	rules []rule.Rule,
	invariants []invariant.Invariant,
	valueBindings []semantic.ValueBinding,
) indexes {
	result := indexes{
		valueBindingsByTarget: make(map[semantic.ValueTarget][]semantic.ValueBinding),
		entityTypeByID: make(
			map[identity.UUID]schema.EntityType,
			len(entityTypes),
		),
		enumDefinitionByID:      make(map[identity.UUID]schema.EnumDefinition, len(enumDefinitions)),
		entityKeyDefinitionByID: make(map[identity.UUID]schema.EntityKeyDefinition, len(entityKeyDefinitions)),

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

		outgoingRelationsByEndpoint: make(
			map[relationTraversalKey][]semantic.Relation,
		),

		incomingRelationsByEndpoint: make(
			map[relationTraversalKey][]semantic.Relation,
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
		invariantByID: make(map[identity.UUID]invariant.Invariant, len(invariants)),

		ruleDependenciesByID: make(
			map[identity.UUID]RuleDependencies,
			len(rules),
		),

		ruleIDsByDependency: make(
			map[address.PropertyAddress][]identity.UUID,
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
	for _, definition := range enumDefinitions {
		result.enumDefinitionByID[definition.ID()] = definition
	}
	for _, definition := range entityKeyDefinitions {
		result.entityKeyDefinitionByID[definition.ID()] = definition
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

		outgoingKey := relationTraversalKey{
			entityID:       relation.From(),
			relationTypeID: relation.TypeID(),
		}
		result.outgoingRelationsByEndpoint[outgoingKey] = append(
			result.outgoingRelationsByEndpoint[outgoingKey],
			relation,
		)

		incomingKey := relationTraversalKey{
			entityID:       relation.To(),
			relationTypeID: relation.TypeID(),
		}
		result.incomingRelationsByEndpoint[incomingKey] = append(
			result.incomingRelationsByEndpoint[incomingKey],
			relation,
		)
	}

	for _, modelConstraint := range constraints {
		result.constraintByID[modelConstraint.ID()] =
			modelConstraint
	}

	for _, modelRule := range rules {
		result.ruleByID[modelRule.ID()] =
			modelRule
	}
	for _, declaration := range invariants {
		result.invariantByID[declaration.ID()] = declaration
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

	for key, relationValues := range result.outgoingRelationsByEndpoint {
		sortRelationsByID(relationValues)
		result.outgoingRelationsByEndpoint[key] = relationValues
	}

	for key, relationValues := range result.incomingRelationsByEndpoint {
		sortRelationsByID(relationValues)
		result.incomingRelationsByEndpoint[key] = relationValues
	}

	for _, binding := range valueBindings {
		target := binding.Target()
		result.valueBindingsByTarget[target] = append(result.valueBindingsByTarget[target], binding)
	}
	return result
}

func sortRelationsByID(relations []semantic.Relation) {
	sort.Slice(relations, func(i, j int) bool {
		return relations[i].ID().String() < relations[j].ID().String()
	})
}
