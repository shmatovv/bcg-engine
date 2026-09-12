package bcg

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
)

// ObjectKind identifies the concrete object that owns a property.
type ObjectKind string

const (
	ObjectEntity   ObjectKind = "Entity"
	ObjectRelation ObjectKind = "Relation"
)

// PropertyAddress identifies one concrete Entity or Relation property.
// It has value semantics and does not imply that the referenced property
// exists in a particular Model.
type PropertyAddress struct {
	objectKind ObjectKind
	objectID   ID
	definition ID
}

// EntityProperty constructs an Entity property address.
func EntityProperty(entityID, definitionID ID) (PropertyAddress, error) {
	return newPropertyAddress(ObjectEntity, entityID, definitionID)
}

// RelationProperty constructs a Relation property address.
func RelationProperty(relationID, definitionID ID) (PropertyAddress, error) {
	return newPropertyAddress(ObjectRelation, relationID, definitionID)
}

func newPropertyAddress(kind ObjectKind, objectID, definitionID ID) (PropertyAddress, error) {
	if _, err := objectID.internal(); err != nil {
		return PropertyAddress{}, err
	}
	if _, err := definitionID.internal(); err != nil {
		return PropertyAddress{}, err
	}
	if kind != ObjectEntity && kind != ObjectRelation {
		return PropertyAddress{}, fmt.Errorf("%w: property object kind is invalid", ErrInvalidArgument)
	}
	return PropertyAddress{objectKind: kind, objectID: objectID, definition: definitionID}, nil
}

// ObjectKind returns whether this is an Entity or Relation property.
func (a PropertyAddress) ObjectKind() ObjectKind { return a.objectKind }

// ObjectID returns the owning Entity or Relation ID.
func (a PropertyAddress) ObjectID() ID { return a.objectID }

// DefinitionID returns the PropertyDefinition ID.
func (a PropertyAddress) DefinitionID() ID { return a.definition }

// IsValid reports whether the address has a valid shape.
func (a PropertyAddress) IsValid() bool {
	return (a.objectKind == ObjectEntity || a.objectKind == ObjectRelation) && !a.objectID.IsZero() && !a.definition.IsZero()
}

// String returns a diagnostic representation. UUID identity remains canonical.
func (a PropertyAddress) String() string {
	return fmt.Sprintf("PropertyAddress(%s,%s,%s)", a.objectKind, a.objectID, a.definition)
}

func addressFromInternal(input address.PropertyAddress) PropertyAddress {
	kind := ObjectEntity
	if input.Object().Kind() == address.ObjectRefRelation {
		kind = ObjectRelation
	}
	return PropertyAddress{objectKind: kind, objectID: idFromInternal(input.Object().ID()), definition: idFromInternal(input.DefinitionID())}
}

func (a PropertyAddress) internal() (address.PropertyAddress, error) {
	if !a.IsValid() {
		return address.PropertyAddress{}, fmt.Errorf("%w: property address is invalid", ErrInvalidArgument)
	}
	objectID, _ := a.objectID.internal()
	definitionID, _ := a.definition.internal()
	switch a.objectKind {
	case ObjectEntity:
		return address.NewPropertyAddress(address.EntityRef(objectID), definitionID), nil
	case ObjectRelation:
		return address.NewPropertyAddress(address.RelationRef(objectID), definitionID), nil
	default:
		return address.PropertyAddress{}, fmt.Errorf("%w: property object kind is invalid", ErrInvalidArgument)
	}
}
