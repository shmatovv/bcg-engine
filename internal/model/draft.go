package model

import (
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
)

// Draft is a mutable-in-the-large description of a model before commit.
//
// Draft may temporarily contain incomplete or inconsistent data while the
// model is being assembled. Model-wide consistency is checked by Validate.
//
// Internal slices are never exposed directly.
type Draft struct {
	// Schema.
	entityTypes         []schema.EntityType
	relationTypes       []schema.RelationType
	traits              []schema.Trait
	propertyContracts   []schema.PropertyContract
	propertyDefinitions []schema.PropertyDefinition
	scopeDefinitions    []scope.Definition

	// Semantic data.
	entities  []semantic.Entity
	relations []semantic.Relation

	// Base values.
	valueBindings []semantic.ValueBinding

	// Declarative business constraints.
	constraints []constraint.Constraint

	// Declarative side-effect-free rules.
	rules []rule.Rule
}

// NewDraft constructs an empty model draft.
func NewDraft() *Draft {
	return &Draft{}
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

func (d *Draft) EntityTypes() []schema.EntityType {
	d.requireNonNil()

	return cloneSlice(d.entityTypes)
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

func (d *Draft) IsEmpty() bool {
	d.requireNonNil()

	return len(d.entityTypes) == 0 &&
		len(d.relationTypes) == 0 &&
		len(d.traits) == 0 &&
		len(d.propertyContracts) == 0 &&
		len(d.propertyDefinitions) == 0 &&
		len(d.scopeDefinitions) == 0 &&
		len(d.entities) == 0 &&
		len(d.relations) == 0 &&
		len(d.valueBindings) == 0 &&
		len(d.constraints) == 0 &&
		len(d.rules) == 0
}

func (d *Draft) requireNonNil() {
	if d == nil {
		panic("model: nil draft")
	}
}
