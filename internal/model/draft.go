package model

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
)

// Draft is a mutable-in-the-large description of a model before commit.
//
// Draft may temporarily contain incomplete or inconsistent data while the
// model is being assembled. Model-wide consistency is checked by Validate.
//
// Internal slices are never exposed directly.
type Draft struct {
	// Schema.
	entityTypes          []schema.EntityType
	enumDefinitions      []schema.EnumDefinition
	entityKeyDefinitions []schema.EntityKeyDefinition
	relationTypes        []schema.RelationType
	traits               []schema.Trait
	propertyContracts    []schema.PropertyContract
	propertyDefinitions  []schema.PropertyDefinition
	scopeDefinitions     []scope.Definition

	// Semantic data.
	entities  []semantic.Entity
	relations []semantic.Relation

	// Base values.
	valueBindings []semantic.ValueBinding

	// Declarative business constraints.
	constraints []constraint.Constraint

	// Declarative side-effect-free rules.
	rules      []rule.Rule
	invariants []invariant.Invariant
}

// NewDraft constructs an empty model draft.
func NewDraft() *Draft {
	return &Draft{}
}

// NewDraftFromVersion constructs an independent editable copy of a committed
// model version.
//
// The returned Draft owns copies of every top-level collection. Domain values
// stored in those collections are immutable, so this is sufficient to keep
// subsequent Draft changes isolated from the committed snapshot. Commit makes
// another defensive snapshot before publishing its result.
//
// NewDraftFromVersion panics when version is not committed. As with
// NewDraft, this is an internal domain-construction boundary; public callers
// are protected by the root facade before reaching it.
func NewDraftFromVersion(version ModelVersion) *Draft {
	if !version.IsCommitted() {
		panic("model: draft source version must be committed")
	}

	return &Draft{
		entityTypes:          version.EntityTypes(),
		enumDefinitions:      version.EnumDefinitions(),
		entityKeyDefinitions: version.EntityKeyDefinitions(),
		relationTypes:        version.RelationTypes(),
		traits:               version.Traits(),
		propertyContracts:    version.PropertyContracts(),
		propertyDefinitions:  version.PropertyDefinitions(),
		scopeDefinitions:     version.ScopeDefinitions(),
		entities:             version.Entities(),
		relations:            version.Relations(),
		valueBindings:        version.ValueBindings(),
		constraints:          version.Constraints(),
		rules:                version.Rules(),
		invariants:           version.Invariants(),
	}
}

func (d *Draft) AddEnumDefinition(value schema.EnumDefinition) {
	d.requireNonNil()
	d.enumDefinitions = append(d.enumDefinitions, value)
}
func (d *Draft) AddEntityKeyDefinition(value schema.EntityKeyDefinition) {
	d.requireNonNil()
	d.entityKeyDefinitions = append(d.entityKeyDefinitions, value)
}
func (d *Draft) AddInvariant(value invariant.Invariant) {
	d.requireNonNil()
	d.invariants = append(d.invariants, value)
}

func (d *Draft) AddEntityType(
	entityType schema.EntityType,
) {
	d.requireNonNil()

	d.entityTypes = append(
		d.entityTypes,
		entityType,
	)
}

func (d *Draft) AddRelationType(
	relationType schema.RelationType,
) {
	d.requireNonNil()

	d.relationTypes = append(
		d.relationTypes,
		relationType,
	)
}

func (d *Draft) AddTrait(
	trait schema.Trait,
) {
	d.requireNonNil()

	d.traits = append(
		d.traits,
		trait,
	)
}

func (d *Draft) AddPropertyContract(
	contract schema.PropertyContract,
) {
	d.requireNonNil()

	d.propertyContracts = append(
		d.propertyContracts,
		contract,
	)
}

func (d *Draft) AddPropertyDefinition(
	definition schema.PropertyDefinition,
) {
	d.requireNonNil()

	d.propertyDefinitions = append(
		d.propertyDefinitions,
		definition,
	)
}

func (d *Draft) AddScopeDefinition(
	definition scope.Definition,
) {
	d.requireNonNil()

	d.scopeDefinitions = append(
		d.scopeDefinitions,
		definition,
	)
}

func (d *Draft) AddEntity(
	entity semantic.Entity,
) {
	d.requireNonNil()

	d.entities = append(
		d.entities,
		entity,
	)
}

func (d *Draft) AddRelation(
	relation semantic.Relation,
) {
	d.requireNonNil()

	d.relations = append(
		d.relations,
		relation,
	)
}

