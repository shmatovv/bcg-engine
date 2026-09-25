package bcg

import (
	"fmt"
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
)

// RuntimeValidator performs locally-provable structural checks only. It does
// not attempt global keys, cardinality, constraints, or contextual invariants.
type RuntimeValidator struct{ model Model }

func (m Model) NewRuntimeValidator() RuntimeValidator { return RuntimeValidator{model: m} }
func (v RuntimeValidator) ValidateEntity(entity Entity) ValidationReport {
	b, err := NewDatasetBuilder(v.model, NewID())
	if err != nil {
		return invalidRuntimeReport("invalid runtime validator")
	}
	_ = b.AddEntity(entity)
	r, _ := b.Validate()
	return r
}
func (v RuntimeValidator) ValidateRelation(relation Relation, from Entity, to Entity) ValidationReport {
	b, err := NewDatasetBuilder(v.model, NewID())
	if err != nil {
		return invalidRuntimeReport("invalid runtime validator")
	}
	_ = b.AddEntity(from)
	_ = b.AddEntity(to)
	_ = b.AddRelation(relation)
	r, _ := b.Validate()
	return r
}

// RuntimeObjectDescriptor describes the concrete owner used for local binding validation.
type RuntimeObjectDescriptor struct {
	entity   Entity
	relation Relation
	kind     ObjectKind
}

func EntityRuntimeObject(entity Entity) RuntimeObjectDescriptor {
	return RuntimeObjectDescriptor{entity: entity, kind: ObjectEntity}
}
func RelationRuntimeObject(relation Relation) RuntimeObjectDescriptor {
	return RuntimeObjectDescriptor{relation: relation, kind: ObjectRelation}
}
func (o RuntimeObjectDescriptor) IsValid() bool {
	return (o.kind == ObjectEntity && o.entity.IsValid()) || (o.kind == ObjectRelation && o.relation.IsValid())
}
func (v RuntimeValidator) ValidateValueBinding(binding ValueBinding, object RuntimeObjectDescriptor) ValidationReport {
	b, err := NewDatasetBuilder(v.model, NewID())
	if err != nil || !object.IsValid() {
		return invalidRuntimeReport("invalid runtime binding object")
	}
	if object.kind == ObjectEntity {
		_ = b.AddEntity(object.entity)
	} else {
		_ = b.AddRelation(object.relation)
	}
	_ = b.AddValueBinding(binding)
	r, _ := b.Validate()
	return r
}
func invalidRuntimeReport(message string) ValidationReport {
	return ValidationReport{issues: []ValidationIssue{{kind: "InvalidRuntimeObject", message: message}}}
}

// IntegrityIssueKind classifies non-fatal runtime business findings.
type IntegrityIssueKind string

const (
	RequiredValueMissing         IntegrityIssueKind = "RequiredValueMissing"
	RequiredValueNull            IntegrityIssueKind = "RequiredValueNull"
	EntityKeyMissingComponent    IntegrityIssueKind = "EntityKeyMissingComponent"
	EntityKeyDuplicate           IntegrityIssueKind = "EntityKeyDuplicate"
	OutgoingCardinalityViolation IntegrityIssueKind = "OutgoingCardinalityViolation"
	IncomingCardinalityViolation IntegrityIssueKind = "IncomingCardinalityViolation"
	IntegrityInvariantViolated   IntegrityIssueKind = "InvariantViolated"
	IntegrityInvariantUndefined  IntegrityIssueKind = "InvariantUndefined"
)

type IntegrityIssue struct {
	kind                                      IntegrityIssueKind
	declarationID, objectID, conflictingID    ID
	hasDeclaration, hasObject, hasConflicting bool
}

func (i IntegrityIssue) Kind() IntegrityIssueKind        { return i.kind }
func (i IntegrityIssue) DeclarationID() (ID, bool)       { return i.declarationID, i.hasDeclaration }
func (i IntegrityIssue) ObjectID() (ID, bool)            { return i.objectID, i.hasObject }
func (i IntegrityIssue) ConflictingObjectID() (ID, bool) { return i.conflictingID, i.hasConflicting }

type IntegrityReport struct {
	modelVersionID, datasetID ID
	issues                    []IntegrityIssue
}

