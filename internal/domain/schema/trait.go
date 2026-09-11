package schema

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// Trait defines a reusable schema contract.
//
// A Trait specifies a set of PropertyContracts that an EntityType
// is required to implement.
//
// Trait does not provide property implementations and does not represent
// inheritance. An EntityType implements a Trait by providing exactly one
// compatible PropertyDefinition for every required PropertyContract.
//
// Trait values are immutable after construction.
type Trait struct {
	id                        identity.UUID
	name                      string
	requiredPropertyContracts []identity.UUID
}

// NewTrait constructs a Trait.
//
// NewTrait panics if:
//
//   - id is the zero UUID;
//   - name is empty;
//   - any required PropertyContract ID is zero;
//   - the same PropertyContract ID occurs more than once.
//
// These conditions represent invalid schema construction and therefore
// programming errors at this layer.
func NewTrait(
	id identity.UUID,
	name string,
	requiredPropertyContracts []identity.UUID,
) Trait {
	if id.IsZero() {
		panic("schema: trait id must not be zero")
	}

	if name == "" {
		panic("schema: trait name must not be empty")
	}

	contracts := make([]identity.UUID, len(requiredPropertyContracts))
	copy(contracts, requiredPropertyContracts)

	seen := make(map[identity.UUID]struct{}, len(contracts))

	for _, contractID := range contracts {
		if contractID.IsZero() {
			panic("schema: trait required property contract id must not be zero")
		}

		if _, exists := seen[contractID]; exists {
			panic("schema: trait contains duplicate required property contract id")
		}

		seen[contractID] = struct{}{}
	}

	return Trait{
		id:                        id,
		name:                      name,
		requiredPropertyContracts: contracts,
	}
}

// ID returns the stable identity of the Trait.
func (t Trait) ID() identity.UUID {
	return t.id
}

// Name returns the presentation name of the Trait.
//
// Name is metadata and is not part of computational identity.
func (t Trait) Name() string {
	return t.name
}

// RequiredPropertyContracts returns the PropertyContracts that an
// EntityType must implement in order to implement this Trait.
//
// A copy is returned to preserve Trait immutability.
func (t Trait) RequiredPropertyContracts() []identity.UUID {
	result := make([]identity.UUID, len(t.requiredPropertyContracts))
	copy(result, t.requiredPropertyContracts)

	return result
}

// IsValid reports whether t is structurally valid.
//
// Referential validity and compatibility of PropertyContracts are checked
// by the schema validation layer.
func (t Trait) IsValid() bool {
	if t.id.IsZero() || t.name == "" {
		return false
	}

	seen := make(map[identity.UUID]struct{}, len(t.requiredPropertyContracts))

	for _, contractID := range t.requiredPropertyContracts {
		if contractID.IsZero() {
			return false
		}

		if _, exists := seen[contractID]; exists {
			return false
		}

		seen[contractID] = struct{}{}
	}

	return true
}

// String returns a human-readable representation of the Trait.
func (t Trait) String() string {
	return fmt.Sprintf("Trait(%s, %s)", t.id.String(), t.name)
}