func (d *Draft) AddValueBinding(
	binding semantic.ValueBinding,
) {
	d.requireNonNil()

	d.valueBindings = append(
		d.valueBindings,
		binding,
	)
}

func (d *Draft) AddConstraint(
	modelConstraint constraint.Constraint,
) {
	d.requireNonNil()

	d.constraints = append(
		d.constraints,
		modelConstraint,
	)
}

// AddRule adds one immutable declarative Rule to the draft.
// Rule condition references are validated model-wide by Validate.
func (d *Draft) AddRule(
	modelRule rule.Rule,
) {
	d.requireNonNil()

	d.rules = append(
		d.rules,
		modelRule,
	)
}

// The replacement and removal operations below are deliberately tolerant of
// invalid replacement values. Draft is an authoring workspace, so a caller
// may replace a previously valid object with an incomplete one, inspect the
// resulting validation issues, and repair it before Commit. Identity selects
// the existing slot; it is not required to equal the replacement's identity.
// This makes the operations usable for both updates and deliberate invalid
// intermediate authoring states.

// ReplaceEntityType replaces the EntityType currently identified by id.
func (d *Draft) ReplaceEntityType(id identity.UUID, value schema.EntityType) bool {
	d.requireNonNil()
	var replaced bool
	d.entityTypes, replaced = replaceDraftItem(d.entityTypes, id, value, func(item schema.EntityType) identity.UUID { return item.ID() })
	return replaced
}

// RemoveEntityType removes the EntityType identified by id.
func (d *Draft) RemoveEntityType(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.entityTypes, removed = removeDraftItem(d.entityTypes, id, func(item schema.EntityType) identity.UUID { return item.ID() })
	return removed
}
func (d *Draft) ReplaceEnumDefinition(id identity.UUID, value schema.EnumDefinition) bool {
	d.requireNonNil()
	var ok bool
	d.enumDefinitions, ok = replaceDraftItem(d.enumDefinitions, id, value, func(v schema.EnumDefinition) identity.UUID { return v.ID() })
	return ok
}
func (d *Draft) RemoveEnumDefinition(id identity.UUID) bool {
	d.requireNonNil()
	var ok bool
	d.enumDefinitions, ok = removeDraftItem(d.enumDefinitions, id, func(v schema.EnumDefinition) identity.UUID { return v.ID() })
	return ok
}
func (d *Draft) ReplaceEntityKeyDefinition(id identity.UUID, value schema.EntityKeyDefinition) bool {
	d.requireNonNil()
	var ok bool
	d.entityKeyDefinitions, ok = replaceDraftItem(d.entityKeyDefinitions, id, value, func(v schema.EntityKeyDefinition) identity.UUID { return v.ID() })
	return ok
}
func (d *Draft) RemoveEntityKeyDefinition(id identity.UUID) bool {
	d.requireNonNil()
	var ok bool
	d.entityKeyDefinitions, ok = removeDraftItem(d.entityKeyDefinitions, id, func(v schema.EntityKeyDefinition) identity.UUID { return v.ID() })
	return ok
}
func (d *Draft) ReplaceInvariant(id identity.UUID, value invariant.Invariant) bool {
	d.requireNonNil()
	var ok bool
	d.invariants, ok = replaceDraftItem(d.invariants, id, value, func(v invariant.Invariant) identity.UUID { return v.ID() })
	return ok
}
func (d *Draft) RemoveInvariant(id identity.UUID) bool {
	d.requireNonNil()
	var ok bool
	d.invariants, ok = removeDraftItem(d.invariants, id, func(v invariant.Invariant) identity.UUID { return v.ID() })
	return ok
}

// ReplaceRelationType replaces the RelationType currently identified by id.
func (d *Draft) ReplaceRelationType(id identity.UUID, value schema.RelationType) bool {
	d.requireNonNil()
	var replaced bool
	d.relationTypes, replaced = replaceDraftItem(d.relationTypes, id, value, func(item schema.RelationType) identity.UUID { return item.ID() })
	return replaced
}

// RemoveRelationType removes the RelationType identified by id.
func (d *Draft) RemoveRelationType(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.relationTypes, removed = removeDraftItem(d.relationTypes, id, func(item schema.RelationType) identity.UUID { return item.ID() })
	return removed
}

// ReplaceTrait replaces the Trait currently identified by id.
func (d *Draft) ReplaceTrait(id identity.UUID, value schema.Trait) bool {
	d.requireNonNil()
	var replaced bool
	d.traits, replaced = replaceDraftItem(d.traits, id, value, func(item schema.Trait) identity.UUID { return item.ID() })
	return replaced
}

