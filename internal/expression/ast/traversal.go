package ast

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
)

// TraversalDirection specifies which direction of the semantic graph
// is traversed relative to the current object.
type TraversalDirection uint8

const (
	TraversalDirectionInvalid TraversalDirection = iota
	TraversalDirectionOutgoing
	TraversalDirectionIncoming
	TraversalDirectionBoth
)

// IsValid reports whether d is a valid traversal direction.
func (d TraversalDirection) IsValid() bool {
	switch d {
	case TraversalDirectionOutgoing,
		TraversalDirectionIncoming,
		TraversalDirectionBoth:
		return true
	default:
		return false
	}
}

// String returns the canonical name of d.
func (d TraversalDirection) String() string {
	switch d {
	case TraversalDirectionOutgoing:
		return "Outgoing"
	case TraversalDirectionIncoming:
		return "Incoming"
	case TraversalDirectionBoth:
		return "Both"
	default:
		return "Invalid"
	}
}

// TraversalOrigin identifies the object from which a TraversalSource starts.
//
// TraversalOriginSelf keeps the original derived-property meaning: traversal
// starts at the expression's Entity self. TraversalOriginAbsoluteEntity makes
// the origin part of the immutable expression data and is therefore valid in
// standalone Constraint and Rule expressions. TraversalOriginBoundEntity
// starts a nested aggregate at the Entity bound by its enclosing aggregate.
//
// There is intentionally no implicit global or scope-derived origin.
type TraversalOrigin uint8

const (
	TraversalOriginInvalid TraversalOrigin = iota
	TraversalOriginSelf
	TraversalOriginAbsoluteEntity
	TraversalOriginBoundEntity
)

// IsValid reports whether o is a supported traversal origin.
func (o TraversalOrigin) IsValid() bool {
	switch o {
	case TraversalOriginSelf,
		TraversalOriginAbsoluteEntity,
		TraversalOriginBoundEntity:
		return true
	default:
		return false
	}
}

// String returns the canonical name of o.
func (o TraversalOrigin) String() string {
	switch o {
	case TraversalOriginSelf:
		return "Self"
	case TraversalOriginAbsoluteEntity:
		return "AbsoluteEntity"
	case TraversalOriginBoundEntity:
		return "BoundEntity"
	default:
		return "Invalid"
	}
}

// TraversalSource describes a one-hop graph traversal used as the source of
// an Aggregate.
//
// relationTypeID restricts traversal to Relations of one RelationType.
//
// For each matched Relation, aggregate evaluation binds both:
//
//   - the Relation itself;
//   - the Entity on the opposite endpoint.
//
// The exact binding semantics are handled by aggregate evaluation.
type TraversalSource struct {
	origin         TraversalOrigin
	absoluteOrigin address.ObjectRef

	direction      TraversalDirection
	relationTypeID identity.UUID
}

// NewTraversalSource constructs a TraversalSource.
func NewTraversalSource(
	direction TraversalDirection,
	relationTypeID identity.UUID,
) TraversalSource {
	if !direction.IsValid() {
		panic("ast: traversal direction must be valid")
	}

	if relationTypeID.IsZero() {
		panic("ast: traversal relation type id must not be zero")
	}

	return TraversalSource{
		origin:         TraversalOriginSelf,
		direction:      direction,
		relationTypeID: relationTypeID,
	}
}

// NewTraversalSourceFromEntity constructs a traversal with an explicit,
// immutable Entity origin. It is the standalone-safe counterpart of
// NewTraversalSource: the evaluator never invents a self object.
func NewTraversalSourceFromEntity(
	origin address.ObjectRef,
	direction TraversalDirection,
	relationTypeID identity.UUID,
) TraversalSource {
	if !origin.IsValid() || origin.Kind() != address.ObjectRefEntity {
		panic("ast: traversal explicit origin must be an entity reference")
	}
	if !direction.IsValid() {
		panic("ast: traversal direction must be valid")
	}
	if relationTypeID.IsZero() {
		panic("ast: traversal relation type id must not be zero")
	}

	return TraversalSource{
		origin:         TraversalOriginAbsoluteEntity,
		absoluteOrigin: origin,
		direction:      direction,
		relationTypeID: relationTypeID,
	}
}

// NewTraversalSourceFromBoundEntity constructs a nested traversal whose
// origin is the related Entity bound by the enclosing aggregate. It is only
// valid inside an aggregate filter or map; static validation rejects it in a
// standalone or self-only context.
func NewTraversalSourceFromBoundEntity(
	direction TraversalDirection,
	relationTypeID identity.UUID,
) TraversalSource {
	if !direction.IsValid() {
		panic("ast: traversal direction must be valid")
	}
	if relationTypeID.IsZero() {
		panic("ast: traversal relation type id must not be zero")
	}

	return TraversalSource{
		origin:         TraversalOriginBoundEntity,
		direction:      direction,
		relationTypeID: relationTypeID,
	}
}

// Origin returns how the traversal selects its origin Entity.
func (s TraversalSource) Origin() TraversalOrigin { return s.origin }

// AbsoluteOrigin returns the explicit Entity origin when Origin is
// TraversalOriginAbsoluteEntity.
func (s TraversalSource) AbsoluteOrigin() (address.ObjectRef, bool) {
	if s.origin != TraversalOriginAbsoluteEntity {
		return address.ObjectRef{}, false
	}
	return s.absoluteOrigin, true
}

// Direction returns the traversal direction.
func (s TraversalSource) Direction() TraversalDirection {
	return s.direction
}

// RelationTypeID returns the RelationType used by the traversal.
func (s TraversalSource) RelationTypeID() identity.UUID {
	return s.relationTypeID
}

// IsValid reports whether s is structurally valid.
func (s TraversalSource) IsValid() bool {
	if !s.origin.IsValid() ||
		!s.direction.IsValid() ||
		s.relationTypeID.IsZero() {
		return false
	}

	switch s.origin {
	case TraversalOriginSelf, TraversalOriginBoundEntity:
		return !s.absoluteOrigin.IsValid()
	case TraversalOriginAbsoluteEntity:
		return s.absoluteOrigin.IsValid() &&
			s.absoluteOrigin.Kind() == address.ObjectRefEntity
	default:
		return false
	}
}
