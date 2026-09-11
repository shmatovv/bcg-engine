package semantic

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// Entity is an instance of an EntityType in the semantic model.
//
// Entity contains identity and schema membership only.
// Property values are stored separately through ValueBindings.
//
// Entity values are immutable after construction.
type Entity struct {
	id identity.UUID

	typeID identity.UUID

	name    string
	hasName bool
}

// NewEntity constructs an Entity.
//
// NewEntity panics if:
//
//   - id is the zero UUID;
//   - typeID is the zero UUID.
//
// Referential validity of typeID is checked by the validation layer.
func NewEntity(
	id identity.UUID,
	typeID identity.UUID,
	name string,
	hasName bool,
) Entity {
	if id.IsZero() {
		panic("semantic: entity id must not be zero")
	}

	if typeID.IsZero() {
		panic("semantic: entity type id must not be zero")
	}

	if !hasName {
		name = ""
	}

	return Entity{
		id:      id,
		typeID:  typeID,
		name:    name,
		hasName: hasName,
	}
}

// ID returns the stable identity of the Entity.
func (e Entity) ID() identity.UUID {
	return e.id
}

// TypeID returns the EntityType referenced by the Entity.
func (e Entity) TypeID() identity.UUID {
	return e.typeID
}

// Name returns the optional presentation name of the Entity.
//
// The boolean result reports whether the Entity has an explicitly
// specified name.
func (e Entity) Name() (string, bool) {
	return e.name, e.hasName
}

// IsValid reports whether e is structurally valid.
//
// Referential validity of TypeID is checked by the validation layer.
func (e Entity) IsValid() bool {
	if e.id.IsZero() {
		return false
	}

	if e.typeID.IsZero() {
		return false
	}

	if !e.hasName && e.name != "" {
		return false
	}

	return true
}

// String returns a human-readable representation of the Entity.
func (e Entity) String() string {
	if e.hasName {
		return fmt.Sprintf(
			"Entity(%s, %s)",
			e.id.String(),
			e.name,
		)
	}

	return fmt.Sprintf(
		"Entity(%s)",
		e.id.String(),
	)
}
