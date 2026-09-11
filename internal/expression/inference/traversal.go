package inference

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
)

// TraversalErrorKind identifies why a traversal cannot produce a valid
// static aggregate binding.
type TraversalErrorKind uint8

const (
	TraversalErrorInvalid TraversalErrorKind = iota
	TraversalErrorInvalidSource
	TraversalErrorRelationTypeNotFound
	TraversalErrorInvalidRelationType
	TraversalErrorMissingSelf
	TraversalErrorInvalidSelf
	TraversalErrorSelfTypeNotFound
	TraversalErrorMissingEndpoint
	TraversalErrorEndpointNotApplicable
)

// IsValid reports whether k is a valid traversal error kind.
func (k TraversalErrorKind) IsValid() bool {
	switch k {
	case TraversalErrorInvalidSource,
		TraversalErrorRelationTypeNotFound,
		TraversalErrorInvalidRelationType,
		TraversalErrorMissingSelf,
		TraversalErrorInvalidSelf,
		TraversalErrorSelfTypeNotFound,
		TraversalErrorMissingEndpoint,
		TraversalErrorEndpointNotApplicable:
		return true
	default:
		return false
	}
}

// String returns the canonical name of k.
func (k TraversalErrorKind) String() string {
	switch k {
	case TraversalErrorInvalidSource:
		return "InvalidSource"
	case TraversalErrorRelationTypeNotFound:
		return "RelationTypeNotFound"
	case TraversalErrorInvalidRelationType:
		return "InvalidRelationType"
	case TraversalErrorMissingSelf:
		return "MissingSelf"
	case TraversalErrorInvalidSelf:
		return "InvalidSelf"
	case TraversalErrorSelfTypeNotFound:
		return "SelfTypeNotFound"
	case TraversalErrorMissingEndpoint:
		return "MissingEndpoint"
	case TraversalErrorEndpointNotApplicable:
		return "EndpointNotApplicable"
	default:
		return "Invalid"
	}
}

// TraversalError describes a static traversal binding failure.
type TraversalError struct {
	kind TraversalErrorKind
}

// Kind returns the traversal error kind.
func (e TraversalError) Kind() TraversalErrorKind {
	return e.kind
}

// Error implements error.
func (e TraversalError) Error() string {
	switch e.kind {
	case TraversalErrorInvalidSource:
		return "inference: invalid traversal source"
	case TraversalErrorRelationTypeNotFound:
		return "inference: traversal relation type not found"
	case TraversalErrorInvalidRelationType:
		return "inference: traversal relation type is invalid"
	case TraversalErrorMissingSelf:
		return "inference: traversal requires self context"
	case TraversalErrorInvalidSelf:
		return "inference: traversal self must be an entity type"
	case TraversalErrorSelfTypeNotFound:
		return "inference: traversal self entity type not found"
	case TraversalErrorMissingEndpoint:
		return "inference: traversal endpoint constraint is unavailable"
	case TraversalErrorEndpointNotApplicable:
		return "inference: traversal relation endpoint is not applicable to self"
	default:
		return "inference: invalid traversal error"
	}
}

// BindingFromTraversal derives the static aggregate binding produced by
// source relative to the current self entity.
//
// Traversal is not interpreted only from RelationType direction. The current
// self entity must also be eligible to occupy the corresponding relation
// endpoint.
//
// RelationType endpoint constraints are optional. An absent From or To
// constraint means AnyEntity for traversal inference; absence is not an
// inference failure.
//
// For:
//
//	A --R--> B
//
// and self=A:
//
//	Outgoing R -> bound.Entity = B
//	Incoming R -> invalid
//	Both R     -> bound.Entity = B
//
// For self=B:
//
//	Outgoing R -> invalid
//	Incoming R -> bound.Entity = A
//	Both R     -> bound.Entity = A
//
// If self satisfies both endpoints, Both exposes the union of both opposite
// endpoint constraints.
//
// bound.Relation always refers to the concrete RelationType selected by
// source.
//
// Endpoint constraints themselves are preserved. Trait and AnyEntity are not
// fabricated into concrete EntityTypes.
func BindingFromTraversal(
	source ast.TraversalSource,
	context Context,
	resolver Resolver,
) (Binding, error) {
	if !source.IsValid() {
		return Binding{}, TraversalError{
			kind: TraversalErrorInvalidSource,
		}
	}

	selfOwner, ok := context.Self()
	if !ok {
		return Binding{}, TraversalError{
			kind: TraversalErrorMissingSelf,
		}
	}

	if !selfOwner.IsValid() ||
		selfOwner.Kind() != schema.PropertyOwnerEntityType {
		return Binding{}, TraversalError{
			kind: TraversalErrorInvalidSelf,
		}
	}

	selfType, ok := resolver.EntityType(selfOwner.ID())
	if !ok ||
		!selfType.IsValid() ||
		selfType.ID() != selfOwner.ID() {
		return Binding{}, TraversalError{
			kind: TraversalErrorSelfTypeNotFound,
		}
	}

	relationType, ok := resolver.RelationType(
		source.RelationTypeID(),
	)
	if !ok {
		return Binding{}, TraversalError{
			kind: TraversalErrorRelationTypeNotFound,
		}
	}

	if !relationType.IsValid() {
		return Binding{}, TraversalError{
			kind: TraversalErrorInvalidRelationType,
		}
	}

	// Defensive consistency check: Resolver must not return another
	// RelationType for the requested ID.
	if relationType.ID() != source.RelationTypeID() {
		return Binding{}, TraversalError{
			kind: TraversalErrorRelationTypeNotFound,
		}
	}

	relationOwner := schema.RelationTypeOwner(
		relationType.ID(),
	)

	switch source.Direction() {
	case ast.TraversalDirectionOutgoing:
		return outgoingBinding(
			selfType,
			relationType,
			relationOwner,
		)

	case ast.TraversalDirectionIncoming:
		return incomingBinding(
			selfType,
			relationType,
			relationOwner,
		)

	case ast.TraversalDirectionBoth:
		return bidirectionalBinding(
			selfType,
			relationType,
			relationOwner,
		)

	default:
		return Binding{}, TraversalError{
			kind: TraversalErrorInvalidSource,
		}
	}
}

