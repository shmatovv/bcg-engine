package model

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// validationInferenceResolver adapts tolerant Draft validation indexes to the
// expression inference Resolver contract.
//
// It is deliberately conservative. When an invalid Draft makes a lookup or a
// polymorphic contract type ambiguous, the resolver returns false and the
// expression validation pass reports the expression as invalid.
type validationInferenceResolver struct {
	indexes validationIndexes
}

func newValidationInferenceResolver(
	indexes validationIndexes,
) validationInferenceResolver {
	return validationInferenceResolver{
		indexes: indexes,
	}
}

func (r validationInferenceResolver) PropertyDefinition(
	id identity.UUID,
) (schema.PropertyDefinition, bool) {
	v, ok := r.indexes.propertyDefinitionByID[id]
	return v, ok
}

func (r validationInferenceResolver) PropertyContract(
	id identity.UUID,
) (schema.PropertyContract, bool) {
	v, ok := r.indexes.propertyContractByID[id]
	return v, ok
}

func (r validationInferenceResolver) EntityType(
	id identity.UUID,
) (schema.EntityType, bool) {
	v, ok := r.indexes.entityTypeByID[id]
	return v, ok
}

func (r validationInferenceResolver) RelationType(
	id identity.UUID,
) (schema.RelationType, bool) {
	v, ok := r.indexes.relationTypeByID[id]
	return v, ok
}

func (r validationInferenceResolver) ObjectType(
	object address.ObjectRef,
) (schema.PropertyOwner, bool) {
	if !object.IsValid() {
		return schema.PropertyOwner{}, false
	}

	switch object.Kind() {
	case address.ObjectRefEntity:
		entity, ok := r.indexes.entityByID[object.ID()]
		if !ok {
			return schema.PropertyOwner{}, false
		}

		if _, ok := r.indexes.entityTypeByID[entity.TypeID()]; !ok {
			return schema.PropertyOwner{}, false
		}

		return schema.EntityTypeOwner(entity.TypeID()), true

	case address.ObjectRefRelation:
		relation, ok := r.indexes.relationByID[object.ID()]
		if !ok {
			return schema.PropertyOwner{}, false
		}

		if _, ok := r.indexes.relationTypeByID[relation.TypeID()]; !ok {
			return schema.PropertyOwner{}, false
		}

		return schema.RelationTypeOwner(relation.TypeID()), true

	default:
		return schema.PropertyOwner{}, false
	}
}

func (r validationInferenceResolver) ContractImplementation(
	owner schema.PropertyOwner,
	contractID identity.UUID,
) (schema.PropertyDefinition, bool) {
	if !owner.IsValid() || contractID.IsZero() {
		return schema.PropertyDefinition{}, false
	}

	contract, ok := r.indexes.propertyContractByID[contractID]
	if !ok || !contract.IsValid() {
		return schema.PropertyDefinition{}, false
	}

	var result schema.PropertyDefinition
	count := 0

	for _, definition := range r.indexes.propertyDefinitionByID {
		if !definition.IsValid() || definition.Owner() != owner {
			continue
		}

		candidateContractID, hasContract := definition.ContractID()
		if !hasContract || candidateContractID != contractID {
			continue
		}

		count++
		if count > 1 {
			return schema.PropertyDefinition{}, false
		}

		if !definitionCompatibleWithContract(
			definition,
			contract,
		) {
			return schema.PropertyDefinition{}, false
		}

		result = definition
	}

	if count != 1 {
		return schema.PropertyDefinition{}, false
	}

	return result, true
}

func (r validationInferenceResolver) ContractType(
	constraint schema.TypeConstraint,
	contractID identity.UUID,
) (typing.Type, bool) {
	if !constraint.IsValid() || contractID.IsZero() {
		return typing.Type{}, false
	}

	contract, ok := r.indexes.propertyContractByID[contractID]
	if !ok || !contract.IsValid() {
		return typing.Type{}, false
	}

	entityTypes := r.entityTypesForConstraint(constraint)
	if len(entityTypes) == 0 {
		return typing.Type{}, false
	}

	var result typing.Type
	hasResult := false

	// entityTypesForConstraint returns EntityTypes in stable UUID order.
	//
	// typecheck.Unify intentionally preserves the left Unit when compatible
	// Units differ. Stable iteration therefore gives ContractType a
	// deterministic common representation without depending on map or Draft
	// insertion order.
	//
	// Example:
	//
	//	Board.price   -> Decimal<RUB>
	//	Service.price -> Decimal<kopeck>
	//
	// may become one common Decimal<RUB> (or Decimal<kopeck>, depending only
	// on stable EntityType identity ordering), provided both Units belong to
	// the same exact automatic conversion space.
	//
	// Runtime evaluation of BoundContractProperty must normalize each
	// concrete implementation value into this inferred common Unit before the
	// enclosing expression consumes it.
	for _, entityType := range entityTypes {
		definition, ok := r.ContractImplementation(
			schema.EntityTypeOwner(entityType.ID()),
			contractID,
		)
		if !ok {
			return typing.Type{}, false
		}

		current, err := inference.TypeFromPropertyDefinition(
			definition,
			r,
		)
		if err != nil || !current.IsValid() {
			return typing.Type{}, false
		}

		if !hasResult {
			result = current
			hasResult = true
			continue
		}

		unified, err := typecheck.Unify(
			result,
			current,
		)
		if err != nil || !unified.IsValid() {
			return typing.Type{}, false
		}

		result = unified
	}

	if !hasResult {
		return typing.Type{}, false
	}

	if result.ValueType() != contract.ValueType() {
		// Integer implementations may be widened to Decimal when different
		// compatible Units require normalization that can produce fractional
		// values.
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

func (r validationInferenceResolver) entityTypesForConstraint(
	constraint schema.TypeConstraint,
) []schema.EntityType {
	var result []schema.EntityType

	switch constraint.Kind() {
	case schema.TypeConstraintEntityType:
		id, ok := constraint.ID()
		if !ok {
			return nil
		}

		entityType, exists := r.indexes.entityTypeByID[id]
		if !exists {
			return nil
		}

		result = append(
			result,
			entityType,
		)

	case schema.TypeConstraintTrait:
		traitID, ok := constraint.ID()
		if !ok {
			return nil
		}

		if _, exists := r.indexes.traitByID[traitID]; !exists {
			return nil
		}

		for _, entityType := range r.indexes.entityTypeByID {
			if entityTypeHasTrait(
				entityType,
				traitID,
			) {
				result = append(
					result,
					entityType,
				)
			}
		}

	case schema.TypeConstraintAnyEntity:
		for _, entityType := range r.indexes.entityTypeByID {
			result = append(
				result,
				entityType,
			)
		}

	default:
		return nil
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			return result[i].ID().String() <
				result[j].ID().String()
		},
	)

	return result
}

func definitionCompatibleWithContract(
	definition schema.PropertyDefinition,
	contract schema.PropertyContract,
) bool {
	if !definition.IsValid() || !contract.IsValid() {
		return false
	}

	contractID, hasContract := definition.ContractID()
	if !hasContract || contractID != contract.ID() {
		return false
	}

	if definition.ValueType() != contract.ValueType() ||
		definition.TemporalMode() != contract.TemporalMode() {
		return false
	}

	requiredDimension, hasDimension := contract.Dimension()
	if !hasDimension {
		return true
	}

	propertyUnit, hasUnit := definition.Unit()
	if !hasUnit {
		return false
	}

	return propertyUnit.Dimension().Equal(
		requiredDimension,
	)
}

func typingTemporalModeForContract(
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

var _ inference.Resolver = validationInferenceResolver{}
