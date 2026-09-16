package dependency

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
)

// Source exposes the immutable model data required to build a concrete
// value dependency graph.
//
// Source intentionally describes capabilities rather than a concrete model
// container. model.ModelVersion satisfies this interface directly.
//
// A Source passed to Build is expected to represent a structurally valid,
// committed model. Build still validates every lookup it relies on and
// returns an error if that invariant is violated.
type Source interface {
	Entities() []semantic.Entity
	Relations() []semantic.Relation

	// OutgoingRelations and IncomingRelations are semantic traversal indexes.
	// They keep aggregate dependency extraction proportional to a traversed
	// neighbourhood rather than to every Relation in a large runtime Dataset.
	OutgoingRelations(entityID identity.UUID, relationTypeID identity.UUID) []semantic.Relation
	IncomingRelations(entityID identity.UUID, relationTypeID identity.UUID) []semantic.Relation

	EntityType(
		id identity.UUID,
	) (schema.EntityType, bool)

	RelationType(
		id identity.UUID,
	) (schema.RelationType, bool)

	PropertyDefinition(
		id identity.UUID,
	) (schema.PropertyDefinition, bool)

	Entity(
		id identity.UUID,
	) (semantic.Entity, bool)

	Relation(
		id identity.UUID,
	) (semantic.Relation, bool)

	ContractImplementation(
		owner schema.PropertyOwner,
		contractID identity.UUID,
	) (schema.PropertyDefinition, bool)
}
