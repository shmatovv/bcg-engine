package semantic

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
)

// Relation is a directed semantic edge between two Entities.
//
// RelationType defines the schema of the edge.
// Applicability may conditionally activate the Relation in a
// CalculationContext.
//
// Reverse traversal operates on the same Relation and does not
// create another edge.
type Relation struct {
	id     identity.UUID
	typeID identity.UUID

	from identity.UUID
	to   identity.UUID

	applicability    ast.Expression
	hasApplicability bool
}

// NewRelation constructs a Relation.
//
// NewRelation panics if:
//
//   - id is the zero UUID;
//   - typeID is the zero UUID;
//   - from is the zero UUID;
//   - to is the zero UUID;
//   - applicability is declared present but is nil.
//
// Referential validity, endpoint type compatibility and expression
// typing are checked by the validation layer.
func NewRelation(
	id identity.UUID,
	typeID identity.UUID,
	from identity.UUID,
	to identity.UUID,
	applicability ast.Expression,
	hasApplicability bool,
) Relation {
	if id.IsZero() {
		panic("semantic: relation id must not be zero")
	}

	if typeID.IsZero() {
		panic("semantic: relation type id must not be zero")
	}

	if from.IsZero() {
		panic("semantic: relation from entity id must not be zero")
	}

	if to.IsZero() {
		panic("semantic: relation to entity id must not be zero")
	}

	if hasApplicability && applicability == nil {
		panic("semantic: relation applicability must not be nil when present")
	}

	if !hasApplicability {
		applicability = nil
	}

	return Relation{
		id:               id,
		typeID:           typeID,
		from:             from,
		to:               to,
		applicability:    applicability,
		hasApplicability: hasApplicability,
	}
}

// ID returns the stable identity of the Relation.
func (r Relation) ID() identity.UUID {
	return r.id
}

// TypeID returns the RelationType referenced by the Relation.
func (r Relation) TypeID() identity.UUID {
	return r.typeID
}

// From returns the source Entity ID.
func (r Relation) From() identity.UUID {
	return r.from
}

// To returns the target Entity ID.
func (r Relation) To() identity.UUID {
	return r.to
}

// Applicability returns the optional applicability expression.
//
// The boolean result reports whether applicability is present.
func (r Relation) Applicability() (ast.Expression, bool) {
	return r.applicability, r.hasApplicability
}

// IsValid reports whether r is structurally valid.
//
// Referential validity, RelationType endpoint constraints and
// applicability expression typing belong to the validation layer.
func (r Relation) IsValid() bool {
	if r.id.IsZero() ||
		r.typeID.IsZero() ||
		r.from.IsZero() ||
		r.to.IsZero() {
		return false
	}

	if r.hasApplicability {
		return r.applicability != nil
	}

	return r.applicability == nil
}

// String returns a human-readable representation of the Relation.
func (r Relation) String() string {
	return fmt.Sprintf(
		"Relation(%s, %s -> %s)",
		r.id.String(),
		r.from.String(),
		r.to.String(),
	)
}
