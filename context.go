package bcg

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// ScopeSelector selects one scope dimension for a calculation.
type ScopeSelector struct {
	definitionID ID
	value        Value
}

// NewScopeSelector creates a type-safe scope selection. Its compatibility with
// a Model's ScopeDefinition is checked by the normal resolver.
func NewScopeSelector(definitionID ID, value Value) (ScopeSelector, error) {
	if _, err := definitionID.internal(); err != nil {
		return ScopeSelector{}, err
	}
	if _, err := value.internal(); err != nil {
		return ScopeSelector{}, err
	}
	return ScopeSelector{definitionID: definitionID, value: value}, nil
}

// DefinitionID returns the selected ScopeDefinition UUID.
func (s ScopeSelector) DefinitionID() ID { return s.definitionID }

// Value returns the selected scope value.
func (s ScopeSelector) Value() Value { return s.value }

// IsValid reports whether s has a valid public shape.
func (s ScopeSelector) IsValid() bool { return !s.definitionID.IsZero() && s.value.IsValid() }

func (s ScopeSelector) internal() (scope.Selector, error) {
	if !s.IsValid() {
		return scope.Selector{}, fmt.Errorf("%w: scope selector is invalid", ErrInvalidArgument)
	}
	id, _ := s.definitionID.internal()
	value, _ := s.value.internal()
	return scope.NewSelector(id, value), nil
}

// Override is one calculation-local scalar replacement. It has higher
// precedence than Scenario and base bindings, but never mutates a Model.
type Override struct {
	target PropertyAddress
	value  semantic.BindingValue
}

// NewOverride creates a defined calculation override.
func NewOverride(target PropertyAddress, value Value) (Override, error) {
	if _, err := target.internal(); err != nil {
		return Override{}, err
	}
	typed, err := value.internal()
	if err != nil {
		return Override{}, err
	}
	return Override{target: target, value: semantic.DefinedBindingValue(typed)}, nil
}

// NullOverride creates an explicit Null calculation override.
func NullOverride(target PropertyAddress) (Override, error) {
	if _, err := target.internal(); err != nil {
		return Override{}, err
	}
	return Override{target: target, value: semantic.NullBindingValue()}, nil
}

// Target returns the overridden property.
func (o Override) Target() PropertyAddress { return o.target }

// Value returns the defined override value. It is false for an explicit Null override.
func (o Override) Value() (Value, bool) {
	value, ok := o.value.Value()
	if !ok {
		return Value{}, false
	}
	return valueFromInternal(value), true
}

// IsNull reports whether o supplies explicit Null.
func (o Override) IsNull() bool { return o.value.Kind() == semantic.BindingValueNull }

// IsValid reports whether o has a valid public shape.
func (o Override) IsValid() bool { return o.target.IsValid() && o.value.IsValid() }

func (o Override) internal() (runtime.Override, error) {
	addressValue, err := o.target.internal()
	if err != nil {
		return runtime.Override{}, err
	}
	if !o.value.IsValid() || o.value.Kind() == semantic.BindingValueSeries {
		return runtime.Override{}, fmt.Errorf("%w: calculation override must be defined or Null scalar value", ErrInvalidArgument)
	}
	return runtime.NewOverride(addressValue, o.value), nil
}

// CalculationContext is immutable calculation input: time, scope, optional
// Scenario, and calculation overrides. It is distinct from Go context.Context.
type CalculationContext struct {
	value runtime.CalculationContext
}

// NewCalculationContextAt creates a scalar calculation context at one instant.
func NewCalculationContextAt(at DateTime) (CalculationContext, error) {
	if !at.IsValid() {
		return CalculationContext{}, fmt.Errorf("%w: calculation At must be valid", ErrInvalidArgument)
	}
	return calculationContext(domaintime.At(at.value), nil, Scenario{}, nil)
}

// NewCalculationContextPeriod creates a half-open [from,to) calculation context.
// Scalar Evaluate retains the core's At-only contract; Period remains available
// to APIs that support it now or in later facade revisions.
func NewCalculationContextPeriod(from, to DateTime) (CalculationContext, error) {
	if !from.IsValid() || !to.IsValid() || from.value.Cmp(to.value) >= 0 {
		return CalculationContext{}, fmt.Errorf("%w: calculation period must satisfy valid from < to", ErrInvalidArgument)
	}
	return calculationContext(domaintime.Period(from.value, to.value), nil, Scenario{}, nil)
}

func calculationContext(timeContext domaintime.Context, selectors []ScopeSelector, scenario Scenario, overrides []Override) (CalculationContext, error) {
	internalSelectors, err := internalSelectors(selectors)
	if err != nil {
		return CalculationContext{}, err
	}
	internalOverrides, err := internalOverrides(overrides)
	if err != nil {
		return CalculationContext{}, err
	}
	context := runtime.NewCalculationContext(timeContext, scope.NewValues(internalSelectors), internalOverrides)
	if scenario.IsValid() {
		context = context.WithScenario(scenario.value)
	}
	return CalculationContext{value: context}, nil
}

