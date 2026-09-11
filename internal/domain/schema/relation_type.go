package schema

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// TypeConstraintKind identifies the kind of entity constraint
// used by a RelationType endpoint.
type TypeConstraintKind uint8

const (
	TypeConstraintInvalid TypeConstraintKind = iota
	TypeConstraintEntityType
	TypeConstraintTrait
	TypeConstraintAnyEntity
)

// IsValid reports whether k is a valid TypeConstraintKind.
func (k TypeConstraintKind) IsValid() bool {
	switch k {
	case TypeConstraintEntityType,
		TypeConstraintTrait,
		TypeConstraintAnyEntity:
		return true
	default:
		return false
	}
}

// String returns the canonical name of k.
func (k TypeConstraintKind) String() string {
	switch k {
	case TypeConstraintEntityType:
		return "EntityType"
	case TypeConstraintTrait:
		return "Trait"
	case TypeConstraintAnyEntity:
		return "AnyEntity"
	default:
		return "Invalid"
	}
}

// TypeConstraint restricts which Entity may occupy one endpoint
// of a Relation.
//
// A constraint may refer to a concrete EntityType, require a Trait,
// or accept any Entity.
type TypeConstraint struct {
	kind TypeConstraintKind
	id   identity.UUID
}

// EntityTypeConstraint constructs a constraint referring to an EntityType.
func EntityTypeConstraint(id identity.UUID) TypeConstraint {
	if id.IsZero() {
		panic("schema: entity type constraint id must not be zero")
	}

	return TypeConstraint{
		kind: TypeConstraintEntityType,
		id:   id,
	}
}

// TraitConstraint constructs a constraint referring to a Trait.
func TraitConstraint(id identity.UUID) TypeConstraint {
	if id.IsZero() {
		panic("schema: trait constraint id must not be zero")
	}

	return TypeConstraint{
		kind: TypeConstraintTrait,
		id:   id,
	}
}

// AnyEntityConstraint constructs a constraint accepting any Entity.
func AnyEntityConstraint() TypeConstraint {
	return TypeConstraint{
		kind: TypeConstraintAnyEntity,
	}
}

// Kind returns the kind of the constraint.
func (c TypeConstraint) Kind() TypeConstraintKind {
	return c.kind
}

// ID returns the referenced EntityType or Trait ID.
//
// The boolean result is false for AnyEntity.
func (c TypeConstraint) ID() (identity.UUID, bool) {
	switch c.kind {
	case TypeConstraintEntityType, TypeConstraintTrait:
		return c.id, true
	default:
		return identity.UUID{}, false
	}
}

// IsValid reports whether c is structurally valid.
func (c TypeConstraint) IsValid() bool {
	switch c.kind {
	case TypeConstraintEntityType, TypeConstraintTrait:
		return !c.id.IsZero()

	case TypeConstraintAnyEntity:
		return c.id.IsZero()

	default:
		return false
	}
}

// RelationType defines the schema of a semantic relation.
//
// RelationType does not assign business meaning to the relation.
// Names such as "contains" or "supplies" are domain-level semantics
// and are opaque to the Core.
//
// RelationType values are immutable after construction.
type RelationType struct {
	id                  identity.UUID
	name                string
	from                TypeConstraint
	hasFrom             bool
	to                  TypeConstraint
	hasTo               bool
	propertyDefinitions []identity.UUID
}

// NewRelationType constructs a RelationType.
//
// from and to are optional. hasFrom and hasTo explicitly distinguish
// an absent endpoint constraint from a present constraint.
//
// NewRelationType panics if:
//
//   - id is the zero UUID;
//   - name is empty;
//   - a present endpoint constraint is invalid;
//   - any PropertyDefinition ID is zero;
//   - PropertyDefinition IDs contain duplicates.
func NewRelationType(
	id identity.UUID,
	name string,
	from TypeConstraint,
	hasFrom bool,
	to TypeConstraint,
	hasTo bool,
	propertyDefinitions []identity.UUID,
) RelationType {
	if id.IsZero() {
		panic("schema: relation type id must not be zero")
	}

	if name == "" {
		panic("schema: relation type name must not be empty")
	}

	if hasFrom && !from.IsValid() {
		panic("schema: relation type from constraint is invalid")
	}

	if hasTo && !to.IsValid() {
		panic("schema: relation type to constraint is invalid")
	}

	if !hasFrom {
		from = TypeConstraint{}
	}

	if !hasTo {
		to = TypeConstraint{}
	}

	definitions := make([]identity.UUID, len(propertyDefinitions))
	seen := make(map[identity.UUID]struct{}, len(propertyDefinitions))

	for i, definitionID := range propertyDefinitions {
		if definitionID.IsZero() {
			panic("schema: relation type property definition id must not be zero")
		}

		if _, exists := seen[definitionID]; exists {
			panic("schema: relation type contains duplicate property definition id")
		}

		seen[definitionID] = struct{}{}
		definitions[i] = definitionID
	}

	return RelationType{
		id:                  id,
		name:                name,
		from:                from,
		hasFrom:             hasFrom,
		to:                  to,
		hasTo:               hasTo,
		propertyDefinitions: definitions,
	}
}

// ID returns the stable identity of the RelationType.
func (t RelationType) ID() identity.UUID {
	return t.id
}

// Name returns the presentation name of the RelationType.
func (t RelationType) Name() string {
	return t.name
}

// From returns the constraint for the source Entity.
//
// The boolean result reports whether a constraint is present.
func (t RelationType) From() (TypeConstraint, bool) {
	return t.from, t.hasFrom
}

// To returns the constraint for the target Entity.
//
// The boolean result reports whether a constraint is present.
func (t RelationType) To() (TypeConstraint, bool) {
	return t.to, t.hasTo
}

// PropertyDefinitions returns the PropertyDefinitions available
// to Relations of this type.
//
// A copy is returned to preserve immutability.
func (t RelationType) PropertyDefinitions() []identity.UUID {
	result := make([]identity.UUID, len(t.propertyDefinitions))
	copy(result, t.propertyDefinitions)

	return result
}

// IsValid reports whether t is structurally valid.
//
// Referential validity and endpoint compatibility are handled
// by the schema validation layer.
func (t RelationType) IsValid() bool {
	if t.id.IsZero() || t.name == "" {
		return false
	}

	if t.hasFrom {
		if !t.from.IsValid() {
			return false
		}
	} else if t.from != (TypeConstraint{}) {
		return false
	}

	if t.hasTo {
		if !t.to.IsValid() {
			return false
		}
	} else if t.to != (TypeConstraint{}) {
		return false
	}

	seen := make(map[identity.UUID]struct{}, len(t.propertyDefinitions))

	for _, definitionID := range t.propertyDefinitions {
		if definitionID.IsZero() {
			return false
		}

		if _, exists := seen[definitionID]; exists {
			return false
		}

		seen[definitionID] = struct{}{}
	}

	return true
}

// String returns a human-readable representation of the RelationType.
func (t RelationType) String() string {
	return fmt.Sprintf("RelationType(%s, %s)", t.id.String(), t.name)
}
