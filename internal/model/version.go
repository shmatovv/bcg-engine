package model

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/v2/internal/function"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
)

type ModelVersion struct {
	id      identity.UUID
	modelID identity.UUID

	parentVersionID identity.UUID
	hasParent       bool

	versionLabel string

	precisionPolicy precision.Policy

	entityTypes          []schema.EntityType
	enumDefinitions      []schema.EnumDefinition
	entityKeyDefinitions []schema.EntityKeyDefinition
	relationTypes        []schema.RelationType
	traits               []schema.Trait
	propertyContracts    []schema.PropertyContract
	propertyDefinitions  []schema.PropertyDefinition
	scopeDefinitions     []scope.Definition

	entities  []semantic.Entity
	relations []semantic.Relation

	valueBindings []semantic.ValueBinding

	constraints []constraint.Constraint

	rules                []rule.Rule
	invariants           []invariant.Invariant
	functionRequirements []function.Requirement

	indexes indexes
}

func newCommittedModelVersion(
	id identity.UUID,
	modelID identity.UUID,
	parentVersionID identity.UUID,
	hasParent bool,
	versionLabel string,
	precisionPolicy precision.Policy,
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
	valueBindings []semantic.ValueBinding,
	constraints []constraint.Constraint,
	rules []rule.Rule,
	invariants []invariant.Invariant,
	functionRequirements []function.Requirement,
) ModelVersion {
	if id.IsZero() {
		panic("model: committed model version id must not be zero")
	}

	if modelID.IsZero() {
		panic("model: committed model id must not be zero")
	}

	if !precisionPolicy.IsValid() {
		panic("model: committed precision policy must be valid")
	}

	if hasParent {
		if parentVersionID.IsZero() {
			panic("model: committed parent version id must not be zero")
		}

		if parentVersionID == id {
			panic("model: model version cannot be its own parent")
		}
	} else if !parentVersionID.IsZero() {
		panic("model: parent version id present without parent flag")
	}

	return newModelVersionSnapshot(
		id,
		modelID,
		parentVersionID,
		hasParent,
		versionLabel,
		precisionPolicy,
		entityTypes,
		enumDefinitions,
		entityKeyDefinitions,
		relationTypes,
		traits,
		propertyContracts,
		propertyDefinitions,
		scopeDefinitions,
		entities,
		relations,
		valueBindings,
		constraints,
		rules,
		invariants,
		functionRequirements,
	)
}

func newModelVersion(
	entityTypes []schema.EntityType,
	relationTypes []schema.RelationType,
	traits []schema.Trait,
	propertyContracts []schema.PropertyContract,
	propertyDefinitions []schema.PropertyDefinition,
	scopeDefinitions []scope.Definition,
	entities []semantic.Entity,
	relations []semantic.Relation,
	valueBindings []semantic.ValueBinding,
) ModelVersion {
	return newModelVersionSnapshot(
		identity.UUID{},
		identity.UUID{},
		identity.UUID{},
		false,
		"",
		precision.Policy{},
		entityTypes,
		nil,
		nil,
		relationTypes,
		traits,
		propertyContracts,
		propertyDefinitions,
		scopeDefinitions,
		entities,
		relations,
		valueBindings,
		nil,
		nil,
		nil,
		nil,
	)
}

func newModelVersionSnapshot(
	id identity.UUID,
	modelID identity.UUID,
	parentVersionID identity.UUID,
	hasParent bool,
	versionLabel string,
	precisionPolicy precision.Policy,
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
	valueBindings []semantic.ValueBinding,
	constraints []constraint.Constraint,
	rules []rule.Rule,
	invariants []invariant.Invariant,
	functionRequirements []function.Requirement,
) ModelVersion {
	entityTypes = cloneSlice(entityTypes)
	enumDefinitions = cloneSlice(enumDefinitions)
	entityKeyDefinitions = cloneSlice(entityKeyDefinitions)
	relationTypes = cloneSlice(relationTypes)
	traits = cloneSlice(traits)
	propertyContracts = cloneSlice(propertyContracts)
	propertyDefinitions = cloneSlice(propertyDefinitions)
	scopeDefinitions = cloneSlice(scopeDefinitions)
	entities = cloneSlice(entities)
	relations = cloneSlice(relations)
	valueBindings = cloneSlice(valueBindings)
	constraints = cloneSlice(constraints)
	rules = cloneSlice(rules)
	invariants = cloneSlice(invariants)
	functionRequirements = cloneSlice(functionRequirements)

	result := ModelVersion{
		id:              id,
		modelID:         modelID,
		parentVersionID: parentVersionID,
		hasParent:       hasParent,
		versionLabel:    versionLabel,
		precisionPolicy: precisionPolicy,

		entityTypes:          entityTypes,
		enumDefinitions:      enumDefinitions,
		entityKeyDefinitions: entityKeyDefinitions,
		relationTypes:        relationTypes,
		traits:               traits,
		propertyContracts:    propertyContracts,
		propertyDefinitions:  propertyDefinitions,
		scopeDefinitions:     scopeDefinitions,
		entities:             entities,
		relations:            relations,
		valueBindings:        valueBindings,
		constraints:          constraints,
		rules:                rules,
		invariants:           invariants,
		functionRequirements: functionRequirements,

		indexes: buildIndexes(
			entityTypes,
			enumDefinitions,
			entityKeyDefinitions,
			relationTypes,
			traits,
			propertyContracts,
			propertyDefinitions,
			scopeDefinitions,
			entities,
			relations,
			constraints,
			rules,
			invariants,
			valueBindings,
			functionRequirements,
		),
	}

	dependencies, reverse, err := buildConstraintDependencyIndexes(result)
	if err != nil {
		panic("model: Constraint dependency extraction failed for internally constructed snapshot")
	}
	result.indexes.constraintDependenciesByID = dependencies
	result.indexes.constraintIDsByDependency = reverse

	ruleDependencies, ruleReverse, err := buildRuleDependencyIndexes(result)
	if err != nil {
		panic("model: Rule dependency extraction failed for internally constructed snapshot")
	}
	result.indexes.ruleDependenciesByID = ruleDependencies
	result.indexes.ruleIDsByDependency = ruleReverse

	return result
}