// IsValid reports whether c was built by a public constructor.
func (c CalculationContext) IsValid() bool { return c.value.IsValid() }

// At returns the calculation instant for an At context.
func (c CalculationContext) At() (DateTime, bool) {
	if !c.IsValid() {
		return DateTime{}, false
	}
	at, ok := c.value.TimeContext().At()
	if !ok {
		return DateTime{}, false
	}
	return DateTime{value: at}, true
}

// Period returns the half-open [from,to) bounds for a Period context.
func (c CalculationContext) Period() (DateTime, DateTime, bool) {
	if !c.IsValid() {
		return DateTime{}, DateTime{}, false
	}
	from, to, ok := c.value.TimeContext().Period()
	if !ok {
		return DateTime{}, DateTime{}, false
	}
	return DateTime{value: from}, DateTime{value: to}, true
}

// WithScopes returns c with its complete scope selection replaced.
func (c CalculationContext) WithScopes(selectors ...ScopeSelector) (CalculationContext, error) {
	if !c.IsValid() {
		return CalculationContext{}, fmt.Errorf("%w: calculation context is invalid", ErrInvalidArgument)
	}
	return calculationContext(c.value.TimeContext(), selectors, c.Scenario(), c.Overrides())
}

// WithScenario returns c selecting scenario. Model-version compatibility is
// validated when an evaluation is requested.
func (c CalculationContext) WithScenario(scenario Scenario) (CalculationContext, error) {
	if !c.IsValid() || !scenario.IsValid() {
		return CalculationContext{}, fmt.Errorf("%w: valid context and Scenario are required", ErrInvalidArgument)
	}
	return calculationContext(c.value.TimeContext(), c.ScopeSelectors(), scenario, c.Overrides())
}

// WithoutScenario returns c without an overlay Scenario.
func (c CalculationContext) WithoutScenario() (CalculationContext, error) {
	if !c.IsValid() {
		return CalculationContext{}, fmt.Errorf("%w: calculation context is invalid", ErrInvalidArgument)
	}
	return calculationContext(c.value.TimeContext(), c.ScopeSelectors(), Scenario{}, c.Overrides())
}

// WithCalculationOverrides returns c with its complete calculation override
// set replaced. Duplicate targets are rejected before reaching internal code.
func (c CalculationContext) WithCalculationOverrides(overrides ...Override) (CalculationContext, error) {
	if !c.IsValid() {
		return CalculationContext{}, fmt.Errorf("%w: calculation context is invalid", ErrInvalidArgument)
	}
	return calculationContext(c.value.TimeContext(), c.ScopeSelectors(), c.Scenario(), overrides)
}

// Scenario returns the optional selected Scenario.
func (c CalculationContext) Scenario() Scenario {
	if scenario, ok := c.value.Scenario(); ok {
		return scenarioFromInternal(scenario)
	}
	return Scenario{}
}

// ScopeSelectors returns an owned deterministic copy.
func (c CalculationContext) ScopeSelectors() []ScopeSelector {
	if !c.IsValid() {
		return nil
	}
	selectors := c.value.ScopeValues().Selectors()
	result := make([]ScopeSelector, len(selectors))
	for index, selector := range selectors {
		result[index] = ScopeSelector{definitionID: idFromInternal(selector.DefinitionID()), value: valueFromInternal(selector.Value())}
	}
	return result
}

// Overrides returns an owned deterministic copy.
func (c CalculationContext) Overrides() []Override {
	if !c.IsValid() {
		return nil
	}
	overrides := c.value.Overrides()
	result := make([]Override, len(overrides))
	for index, override := range overrides {
		result[index] = Override{target: addressFromInternal(override.Target()), value: override.Value()}
	}
	return result
}

func (c CalculationContext) internal() (runtime.CalculationContext, error) {
	if !c.IsValid() {
		return runtime.CalculationContext{}, fmt.Errorf("%w: calculation context is invalid", ErrInvalidArgument)
	}
	return c.value, nil
}

func internalSelectors(selectors []ScopeSelector) ([]scope.Selector, error) {
	result := make([]scope.Selector, len(selectors))
	seen := make(map[ID]struct{}, len(selectors))
	for index, selector := range selectors {
		if _, exists := seen[selector.definitionID]; exists {
			return nil, fmt.Errorf("%w: duplicate scope definition %s", ErrInvalidArgument, selector.definitionID)
		}
		seen[selector.definitionID] = struct{}{}
		converted, err := selector.internal()
		if err != nil {
			return nil, err
		}
		result[index] = converted
	}
	return result, nil
}

func internalOverrides(overrides []Override) ([]runtime.Override, error) {
	result := make([]runtime.Override, len(overrides))
	seen := make(map[string]struct{}, len(overrides))
	for index, override := range overrides {
		key := override.target.String()
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%w: duplicate calculation override target", ErrInvalidArgument)
		}
		seen[key] = struct{}{}
		converted, err := override.internal()
		if err != nil {
			return nil, err
		}
		result[index] = converted
	}
	return result, nil
}
