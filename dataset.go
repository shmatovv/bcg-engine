package bcg

import (
	"fmt"
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/model"
)

// Dataset is an immutable, fully materialized external runtime data snapshot
// bound to one exact Model version. It contains only Entity, Relation and
// object-level ValueBinding data; schema and declarations remain model-owned.
//
// A Dataset is the complete runtime semantic graph for an evaluation: an
// absent relation is absent data, not data pending lazy loading. It is
// intentionally runtime-only and is never serialized into a .bcg artifact.
// Callers supply its stable ID and must not reuse that ID for changed contents.
type Dataset struct{ value model.Dataset }

func (d Dataset) IsValid() bool      { return d.value.IsValid() }
func (d Dataset) ID() ID             { return idFromInternal(d.value.ID()) }
func (d Dataset) ModelVersionID() ID { return idFromInternal(d.value.ModelVersionID()) }

// Entities returns a defensive deterministic copy of Dataset-owned entities.
func (d Dataset) Entities() []Entity {
	if !d.IsValid() {
		return nil
	}
	items := d.value.Entities()
	result := make([]Entity, len(items))
	for i, item := range items {
		result[i] = entityFromInternal(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return result
}

func (d Dataset) Relations() []Relation {
	if !d.IsValid() {
		return nil
	}
	items := d.value.Relations()
	result := make([]Relation, len(items))
	for i, item := range items {
		result[i] = relationFromInternal(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return result
}

func (d Dataset) ValueBindings() []ValueBinding {
	if !d.IsValid() {
		return nil
	}
	items := d.value.ValueBindings()
	result := make([]ValueBinding, len(items))
	for i, item := range items {
		result[i] = valueBindingFromInternal(item)
	}
	sort.SliceStable(result, func(i, j int) bool { return valueBindingSortKey(result[i]) < valueBindingSortKey(result[j]) })
	return result
}

func (d Dataset) Entity(id ID) (Entity, bool) {
	if !d.IsValid() || id.IsZero() {
		return Entity{}, false
	}
	value, ok := d.value.Entity(id.value)
	return entityFromInternal(value), ok
}
func (d Dataset) Relation(id ID) (Relation, bool) {
	if !d.IsValid() || id.IsZero() {
		return Relation{}, false
	}
	value, ok := d.value.Relation(id.value)
	return relationFromInternal(value), ok
}

// DatasetBuilder is mutable only until Build succeeds. It is intentionally
// not concurrency-safe; the resulting Dataset is safe for concurrent reads.
// datasetID is a caller-supplied immutable snapshot identity and must not be
// reused for different Dataset contents.
type DatasetBuilder struct{ value *model.DatasetBuilder }

func NewDatasetBuilder(base Model, datasetID ID) (*DatasetBuilder, error) {
	if !base.IsValid() || datasetID.IsZero() {
		return nil, fmt.Errorf("%w: committed Model and Dataset ID are required", ErrInvalidArgument)
	}
	builder, err := model.NewDatasetBuilder(base.version, datasetID.value)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return &DatasetBuilder{value: builder}, nil
}

func (b *DatasetBuilder) IsValid() bool { return b != nil && b.value != nil && b.value.IsValid() }
func (b *DatasetBuilder) ID() ID {
	if b == nil || b.value == nil {
		return ID{}
	}
	return idFromInternal(b.value.ID())
}
func (b *DatasetBuilder) ModelVersionID() ID {
	if b == nil || b.value == nil {
		return ID{}
	}
	return idFromInternal(b.value.ModelVersionID())
}
func (b *DatasetBuilder) AddEntity(value Entity) error {
	if !b.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid unbuilt DatasetBuilder and Entity are required", ErrInvalidArgument)
	}
	if err := b.value.AddEntity(value.value); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return nil
}
func (b *DatasetBuilder) AddRelation(value Relation) error {
	if !b.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid unbuilt DatasetBuilder and Relation are required", ErrInvalidArgument)
	}
	if err := b.value.AddRelation(value.value); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return nil
}
func (b *DatasetBuilder) AddValueBinding(value ValueBinding) error {
	if !b.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid unbuilt DatasetBuilder and ValueBinding are required", ErrInvalidArgument)
	}
	if err := b.value.AddValueBinding(value.value); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return nil
}

// Validate returns every deterministic Dataset and combined semantic-data
// issue. A non-OK report is not a Go operational error; Build converts it to
// the ordinary public ValidationError.
func (b *DatasetBuilder) Validate() (ValidationReport, error) {
	if b == nil || b.value == nil {
		return ValidationReport{}, fmt.Errorf("%w: DatasetBuilder is invalid", ErrInvalidArgument)
	}
	return validationReportFromInternal(b.value.Validate()), nil
}

func (b *DatasetBuilder) Build() (Dataset, error) {
	if !b.IsValid() {
		return Dataset{}, fmt.Errorf("%w: DatasetBuilder is invalid", ErrInvalidArgument)
	}
	value, err := b.value.Build()
	if err != nil {
		return Dataset{}, publicError(err)
	}
	return Dataset{value: value}, nil
}
