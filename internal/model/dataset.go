package model

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
)

// Dataset is an immutable, fully materialized runtime semantic-data snapshot
// for one exact committed ModelVersion. It owns only concrete objects,
// relations and object-level value bindings; schema remains model-owned.
type Dataset struct {
	id             identity.UUID
	modelVersionID identity.UUID

	entities      []semantic.Entity
	relations     []semantic.Relation
	valueBindings []semantic.ValueBinding
	indexes       indexes
}

func newDataset(id, modelVersionID identity.UUID, entities []semantic.Entity, relations []semantic.Relation, bindings []semantic.ValueBinding) Dataset {
	return Dataset{
		id:             id,
		modelVersionID: modelVersionID,
		entities:       cloneSlice(entities),
		relations:      cloneSlice(relations),
		valueBindings:  cloneSlice(bindings),
		indexes: buildIndexes(nil, nil, nil, nil, nil, nil,
			entities, relations, nil, nil, bindings),
	}
}

func (d Dataset) IsValid() bool {
	return !d.id.IsZero() && !d.modelVersionID.IsZero()
}

func (d Dataset) ID() identity.UUID                      { return d.id }
func (d Dataset) ModelVersionID() identity.UUID          { return d.modelVersionID }
func (d Dataset) Entities() []semantic.Entity            { return cloneSlice(d.entities) }
func (d Dataset) Relations() []semantic.Relation         { return cloneSlice(d.relations) }
func (d Dataset) ValueBindings() []semantic.ValueBinding { return cloneSlice(d.valueBindings) }
func (d Dataset) Entity(id identity.UUID) (semantic.Entity, bool) {
	v, ok := d.indexes.entityByID[id]
	return v, ok
}
func (d Dataset) Relation(id identity.UUID) (semantic.Relation, bool) {
	v, ok := d.indexes.relationByID[id]
	return v, ok
}
func (d Dataset) OutgoingRelations(entityID, relationTypeID identity.UUID) []semantic.Relation {
	return cloneSlice(d.indexes.outgoingRelationsByEndpoint[relationTraversalKey{entityID: entityID, relationTypeID: relationTypeID}])
}
func (d Dataset) IncomingRelations(entityID, relationTypeID identity.UUID) []semantic.Relation {
	return cloneSlice(d.indexes.incomingRelationsByEndpoint[relationTraversalKey{entityID: entityID, relationTypeID: relationTypeID}])
}
func (d Dataset) ValueBindingsForTarget(target semantic.ValueTarget) []semantic.ValueBinding {
	return cloneSlice(d.indexes.valueBindingsByTarget[target])
}

// DatasetBuilder is the mutable construction boundary for one Dataset.
// It is not safe for concurrent use and is sealed after a successful Build.
type DatasetBuilder struct {
	base ModelVersion
	id   identity.UUID

	entities      []semantic.Entity
	relations     []semantic.Relation
	valueBindings []semantic.ValueBinding
	built         bool
}

func NewDatasetBuilder(base ModelVersion, id identity.UUID) (*DatasetBuilder, error) {
	if !base.IsCommitted() {
		return nil, fmt.Errorf("model: Dataset requires a committed ModelVersion")
	}
	if id.IsZero() {
		return nil, fmt.Errorf("model: Dataset id must not be zero")
	}
	return &DatasetBuilder{base: base, id: id}, nil
}

func (b *DatasetBuilder) IsValid() bool {
	return b != nil && b.base.IsCommitted() && !b.id.IsZero() && !b.built
}
func (b *DatasetBuilder) ID() identity.UUID {
	if b == nil {
		return identity.UUID{}
	}
	return b.id
}
func (b *DatasetBuilder) ModelVersionID() identity.UUID {
	if b == nil {
		return identity.UUID{}
	}
	return b.base.ID()
}

func (b *DatasetBuilder) AddEntity(value semantic.Entity) error {
	if !b.IsValid() || !value.IsValid() {
		return fmt.Errorf("model: valid unbuilt DatasetBuilder and Entity are required")
	}
	b.entities = append(b.entities, value)
	return nil
}
func (b *DatasetBuilder) AddRelation(value semantic.Relation) error {
	if !b.IsValid() || !value.IsValid() {
		return fmt.Errorf("model: valid unbuilt DatasetBuilder and Relation are required")
	}
	b.relations = append(b.relations, value)
	return nil
}
func (b *DatasetBuilder) AddValueBinding(value semantic.ValueBinding) error {
	if !b.IsValid() || !value.IsValid() {
		return fmt.Errorf("model: valid unbuilt DatasetBuilder and ValueBinding are required")
	}
	b.valueBindings = append(b.valueBindings, value)
	return nil
}

// Validate reports all Dataset-local and ModelVersion+Dataset semantic
// violations except dependency cycles. Cycles are a property of the effective
// graph and are deliberately rejected by NewEvaluationSource, which builds it
// once for the immutable EvaluationSnapshot.
func (b *DatasetBuilder) Validate() ValidationReport {
	var report ValidationReport
	if !b.IsValid() {
		report.append(newValidationIssue(ValidationIssueInvalidDataset, "dataset builder is invalid or already built"))
		return report
	}

	datasetEntities := make(map[identity.UUID]struct{}, len(b.entities))
	for _, entity := range b.entities {
		datasetEntities[entity.ID()] = struct{}{}
	}
	datasetRelations := make(map[identity.UUID]struct{}, len(b.relations))
	for _, relation := range b.relations {
		datasetRelations[relation.ID()] = struct{}{}
	}
	datasetObject := func(object semantic.ObjectRef) bool {
		switch object.Kind() {
		case semantic.ObjectRefEntity:
			_, ok := datasetEntities[object.ID()]
			return ok
		case semantic.ObjectRefRelation:
			_, ok := datasetRelations[object.ID()]
			return ok
		default:
			return false
		}
	}

	for _, relation := range b.relations {
		if !relation.IsValid() {
			continue
		}
		_, fromDataset := datasetEntities[relation.From()]
		_, toDataset := datasetEntities[relation.To()]
		if !fromDataset && !toDataset {
			report.append(newSubjectValidationIssue(ValidationIssueDatasetRelationRequiresDatasetEndpoint, relation.ID(), "dataset relation must have at least one dataset-owned endpoint"))
		}
	}
	for _, binding := range b.valueBindings {
		if !binding.IsValid() {
			continue
		}
		if binding.Target().Kind() != semantic.ValueTargetObjectProperty {
			report.append(newValidationIssue(ValidationIssueDatasetTypeBindingNotAllowed, "dataset permits object-level value bindings only"))
			continue
		}
		address, _ := binding.Target().ObjectProperty()
		if !datasetObject(address.Object()) {
			report.append(newRelatedValidationIssue(ValidationIssueDatasetBindingTargetNotOwned, address.Object().ID(), address.DefinitionID(), "dataset value binding must target a dataset-owned object"))
		}
	}

	combined := NewDraftFromVersion(b.base)
	combined.entities = append(combined.entities, b.entities...)
	combined.relations = append(combined.relations, b.relations...)
	combined.valueBindings = append(combined.valueBindings, b.valueBindings...)
	baseReport := combined.validateWithoutDependencyGraph()
	for _, issue := range baseReport.Issues() {
		report.append(issue)
	}
	report.sortDeterministically()
	return report
}

func (b *DatasetBuilder) Build() (Dataset, error) {
	report := b.Validate()
	if !report.OK() {
		return Dataset{}, report
	}
	result := newDataset(b.id, b.base.ID(), b.entities, b.relations, b.valueBindings)
	b.built = true
	return result, nil
}