// RemoveTrait removes the Trait identified by id.
func (d *Draft) RemoveTrait(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.traits, removed = removeDraftItem(d.traits, id, func(item schema.Trait) identity.UUID { return item.ID() })
	return removed
}

// ReplacePropertyContract replaces the PropertyContract currently identified by id.
func (d *Draft) ReplacePropertyContract(id identity.UUID, value schema.PropertyContract) bool {
	d.requireNonNil()
	var replaced bool
	d.propertyContracts, replaced = replaceDraftItem(d.propertyContracts, id, value, func(item schema.PropertyContract) identity.UUID { return item.ID() })
	return replaced
}

// RemovePropertyContract removes the PropertyContract identified by id.
func (d *Draft) RemovePropertyContract(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.propertyContracts, removed = removeDraftItem(d.propertyContracts, id, func(item schema.PropertyContract) identity.UUID { return item.ID() })
	return removed
}

// ReplacePropertyDefinition replaces the PropertyDefinition currently identified by id.
func (d *Draft) ReplacePropertyDefinition(id identity.UUID, value schema.PropertyDefinition) bool {
	d.requireNonNil()
	var replaced bool
	d.propertyDefinitions, replaced = replaceDraftItem(d.propertyDefinitions, id, value, func(item schema.PropertyDefinition) identity.UUID { return item.ID() })
	return replaced
}

// RemovePropertyDefinition removes the PropertyDefinition identified by id.
func (d *Draft) RemovePropertyDefinition(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.propertyDefinitions, removed = removeDraftItem(d.propertyDefinitions, id, func(item schema.PropertyDefinition) identity.UUID { return item.ID() })
	return removed
}

// ReplaceScopeDefinition replaces the Scope Definition currently identified by id.
func (d *Draft) ReplaceScopeDefinition(id identity.UUID, value scope.Definition) bool {
	d.requireNonNil()
	var replaced bool
	d.scopeDefinitions, replaced = replaceDraftItem(d.scopeDefinitions, id, value, func(item scope.Definition) identity.UUID { return item.ID() })
	return replaced
}

// RemoveScopeDefinition removes the Scope Definition identified by id.
func (d *Draft) RemoveScopeDefinition(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.scopeDefinitions, removed = removeDraftItem(d.scopeDefinitions, id, func(item scope.Definition) identity.UUID { return item.ID() })
	return removed
}

// ReplaceEntity replaces the Entity currently identified by id.
func (d *Draft) ReplaceEntity(id identity.UUID, value semantic.Entity) bool {
	d.requireNonNil()
	var replaced bool
	d.entities, replaced = replaceDraftItem(d.entities, id, value, func(item semantic.Entity) identity.UUID { return item.ID() })
	return replaced
}

// RemoveEntity removes the Entity identified by id.
func (d *Draft) RemoveEntity(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.entities, removed = removeDraftItem(d.entities, id, func(item semantic.Entity) identity.UUID { return item.ID() })
	return removed
}

// ReplaceRelation replaces the Relation currently identified by id.
func (d *Draft) ReplaceRelation(id identity.UUID, value semantic.Relation) bool {
	d.requireNonNil()
	var replaced bool
	d.relations, replaced = replaceDraftItem(d.relations, id, value, func(item semantic.Relation) identity.UUID { return item.ID() })
	return replaced
}

// RemoveRelation removes the Relation identified by id.
func (d *Draft) RemoveRelation(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.relations, removed = removeDraftItem(d.relations, id, func(item semantic.Relation) identity.UUID { return item.ID() })
	return removed
}

// ReplaceValueBinding replaces the binding at index. The index comes from a
// previous ValueBindings result and remains stable until this Draft's binding
// collection is changed.
func (d *Draft) ReplaceValueBinding(index int, value semantic.ValueBinding) bool {
	d.requireNonNil()
	if index < 0 || index >= len(d.valueBindings) {
		return false
	}
	d.valueBindings[index] = value
	return true
}

// RemoveValueBinding removes the binding at index.
func (d *Draft) RemoveValueBinding(index int) bool {
	d.requireNonNil()
	if index < 0 || index >= len(d.valueBindings) {
		return false
	}
	copy(d.valueBindings[index:], d.valueBindings[index+1:])
	d.valueBindings[len(d.valueBindings)-1] = semantic.ValueBinding{}
	d.valueBindings = d.valueBindings[:len(d.valueBindings)-1]
	return true
}

// ReplaceConstraint replaces the Constraint currently identified by id.
func (d *Draft) ReplaceConstraint(id identity.UUID, value constraint.Constraint) bool {
	d.requireNonNil()
	var replaced bool
	d.constraints, replaced = replaceDraftItem(d.constraints, id, value, func(item constraint.Constraint) identity.UUID { return item.ID() })
	return replaced
}

