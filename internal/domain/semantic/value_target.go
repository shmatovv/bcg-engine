package semantic

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
)

type ValueTargetKind uint8

const (
	ValueTargetInvalid ValueTargetKind = iota
	ValueTargetObjectProperty
	ValueTargetTypeProperty
)

type ValueTarget struct {
	kind ValueTargetKind

	address PropertyAddress

	owner        schema.PropertyOwner
	definitionID identity.UUID
}

func ObjectPropertyTarget(address PropertyAddress) ValueTarget {
	if !address.IsValid() {
		panic("semantic: value target property address is invalid")
	}

	return ValueTarget{
		kind:    ValueTargetObjectProperty,
		address: address,
	}
}

func TypePropertyTarget(
	owner schema.PropertyOwner,
	definitionID identity.UUID,
) ValueTarget {
	if !owner.IsValid() {
		panic("semantic: value target type reference is invalid")
	}

	if definitionID.IsZero() {
		panic("semantic: value target definition id must not be zero")
	}

	return ValueTarget{
		kind:         ValueTargetTypeProperty,
		owner:        owner,
		definitionID: definitionID,
	}
}

func (t ValueTarget) Kind() ValueTargetKind {
	return t.kind
}

func (t ValueTarget) ObjectProperty() (PropertyAddress, bool) {
	if t.kind != ValueTargetObjectProperty {
		return PropertyAddress{}, false
	}

	return t.address, true
}

func (t ValueTarget) TypeProperty() (
	schema.PropertyOwner,
	identity.UUID,
	bool,
) {
	if t.kind != ValueTargetTypeProperty {
		return schema.PropertyOwner{}, identity.UUID{}, false
	}

	return t.owner, t.definitionID, true
}

func (t ValueTarget) IsValid() bool {
	switch t.kind {
	case ValueTargetObjectProperty:
		return t.address.IsValid()

	case ValueTargetTypeProperty:
		return t.owner.IsValid() &&
			!t.definitionID.IsZero()

	default:
		return false
	}
}
