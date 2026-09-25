package inference

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
)

// Context describes the static environment in which an expression is
// inferred.
//
// Context contains no model data. Schema/model lookup belongs to Resolver.
//
// Context contains only lexical/static bindings:
//
//   - self: the schema owner whose expression is currently being checked;
//   - aggregate binding: the Entity and Relation visible through bound
//     references while checking an aggregate filter or map.
//
// Context values are immutable. Methods that introduce or remove bindings
// return modified copies.
type Context struct {
	self    schema.PropertyOwner
	hasSelf bool

	binding    Binding
	hasBinding bool
}

type EntityBinding struct {
	constraints []schema.TypeConstraint
}

// Binding describes the objects exposed by one Aggregate traversal.
//
// Every traversal item binds:
//
//   - one concrete traversed RelationType;
//   - one Entity selected from one or more possible endpoint constraints.
//
// Relation is always represented by one concrete PropertyOwner because a
// TraversalSource names one concrete RelationType.
//
// Entity may be heterogeneous and is therefore represented by EntityBinding
// rather than by one fabricated EntityType.
type Binding struct {
	relationOwner schema.PropertyOwner
	entity        EntityBinding
}

// NewContext constructs an inference context with a self owner and no
// aggregate binding.
func NewContext(
	self schema.PropertyOwner,
) Context {
	if !self.IsValid() {
		panic("inference: self owner must be valid")
	}

	return Context{
		self:    self,
		hasSelf: true,
	}
}

// EmptyContext constructs a context without self or aggregate bindings.
//
// This is useful for expressions that are not owned by a
// PropertyDefinition.
func EmptyContext() Context {
	return Context{}
}

// Self returns the current self owner.
//
// The boolean result reports whether self is available.
func (c Context) Self() (schema.PropertyOwner, bool) {
	return c.self, c.hasSelf
}

// Binding returns the current aggregate binding.
//
// The boolean result reports whether inference is currently inside an
// aggregate filter or map.
func (c Context) Binding() (Binding, bool) {
	return c.binding, c.hasBinding
}

// WithBinding returns a copy of c containing binding.
//
// Existing self is preserved.
//
// Nested aggregates naturally shadow the outer aggregate binding while
// retaining the same self environment.
func (c Context) WithBinding(
	binding Binding,
) Context {
	if !binding.IsValid() {
		panic("inference: aggregate binding must be valid")
	}

	c.binding = binding
	c.hasBinding = true

	return c
}

// WithoutBinding returns a copy of c without an aggregate binding.
//
// Self, when present, is preserved.
func (c Context) WithoutBinding() Context {
	c.binding = Binding{}
	c.hasBinding = false

	return c
}

// IsValid reports whether c is structurally valid.
func (c Context) IsValid() bool {
	if c.hasSelf {
		if !c.self.IsValid() {
			return false
		}
	} else if c.self.IsValid() {
		return false
	}

	if c.hasBinding {
		if !c.binding.IsValid() {
			return false
		}
	} else if c.binding.IsValid() {
		return false
	}

	return true
}

// NewEntityBinding constructs an entity binding from all endpoint
// constraints that may produce bound.Entity.
//
// At least one valid constraint is required.
//
// Duplicate constraints are removed while preserving first occurrence order.
// This is useful for bidirectional traversals whose From and To constraints
// are identical.
func NewEntityBinding(
	constraints ...schema.TypeConstraint,
) EntityBinding {
	if len(constraints) == 0 {
		panic("inference: entity binding requires at least one constraint")
	}

	result := make(
		[]schema.TypeConstraint,
		0,
		len(constraints),
	)

	for _, constraint := range constraints {
		if !constraint.IsValid() {
			panic("inference: entity binding constraint must be valid")
		}

		if containsTypeConstraint(result, constraint) {
			continue
		}

		result = append(result, constraint)
	}

	return EntityBinding{
		constraints: result,
	}
}

// Constraints returns every possible entity constraint.
//
// A copy is returned to preserve immutability.
func (b EntityBinding) Constraints() []schema.TypeConstraint {
	result := make(
		[]schema.TypeConstraint,
		len(b.constraints),
	)

	copy(result, b.constraints)

	return result
}