// RemoveConstraint removes the Constraint identified by id.
func (d *Draft) RemoveConstraint(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.constraints, removed = removeDraftItem(d.constraints, id, func(item constraint.Constraint) identity.UUID { return item.ID() })
	return removed
}

// ReplaceRule replaces the Rule currently identified by id.
func (d *Draft) ReplaceRule(id identity.UUID, value rule.Rule) bool {
	d.requireNonNil()
	var replaced bool
	d.rules, replaced = replaceDraftItem(d.rules, id, value, func(item rule.Rule) identity.UUID { return item.ID() })
	return replaced
}

// RemoveRule removes the Rule identified by id.
func (d *Draft) RemoveRule(id identity.UUID) bool {
	d.requireNonNil()
	var removed bool
	d.rules, removed = removeDraftItem(d.rules, id, func(item rule.Rule) identity.UUID { return item.ID() })
	return removed
}

func replaceDraftItem[T any](items []T, id identity.UUID, replacement T, itemID func(T) identity.UUID) ([]T, bool) {
	if id.IsZero() {
		return items, false
	}
	for index, item := range items {
		if itemID(item) == id {
			items[index] = replacement
			return items, true
		}
	}
	return items, false
}

func removeDraftItem[T any](items []T, id identity.UUID, itemID func(T) identity.UUID) ([]T, bool) {
	if id.IsZero() {
		return items, false
	}
	for index, item := range items {
		if itemID(item) == id {
			copy(items[index:], items[index+1:])
			var zero T
			items[len(items)-1] = zero
			return items[:len(items)-1], true
		}
	}
	return items, false
}

func (d *Draft) EntityTypes() []schema.EntityType {
	d.requireNonNil()

	return cloneSlice(d.entityTypes)
}
func (d *Draft) EnumDefinitions() []schema.EnumDefinition {
	d.requireNonNil()
	return cloneSlice(d.enumDefinitions)
}
func (d *Draft) EntityKeyDefinitions() []schema.EntityKeyDefinition {
	d.requireNonNil()
	return cloneSlice(d.entityKeyDefinitions)
}

func (d *Draft) RelationTypes() []schema.RelationType {
	d.requireNonNil()

	return cloneSlice(d.relationTypes)
}

func (d *Draft) Traits() []schema.Trait {
	d.requireNonNil()

	return cloneSlice(d.traits)
}

func (d *Draft) PropertyContracts() []schema.PropertyContract {
	d.requireNonNil()

	return cloneSlice(d.propertyContracts)
}

func (d *Draft) PropertyDefinitions() []schema.PropertyDefinition {
	d.requireNonNil()

	return cloneSlice(d.propertyDefinitions)
}

func (d *Draft) ScopeDefinitions() []scope.Definition {
	d.requireNonNil()

	return cloneSlice(d.scopeDefinitions)
}

func (d *Draft) Entities() []semantic.Entity {
	d.requireNonNil()

	return cloneSlice(d.entities)
}

func (d *Draft) Relations() []semantic.Relation {
	d.requireNonNil()

	return cloneSlice(d.relations)
}

func (d *Draft) ValueBindings() []semantic.ValueBinding {
	d.requireNonNil()

	return cloneSlice(d.valueBindings)
}

func (d *Draft) Constraints() []constraint.Constraint {
	d.requireNonNil()

	return cloneSlice(d.constraints)
}

// Rules returns a copy of all declarative Rules currently in the draft.
func (d *Draft) Rules() []rule.Rule {
	d.requireNonNil()

	return cloneSlice(d.rules)
}
func (d *Draft) Invariants() []invariant.Invariant {
	d.requireNonNil()
	return cloneSlice(d.invariants)
}

func (d *Draft) IsEmpty() bool {
	d.requireNonNil()

	return len(d.entityTypes) == 0 &&
		len(d.enumDefinitions) == 0 && len(d.entityKeyDefinitions) == 0 &&
		len(d.relationTypes) == 0 &&
		len(d.traits) == 0 &&
		len(d.propertyContracts) == 0 &&
		len(d.propertyDefinitions) == 0 &&
		len(d.scopeDefinitions) == 0 &&
		len(d.entities) == 0 &&
		len(d.relations) == 0 &&
		len(d.valueBindings) == 0 &&
		len(d.constraints) == 0 &&
		len(d.rules) == 0 && len(d.invariants) == 0
}

func (d *Draft) requireNonNil() {
	if d == nil {
		panic("model: nil draft")
	}
}
