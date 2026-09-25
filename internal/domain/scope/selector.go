package scope

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

type Selector struct {
	definitionID identity.UUID
	value        value.Typed
}

func NewSelector(
	definitionID identity.UUID,
	v value.Typed,
) Selector {
	if definitionID.IsZero() {
		panic("scope: definition id must not be zero")
	}

	if !v.IsValid() {
		panic("scope: selector value must be valid")
	}

	return Selector{
		definitionID: definitionID,
		value:        v,
	}
}

func (s Selector) DefinitionID() identity.UUID {
	return s.definitionID
}

func (s Selector) Value() value.Typed {
	return s.value
}

func (s Selector) IsValid() bool {
	return !s.definitionID.IsZero() &&
		s.value.IsValid()
}
