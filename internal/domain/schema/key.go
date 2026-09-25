package schema

import "github.com/shmatovv/bcg-engine/v2/internal/domain/identity"

type EntityKeyDefinition struct {
	id                    identity.UUID
	name                  string
	entityTypeID          identity.UUID
	propertyDefinitionIDs []identity.UUID
}

func NewEntityKeyDefinition(id identity.UUID, name string, entityTypeID identity.UUID, propertyDefinitionIDs []identity.UUID) EntityKeyDefinition {
	if id.IsZero() || name == "" || entityTypeID.IsZero() || len(propertyDefinitionIDs) == 0 {
		panic("schema: invalid entity key")
	}
	seen := map[identity.UUID]struct{}{}
	ids := make([]identity.UUID, len(propertyDefinitionIDs))
	for i, item := range propertyDefinitionIDs {
		if item.IsZero() {
			panic("schema: zero entity key component")
		}
		if _, ok := seen[item]; ok {
			panic("schema: duplicate entity key component")
		}
		seen[item] = struct{}{}
		ids[i] = item
	}
	return EntityKeyDefinition{id: id, name: name, entityTypeID: entityTypeID, propertyDefinitionIDs: ids}
}
func (d EntityKeyDefinition) ID() identity.UUID           { return d.id }
func (d EntityKeyDefinition) Name() string                { return d.name }
func (d EntityKeyDefinition) EntityTypeID() identity.UUID { return d.entityTypeID }
func (d EntityKeyDefinition) PropertyDefinitionIDs() []identity.UUID {
	return append([]identity.UUID(nil), d.propertyDefinitionIDs...)
}
func (d EntityKeyDefinition) IsValid() bool {
	if d.id.IsZero() || d.name == "" || d.entityTypeID.IsZero() || len(d.propertyDefinitionIDs) == 0 {
		return false
	}
	seen := map[identity.UUID]struct{}{}
	for _, item := range d.propertyDefinitionIDs {
		if item.IsZero() {
			return false
		}
		if _, ok := seen[item]; ok {
			return false
		}
		seen[item] = struct{}{}
	}
	return true
}
