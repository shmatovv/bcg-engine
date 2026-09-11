package scope

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// Definition declares one independent scope dimension used during value
// resolution.
//
// Core v1 currently models the scope domain by ValueType. A richer finite or
// constrained domain can be added later without changing Selector identity.
type Definition struct {
	id        identity.UUID
	name      string
	valueType value.Type
}

// NewDefinition constructs a scope Definition.
func NewDefinition(
	id identity.UUID,
	name string,
	valueType value.Type,
) Definition {
	if id.IsZero() {
		panic("scope: definition id must not be zero")
	}

	if name == "" {
		panic("scope: definition name must not be empty")
	}

	if !valueType.IsValid() {
		panic("scope: definition value type must be valid")
	}

	return Definition{
		id:        id,
		name:      name,
		valueType: valueType,
	}
}

func (d Definition) ID() identity.UUID {
	return d.id
}

func (d Definition) Name() string {
	return d.name
}

func (d Definition) ValueType() value.Type {
	return d.valueType
}

func (d Definition) IsValid() bool {
	return !d.id.IsZero() &&
		d.name != "" &&
		d.valueType.IsValid()
}

func (d Definition) String() string {
	return fmt.Sprintf(
		"ScopeDefinition(%s, %s)",
		d.id.String(),
		d.name,
	)
}
