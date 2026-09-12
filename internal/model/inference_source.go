package model

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// ObjectType resolves the schema owner of one concrete object in this
// committed ModelVersion.
//
// ObjectType is part of the expression inference Resolver capability used by
// runtime evaluation as well as Draft validation.
func (v ModelVersion) ObjectType(
	object address.ObjectRef,
) (schema.PropertyOwner, bool) {
	if !object.IsValid() {
		return schema.PropertyOwner{}, false
	}

	switch object.Kind() {
	case address.ObjectRefEntity:
		entity, ok := v.Entity(object.ID())
		if !ok || !entity.IsValid() {
			return schema.PropertyOwner{}, false
		}

		entityType, ok := v.EntityType(entity.TypeID())
		if !ok || entityType.ID() != entity.TypeID() {
			return schema.PropertyOwner{}, false
		}

		return schema.EntityTypeOwner(entityType.ID()), true

	case address.ObjectRefRelation:
		relation, ok := v.Relation(object.ID())
		if !ok || !relation.IsValid() {
			return schema.PropertyOwner{}, false
		}

		relationType, ok := v.RelationType(relation.TypeID())
		if !ok || relationType.ID() != relation.TypeID() {
			return schema.PropertyOwner{}, false
		}

		return schema.RelationTypeOwner(relationType.ID()), true

	default:
		return schema.PropertyOwner{}, false
	}
}

// ContractType resolves the single deterministic static expression type of a
// PropertyContract for every EntityType allowed by constraint.
//
// The selected common Unit is deterministic. EntityTypes are processed in
// stable UUID order before typecheck.Unify is applied.
func (v ModelVersion) ContractType(
	constraint schema.TypeConstraint,
	contractID identity.UUID,
) (typing.Type, bool) {
	if !v.IsCommitted() ||
		!constraint.IsValid() ||
		contractID.IsZero() {
		return typing.Type{}, false
	}

	contract, ok := v.PropertyContract(contractID)
	if !ok || !contract.IsValid() {
		return typing.Type{}, false
	}

	entityTypes := v.entityTypesForInferenceConstraint(constraint)
	if len(entityTypes) == 0 {
		return typing.Type{}, false
	}

	var result typing.Type
	hasResult := false

	for _, entityType := range entityTypes {
		definition, ok := v.ContractImplementation(
			schema.EntityTypeOwner(entityType.ID()),
			contractID,
		)
		if !ok || !definition.IsValid() {
			return typing.Type{}, false
		}

		if !definitionCompatibleWithContract(definition, contract) {
			return typing.Type{}, false
		}

		current, err := inference.TypeFromPropertyDefinition(
			definition,
			v,
		)
		if err != nil || !current.IsValid() {
			return typing.Type{}, false
		}

		if !hasResult {
			result = current
			hasResult = true
			continue
		}

		unified, err := typecheck.Unify(result, current)
		if err != nil || !unified.IsValid() {
			return typing.Type{}, false
		}

		result = unified
	}

	if !hasResult {
		return typing.Type{}, false
	}

	if result.ValueType() != contract.ValueType() {
		// Compatible Integer implementations may need Decimal as the common
		// representation when unit normalization can be fractional.
		if contract.ValueType() != value.Integer ||
			result.ValueType() != value.Decimal {
			return typing.Type{}, false
		}
	}

	expectedTemporalMode, ok := typingTemporalModeForContract(
		contract.TemporalMode(),
	)
	if !ok || result.TemporalMode() != expectedTemporalMode {
		return typing.Type{}, false
	}

	if result.Nullable() != contract.Nullable() {
		return typing.Type{}, false
	}

	return result, true
}

func (v ModelVersion) entityTypesForInferenceConstraint(
	constraint schema.TypeConstraint,
) []schema.EntityType {
	var result []schema.EntityType

	switch constraint.Kind() {
	case schema.TypeConstraintEntityType:
		id, ok := constraint.ID()
		if !ok {
			return nil
		}

		entityType, exists := v.EntityType(id)
		if !exists || !entityType.IsValid() {
			return nil
		}

		result = append(result, entityType)

	case schema.TypeConstraintTrait:
		traitID, ok := constraint.ID()
		if !ok {
			return nil
		}

		trait, exists := v.Trait(traitID)
		if !exists || !trait.IsValid() {
			return nil
		}

		result = append(
			result,
			v.EntityTypesImplementingTrait(traitID)...,
		)

	case schema.TypeConstraintAnyEntity:
		result = v.EntityTypes()

	default:
		return nil
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			return compareInferenceUUID(
				result[i].ID(),
				result[j].ID(),
			) < 0
		},
	)

	return result
}

func compareInferenceUUID(
	left identity.UUID,
	right identity.UUID,
) int {
	leftBytes := left.Bytes()
	rightBytes := right.Bytes()

	for index := range leftBytes {
		switch {
		case leftBytes[index] < rightBytes[index]:
			return -1
		case leftBytes[index] > rightBytes[index]:
			return 1
		}
	}

	return 0
}

var _ inference.Resolver = ModelVersion{}
