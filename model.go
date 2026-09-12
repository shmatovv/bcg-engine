package bcg

import (
	"fmt"
	"sort"

	constraintcheck "github.com/shmatovv/bcg-engine/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/impact"
	"github.com/shmatovv/bcg-engine/internal/model"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	ruleeval "github.com/shmatovv/bcg-engine/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// Model identifies one logical calculation model and, when committed, one
// immutable calculation-model version.
//
// NewModel creates an uncommitted logical model identity. A Model returned by
// Artifact, Unmarshal, or Draft.Commit is committed and can be evaluated.
type Model struct {
	modelID identity.UUID
	name    string
	version model.ModelVersion
}

func newModel(name string, version model.ModelVersion) Model {
	return Model{modelID: version.ModelID(), name: name, version: version}
}

// IsValid reports whether m wraps a committed immutable model version.
func (m Model) IsValid() bool {
	return !m.modelID.IsZero() &&
		m.name != "" &&
		m.version.IsCommitted() &&
		m.version.ModelID() == m.modelID
}

// Name returns the long-lived model presentation name.
func (m Model) Name() string { return m.name }

// ModelID returns the long-lived model UUID.
func (m Model) ModelID() ID { return idFromInternal(m.modelID) }

// VersionID returns this immutable model-version UUID.
func (m Model) VersionID() ID { return idFromInternal(m.version.ID()) }

// ParentVersionID returns the optional parent version identity.
func (m Model) ParentVersionID() (ID, bool) {
	id, ok := m.version.ParentVersionID()
	if !ok {
		return ID{}, false
	}
	return idFromInternal(id), true
}

// VersionLabel returns the user-facing version label.
func (m Model) VersionLabel() string { return m.version.VersionLabel() }

// Evaluate evaluates one scalar property with no persistent runtime cache.
func (m Model) Evaluate(target PropertyAddress, context CalculationContext) (Result, error) {
	version, addressValue, contextValue, err := m.request(target, context)
	if err != nil {
		return Result{}, err
	}
	result, err := evaluator.Evaluate(version, addressValue, contextValue)
	if err != nil {
		return Result{}, publicError(err)
	}
	return resultFromInternal(result), nil
}

// EvaluateMany evaluates scalar targets in caller order. Duplicate targets are
// preserved in the result slice.
func (m Model) EvaluateMany(targets []PropertyAddress, context CalculationContext) ([]Result, error) {
	if !m.IsValid() {
		return nil, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	contextValue, err := context.internal()
	if err != nil {
		return nil, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return []Result{}, nil
	}
	session, err := evaluator.NewSession(m.version)
	if err != nil {
		return nil, publicError(err)
	}
	internalTargets := make([]address.PropertyAddress, len(targets))
	for index, target := range targets {
		converted, err := target.internal()
		if err != nil {
			return nil, err
		}
		internalTargets[index] = converted
	}
	results, err := session.EvaluateMany(internalTargets, contextValue)
	if err != nil {
		return nil, publicError(err)
	}
	return resultsFromInternal(results), nil
}

// EvaluateSeries evaluates a Series property through the established Series evaluator.
func (m Model) EvaluateSeries(target PropertyAddress, context CalculationContext) (SeriesResult, error) {
	version, addressValue, contextValue, err := m.request(target, context)
	if err != nil {
		return SeriesResult{}, err
	}
	result, err := evaluator.EvaluateSeries(version, addressValue, contextValue)
	if err != nil {
		return SeriesResult{}, publicError(err)
	}
	return seriesResultFromInternal(result), nil
}

// Explain evaluates one scalar property and returns actual structured provenance.
func (m Model) Explain(target PropertyAddress, context CalculationContext) (Explanation, error) {
	version, addressValue, contextValue, err := m.request(target, context)
	if err != nil {
		return Explanation{}, err
	}
	explained, err := evaluator.Explain(version, addressValue, contextValue)
	if err != nil {
		return Explanation{}, publicError(err)
	}
	return explanationFromInternal(explained), nil
}

// ExplainSeries returns per-sample actual Series provenance.
func (m Model) ExplainSeries(target PropertyAddress, context CalculationContext) (SeriesExplanation, error) {
	version, addressValue, contextValue, err := m.request(target, context)
	if err != nil {
		return SeriesExplanation{}, err
	}
	explained, err := evaluator.ExplainSeries(version, addressValue, contextValue)
	if err != nil {
		return SeriesExplanation{}, publicError(err)
	}
	return seriesExplanationFromInternal(explained), nil
}

// AnalyzeImpact analyzes downstream consequences using the selected semantics.
func (m Model) AnalyzeImpact(mode ImpactMode, source PropertyAddress, context CalculationContext) (ImpactResult, error) {
	if !m.IsValid() {
		return ImpactResult{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	addressValue, err := source.internal()
	if err != nil {
		return ImpactResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return ImpactResult{}, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return ImpactResult{}, err
	}
	var result impact.Result
	switch mode {
	case StructuralImpact:
		result, err = impact.AnalyzeStructural(m.version, addressValue)
	case EffectiveImpact:
		result, err = impact.AnalyzeEffective(m.version, addressValue, contextValue)
	case ObservedImpact:
		result, err = impact.AnalyzeObserved(m.version, addressValue, contextValue)
	default:
		return ImpactResult{}, fmt.Errorf("%w: impact mode is invalid", ErrInvalidArgument)
	}
	if err != nil {
		return ImpactResult{}, publicError(err)
	}
	return impactResultFromInternal(result), nil
}

// CheckConstraint checks one declared Constraint.
func (m Model) CheckConstraint(id ID, context CalculationContext) (ConstraintResult, error) {
	if !m.IsValid() {
		return ConstraintResult{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	constraintID, err := id.internal()
	if err != nil {
		return ConstraintResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return ConstraintResult{}, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return ConstraintResult{}, err
	}
	result, err := constraintcheck.One(m.version, constraintID, contextValue)
	if err != nil {
		return ConstraintResult{}, publicError(err)
	}
	return constraintResultFromInternal(result), nil
}

// CheckConstraints checks every declared Constraint in deterministic ID order.
func (m Model) CheckConstraints(context CalculationContext) (ConstraintReport, error) {
	if !m.IsValid() {
		return ConstraintReport{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	contextValue, err := context.internal()
	if err != nil {
		return ConstraintReport{}, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return ConstraintReport{}, err
	}
	report, err := constraintcheck.All(m.version, contextValue)
	if err != nil {
		return ConstraintReport{}, publicError(err)
	}
	return constraintReportFromInternal(report), nil
}

// CheckFeasibility assesses all Hard Constraints.
func (m Model) CheckFeasibility(context CalculationContext) (FeasibilityResult, error) {
	if !m.IsValid() {
		return FeasibilityResult{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	contextValue, err := context.internal()
	if err != nil {
		return FeasibilityResult{}, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return FeasibilityResult{}, err
	}
	result, err := feasibility.Evaluate(m.version, contextValue)
	if err != nil {
		return FeasibilityResult{}, publicError(err)
	}
	return feasibilityResultFromInternal(result), nil
}

// EvaluateRule evaluates one declared Rule.
func (m Model) EvaluateRule(id ID, context CalculationContext) (RuleResult, error) {
	if !m.IsValid() {
		return RuleResult{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	ruleID, err := id.internal()
	if err != nil {
		return RuleResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return RuleResult{}, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return RuleResult{}, err
	}
	result, err := ruleeval.One(m.version, ruleID, contextValue)
	if err != nil {
		return RuleResult{}, publicError(err)
	}
	return ruleResultFromInternal(result), nil
}

// EvaluateRules evaluates every declared Rule in deterministic ID order.
func (m Model) EvaluateRules(context CalculationContext) (RuleReport, error) {
	if !m.IsValid() {
		return RuleReport{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	contextValue, err := context.internal()
	if err != nil {
		return RuleReport{}, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return RuleReport{}, err
	}
	report, err := ruleeval.All(m.version, contextValue)
	if err != nil {
		return RuleReport{}, publicError(err)
	}
	return ruleReportFromInternal(report), nil
}

// NewSession creates a reusable incremental scalar-evaluation session in context.
func (m Model) NewSession(context CalculationContext) (*Session, error) {
	if !m.IsValid() {
		return nil, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	contextValue, err := context.internal()
	if err != nil {
		return nil, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return nil, err
	}
	session, err := evaluator.NewSession(m.version)
	if err != nil {
		return nil, publicError(err)
	}
	return newSession(m, session, contextValue), nil
}

func (m Model) request(target PropertyAddress, context CalculationContext) (model.ModelVersion, address.PropertyAddress, runtime.CalculationContext, error) {
	if !m.IsValid() {
		return model.ModelVersion{}, address.PropertyAddress{}, runtime.CalculationContext{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	addressValue, err := target.internal()
	if err != nil {
		return model.ModelVersion{}, address.PropertyAddress{}, runtime.CalculationContext{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return model.ModelVersion{}, address.PropertyAddress{}, runtime.CalculationContext{}, err
	}
	if err := m.validateContext(contextValue); err != nil {
		return model.ModelVersion{}, address.PropertyAddress{}, runtime.CalculationContext{}, err
	}
	return m.version, addressValue, contextValue, nil
}

func (m Model) validateContext(context runtime.CalculationContext) error {
	if err := resolvervalue.ValidateContext(m.version, context); err != nil {
		return publicError(err)
	}
	return nil
}

// EntityInfo is immutable read-only semantic graph information.
type EntityInfo struct {
	id, typeID ID
	name       string
	hasName    bool
}

// ID returns the Entity UUID.
func (e EntityInfo) ID() ID { return e.id }

// TypeID returns the EntityType UUID.
func (e EntityInfo) TypeID() ID { return e.typeID }

// Name returns the optional display name.
func (e EntityInfo) Name() (string, bool) { return e.name, e.hasName }

// RelationInfo is immutable read-only semantic graph information.
type RelationInfo struct{ id, typeID, from, to ID }

// ID returns the Relation UUID.
func (r RelationInfo) ID() ID { return r.id }

// TypeID returns the RelationType UUID.
func (r RelationInfo) TypeID() ID { return r.typeID }

// From returns the source Entity UUID.
func (r RelationInfo) From() ID { return r.from }

// To returns the target Entity UUID.
func (r RelationInfo) To() ID { return r.to }

// NamedInfo identifies a named schema object.
type NamedInfo struct {
	id   ID
	name string
}

// ID returns the schema object's UUID.
func (n NamedInfo) ID() ID { return n.id }

// Name returns the schema object's display name.
func (n NamedInfo) Name() string { return n.name }

// PropertyInfo is a stable read-only view of property schema, without AST internals.
type PropertyInfo struct {
	id, ownerID, contractID   ID
	name                      string
	hasContract               bool
	ownerKind                 ObjectKind
	valueKind                 ValueKind
	temporalMode, computation string
	unit                      Unit
	hasUnit                   bool
}

// ID returns the PropertyDefinition UUID.
func (p PropertyInfo) ID() ID { return p.id }

// Name returns the schema property's display name.
func (p PropertyInfo) Name() string { return p.name }

// OwnerID returns the EntityType or RelationType UUID that owns p.
func (p PropertyInfo) OwnerID() ID { return p.ownerID }

// OwnerKind returns whether p belongs to Entity or Relation schema.
func (p PropertyInfo) OwnerKind() ObjectKind { return p.ownerKind }

// ContractID returns p's implemented PropertyContract when one exists.
func (p PropertyInfo) ContractID() (ID, bool) { return p.contractID, p.hasContract }

// ValueKind returns p's scalar value kind.
func (p PropertyInfo) ValueKind() ValueKind { return p.valueKind }

// TemporalMode returns the stable model temporal-mode label.
func (p PropertyInfo) TemporalMode() string { return p.temporalMode }

// Computation returns the stable input or derived computation label.
func (p PropertyInfo) Computation() string { return p.computation }

// Unit returns p's optional declared unit.
func (p PropertyInfo) Unit() (Unit, bool) { return p.unit, p.hasUnit }

// Entities returns deterministic Entity descriptors.
func (m Model) Entities() []EntityInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.Entities()
	result := make([]EntityInfo, len(items))
	for index, item := range items {
		name, hasName := item.Name()
		result[index] = EntityInfo{id: idFromInternal(item.ID()), typeID: idFromInternal(item.TypeID()), name: name, hasName: hasName}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id.String() < result[j].id.String() })
	return result
}

// Relations returns deterministic Relation descriptors.
func (m Model) Relations() []RelationInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.Relations()
	result := make([]RelationInfo, len(items))
	for index, item := range items {
		result[index] = RelationInfo{id: idFromInternal(item.ID()), typeID: idFromInternal(item.TypeID()), from: idFromInternal(item.From()), to: idFromInternal(item.To())}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id.String() < result[j].id.String() })
	return result
}

// EntityTypes returns deterministic named schema descriptors.
func (m Model) EntityTypes() []NamedInfo { return namedEntityTypes(m) }

// RelationTypes returns deterministic named schema descriptors.
func (m Model) RelationTypes() []NamedInfo { return namedRelationTypes(m) }

// Traits returns deterministic named schema descriptors.
func (m Model) Traits() []NamedInfo { return namedTraits(m) }

// PropertyContracts returns deterministic named schema descriptors.
func (m Model) PropertyContracts() []NamedInfo { return namedContracts(m) }

// ScopeDefinitionInfo is one scope dimension and the type of its selected Value.
type ScopeDefinitionInfo struct {
	id        ID
	name      string
	valueKind ValueKind
}

// ID returns the ScopeDefinition UUID.
func (s ScopeDefinitionInfo) ID() ID { return s.id }

// Name returns the scope dimension's display name.
func (s ScopeDefinitionInfo) Name() string { return s.name }

// ValueKind returns the type required by this scope dimension.
func (s ScopeDefinitionInfo) ValueKind() ValueKind { return s.valueKind }

// ScopeDefinitions returns deterministic scope-dimension descriptors.
func (m Model) ScopeDefinitions() []ScopeDefinitionInfo { return scopeInfos(m) }

// PropertyDefinitions returns deterministic read-only property descriptors.
func (m Model) PropertyDefinitions() []PropertyInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.PropertyDefinitions()
	result := make([]PropertyInfo, len(items))
	for index, item := range items {
		result[index] = propertyInfo(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id.String() < result[j].id.String() })
	return result
}

// Constraints returns deterministic read-only Constraint descriptors.
func (m Model) Constraints() []ConstraintInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.Constraints()
	result := make([]ConstraintInfo, len(items))
	for index, item := range items {
		result[index] = constraintInfoFromInternal(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id.String() < result[j].id.String() })
	return result
}

// Rules returns deterministic read-only Rule descriptors.
func (m Model) Rules() []RuleInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.Rules()
	result := make([]RuleInfo, len(items))
	for index, item := range items {
		result[index] = ruleInfoFromInternal(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id.String() < result[j].id.String() })
	return result
}

// Entity looks up one Entity descriptor by UUID.
func (m Model) Entity(id ID) (EntityInfo, bool) {
	if !m.IsValid() || id.IsZero() {
		return EntityInfo{}, false
	}
	item, ok := m.version.Entity(id.value)
	if !ok {
		return EntityInfo{}, false
	}
	name, hasName := item.Name()
	return EntityInfo{id: idFromInternal(item.ID()), typeID: idFromInternal(item.TypeID()), name: name, hasName: hasName}, true
}

// Relation looks up one Relation descriptor by UUID.
func (m Model) Relation(id ID) (RelationInfo, bool) {
	if !m.IsValid() || id.IsZero() {
		return RelationInfo{}, false
	}
	item, ok := m.version.Relation(id.value)
	if !ok {
		return RelationInfo{}, false
	}
	return RelationInfo{id: idFromInternal(item.ID()), typeID: idFromInternal(item.TypeID()), from: idFromInternal(item.From()), to: idFromInternal(item.To())}, true
}

// PropertyDefinition looks up one property-schema descriptor by UUID.
func (m Model) PropertyDefinition(id ID) (PropertyInfo, bool) {
	if !m.IsValid() || id.IsZero() {
		return PropertyInfo{}, false
	}
	item, ok := m.version.PropertyDefinition(id.value)
	if !ok {
		return PropertyInfo{}, false
	}
	return propertyInfo(item), true
}

func namedEntityTypes(m Model) []NamedInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.EntityTypes()
	result := make([]NamedInfo, len(items))
	for i, item := range items {
		result[i] = NamedInfo{id: idFromInternal(item.ID()), name: item.Name()}
	}
	return sortedNamed(result)
}
func namedRelationTypes(m Model) []NamedInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.RelationTypes()
	result := make([]NamedInfo, len(items))
	for i, item := range items {
		result[i] = NamedInfo{id: idFromInternal(item.ID()), name: item.Name()}
	}
	return sortedNamed(result)
}
func namedTraits(m Model) []NamedInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.Traits()
	result := make([]NamedInfo, len(items))
	for i, item := range items {
		result[i] = NamedInfo{id: idFromInternal(item.ID()), name: item.Name()}
	}
	return sortedNamed(result)
}
func namedContracts(m Model) []NamedInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.PropertyContracts()
	result := make([]NamedInfo, len(items))
	for i, item := range items {
		result[i] = NamedInfo{id: idFromInternal(item.ID()), name: item.Name()}
	}
	return sortedNamed(result)
}
func scopeInfos(m Model) []ScopeDefinitionInfo {
	if !m.IsValid() {
		return nil
	}
	items := m.version.ScopeDefinitions()
	result := make([]ScopeDefinitionInfo, len(items))
	for i, item := range items {
		result[i] = ScopeDefinitionInfo{id: idFromInternal(item.ID()), name: item.Name(), valueKind: valueKindFromInternal(item.ValueType())}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id.String() < result[j].id.String() })
	return result
}
func sortedNamed(items []NamedInfo) []NamedInfo {
	sort.Slice(items, func(i, j int) bool { return items[i].id.String() < items[j].id.String() })
	return items
}

func propertyInfo(item schema.PropertyDefinition) PropertyInfo {
	ownerKind := ObjectEntity
	if item.Owner().Kind() == schema.PropertyOwnerRelationType {
		ownerKind = ObjectRelation
	}
	result := PropertyInfo{id: idFromInternal(item.ID()), ownerID: idFromInternal(item.Owner().ID()), ownerKind: ownerKind, name: item.Name(), valueKind: valueKindFromInternal(item.ValueType()), temporalMode: item.TemporalMode().String(), computation: item.Computation().Kind().String()}
	if contractID, ok := item.ContractID(); ok {
		result.contractID, result.hasContract = idFromInternal(contractID), true
	}
	if propertyUnit, ok := item.Unit(); ok {
		result.unit, result.hasUnit = Unit{value: propertyUnit}, true
	}
	return result
}
