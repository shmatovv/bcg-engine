package ast

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// SelfPropertyReference references a PropertyDefinition on the object
// currently being evaluated.
//
// Example:
//
//	self.length
//
// The concrete PropertyAddress is resolved from the current object
// and definitionID during dependency extraction/evaluation.
type SelfPropertyReference struct {
	definitionID identity.UUID
}

// NewSelfPropertyReference constructs a SelfPropertyReference.
func NewSelfPropertyReference(
	definitionID identity.UUID,
) SelfPropertyReference {
	if definitionID.IsZero() {
		panic("ast: self property reference definition id must not be zero")
	}

	return SelfPropertyReference{
		definitionID: definitionID,
	}
}

// DefinitionID returns the referenced PropertyDefinition ID.
func (r SelfPropertyReference) DefinitionID() identity.UUID {
	return r.definitionID
}

// IsValid reports whether r is structurally valid.
func (r SelfPropertyReference) IsValid() bool {
	return !r.definitionID.IsZero()
}

func (SelfPropertyReference) expressionNode() {}

// AbsolutePropertyReference references one concrete property of one
// concrete Entity or Relation.
//
// Unlike SelfPropertyReference, the target object is fixed directly
// in the expression.
type AbsolutePropertyReference struct {
	address address.PropertyAddress
}

// NewAbsolutePropertyReference constructs an AbsolutePropertyReference.
func NewAbsolutePropertyReference(
	propertyAddress address.PropertyAddress,
) AbsolutePropertyReference {
	if !propertyAddress.IsValid() {
		panic("ast: absolute property reference address must be valid")
	}

	return AbsolutePropertyReference{
		address: propertyAddress,
	}
}

// Address returns the concrete PropertyAddress.
func (r AbsolutePropertyReference) Address() address.PropertyAddress {
	return r.address
}

// IsValid reports whether r is structurally valid.
func (r AbsolutePropertyReference) IsValid() bool {
	return r.address.IsValid()
}

func (AbsolutePropertyReference) expressionNode() {}

// BoundTarget identifies which object from an Aggregate traversal
// binding is referenced by a bound property expression.
//
// Every traversal item binds two objects:
//
//   - the Relation being traversed;
//   - the Entity on the opposite endpoint.
//
// BoundTarget makes a bound reference explicit and unambiguous.
type BoundTarget uint8

const (
	BoundTargetInvalid BoundTarget = iota
	BoundTargetEntity
	BoundTargetRelation
)

// IsValid reports whether t is a valid BoundTarget.
func (t BoundTarget) IsValid() bool {
	switch t {
	case BoundTargetEntity, BoundTargetRelation:
		return true
	default:
		return false
	}
}

// String returns the canonical name of t.
func (t BoundTarget) String() string {
	switch t {
	case BoundTargetEntity:
		return "Entity"
	case BoundTargetRelation:
		return "Relation"
	default:
		return "Invalid"
	}
}

// BoundPropertyDefinitionReference references a PropertyDefinition on
// one of the objects currently bound by an Aggregate traversal.
//
// target explicitly selects either:
//
//   - the opposite Entity;
//   - the traversed Relation.
//
// Example:
//
//	boundEntity.price
//	boundRelation.quantity
//
// The concrete PropertyAddress is resolved from the aggregate binding
// environment during dependency extraction/evaluation.
type BoundPropertyDefinitionReference struct {
	target       BoundTarget
	definitionID identity.UUID
}

// NewBoundPropertyDefinitionReference constructs a
// BoundPropertyDefinitionReference.
func NewBoundPropertyDefinitionReference(
	target BoundTarget,
	definitionID identity.UUID,
) BoundPropertyDefinitionReference {
	if !target.IsValid() {
		panic("ast: bound property definition target must be valid")
	}

	if definitionID.IsZero() {
		panic("ast: bound property definition reference id must not be zero")
	}

	return BoundPropertyDefinitionReference{
		target:       target,
		definitionID: definitionID,
	}
}

// Target returns which traversal-bound object is referenced.
func (r BoundPropertyDefinitionReference) Target() BoundTarget {
	return r.target
}

// DefinitionID returns the referenced PropertyDefinition ID.
func (r BoundPropertyDefinitionReference) DefinitionID() identity.UUID {
	return r.definitionID
}

// IsValid reports whether r is structurally valid.
func (r BoundPropertyDefinitionReference) IsValid() bool {
	return r.target.IsValid() &&
		!r.definitionID.IsZero()
}

func (BoundPropertyDefinitionReference) expressionNode() {}

// BoundContractPropertyReference references a PropertyContract on one
// of the objects currently bound by an Aggregate traversal.
//
// target explicitly selects either:
//
//   - the opposite Entity;
//   - the traversed Relation.
//
// Unlike BoundPropertyDefinitionReference, the concrete
// PropertyDefinition is selected from the bound object's type through
// its PropertyContract implementation.
//
// Example:
//
//	boundEntity.PRICE
type BoundContractPropertyReference struct {
	target     BoundTarget
	contractID identity.UUID
}

// NewBoundContractPropertyReference constructs a
// BoundContractPropertyReference.
func NewBoundContractPropertyReference(
	target BoundTarget,
	contractID identity.UUID,
) BoundContractPropertyReference {
	if !target.IsValid() {
		panic("ast: bound contract property target must be valid")
	}

	if contractID.IsZero() {
		panic("ast: bound contract property reference id must not be zero")
	}

	return BoundContractPropertyReference{
		target:     target,
		contractID: contractID,
	}
}

// Target returns which traversal-bound object is referenced.
func (r BoundContractPropertyReference) Target() BoundTarget {
	return r.target
}

// ContractID returns the referenced PropertyContract ID.
func (r BoundContractPropertyReference) ContractID() identity.UUID {
	return r.contractID
}

// IsValid reports whether r is structurally valid.
func (r BoundContractPropertyReference) IsValid() bool {
	return r.target.IsValid() &&
		!r.contractID.IsZero()
}

func (BoundContractPropertyReference) expressionNode() {}
