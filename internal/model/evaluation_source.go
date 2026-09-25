package model

import (
	"fmt"
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/function"
	"github.com/shmatovv/bcg-engine/v2/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
)

// EvaluationSource composes immutable model-owned data with one immutable
// Dataset. It is a concrete adapter rather than a new broad abstraction:
// consumers continue to declare only the narrow capabilities they use.
//
// The dependency graph is built exactly once while creating the source and is
// safe to reuse by plans, sessions and impact analysis.
type EvaluationSource struct {
	version  ModelVersion
	dataset  Dataset
	identity EffectiveSourceIdentity
	graph    dependency.Graph
}

func NewEvaluationSource(version ModelVersion, dataset Dataset) (EvaluationSource, error) {
	if !version.IsCommitted() || !dataset.IsValid() {
		return EvaluationSource{}, fmt.Errorf("model: committed ModelVersion and valid Dataset are required")
	}
	if dataset.ModelVersionID() != version.ID() {
		return EvaluationSource{}, fmt.Errorf("model: Dataset belongs to a different ModelVersion")
	}
	source := EvaluationSource{version: version, dataset: dataset, identity: NewDatasetSourceIdentity(version.ID(), dataset.ID())}
	graph, err := dependency.Build(source)
	if err != nil {
		return EvaluationSource{}, fmt.Errorf("model: combined dependency graph: %w", err)
	}
	if cycles := dependency.FindCycles(graph); len(cycles) > 0 {
		return EvaluationSource{}, fmt.Errorf("model: Dataset introduces a value dependency cycle: %s", formatDependencyCycle(cycles[0].Vertices()))
	}
	source.graph = graph
	return source, nil
}

func (s EvaluationSource) IsCommitted() bool {
	return s.version.IsCommitted() && s.dataset.IsValid() && s.dataset.ModelVersionID() == s.version.ID() && s.identity.IsValid()
}
func (s EvaluationSource) ID() identity.UUID                       { return s.version.ID() }
func (s EvaluationSource) ModelVersion() ModelVersion              { return s.version }
func (s EvaluationSource) Dataset() Dataset                        { return s.dataset }
func (s EvaluationSource) SourceIdentity() EffectiveSourceIdentity { return s.identity }
func (s EvaluationSource) DependencyGraph() dependency.Graph       { return s.graph }
func (s EvaluationSource) PrecisionPolicy() precision.Policy       { return s.version.PrecisionPolicy() }

func (s EvaluationSource) Entities() []semantic.Entity {
	result := append(s.version.Entities(), s.dataset.Entities()...)
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return result
}
func (s EvaluationSource) Relations() []semantic.Relation {
	result := append(s.version.Relations(), s.dataset.Relations()...)
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return result
}
func mergeRelations(left, right []semantic.Relation) []semantic.Relation {
	result := make([]semantic.Relation, 0, len(left)+len(right))
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		if left[i].ID().String() < right[j].ID().String() {
			result = append(result, left[i])
			i++
		} else {
			result = append(result, right[j])
			j++
		}
	}
	result = append(result, left[i:]...)
	result = append(result, right[j:]...)
	return result
}
func (s EvaluationSource) OutgoingRelations(entityID, relationTypeID identity.UUID) []semantic.Relation {
	return mergeRelations(s.version.OutgoingRelations(entityID, relationTypeID), s.dataset.OutgoingRelations(entityID, relationTypeID))
}
func (s EvaluationSource) IncomingRelations(entityID, relationTypeID identity.UUID) []semantic.Relation {
	return mergeRelations(s.version.IncomingRelations(entityID, relationTypeID), s.dataset.IncomingRelations(entityID, relationTypeID))
}
func (s EvaluationSource) Entity(id identity.UUID) (semantic.Entity, bool) {
	if value, ok := s.dataset.Entity(id); ok {
		return value, true
	}
	return s.version.Entity(id)
}
func (s EvaluationSource) Relation(id identity.UUID) (semantic.Relation, bool) {
	if value, ok := s.dataset.Relation(id); ok {
		return value, true
	}
	return s.version.Relation(id)
}
func (s EvaluationSource) ValueBindingsForTarget(target semantic.ValueTarget) []semantic.ValueBinding {
	if target.Kind() == semantic.ValueTargetObjectProperty {
		addressValue, _ := target.ObjectProperty()
		if s.datasetObject(addressValue.Object()) {
			return s.dataset.ValueBindingsForTarget(target)
		}
	}
	return s.version.ValueBindingsForTarget(target)
}
func (s EvaluationSource) datasetObject(object address.ObjectRef) bool {
	switch object.Kind() {
	case address.ObjectRefEntity:
		_, ok := s.dataset.Entity(object.ID())
		return ok
	case address.ObjectRefRelation:
		_, ok := s.dataset.Relation(object.ID())
		return ok
	default:
		return false
	}
}

// Schema and declaration capabilities remain strictly model-owned.
func (s EvaluationSource) EntityType(id identity.UUID) (schema.EntityType, bool) {
	return s.version.EntityType(id)
}
func (s EvaluationSource) RelationType(id identity.UUID) (schema.RelationType, bool) {
	return s.version.RelationType(id)
}
func (s EvaluationSource) Trait(id identity.UUID) (schema.Trait, bool) { return s.version.Trait(id) }
func (s EvaluationSource) PropertyContract(id identity.UUID) (schema.PropertyContract, bool) {
	return s.version.PropertyContract(id)
}
func (s EvaluationSource) PropertyDefinition(id identity.UUID) (schema.PropertyDefinition, bool) {
	return s.version.PropertyDefinition(id)
}
func (s EvaluationSource) ScopeDefinition(id identity.UUID) (scope.Definition, bool) {
	return s.version.ScopeDefinition(id)
}
func (s EvaluationSource) ContractImplementation(owner schema.PropertyOwner, contractID identity.UUID) (schema.PropertyDefinition, bool) {
	return s.version.ContractImplementation(owner, contractID)
}
func (s EvaluationSource) ContractType(constraint schema.TypeConstraint, contractID identity.UUID) (typing.Type, bool) {
	return s.version.ContractType(constraint, contractID)
}
func (s EvaluationSource) FunctionRequirement(ref function.Ref) (function.Requirement, bool) {
	return s.version.FunctionRequirement(ref)
}
func (s EvaluationSource) Constraints() []constraint.Constraint { return s.version.Constraints() }
func (s EvaluationSource) Constraint(id identity.UUID) (constraint.Constraint, bool) {
	return s.version.Constraint(id)
}
func (s EvaluationSource) Rules() []rule.Rule                      { return s.version.Rules() }
func (s EvaluationSource) Rule(id identity.UUID) (rule.Rule, bool) { return s.version.Rule(id) }
func (s EvaluationSource) Invariants() []invariant.Invariant       { return s.version.Invariants() }
func (s EvaluationSource) Invariant(id identity.UUID) (invariant.Invariant, bool) {
	return s.version.Invariant(id)
}
func (s EvaluationSource) ObjectType(object address.ObjectRef) (schema.PropertyOwner, bool) {
	switch object.Kind() {
	case address.ObjectRefEntity:
		entity, ok := s.Entity(object.ID())
		if !ok {
			return schema.PropertyOwner{}, false
		}
		return schema.EntityTypeOwner(entity.TypeID()), true
	case address.ObjectRefRelation:
		relation, ok := s.Relation(object.ID())
		if !ok {
			return schema.PropertyOwner{}, false
		}
		return schema.RelationTypeOwner(relation.TypeID()), true
	default:
		return schema.PropertyOwner{}, false
	}
}

var _ dependency.Source = EvaluationSource{}
