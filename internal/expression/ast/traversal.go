package ast

import "github.com/shmatovv/bcg-engine/internal/domain/identity"

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

// TraversalSource describes a graph traversal used as the source
// of an Aggregate.
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
		direction:      direction,
		relationTypeID: relationTypeID,
	}
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
	return s.direction.IsValid() &&
		!s.relationTypeID.IsZero()
}