// outgoingBinding derives:
//
//	self --R--> bound.Entity
//
// Therefore self must satisfy relation.From and bound.Entity is relation.To.
// An absent endpoint constraint is equivalent to AnyEntity.
func outgoingBinding(
	self schema.EntityType,
	relationType schema.RelationType,
	relationOwner schema.PropertyOwner,
) (Binding, error) {
	from := relationFromConstraint(relationType)
	to := relationToConstraint(relationType)

	if !entityTypeSatisfiesConstraint(self, from) {
		return Binding{}, TraversalError{
			kind: TraversalErrorEndpointNotApplicable,
		}
	}

	return NewBinding(
		relationOwner,
		NewEntityBinding(to),
	), nil
}

// incomingBinding derives:
//
//	bound.Entity --R--> self
//
// Therefore self must satisfy relation.To and bound.Entity is relation.From.
// An absent endpoint constraint is equivalent to AnyEntity.
func incomingBinding(
	self schema.EntityType,
	relationType schema.RelationType,
	relationOwner schema.PropertyOwner,
) (Binding, error) {
	from := relationFromConstraint(relationType)
	to := relationToConstraint(relationType)

	if !entityTypeSatisfiesConstraint(self, to) {
		return Binding{}, TraversalError{
			kind: TraversalErrorEndpointNotApplicable,
		}
	}

	return NewBinding(
		relationOwner,
		NewEntityBinding(from),
	), nil
}

// bidirectionalBinding derives every opposite endpoint reachable from self.
//
// If self satisfies From:
//
//	self --R--> To
//
// so To is one possible bound.Entity.
//
// If self satisfies To:
//
//	From --R--> self
//
// so From is one possible bound.Entity.
//
// If self satisfies both sides, both opposite constraints are retained.
// NewEntityBinding removes duplicates deterministically.
//
// An absent From or To constraint is equivalent to AnyEntity.
func bidirectionalBinding(
	self schema.EntityType,
	relationType schema.RelationType,
	relationOwner schema.PropertyOwner,
) (Binding, error) {
	from := relationFromConstraint(relationType)
	to := relationToConstraint(relationType)

	selfMatchesFrom := entityTypeSatisfiesConstraint(
		self,
		from,
	)

	selfMatchesTo := entityTypeSatisfiesConstraint(
		self,
		to,
	)

	switch {
	case selfMatchesFrom && selfMatchesTo:
		return NewBinding(
			relationOwner,
			NewEntityBinding(
				to,
				from,
			),
		), nil

	case selfMatchesFrom:
		return NewBinding(
			relationOwner,
			NewEntityBinding(to),
		), nil

	case selfMatchesTo:
		return NewBinding(
			relationOwner,
			NewEntityBinding(from),
		), nil

	default:
		return Binding{}, TraversalError{
			kind: TraversalErrorEndpointNotApplicable,
		}
	}
}

// relationFromConstraint returns the effective From endpoint constraint.
//
// RelationType uses absence to mean that no restriction is imposed on the
// endpoint. Static traversal inference represents that semantics explicitly
// as AnyEntity.
func relationFromConstraint(
	relationType schema.RelationType,
) schema.TypeConstraint {
	constraint, ok := relationType.From()
	if ok {
		return constraint
	}

	return schema.AnyEntityConstraint()
}

// relationToConstraint returns the effective To endpoint constraint.
//
// RelationType uses absence to mean that no restriction is imposed on the
// endpoint. Static traversal inference represents that semantics explicitly
// as AnyEntity.
func relationToConstraint(
	relationType schema.RelationType,
) schema.TypeConstraint {
	constraint, ok := relationType.To()
	if ok {
		return constraint
	}

	return schema.AnyEntityConstraint()
}

// entityTypeSatisfiesConstraint reports whether entityType may legally occupy
// an endpoint described by constraint.
//
// Core v1 has no EntityType inheritance. Therefore the rules are:
//
//   - EntityType constraint: exact EntityType identity;
//   - Trait constraint: EntityType explicitly declares the Trait;
//   - AnyEntity constraint: every valid EntityType satisfies it.
//
// Trait semantic validity itself belongs to schema/model validation. Static
// traversal inference only needs the already-declared EntityType membership.
func entityTypeSatisfiesConstraint(
	entityType schema.EntityType,
	constraint schema.TypeConstraint,
) bool {
	if !entityType.IsValid() || !constraint.IsValid() {
		return false
	}

	switch constraint.Kind() {
	case schema.TypeConstraintEntityType:
		constraintID, ok := constraint.ID()
		if !ok {
			return false
		}

		return entityType.ID() == constraintID

	case schema.TypeConstraintTrait:
		traitID, ok := constraint.ID()
		if !ok {
			return false
		}

		return containsIdentity(
			entityType.Traits(),
			traitID,
		)

	case schema.TypeConstraintAnyEntity:
		return true

	default:
		return false
	}
}

func containsIdentity(
	ids []identity.UUID,
	target identity.UUID,
) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}

	return false
}

var _ error = TraversalError{}