func (v ModelVersion) ID() identity.UUID {
	return v.id
}

// ModelVersion makes a model-only source expose its underlying version through
// the same narrow runtime capability as a combined EvaluationSource.
func (v ModelVersion) ModelVersion() ModelVersion { return v }

// SourceIdentity identifies model-only evaluation against this version.
func (v ModelVersion) SourceIdentity() EffectiveSourceIdentity {
	if !v.IsCommitted() {
		return EffectiveSourceIdentity{}
	}
	return NewModelSourceIdentity(v.id)
}

func (v ModelVersion) ModelID() identity.UUID {
	return v.modelID
}

func (v ModelVersion) ParentVersionID() (
	identity.UUID,
	bool,
) {
	return v.parentVersionID, v.hasParent
}

func (v ModelVersion) VersionLabel() string {
	return v.versionLabel
}

func (v ModelVersion) PrecisionPolicy() precision.Policy {
	return v.precisionPolicy
}

func (v ModelVersion) IsCommitted() bool {
	if v.id.IsZero() ||
		v.modelID.IsZero() ||
		!v.precisionPolicy.IsValid() {
		return false
	}

	if v.hasParent {
		return !v.parentVersionID.IsZero() &&
			v.parentVersionID != v.id
	}

	return v.parentVersionID.IsZero()
}

func (v ModelVersion) EntityTypes() []schema.EntityType {
	return cloneSlice(v.entityTypes)
}

func (v ModelVersion) EnumDefinitions() []schema.EnumDefinition { return cloneSlice(v.enumDefinitions) }
func (v ModelVersion) EntityKeyDefinitions() []schema.EntityKeyDefinition {
	return cloneSlice(v.entityKeyDefinitions)
}

func (v ModelVersion) RelationTypes() []schema.RelationType {
	return cloneSlice(v.relationTypes)
}

func (v ModelVersion) Traits() []schema.Trait {
	return cloneSlice(v.traits)
}

func (v ModelVersion) PropertyContracts() []schema.PropertyContract {
	return cloneSlice(v.propertyContracts)
}

func (v ModelVersion) PropertyDefinitions() []schema.PropertyDefinition {
	return cloneSlice(v.propertyDefinitions)
}

func (v ModelVersion) ScopeDefinitions() []scope.Definition {
	return cloneSlice(v.scopeDefinitions)
}

func (v ModelVersion) Entities() []semantic.Entity {
	return cloneSlice(v.entities)
}

func (v ModelVersion) Relations() []semantic.Relation {
	return cloneSlice(v.relations)
}

// OutgoingRelations returns the immutable model's Relations of relationTypeID
// whose From endpoint is entityID. Results are in canonical Relation UUID
// order and are defensively copied. This is semantic-graph membership data,
// not a value dependency query.
func (v ModelVersion) OutgoingRelations(
	entityID identity.UUID,
	relationTypeID identity.UUID,
) []semantic.Relation {
	if entityID.IsZero() || relationTypeID.IsZero() {
		return nil
	}

	return cloneSlice(v.indexes.outgoingRelationsByEndpoint[relationTraversalKey{
		entityID:       entityID,
		relationTypeID: relationTypeID,
	}])
}

