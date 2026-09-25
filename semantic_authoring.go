package bcg

import (
	"fmt"
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
)

// EntitySpec describes one semantic Entity. Name is optional; a non-nil
// pointer preserves even an explicitly empty presentation name.
type EntitySpec struct {
	ID     ID
	TypeID ID
	Name   *string
}

// Entity is an immutable semantic object. Its property values are supplied
// separately by ValueBindings.
type Entity struct{ value semantic.Entity }

// NewEntity constructs an Entity with an explicit identity and EntityType.
// The referenced EntityType is checked by Draft.Validate and Draft.Commit.
func NewEntity(spec EntitySpec) (Entity, error) {
	id, err := spec.ID.internal()
	if err != nil {
		return Entity{}, fmt.Errorf("%w: entity ID: %v", ErrInvalidArgument, err)
	}
	typeID, err := spec.TypeID.internal()
	if err != nil {
		return Entity{}, fmt.Errorf("%w: entity type ID: %v", ErrInvalidArgument, err)
	}
	name, hasName := "", spec.Name != nil
	if hasName {
		name = *spec.Name
	}
	return Entity{value: semantic.NewEntity(id, typeID, name, hasName)}, nil
}

func entityFromInternal(value semantic.Entity) Entity { return Entity{value: value} }

// ID returns the Entity identity.
func (e Entity) ID() ID { return idFromInternal(e.value.ID()) }

// TypeID returns the referenced EntityType identity.
func (e Entity) TypeID() ID { return idFromInternal(e.value.TypeID()) }

// Name returns the optional presentation name.
func (e Entity) Name() (string, bool) { return e.value.Name() }

// IsValid reports structural Entity validity. Cross-model references are
// intentionally checked only by Draft.Validate and Draft.Commit.
func (e Entity) IsValid() bool { return e.value.IsValid() }

// RelationSpec describes one directed semantic Relation. Applicability is
// optional; its zero Expression means that the Relation is always applicable.
type RelationSpec struct {
	ID            ID
	TypeID        ID
	From          ID
	To            ID
	Applicability Expression
}

// Relation is an immutable directed edge between two Entities.
type Relation struct{ value semantic.Relation }

// NewRelation constructs a Relation. Endpoint constraints and optional
// applicability typing are validated by the ordinary Draft validation pass.
func NewRelation(spec RelationSpec) (Relation, error) {
	id, err := spec.ID.internal()
	if err != nil {
		return Relation{}, fmt.Errorf("%w: relation ID: %v", ErrInvalidArgument, err)
	}
	typeID, err := spec.TypeID.internal()
	if err != nil {
		return Relation{}, fmt.Errorf("%w: relation type ID: %v", ErrInvalidArgument, err)
	}
	from, err := spec.From.internal()
	if err != nil {
		return Relation{}, fmt.Errorf("%w: relation from Entity ID: %v", ErrInvalidArgument, err)
	}
	to, err := spec.To.internal()
	if err != nil {
		return Relation{}, fmt.Errorf("%w: relation to Entity ID: %v", ErrInvalidArgument, err)
	}
	if spec.Applicability.value == nil {
		return Relation{value: semantic.NewRelation(id, typeID, from, to, nil, false)}, nil
	}
	if !spec.Applicability.IsValid() {
		return Relation{}, fmt.Errorf("%w: relation applicability expression is invalid", ErrInvalidArgument)
	}
	return Relation{value: semantic.NewRelation(id, typeID, from, to, spec.Applicability.value, true)}, nil
}

func relationFromInternal(value semantic.Relation) Relation { return Relation{value: value} }

// ID returns the Relation identity.
func (r Relation) ID() ID { return idFromInternal(r.value.ID()) }

// TypeID returns the referenced RelationType identity.
func (r Relation) TypeID() ID { return idFromInternal(r.value.TypeID()) }

// From returns the source Entity identity.
func (r Relation) From() ID { return idFromInternal(r.value.From()) }

// To returns the destination Entity identity.
func (r Relation) To() ID { return idFromInternal(r.value.To()) }

