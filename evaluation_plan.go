package bcg

import (
	"fmt"
	"sort"
)

// PropertyTarget identifies a property definition on its owning schema type.
// Unlike PropertyAddress it deliberately has no runtime object identity, which
// makes it suitable for planning a Dataset closure before records are loaded.
type PropertyTarget struct {
	owner PropertyOwner
	id    ID
}

// NewPropertyTarget constructs a schema-level evaluation target.
func NewPropertyTarget(owner PropertyOwner, definitionID ID) (PropertyTarget, error) {
	if !owner.IsValid() || definitionID.IsZero() {
		return PropertyTarget{}, fmt.Errorf("%w: property target owner and definition ID are required", ErrInvalidArgument)
	}
	return PropertyTarget{owner: owner, id: definitionID}, nil
}

// PropertyDefinitionTarget constructs a target from a property declaration.
func PropertyDefinitionTarget(definition PropertyDefinition) (PropertyTarget, error) {
	if !definition.IsValid() {
		return PropertyTarget{}, fmt.Errorf("%w: property definition is invalid", ErrInvalidArgument)
	}
	return NewPropertyTarget(definition.Owner(), definition.ID())
}

func (t PropertyTarget) Owner() PropertyOwner     { return t.owner }
func (t PropertyTarget) PropertyDefinitionID() ID { return t.id }
func (t PropertyTarget) IsValid() bool            { return t.owner.IsValid() && !t.id.IsZero() }

// TraversalRequirement describes topology which a host may need to include in
// a Dataset closure. It intentionally identifies types and direction, never
// unknown concrete runtime relations or endpoint IDs.
type TraversalRequirement struct {
	source TraversalSource
	filter []PropertyDefinition
	mapped []PropertyDefinition
}

func (r TraversalRequirement) Source() TraversalSource       { return r.source }
func (r TraversalRequirement) RelationTypeID() ID            { return r.source.RelationTypeID() }
func (r TraversalRequirement) Direction() TraversalDirection { return r.source.Direction() }
func (r TraversalRequirement) Origin() TraversalOrigin       { return r.source.Origin() }
func (r TraversalRequirement) FilterDependencies() []PropertyDefinition {
	return append([]PropertyDefinition(nil), r.filter...)
}
func (r TraversalRequirement) MapDependencies() []PropertyDefinition {
	return append([]PropertyDefinition(nil), r.mapped...)
}

// DependencyPlan is an immutable, deterministic schema/runtime dependency
// description. Returned slices are always defensive copies and are safe to
// retain or read concurrently.
type DependencyPlan struct {
	targets, dependencies []PropertyDefinition
	embedded              []PropertyAddress
	entityTypes           []EntityType
	relationTypes         []RelationType
	traversals            []TraversalRequirement
	scopes                []ScopeDefinition
	temporal              bool
	functions             []FunctionRequirement
}

func (p DependencyPlan) IsValid() bool { return p.targets != nil }
func (p DependencyPlan) Targets() []PropertyDefinition {
	return append([]PropertyDefinition(nil), p.targets...)
}

// RequestedPropertyDefinitions is an explicit spelling of Targets.
func (p DependencyPlan) RequestedPropertyDefinitions() []PropertyDefinition { return p.Targets() }
func (p DependencyPlan) DependentPropertyDefinitions() []PropertyDefinition {
	return append([]PropertyDefinition(nil), p.dependencies...)
}
func (p DependencyPlan) EmbeddedObjectAddresses() []PropertyAddress {
	return append([]PropertyAddress(nil), p.embedded...)
}
func (p DependencyPlan) RequiredEntityTypes() []EntityType {
	return append([]EntityType(nil), p.entityTypes...)
}
func (p DependencyPlan) RequiredRelationTypes() []RelationType {
	return append([]RelationType(nil), p.relationTypes...)
}
func (p DependencyPlan) TraversalRequirements() []TraversalRequirement {
	r := make([]TraversalRequirement, len(p.traversals))
	for i, v := range p.traversals {
		r[i] = TraversalRequirement{source: v.source, filter: append([]PropertyDefinition(nil), v.filter...), mapped: append([]PropertyDefinition(nil), v.mapped...)}
	}
	return r
}
func (p DependencyPlan) ScopeDependencies() []ScopeDefinition {
	return append([]ScopeDefinition(nil), p.scopes...)
}

