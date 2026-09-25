package semantic

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
)

type ObjectRefKind = address.ObjectRefKind

const (
	ObjectRefInvalid  = address.ObjectRefInvalid
	ObjectRefEntity   = address.ObjectRefEntity
	ObjectRefRelation = address.ObjectRefRelation
)

type ObjectRef = address.ObjectRef

type PropertyAddress = address.PropertyAddress

func EntityRef(id identity.UUID) ObjectRef {
	return address.EntityRef(id)
}

func RelationRef(id identity.UUID) ObjectRef {
	return address.RelationRef(id)
}

func NewPropertyAddress(
	object ObjectRef,
	definitionID identity.UUID,
) PropertyAddress {
	return address.NewPropertyAddress(object, definitionID)
}
