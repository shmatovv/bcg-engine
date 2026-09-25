package value

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
)

// ModelSource is the narrow immutable capability needed by value resolution.
// Both ModelVersion and the combined model.EvaluationSource satisfy it.
type ModelSource interface {
	IsCommitted() bool
	ID() identity.UUID
	SourceIdentity() model.EffectiveSourceIdentity
	Entity(identity.UUID) (semantic.Entity, bool)
	Relation(identity.UUID) (semantic.Relation, bool)
	EntityType(identity.UUID) (schema.EntityType, bool)
	RelationType(identity.UUID) (schema.RelationType, bool)
	PropertyDefinition(identity.UUID) (schema.PropertyDefinition, bool)
	PropertyContract(identity.UUID) (schema.PropertyContract, bool)
	ScopeDefinition(identity.UUID) (scope.Definition, bool)
	ValueBindingsForTarget(semantic.ValueTarget) []semantic.ValueBinding
	ObjectType(address.ObjectRef) (schema.PropertyOwner, bool)
}