// IncomingRelations returns the immutable model's Relations of relationTypeID
// whose To endpoint is entityID. Results are in canonical Relation UUID order
// and are defensively copied. This is semantic-graph membership data, not a
// value dependency query.
func (v ModelVersion) IncomingRelations(
	entityID identity.UUID,
	relationTypeID identity.UUID,
) []semantic.Relation {
	if entityID.IsZero() || relationTypeID.IsZero() {
		return nil
	}

	return cloneSlice(v.indexes.incomingRelationsByEndpoint[relationTraversalKey{
		entityID:       entityID,
		relationTypeID: relationTypeID,
	}])
}

func (v ModelVersion) ValueBindings() []semantic.ValueBinding {
	return cloneSlice(v.valueBindings)
}

func (v ModelVersion) Constraints() []constraint.Constraint {
	return cloneSlice(v.constraints)
}

func (v ModelVersion) Rules() []rule.Rule {
	return cloneSlice(v.rules)
}
func (v ModelVersion) Invariants() []invariant.Invariant { return cloneSlice(v.invariants) }
func (v ModelVersion) FunctionRequirements() []function.Requirement {
	return cloneSlice(v.functionRequirements)
}
func (v ModelVersion) FunctionRequirement(ref function.Ref) (function.Requirement, bool) {
	value, ok := v.indexes.functionRequirementByRef[ref.Key()]
	return value, ok
}

func (v ModelVersion) EntityType(
	id identity.UUID,
) (schema.EntityType, bool) {
	result, ok :=
		v.indexes.entityTypeByID[id]

	return result, ok
}
func (v ModelVersion) EnumDefinition(id identity.UUID) (schema.EnumDefinition, bool) {
	result, ok := v.indexes.enumDefinitionByID[id]
	return result, ok
}
func (v ModelVersion) EntityKeyDefinition(id identity.UUID) (schema.EntityKeyDefinition, bool) {
	result, ok := v.indexes.entityKeyDefinitionByID[id]
	return result, ok
}

func (v ModelVersion) RelationType(
	id identity.UUID,
) (schema.RelationType, bool) {
	result, ok :=
		v.indexes.relationTypeByID[id]

	return result, ok
}

func (v ModelVersion) Trait(
	id identity.UUID,
) (schema.Trait, bool) {
	result, ok :=
		v.indexes.traitByID[id]

	return result, ok
}

func (v ModelVersion) PropertyContract(
	id identity.UUID,
) (schema.PropertyContract, bool) {
	result, ok :=
		v.indexes.propertyContractByID[id]

	return result, ok
}

func (v ModelVersion) PropertyDefinition(
	id identity.UUID,
) (schema.PropertyDefinition, bool) {
	result, ok :=
		v.indexes.propertyDefinitionByID[id]

	return result, ok
}

func (v ModelVersion) ScopeDefinition(
	id identity.UUID,
) (scope.Definition, bool) {
	result, ok :=
		v.indexes.scopeDefinitionByID[id]

	return result, ok
}

func (v ModelVersion) Entity(
	id identity.UUID,
) (semantic.Entity, bool) {
	result, ok :=
		v.indexes.entityByID[id]

	return result, ok
}

func (v ModelVersion) Relation(
	id identity.UUID,
) (semantic.Relation, bool) {
	result, ok :=
		v.indexes.relationByID[id]

	return result, ok
}

func (v ModelVersion) Constraint(
	id identity.UUID,
) (constraint.Constraint, bool) {
	result, ok :=
		v.indexes.constraintByID[id]

	return result, ok
}

func (v ModelVersion) Rule(
	id identity.UUID,
) (rule.Rule, bool) {
	result, ok :=
		v.indexes.ruleByID[id]

	return result, ok
}
func (v ModelVersion) Invariant(id identity.UUID) (invariant.Invariant, bool) {
	result, ok := v.indexes.invariantByID[id]
	return result, ok
}

func (v ModelVersion) EntityTypesImplementingTrait(
	traitID identity.UUID,
) []schema.EntityType {
	return cloneSlice(
		v.indexes.entityTypesByTrait[traitID],
	)
}

func (v ModelVersion) ContractImplementation(
	owner schema.PropertyOwner,
	contractID identity.UUID,
) (schema.PropertyDefinition, bool) {
	if !owner.IsValid() ||
		contractID.IsZero() {
		return schema.PropertyDefinition{},
			false
	}

	definitions, ok :=
		v.indexes.
			contractImplementationByOwner[owner]

	if !ok {
		return schema.PropertyDefinition{},
			false
	}

	definition, ok :=
		definitions[contractID]

	return definition, ok
}

func cloneSlice[T any](
	source []T,
) []T {
	if len(source) == 0 {
		return nil
	}

	result := make(
		[]T,
		len(source),
	)

	copy(
		result,
		source,
	)

	return result
}

// ValueBindingsForTarget returns an owned slice of base bindings for one exact
// object or type target. Scope/time selection remains the resolver's concern.
func (v ModelVersion) ValueBindingsForTarget(target semantic.ValueTarget) []semantic.ValueBinding {
	return cloneSlice(v.indexes.valueBindingsByTarget[target])
}
