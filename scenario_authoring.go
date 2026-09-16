package bcg

import (
	"fmt"
	"sort"

	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	modelscenario "github.com/shmatovv/bcg-engine/internal/scenario"
)

// ScenarioOverrideSpec describes one immutable Scenario value difference. A
// Scenario override always targets a concrete Entity or Relation property and
// may replace either an Input or a Derived property for matching contexts.
type ScenarioOverrideSpec struct {
	Target          PropertyAddress
	Value           BindingValue
	ScopeSelectors  []ScopeSelector
	EffectivePeriod *EffectivePeriod
}

// ScenarioOverride is an immutable value-only Scenario difference.
type ScenarioOverride struct{ value domainscenario.Override }

// NewScenarioOverride constructs one immutable override. Target ownership,
// value compatibility, nullability, and temporal semantics are checked when
// NewScenario binds the override to a committed Model version.
func NewScenarioOverride(spec ScenarioOverrideSpec) (ScenarioOverride, error) {
	target, err := spec.Target.internal()
	if err != nil {
		return ScenarioOverride{}, fmt.Errorf("%w: Scenario override target: %v", ErrInvalidArgument, err)
	}
	if !spec.Value.IsValid() {
		return ScenarioOverride{}, fmt.Errorf("%w: Scenario override value is invalid", ErrInvalidArgument)
	}
	selectors, _, err := canonicalBindingSelectors(spec.ScopeSelectors)
	if err != nil {
		return ScenarioOverride{}, err
	}
	period, hasPeriod, err := bindingEffectivePeriod(spec.EffectivePeriod)
	if err != nil {
		return ScenarioOverride{}, err
	}
	return ScenarioOverride{value: domainscenario.NewOverride(target, spec.Value.value, selectors, period, hasPeriod)}, nil
}

func scenarioOverrideFromInternal(value domainscenario.Override) ScenarioOverride {
	return ScenarioOverride{value: value}
}

// Target returns the concrete property replaced by this override.
func (o ScenarioOverride) Target() PropertyAddress { return addressFromInternal(o.value.Target()) }

// Value returns the immutable override value.
func (o ScenarioOverride) Value() BindingValue { return BindingValue{value: o.value.Value()} }

// ScopeSelectors returns an owned canonical selector slice.
func (o ScenarioOverride) ScopeSelectors() []ScopeSelector {
	return publicSelectorsFromInternal(o.value.ScopeSelectors())
}

// EffectivePeriod returns the optional half-open effective selector.
func (o ScenarioOverride) EffectivePeriod() (EffectivePeriod, bool) {
	value, ok := o.value.EffectivePeriod()
	return EffectivePeriod{value: value}, ok
}

// IsValid reports structural Scenario override validity.
func (o ScenarioOverride) IsValid() bool { return o.value.IsValid() }

// ScenarioSpec describes an immutable overlay. Base-version ownership is
// supplied by NewScenario's committed Model parameter, or by
// EvaluationSnapshot.NewScenario when an override targets runtime Dataset
// data, instead of a mutable exposed version reference.
type ScenarioSpec struct {
	ID        ID
	Name      string
	Overrides []ScenarioOverride
}

// NewScenario constructs a portable model-only Scenario tied to base's exact
// committed ModelVersion. Dataset-owned targets require
// EvaluationSnapshot.NewScenario, which creates a runtime-only
// Dataset-bound Scenario. It validates every override with the existing
// authoritative Scenario validation machinery before returning it.
func NewScenario(base Model, spec ScenarioSpec) (Scenario, error) {
	if !base.IsValid() {
		return Scenario{}, fmt.Errorf("%w: committed base Model is required", ErrInvalidArgument)
	}
	id, err := spec.ID.internal()
	if err != nil {
		return Scenario{}, fmt.Errorf("%w: Scenario ID: %v", ErrInvalidArgument, err)
	}
	if spec.Name == "" {
		return Scenario{}, fmt.Errorf("%w: Scenario name must not be empty", ErrInvalidArgument)
	}
	overrides := make([]domainscenario.Override, len(spec.Overrides))
	for index, override := range spec.Overrides {
		if !override.IsValid() {
			return Scenario{}, fmt.Errorf("%w: Scenario override %d is invalid", ErrInvalidArgument, index)
		}
		overrides[index] = override.value
	}
	candidate := domainscenario.New(id, spec.Name, base.version.ID(), overrides)
	if report := modelscenario.Validate(base.version, candidate); !report.OK() {
		return Scenario{}, fmt.Errorf("%w: Scenario is incompatible with base ModelVersion: %s", ErrInvalidArgument, report.Error())
	}
	return scenarioFromInternal(candidate), nil
}

// Overrides returns an owned deterministic copy of Scenario differences.
func (s Scenario) Overrides() []ScenarioOverride {
	if !s.IsValid() {
		return nil
	}
	items := s.value.Overrides()
	result := make([]ScenarioOverride, len(items))
	for index, item := range items {
		result[index] = scenarioOverrideFromInternal(item)
	}
	sort.SliceStable(result, func(i, j int) bool { return scenarioOverrideSortKey(result[i]) < scenarioOverrideSortKey(result[j]) })
	return result
}

// WithOverrides returns a replacement immutable Scenario with the same ID and
// name, tied to base. It is the editing path for Scenario data; no Scenario
// mutates in place.
func (s Scenario) WithOverrides(base Model, overrides ...ScenarioOverride) (Scenario, error) {
	if !s.IsValid() {
		return Scenario{}, fmt.Errorf("%w: Scenario is invalid", ErrInvalidArgument)
	}
	if _, datasetBound := s.BaseDatasetID(); datasetBound {
		return Scenario{}, fmt.Errorf("%w: Dataset-bound Scenario must be replaced through EvaluationSnapshot.NewScenario", ErrInvalidArgument)
	}
	if !base.IsValid() || !s.BaseVersionID().Equal(base.VersionID()) {
		return Scenario{}, fmt.Errorf("%w: Scenario belongs to a different ModelVersion", ErrInvalidArgument)
	}
	return NewScenario(base, ScenarioSpec{ID: s.ID(), Name: s.Name(), Overrides: overrides})
}

func scenarioOverrideSortKey(value ScenarioOverride) string {
	target := value.Target()
	result := string(target.ObjectKind()) + ":" + target.ObjectID().String() + ":" + target.DefinitionID().String()
	for _, selector := range value.ScopeSelectors() {
		result += ":" + selector.DefinitionID().String()
	}
	if period, ok := value.EffectivePeriod(); ok {
		from, hasFrom := period.From()
		to, hasTo := period.To()
		result += ":" + boolDateTime(hasFrom, from) + "/" + boolDateTime(hasTo, to)
	}
	return result
}