// ConcreteOwner returns the EntityType owner when every possible constraint
// resolves to the same concrete EntityType.
//
// This succeeds for:
//
//	EntityType(Board)
//
// and also for a deduplicated binding such as:
//
//	EntityType(Board) OR EntityType(Board)
//
// It intentionally fails for:
//
//	Trait(Priced)
//	AnyEntity
//	EntityType(Board) OR EntityType(Nail)
//
// A concrete PropertyDefinition reference is only safe when one concrete
// owner is statically known.
func (b EntityBinding) ConcreteOwner() (
	schema.PropertyOwner,
	bool,
) {
	if !b.IsValid() {
		return schema.PropertyOwner{}, false
	}

	var owner schema.PropertyOwner
	hasOwner := false

	for _, constraint := range b.constraints {
		if constraint.Kind() !=
			schema.TypeConstraintEntityType {
			return schema.PropertyOwner{}, false
		}

		entityTypeID, ok := constraint.ID()
		if !ok {
			return schema.PropertyOwner{}, false
		}

		current := schema.EntityTypeOwner(entityTypeID)

		if !hasOwner {
			owner = current
			hasOwner = true
			continue
		}

		if !propertyOwnersEqual(owner, current) {
			return schema.PropertyOwner{}, false
		}
	}

	if !hasOwner {
		return schema.PropertyOwner{}, false
	}

	return owner, true
}

// IsValid reports whether b is structurally valid.
func (b EntityBinding) IsValid() bool {
	if len(b.constraints) == 0 {
		return false
	}

	for i, constraint := range b.constraints {
		if !constraint.IsValid() {
			return false
		}

		for j := 0; j < i; j++ {
			if typeConstraintsEqual(
				b.constraints[j],
				constraint,
			) {
				return false
			}
		}
	}

	return true
}

// NewBinding constructs an aggregate binding.
//
// relationOwner must identify the concrete RelationType selected by the
// traversal.
//
// entityBinding describes every endpoint constraint that may be visible as
// bound.Entity.
func NewBinding(
	relationOwner schema.PropertyOwner,
	entityBinding EntityBinding,
) Binding {
	if !relationOwner.IsValid() {
		panic("inference: bound relation owner must be valid")
	}

	if relationOwner.Kind() !=
		schema.PropertyOwnerRelationType {
		panic("inference: bound relation owner must be a relation type")
	}

	if !entityBinding.IsValid() {
		panic("inference: bound entity binding must be valid")
	}

	return Binding{
		relationOwner: relationOwner,
		entity:        entityBinding,
	}
}

// RelationOwner returns the concrete RelationType owner bound as
// bound.Relation.
func (b Binding) RelationOwner() schema.PropertyOwner {
	return b.relationOwner
}

// Entity returns the entity binding exposed as bound.Entity.
func (b Binding) Entity() EntityBinding {
	return b.entity
}

// TargetOwner returns one concrete schema owner for target when such an
// owner is statically known.
//
// Relation is always concrete.
//
// Entity is concrete only when every possible entity constraint identifies
// the same concrete EntityType.
func (b Binding) TargetOwner(
	target ast.BoundTarget,
) (schema.PropertyOwner, bool) {
	switch target {
	case ast.BoundTargetRelation:
		if !b.relationOwner.IsValid() {
			return schema.PropertyOwner{}, false
		}

		return b.relationOwner, true

	case ast.BoundTargetEntity:
		return b.entity.ConcreteOwner()

	default:
		return schema.PropertyOwner{}, false
	}
}

// IsValid reports whether b is structurally valid.
func (b Binding) IsValid() bool {
	if !b.relationOwner.IsValid() ||
		b.relationOwner.Kind() !=
			schema.PropertyOwnerRelationType {
		return false
	}

	return b.entity.IsValid()
}

func containsTypeConstraint(
	constraints []schema.TypeConstraint,
	target schema.TypeConstraint,
) bool {
	for _, constraint := range constraints {
		if typeConstraintsEqual(constraint, target) {
			return true
		}
	}

	return false
}

func typeConstraintsEqual(
	left schema.TypeConstraint,
	right schema.TypeConstraint,
) bool {
	if !left.IsValid() || !right.IsValid() {
		return false
	}

	if left.Kind() != right.Kind() {
		return false
	}

	leftID, leftHasID := left.ID()
	rightID, rightHasID := right.ID()

	if leftHasID != rightHasID {
		return false
	}

	if !leftHasID {
		return true
	}

	return leftID == rightID
}