// RequiresTemporalContext reports whether binding/override resolution may
// depend on the calculation effective time.
func (p DependencyPlan) RequiresTemporalContext() bool { return p.temporal }
func (p DependencyPlan) FunctionRequirements() []FunctionRequirement {
	return append([]FunctionRequirement(nil), p.functions...)
}

// PlanEvaluation conservatively describes dependencies of schema-level
// targets. It does not access a Dataset and cannot imply that missing records
// exist outside a Dataset subsequently supplied for evaluation.
func (m Model) PlanEvaluation(targets []PropertyTarget) (DependencyPlan, error) {
	return m.planEvaluation(targets, CalculationContext{}, false)
}

// PlanEvaluationWithContext validates ctx against the Model and produces the
// same conservative semantic plan. Context is accepted so callers can use one
// planning entry point alongside evaluation; dependencies are not pruned from
// runtime branch values or unknown Dataset contents.
func (m Model) PlanEvaluationWithContext(targets []PropertyTarget, ctx CalculationContext) (DependencyPlan, error) {
	return m.planEvaluation(targets, ctx, true)
}

func (m Model) planEvaluation(targets []PropertyTarget, ctx CalculationContext, hasContext bool) (DependencyPlan, error) {
	if !m.IsValid() {
		return DependencyPlan{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	if hasContext {
		v, err := ctx.internal()
		if err != nil {
			return DependencyPlan{}, err
		}
		if err := m.validateContext(v); err != nil {
			return DependencyPlan{}, err
		}
	}
	if len(targets) == 0 {
		return DependencyPlan{targets: []PropertyDefinition{}}, nil
	}
	byID := make(map[ID]PropertyDefinition)
	for _, d := range m.PropertyDefinitionDeclarations() {
		byID[d.ID()] = d
	}
	requested := make(map[ID]PropertyDefinition)
	for _, t := range targets {
		if !t.IsValid() {
			return DependencyPlan{}, fmt.Errorf("%w: property target is invalid", ErrInvalidArgument)
		}
		d, ok := byID[t.id]
		if !ok || d.Owner() != t.owner {
			return DependencyPlan{}, fmt.Errorf("%w: property target does not belong to this Model", ErrInvalidArgument)
		}
		requested[d.ID()] = d
	}
	b := planBuilder{model: m, byID: byID, all: make(map[ID]PropertyDefinition), embedded: make(map[PropertyAddress]struct{}), entityTypes: make(map[ID]EntityType), relationTypes: make(map[ID]RelationType), scopes: make(map[ID]ScopeDefinition), functions: make(map[string]FunctionRequirement)}
	for _, d := range sortedDefinitions(requested) {
		b.walkDefinition(d)
	}
	if hasContext {
		if scenario := ctx.Scenario(); scenario.IsValid() {
			for _, override := range scenario.Overrides() {
				if _, relevant := b.all[override.Target().DefinitionID()]; !relevant {
					continue
				}
				for _, selector := range override.ScopeSelectors() {
					for _, definition := range m.ScopeDefinitionDeclarations() {
						if definition.ID() == selector.DefinitionID() {
							b.scopes[definition.ID()] = definition
						}
					}
				}
				if _, ok := override.EffectivePeriod(); ok {
					b.temporal = true
				}
			}
		}
	}
	p := DependencyPlan{targets: sortedDefinitions(requested), dependencies: sortedDefinitions(b.all), embedded: sortedAddresses(b.embedded), entityTypes: sortedEntityTypes(b.entityTypes), relationTypes: sortedRelationTypes(b.relationTypes), traversals: sortedTraversalRequirements(b.traversals), scopes: sortedScopes(b.scopes), temporal: b.temporal, functions: sortedFunctionRequirements(b.functions)}
	// Requested declarations are not repeated as dependent declarations.
	deps := p.dependencies[:0]
	for _, d := range p.dependencies {
		if _, isTarget := requested[d.ID()]; !isTarget {
			deps = append(deps, d)
		}
	}
	p.dependencies = append([]PropertyDefinition(nil), deps...)
	return p, nil
}

type planBuilder struct {
	model         Model
	byID          map[ID]PropertyDefinition
	all           map[ID]PropertyDefinition
	visiting      map[ID]bool
	embedded      map[PropertyAddress]struct{}
	entityTypes   map[ID]EntityType
	relationTypes map[ID]RelationType
	scopes        map[ID]ScopeDefinition
	functions     map[string]FunctionRequirement
	traversals    []TraversalRequirement
	temporal      bool
}

func (b *planBuilder) walkDefinition(d PropertyDefinition) {
	if _, seen := b.all[d.ID()]; seen {
		return
	}
	b.all[d.ID()] = d
	if d.Owner().Kind() == PropertyOwnerEntityType {
		for _, e := range b.model.EntityTypeDefinitions() {
			if e.ID() == d.Owner().ID() {
				b.entityTypes[e.ID()] = e
			}
		}
	} else {
		for _, r := range b.model.RelationTypeDefinitions() {
			if r.ID() == d.Owner().ID() {
				b.addRelationType(r)
			}
		}
	}
	for _, binding := range b.model.ValueBindings() {
		target := binding.Target()
		id := ID{}
		if _, value, ok := target.TypeProperty(); ok {
			id = value
		} else if value, ok := target.ObjectProperty(); ok {
			id = value.DefinitionID()
		}
		if id == d.ID() {
			for _, s := range binding.ScopeSelectors() {
				for _, sd := range b.model.ScopeDefinitionDeclarations() {
					if sd.ID() == s.DefinitionID() {
						b.scopes[sd.ID()] = sd
					}
				}
			}
			if _, ok := binding.EffectivePeriod(); ok {
				b.temporal = true
			}
		}
	}
	c := d.Computation()
	expr, ok := c.Expression()
	if ok {
		b.walkExpression(expr, nil, nil)
	}
}
func (b *planBuilder) addDefinition(id ID) {
	if d, ok := b.byID[id]; ok {
		b.walkDefinition(d)
	}
}
func (b *planBuilder) walkExpression(e Expression, filter, mapped map[ID]PropertyDefinition) {
	if ref, ok := e.PropertyReference(); ok {
		switch ref.Kind() {
		case PropertyReferenceSelf, PropertyReferenceBoundDefinition:
			if id, ok := ref.DefinitionID(); ok {
				b.addDefinition(id)
				if filter != nil {
					filter[id] = b.byID[id]
				}
				if mapped != nil {
					mapped[id] = b.byID[id]
				}
			}
		case PropertyReferenceAbsolute:
			if a, ok := ref.Address(); ok {
				b.embedded[a] = struct{}{}
				b.addDefinition(a.DefinitionID())
				if filter != nil {
					filter[a.DefinitionID()] = b.byID[a.DefinitionID()]
				}
				if mapped != nil {
					mapped[a.DefinitionID()] = b.byID[a.DefinitionID()]
				}
			}
		case PropertyReferenceBoundContract:
			if id, ok := ref.ContractID(); ok {
				for _, d := range b.byID {
					c, has := d.ContractID()
					if has && c == id {
						b.walkDefinition(d)
						if filter != nil {
							filter[d.ID()] = d
						}
						if mapped != nil {
							mapped[d.ID()] = d
						}
					}
				}
			}
		}
	}
	if aggregate, ok := e.AggregateOperation(); ok {
		source := aggregate.Source()
		for _, r := range b.model.RelationTypeDefinitions() {
			if r.ID() == source.RelationTypeID() {
				b.addRelationType(r)
			}
		}
		for _, relation := range b.model.Relations() {
			if relation.TypeID() == source.RelationTypeID() {
				if applicability, ok := relation.Applicability(); ok {
					b.walkExpression(applicability, nil, nil)
				}
			}
		}
		if id, ok := source.AbsoluteEntityID(); ok { // this is known concrete embedded runtime identity, but no property address exists for it
			_ = id
		}
		f, mp := map[ID]PropertyDefinition{}, map[ID]PropertyDefinition{}
		if x, ok := aggregate.Filter(); ok {
			b.walkExpression(x, f, nil)
		}
		if x, ok := aggregate.Map(); ok {
			b.walkExpression(x, nil, mp)
		}
		b.traversals = append(b.traversals, TraversalRequirement{source: source, filter: sortedDefinitions(f), mapped: sortedDefinitions(mp)})
		return
	}
	if call, ok := e.FunctionCall(); ok {
		for _, r := range b.model.FunctionRequirements() {
			if r.Ref().String() == call.Ref().String() {
				b.functions[r.Ref().String()] = r
			}
		}
	}
	for _, child := range e.Children() {
		b.walkExpression(child, filter, mapped)
	}
}

func (b *planBuilder) addRelationType(relation RelationType) {
	b.relationTypes[relation.ID()] = relation
	endpoints := make([]EndpointConstraint, 0, 2)
	if endpoint, ok := relation.From(); ok {
		endpoints = append(endpoints, endpoint)
	}
	if endpoint, ok := relation.To(); ok {
		endpoints = append(endpoints, endpoint)
	}
	for _, endpoint := range endpoints {
		if endpoint.Kind() == EndpointEntityType {
			if id, ok := endpoint.ID(); ok {
				for _, entity := range b.model.EntityTypeDefinitions() {
					if entity.ID() == id {
						b.entityTypes[id] = entity
					}
				}
			}
			continue
		}
		// Trait and unconstrained endpoints intentionally remain a class-level
		// requirement: every compatible EntityType may provide a runtime endpoint.
		if endpoint.Kind() == EndpointTrait {
			if traitID, ok := endpoint.ID(); ok {
				for _, entity := range b.model.EntityTypeDefinitions() {
					for _, candidate := range entity.Traits() {
						if candidate == traitID {
							b.entityTypes[entity.ID()] = entity
							break
						}
					}
				}
			}
		} else if endpoint.Kind() == EndpointAnyEntity {
			for _, entity := range b.model.EntityTypeDefinitions() {
				b.entityTypes[entity.ID()] = entity
			}
		}
	}
}
func sortedDefinitions(in map[ID]PropertyDefinition) []PropertyDefinition {
	out := make([]PropertyDefinition, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID().String() < out[j].ID().String() })
	return out
}
func sortedAddresses(in map[PropertyAddress]struct{}) []PropertyAddress {
	out := make([]PropertyAddress, 0, len(in))
	for v := range in {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObjectKind() != out[j].ObjectKind() {
			return out[i].ObjectKind() < out[j].ObjectKind()
		}
		if out[i].ObjectID() != out[j].ObjectID() {
			return out[i].ObjectID().String() < out[j].ObjectID().String()
		}
		return out[i].DefinitionID().String() < out[j].DefinitionID().String()
	})
	return out
}
func sortedEntityTypes(in map[ID]EntityType) []EntityType {
	out := make([]EntityType, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID().String() < out[j].ID().String() })
	return out
}
func sortedRelationTypes(in map[ID]RelationType) []RelationType {
	out := make([]RelationType, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID().String() < out[j].ID().String() })
	return out
}
func sortedScopes(in map[ID]ScopeDefinition) []ScopeDefinition {
	out := make([]ScopeDefinition, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID().String() < out[j].ID().String() })
	return out
}
func sortedFunctionRequirements(in map[string]FunctionRequirement) []FunctionRequirement {
	out := make([]FunctionRequirement, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref().String() < out[j].Ref().String() })
	return out
}

func sortedTraversalRequirements(in []TraversalRequirement) []TraversalRequirement {
	out := append([]TraversalRequirement(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		left, right := out[i].source, out[j].source
		lk := left.RelationTypeID().String() + ":" + left.Origin().String() + ":" + left.Direction().String()
		rk := right.RelationTypeID().String() + ":" + right.Origin().String() + ":" + right.Direction().String()
		if lk != rk {
			return lk < rk
		}
		return traversalDefinitionKey(out[i]) < traversalDefinitionKey(out[j])
	})
	return out
}

func traversalDefinitionKey(requirement TraversalRequirement) string {
	key := ""
	for _, d := range requirement.filter {
		key += "f:" + d.ID().String()
	}
	for _, d := range requirement.mapped {
		key += "m:" + d.ID().String()
	}
	return key
}
