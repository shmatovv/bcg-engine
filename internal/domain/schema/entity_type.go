package schema

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// EntityType defines the schema of a group of entities.
//
// An EntityType declares which Traits it implements and which
// PropertyDefinitions are available to its Entity instances.
//
// EntityType does not provide inheritance. Composition is expressed
// through Traits.
//
// EntityType values are immutable after construction.
type EntityType struct {
	id                  identity.UUID
	name                string
	traits              []identity.UUID
	propertyDefinitions []identity.UUID
}

// NewEntityType constructs an EntityType.
//
// NewEntityType panics if:
//
//   - id is the zero UUID;
//   - name is empty;
//   - any Trait ID is zero;
//   - any PropertyDefinition ID is zero;
//   - Trait IDs contain duplicates;
//   - PropertyDefinition IDs contain duplicates.
//
// Referential and semantic validity are handled by schema validation.
func NewEntityType(
	id identity.UUID,
	name string,
	traits []identity.UUID,
	propertyDefinitions []identity.UUID,
) EntityType {
	if id.IsZero() {
		panic("schema: entity type id must not be zero")
	}

	if name == "" {
		panic("schema: entity type name must not be empty")
	}

	traitIDs := copyUniqueIDs(
		traits,
		"schema: entity type trait id must not be zero",
		"schema: entity type contains duplicate trait id",
	)

	definitionIDs := copyUniqueIDs(
		propertyDefinitions,
		"schema: entity type property definition id must not be zero",
		"schema: entity type contains duplicate property definition id",
	)

	return EntityType{
		id:                  id,
		name:                name,
		traits:              traitIDs,
		propertyDefinitions: definitionIDs,
	}
}

// ID returns the stable identity of the EntityType.
func (t EntityType) ID() identity.UUID {
	return t.id
}

// Name returns the presentation name of the EntityType.
//
// Name is metadata and is not part of computational identity.
func (t EntityType) Name() string {
	return t.name
}

// Traits returns the Traits declared by the EntityType.
//
// A copy is returned to preserve immutability.
func (t EntityType) Traits() []identity.UUID {
	return copyIDs(t.traits)
}

// PropertyDefinitions returns the PropertyDefinitions available
// to entities of this type.
//
// A copy is returned to preserve immutability.
func (t EntityType) PropertyDefinitions() []identity.UUID {
	return copyIDs(t.propertyDefinitions)
}

// IsValid reports whether t is structurally valid.
//
// Referential validity, Trait implementation and PropertyDefinition
// ownership are checked by the schema validation layer.
func (t EntityType) IsValid() bool {
	if t.id.IsZero() || t.name == "" {
		return false
	}

	if !validUniqueIDs(t.traits) {
		return false
	}

	if !validUniqueIDs(t.propertyDefinitions) {
		return false
	}

	return true
}

// String returns a human-readable representation of the EntityType.
func (t EntityType) String() string {
	return fmt.Sprintf("EntityType(%s, %s)", t.id.String(), t.name)
}

func copyUniqueIDs(
	ids []identity.UUID,
	zeroMessage string,
	duplicateMessage string,
) []identity.UUID {
	result := make([]identity.UUID, len(ids))
	seen := make(map[identity.UUID]struct{}, len(ids))

	for i, id := range ids {
		if id.IsZero() {
			panic(zeroMessage)
		}

		if _, exists := seen[id]; exists {
			panic(duplicateMessage)
		}

		seen[id] = struct{}{}
		result[i] = id
	}

	return result
}

func copyIDs(ids []identity.UUID) []identity.UUID {
	result := make([]identity.UUID, len(ids))
	copy(result, ids)
	return result
}

func validUniqueIDs(ids []identity.UUID) bool {
	seen := make(map[identity.UUID]struct{}, len(ids))

	for _, id := range ids {
		if id.IsZero() {
			return false
		}

		if _, exists := seen[id]; exists {
			return false
		}

		seen[id] = struct{}{}
	}

	return true
}