func (r IntegrityReport) ModelVersionID() ID       { return r.modelVersionID }
func (r IntegrityReport) DatasetID() ID            { return r.datasetID }
func (r IntegrityReport) Issues() []IntegrityIssue { return append([]IntegrityIssue(nil), r.issues...) }
func (r IntegrityReport) OK() bool                 { return len(r.issues) == 0 }
func (r IntegrityReport) Len() int                 { return len(r.issues) }

func (s EvaluationSnapshot) CheckStructuralIntegrity() IntegrityReport {
	r := IntegrityReport{modelVersionID: s.ModelVersionID(), datasetID: s.DatasetID()}
	if !s.IsValid() {
		return r
	}
	entities := snapshotEntities(s)
	relations := snapshotRelations(s)
	bindings := append(s.model.ValueBindings(), s.dataset.ValueBindings()...)
	base := map[string]ValueBinding{}
	for _, b := range bindings {
		k := b.Key()
		if a, ok := k.Target().ObjectProperty(); ok && len(k.ScopeSelectors()) == 0 {
			if _, period := k.EffectivePeriod(); !period {
				base[objectPropertyKey(a)] = b
			}
		}
	}
	for _, p := range s.model.PropertyDefinitionDeclarations() {
		comp := p.Computation()
		presence, input := comp.Presence()
		if !input || presence != PresenceRequired {
			continue
		}
		for _, e := range entities {
			if p.Owner().Kind() == PropertyOwnerEntityType && p.Owner().ID() == e.TypeID() {
				r.required(&base, e.ID(), p.ID())
			}
		}
		for _, rel := range relations {
			if p.Owner().Kind() == PropertyOwnerRelationType && p.Owner().ID() == rel.TypeID() {
				r.required(&base, rel.ID(), p.ID())
			}
		}
	}
	for _, key := range s.model.EntityKeyDefinitions() {
		seen := map[string]ID{}
		for _, e := range entities {
			if e.TypeID() != key.EntityTypeID() {
				continue
			}
			parts := make([]string, 0, len(key.PropertyDefinitionIDs()))
			valid := true
			for _, pid := range key.PropertyDefinitionIDs() {
				b, ok := base[objectPropertyKeyMust(e.ID(), pid)]
				if !ok || b.Value().IsNull() {
					r.add(IntegrityIssue{kind: EntityKeyMissingComponent, declarationID: key.ID(), hasDeclaration: true, objectID: e.ID(), hasObject: true})
					valid = false
					break
				}
				value, _ := b.Value().Value()
				parts = append(parts, typedValueKey(value))
			}
			if !valid {
				continue
			}
			composite := ""
			for _, p := range parts {
				composite += p + "\x00"
			}
			if prior, ok := seen[composite]; ok {
				r.add(IntegrityIssue{kind: EntityKeyDuplicate, declarationID: key.ID(), hasDeclaration: true, objectID: e.ID(), hasObject: true, conflictingID: prior, hasConflicting: true})
			} else {
				seen[composite] = e.ID()
			}
		}
	}
	for _, typ := range s.model.RelationTypeDefinitions() {
		out, in := map[ID]uint64{}, map[ID]uint64{}
		for _, rel := range relations {
			if rel.TypeID() == typ.ID() {
				out[rel.From()]++
				in[rel.To()]++
			}
		}
		for _, e := range entities {
			if violates(typ.OutgoingCardinality(), out[e.ID()]) {
				r.add(IntegrityIssue{kind: OutgoingCardinalityViolation, declarationID: typ.ID(), hasDeclaration: true, objectID: e.ID(), hasObject: true})
			}
			if violates(typ.IncomingCardinality(), in[e.ID()]) {
				r.add(IntegrityIssue{kind: IncomingCardinalityViolation, declarationID: typ.ID(), hasDeclaration: true, objectID: e.ID(), hasObject: true})
			}
		}
	}
	r.sort()
	return r
}
func (r *IntegrityReport) required(base *map[string]ValueBinding, object, property ID) {
	b, ok := (*base)[objectPropertyKeyMust(object, property)]
	if !ok {
		r.add(IntegrityIssue{kind: RequiredValueMissing, declarationID: property, hasDeclaration: true, objectID: object, hasObject: true})
		return
	}
	if b.Value().IsNull() { // nullability is enforced structurally; required-nullable is valid.
		return
	}
}
func objectPropertyKey(a PropertyAddress) string {
	return objectPropertyKeyMust(a.ObjectID(), a.DefinitionID())
}
func objectPropertyKeyMust(o, p ID) string { return o.String() + ":" + p.String() }
func typedValueKey(v Value) string         { return publicValueSortKey(v) }
func violates(c Cardinality, n uint64) bool {
	if n < c.Min() {
		return true
	}
	if m, ok := c.Max(); ok && n > m {
		return true
	}
	return false
}
func (r *IntegrityReport) add(i IntegrityIssue) { r.issues = append(r.issues, i) }
func (r *IntegrityReport) sort() {
	sort.Slice(r.issues, func(i, j int) bool {
		a, b := r.issues[i], r.issues[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.declarationID != b.declarationID {
			return a.declarationID.String() < b.declarationID.String()
		}
		return a.objectID.String() < b.objectID.String()
	})
}

// CheckIntegrity adds contextual invariant findings to the context-independent report.
func (s EvaluationSnapshot) CheckIntegrity(context CalculationContext) IntegrityReport {
	r := s.CheckStructuralIntegrity()
	if !s.IsValid() {
		return r
	}
	ctx, err := context.internal()
	if err != nil {
		return r
	}
	for _, e := range snapshotEntities(s) {
		x, err := checkInvariantsForObjectSource(s.evaluationSource(), address.EntityRef(e.value.ID()), ctx)
		if err != nil {
			continue
		}
		for _, v := range x.Results() {
			if v.State() == InvariantViolated {
				r.add(IntegrityIssue{kind: IntegrityInvariantViolated, declarationID: v.Invariant().ID(), hasDeclaration: true, objectID: v.SubjectID(), hasObject: true})
			}
			if v.State() == InvariantUndefined {
				r.add(IntegrityIssue{kind: IntegrityInvariantUndefined, declarationID: v.Invariant().ID(), hasDeclaration: true, objectID: v.SubjectID(), hasObject: true})
			}
		}
	}
	r.sort()
	return r
}

func snapshotEntities(s EvaluationSnapshot) []Entity {
	r := make([]Entity, 0, len(s.model.Entities())+len(s.dataset.Entities()))
	for _, x := range s.model.Entities() {
		r = append(r, x.Declaration())
	}
	return append(r, s.dataset.Entities()...)
}
func snapshotRelations(s EvaluationSnapshot) []Relation {
	r := make([]Relation, 0, len(s.model.Relations())+len(s.dataset.Relations()))
	for _, x := range s.model.Relations() {
		r = append(r, x.Declaration())
	}
	return append(r, s.dataset.Relations()...)
}

// DatasetPatch is an immutable sequence of operations. Operations are applied
// to the final candidate world atomically; no intermediate state is published.
type DatasetPatch struct {
	base       ID
	operations []datasetPatchOperation
}
type datasetPatchOperation struct {
	kind     string
	entity   Entity
	relation Relation
	binding  ValueBinding
	id       ID
	key      ValueBindingKey
}

func (p DatasetPatch) BaseDatasetID() ID { return p.base }
func (p DatasetPatch) Len() int          { return len(p.operations) }

type DatasetPatchBuilder struct {
	base       ID
	operations []datasetPatchOperation
}

func NewDatasetPatchBuilder(baseDatasetID ID) (*DatasetPatchBuilder, error) {
	if baseDatasetID.IsZero() {
		return nil, fmt.Errorf("%w: base Dataset ID is required", ErrInvalidArgument)
	}
	return &DatasetPatchBuilder{base: baseDatasetID}, nil
}
func (b *DatasetPatchBuilder) AddEntity(v Entity) error     { return b.entity("addEntity", v) }
func (b *DatasetPatchBuilder) ReplaceEntity(v Entity) error { return b.entity("replaceEntity", v) }
func (b *DatasetPatchBuilder) entity(k string, v Entity) error {
	if b == nil || !v.IsValid() {
		return fmt.Errorf("%w: valid entity required", ErrInvalidArgument)
	}
	b.operations = append(b.operations, datasetPatchOperation{kind: k, entity: v})
	return nil
}
func (b *DatasetPatchBuilder) RemoveEntity(id ID) error     { return b.remove("removeEntity", id) }
func (b *DatasetPatchBuilder) AddRelation(v Relation) error { return b.relation("addRelation", v) }
func (b *DatasetPatchBuilder) ReplaceRelation(v Relation) error {
	return b.relation("replaceRelation", v)
}
func (b *DatasetPatchBuilder) relation(k string, v Relation) error {
	if b == nil || !v.IsValid() {
		return fmt.Errorf("%w: valid relation required", ErrInvalidArgument)
	}
	b.operations = append(b.operations, datasetPatchOperation{kind: k, relation: v})
	return nil
}
func (b *DatasetPatchBuilder) RemoveRelation(id ID) error { return b.remove("removeRelation", id) }
func (b *DatasetPatchBuilder) remove(k string, id ID) error {
	if b == nil || id.IsZero() {
		return fmt.Errorf("%w: ID required", ErrInvalidArgument)
	}
	b.operations = append(b.operations, datasetPatchOperation{kind: k, id: id})
	return nil
}
func (b *DatasetPatchBuilder) SetValueBinding(v ValueBinding) error {
	if b == nil || !v.IsValid() {
		return fmt.Errorf("%w: valid binding required", ErrInvalidArgument)
	}
	b.operations = append(b.operations, datasetPatchOperation{kind: "setBinding", binding: v})
	return nil
}

// ReplaceValueBinding requires an existing binding with the same compound key.
func (b *DatasetPatchBuilder) ReplaceValueBinding(key ValueBindingKey, v ValueBinding) error {
	if b == nil || !key.IsValid() || !v.IsValid() || !key.Equal(v.Key()) {
		return fmt.Errorf("%w: valid matching binding key and value required", ErrInvalidArgument)
	}
	b.operations = append(b.operations, datasetPatchOperation{kind: "replaceBinding", key: key, binding: v})
	return nil
}
func (b *DatasetPatchBuilder) RemoveValueBinding(k ValueBindingKey) error {
	if b == nil || !k.IsValid() {
		return fmt.Errorf("%w: valid binding key required", ErrInvalidArgument)
	}
	b.operations = append(b.operations, datasetPatchOperation{kind: "removeBinding", key: k})
	return nil
}
func (b *DatasetPatchBuilder) Build() (DatasetPatch, error) {
	if b == nil {
		return DatasetPatch{}, fmt.Errorf("%w: patch builder invalid", ErrInvalidArgument)
	}
	return DatasetPatch{base: b.base, operations: append([]datasetPatchOperation(nil), b.operations...)}, nil
}

func (d Dataset) Apply(model Model, newDatasetID ID, patch DatasetPatch) (Dataset, ValidationReport, error) {
	if !d.IsValid() || !model.IsValid() || newDatasetID.IsZero() || newDatasetID == d.ID() {
		return Dataset{}, ValidationReport{}, fmt.Errorf("%w: valid Dataset, Model and new Dataset ID required", ErrInvalidArgument)
	}
	if d.ID() != patch.base {
		return Dataset{}, ValidationReport{}, fmt.Errorf("%w: patch base Dataset ID mismatch", ErrInvalidArgument)
	}
	if d.ModelVersionID() != model.VersionID() {
		return Dataset{}, ValidationReport{}, fmt.Errorf("%w: Dataset belongs to another ModelVersion", ErrInvalidArgument)
	}
	es := map[ID]Entity{}
	rs := map[ID]Relation{}
	bs := []ValueBinding{}
	for _, x := range d.Entities() {
		es[x.ID()] = x
	}
	for _, x := range d.Relations() {
		rs[x.ID()] = x
	}
	bs = append(bs, d.ValueBindings()...)
	for _, o := range patch.operations {
		switch o.kind {
		case "addEntity":
			if _, ok := es[o.entity.ID()]; ok {
				return Dataset{}, ValidationReport{}, fmt.Errorf("%w: entity already exists", ErrInvalidArgument)
			}
			es[o.entity.ID()] = o.entity
		case "replaceEntity":
			if _, ok := es[o.entity.ID()]; !ok {
				return Dataset{}, ValidationReport{}, fmt.Errorf("%w: entity not found", ErrInvalidArgument)
			}
			es[o.entity.ID()] = o.entity
		case "removeEntity":
			if _, ok := es[o.id]; !ok {
				return Dataset{}, ValidationReport{}, fmt.Errorf("%w: entity not found", ErrInvalidArgument)
			}
			delete(es, o.id)
		case "addRelation":
			if _, ok := rs[o.relation.ID()]; ok {
				return Dataset{}, ValidationReport{}, fmt.Errorf("%w: relation already exists", ErrInvalidArgument)
			}
			rs[o.relation.ID()] = o.relation
		case "replaceRelation":
			if _, ok := rs[o.relation.ID()]; !ok {
				return Dataset{}, ValidationReport{}, fmt.Errorf("%w: relation not found", ErrInvalidArgument)
			}
			rs[o.relation.ID()] = o.relation
		case "removeRelation":
			if _, ok := rs[o.id]; !ok {
				return Dataset{}, ValidationReport{}, fmt.Errorf("%w: relation not found", ErrInvalidArgument)
			}
			delete(rs, o.id)
		case "setBinding":
			found := false
			for i, x := range bs {
				if x.Key().Equal(o.binding.Key()) {
					bs[i] = o.binding
					found = true
				}
			}
			if !found {
				bs = append(bs, o.binding)
			}
		case "replaceBinding":
			found := false
			for i, x := range bs {
				if x.Key().Equal(o.key) {
					bs[i] = o.binding
					found = true
				}
			}
			if !found {
				return Dataset{}, ValidationReport{}, fmt.Errorf("%w: binding not found", ErrInvalidArgument)
			}
		case "removeBinding":
			out := bs[:0]
			found := false
			for _, x := range bs {
				if x.Key().Equal(o.key) {
					found = true
					continue
				}
				out = append(out, x)
			}
			if !found {
				return Dataset{}, ValidationReport{}, fmt.Errorf("%w: binding not found", ErrInvalidArgument)
			}
			bs = out
		}
	}
	b, err := NewDatasetBuilder(model, newDatasetID)
	if err != nil {
		return Dataset{}, ValidationReport{}, err
	}
	entityIDs := make([]ID, 0, len(es))
	for id := range es {
		entityIDs = append(entityIDs, id)
	}
	sort.Slice(entityIDs, func(i, j int) bool { return entityIDs[i].String() < entityIDs[j].String() })
	for _, id := range entityIDs {
		_ = b.AddEntity(es[id])
	}
	relationIDs := make([]ID, 0, len(rs))
	for id := range rs {
		relationIDs = append(relationIDs, id)
	}
	sort.Slice(relationIDs, func(i, j int) bool { return relationIDs[i].String() < relationIDs[j].String() })
	for _, id := range relationIDs {
		_ = b.AddRelation(rs[id])
	}
	for _, x := range bs {
		_ = b.AddValueBinding(x)
	}
	report, _ := b.Validate()
	if !report.OK() {
		return Dataset{}, report, nil
	}
	next, err := b.Build()
	return next, report, err
}

func (d Dataset) Rebase(oldModel, newModel Model, newDatasetID ID) (Dataset, ValidationReport, error) {
	if !oldModel.IsValid() || !newModel.IsValid() || oldModel.ModelID() != newModel.ModelID() {
		return Dataset{}, ValidationReport{}, fmt.Errorf("%w: compatible committed Models required", ErrDifferentLogicalModel)
	}
	if newDatasetID.IsZero() || newDatasetID == d.ID() {
		return Dataset{}, ValidationReport{}, fmt.Errorf("%w: a new Dataset ID is required", ErrInvalidArgument)
	}
	if d.ModelVersionID() != oldModel.VersionID() {
		return Dataset{}, ValidationReport{}, fmt.Errorf("%w: Dataset does not belong to old Model", ErrInvalidArgument)
	}
	b, err := NewDatasetBuilder(newModel, newDatasetID)
	if err != nil {
		return Dataset{}, ValidationReport{}, err
	}
	for _, x := range d.Entities() {
		_ = b.AddEntity(x)
	}
	for _, x := range d.Relations() {
		_ = b.AddRelation(x)
	}
	for _, x := range d.ValueBindings() {
		_ = b.AddValueBinding(x)
	}
	report, _ := b.Validate()
	if !report.OK() {
		return Dataset{}, report, nil
	}
	next, err := b.Build()
	return next, report, err
}