// Applicability returns the optional Boolean applicability expression.
func (r Relation) Applicability() (Expression, bool) {
	expression, ok := r.value.Applicability()
	if !ok {
		return Expression{}, false
	}
	return expressionFromInternal(expression), true
}

// IsValid reports structural Relation validity. Referential and expression
// semantics remain the responsibility of Draft.Validate and Draft.Commit.
func (r Relation) IsValid() bool { return r.value.IsValid() }

// AddEntity adds an immutable Entity to d. It does not perform model-wide
// validation, so a Draft can temporarily contain a dangling EntityType.
func (d *Draft) AddEntity(value Entity) error {
	if !d.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft and Entity are required", ErrInvalidArgument)
	}
	d.value.AddEntity(value.value)
	return nil
}

// ReplaceEntity replaces the Entity with id. The replacement must retain the
// selected identity; use RemoveEntity followed by AddEntity for an identity
// change.
func (d *Draft) ReplaceEntity(id ID, value Entity) error {
	if !d.IsValid() || id.IsZero() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft, Entity ID, and Entity are required", ErrInvalidArgument)
	}
	if !value.ID().Equal(id) {
		return fmt.Errorf("%w: replacement Entity must retain its ID", ErrInvalidArgument)
	}
	if !d.value.ReplaceEntity(id.value, value.value) {
		return fmt.Errorf("%w: Entity %s was not found", ErrInvalidArgument, id)
	}
	return nil
}

// RemoveEntity removes exactly one Entity. It intentionally does not cascade
// to Relations or ValueBindings; validation reports resulting dangling data.
func (d *Draft) RemoveEntity(id ID) error {
	if !d.IsValid() || id.IsZero() {
		return fmt.Errorf("%w: valid Draft and Entity ID are required", ErrInvalidArgument)
	}
	if !d.value.RemoveEntity(id.value) {
		return fmt.Errorf("%w: Entity %s was not found", ErrInvalidArgument, id)
	}
	return nil
}

// AddRelation adds an immutable Relation to d. Endpoint and applicability
// semantics are checked by Draft.Validate and Draft.Commit.
func (d *Draft) AddRelation(value Relation) error {
	if !d.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft and Relation are required", ErrInvalidArgument)
	}
	d.value.AddRelation(value.value)
	return nil
}

// ReplaceRelation replaces the Relation with id while preserving identity.
func (d *Draft) ReplaceRelation(id ID, value Relation) error {
	if !d.IsValid() || id.IsZero() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft, Relation ID, and Relation are required", ErrInvalidArgument)
	}
	if !value.ID().Equal(id) {
		return fmt.Errorf("%w: replacement Relation must retain its ID", ErrInvalidArgument)
	}
	if !d.value.ReplaceRelation(id.value, value.value) {
		return fmt.Errorf("%w: Relation %s was not found", ErrInvalidArgument, id)
	}
	return nil
}

// RemoveRelation removes exactly one Relation without cascading to bindings.
func (d *Draft) RemoveRelation(id ID) error {
	if !d.IsValid() || id.IsZero() {
		return fmt.Errorf("%w: valid Draft and Relation ID are required", ErrInvalidArgument)
	}
	if !d.value.RemoveRelation(id.value) {
		return fmt.Errorf("%w: Relation %s was not found", ErrInvalidArgument, id)
	}
	return nil
}

// Entities returns an owned deterministic view of the semantic Entities in d.
func (d *Draft) Entities() []Entity {
	if !d.IsValid() {
		return nil
	}
	items := d.value.Entities()
	result := make([]Entity, len(items))
	for index, item := range items {
		result[index] = entityFromInternal(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return result
}

// Relations returns an owned deterministic view of the semantic Relations in d.
func (d *Draft) Relations() []Relation {
	if !d.IsValid() {
		return nil
	}
	items := d.value.Relations()
	result := make([]Relation, len(items))
	for index, item := range items {
		result[index] = relationFromInternal(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return result
}
