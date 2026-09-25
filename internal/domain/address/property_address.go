package address

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
)

// ObjectRefKind identifies the kind of model object referenced
// by an ObjectRef.
type ObjectRefKind uint8

const (
	ObjectRefInvalid ObjectRefKind = iota
	ObjectRefEntity
	ObjectRefRelation
)

func (k ObjectRefKind) IsValid() bool {
	return k == ObjectRefEntity || k == ObjectRefRelation
}

func (k ObjectRefKind) String() string {
	switch k {
	case ObjectRefEntity:
		return "Entity"
	case ObjectRefRelation:
		return "Relation"
	default:
		return "Invalid"
	}
}

type ObjectRef struct {
	kind ObjectRefKind
	id   identity.UUID
}

func EntityRef(id identity.UUID) ObjectRef {
	if id.IsZero() {
		panic("address: entity reference id must not be zero")
	}

	return ObjectRef{
		kind: ObjectRefEntity,
		id:   id,
	}
}

func RelationRef(id identity.UUID) ObjectRef {
	if id.IsZero() {
		panic("address: relation reference id must not be zero")
	}

	return ObjectRef{
		kind: ObjectRefRelation,
		id:   id,
	}
}

func (r ObjectRef) Kind() ObjectRefKind {
	return r.kind
}

func (r ObjectRef) ID() identity.UUID {
	return r.id
}

func (r ObjectRef) IsValid() bool {
	return r.kind.IsValid() && !r.id.IsZero()
}

func (r ObjectRef) String() string {
	return fmt.Sprintf(
		"%sRef(%s)",
		r.kind.String(),
		r.id.String(),
	)
}

// PropertyAddress identifies one concrete property of one concrete object.
type PropertyAddress struct {
	object       ObjectRef
	definitionID identity.UUID
}

func NewPropertyAddress(
	object ObjectRef,
	definitionID identity.UUID,
) PropertyAddress {
	if !object.IsValid() {
		panic("address: property address object reference is invalid")
	}

	if definitionID.IsZero() {
		panic("address: property address definition id must not be zero")
	}

	return PropertyAddress{
		object:       object,
		definitionID: definitionID,
	}
}

func (a PropertyAddress) Object() ObjectRef {
	return a.object
}

func (a PropertyAddress) DefinitionID() identity.UUID {
	return a.definitionID
}

func (a PropertyAddress) IsValid() bool {
	return a.object.IsValid() && !a.definitionID.IsZero()
}

func (a PropertyAddress) String() string {
	return fmt.Sprintf(
		"PropertyAddress(%s, %s)",
		a.object.String(),
		a.definitionID.String(),
	)
}
