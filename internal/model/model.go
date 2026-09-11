package model

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// Model is the long-lived identity of one business model.
//
// Model does not contain schema, semantic data or values. Those belong to
// immutable ModelVersion snapshots.
//
// Name is presentation metadata and is not part of computational identity.
type Model struct {
	id   identity.UUID
	name string
}

// NewModel constructs a Model.
//
// NewModel panics if id is zero or name is empty. Both conditions represent
// programmer errors at the domain-construction boundary.
func NewModel(
	id identity.UUID,
	name string,
) Model {
	if id.IsZero() {
		panic("model: model id must not be zero")
	}

	if name == "" {
		panic("model: model name must not be empty")
	}

	return Model{
		id:   id,
		name: name,
	}
}

// ID returns the stable long-lived identity of the model.
func (m Model) ID() identity.UUID {
	return m.id
}

// Name returns the presentation name of the model.
func (m Model) Name() string {
	return m.name
}

// IsValid reports whether m is structurally valid.
func (m Model) IsValid() bool {
	return !m.id.IsZero() && m.name != ""
}

// String returns a human-readable representation of the model.
func (m Model) String() string {
	return fmt.Sprintf(
		"Model(%s, %s)",
		m.id.String(),
		m.name,
	)
}
