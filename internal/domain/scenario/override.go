package scenario

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
)

// Override is one immutable value override stored by a Scenario.
//
// Scenario v1 changes effective values only. It never changes schema,
// structure or Expression AST.
//
// target is a concrete PropertyAddress. Both Input and Derived properties may
// be targeted; compatibility with the base ModelVersion is checked by the
// scenario/resolution validation layer.
//
// Scope selectors and EffectivePeriod use the same applicability semantics as
// base ValueBinding:
//
//   - every selector must match;
//   - an absent selector set means default applicability;
//   - effective intervals are half-open [from, to);
//   - equal-specificity ambiguity is never resolved by insertion order.
type Override struct {
	target address.PropertyAddress
	value  semantic.BindingValue

	scopeSelectors []scope.Selector

	effectivePeriod    domaintime.EffectivePeriod
	hasEffectivePeriod bool
}

// NewOverride constructs an immutable Scenario override.
//
// NewOverride panics when:
//
//   - target is invalid;
//   - value is invalid;
//   - a scope selector is invalid;
//   - one ScopeDefinition occurs more than once;
//   - a declared EffectivePeriod is structurally invalid.
//
// Model-dependent checks intentionally do not belong here. In particular,
// this constructor does not know whether target exists in the base
// ModelVersion or whether value satisfies its PropertyDefinition.
func NewOverride(
	target address.PropertyAddress,
	bindingValue semantic.BindingValue,
	scopeSelectors []scope.Selector,
	effectivePeriod domaintime.EffectivePeriod,
	hasEffectivePeriod bool,
) Override {
	if !target.IsValid() {
		panic("scenario: override target is invalid")
	}

	if !bindingValue.IsValid() {
		panic("scenario: override value is invalid")
	}

	selectors := cloneAndValidateSelectors(scopeSelectors)

	if hasEffectivePeriod && !effectivePeriod.IsValid() {
		panic("scenario: override effective period is invalid")
	}

	if !hasEffectivePeriod {
		effectivePeriod = domaintime.EffectivePeriod{}
	}

	return Override{
		target:             target,
		value:              bindingValue,
		scopeSelectors:     selectors,
		effectivePeriod:    effectivePeriod,
		hasEffectivePeriod: hasEffectivePeriod,
	}
}

// Target returns the concrete property replaced by this override when the
// override is applicable in the current calculation context.
func (o Override) Target() address.PropertyAddress {
	return o.target
}

// Value returns the effective value supplied by the override.
func (o Override) Value() semantic.BindingValue {
	return o.value
}

// ScopeSelectors returns a defensive copy of the override scope selectors.
func (o Override) ScopeSelectors() []scope.Selector {
	if len(o.scopeSelectors) == 0 {
		return nil
	}

	result := make(
		[]scope.Selector,
		len(o.scopeSelectors),
	)

	copy(result, o.scopeSelectors)

	return result
}

// EffectivePeriod returns the optional half-open effective interval.
func (o Override) EffectivePeriod() (
	domaintime.EffectivePeriod,
	bool,
) {
	return o.effectivePeriod,
		o.hasEffectivePeriod
}

// IsValid reports structural validity only.
//
// Compatibility with a particular ModelVersion is intentionally a separate
// concern because Scenario is an overlay over an external immutable snapshot.
func (o Override) IsValid() bool {
	if !o.target.IsValid() ||
		!o.value.IsValid() {
		return false
	}

	seen := make(
		map[identity.UUID]struct{},
		len(o.scopeSelectors),
	)

	for _, selector := range o.scopeSelectors {
		if !selector.IsValid() {
			return false
		}

		definitionID := selector.DefinitionID()

		if _, exists := seen[definitionID]; exists {
			return false
		}

		seen[definitionID] = struct{}{}
	}

	if o.hasEffectivePeriod {
		return o.effectivePeriod.IsValid()
	}

	return o.effectivePeriod == domaintime.EffectivePeriod{}
}

func cloneAndValidateSelectors(
	source []scope.Selector,
) []scope.Selector {
	if len(source) == 0 {
		return nil
	}

	result := make(
		[]scope.Selector,
		len(source),
	)

	seen := make(
		map[identity.UUID]struct{},
		len(source),
	)

	for index, selector := range source {
		if !selector.IsValid() {
			panic("scenario: override scope selector is invalid")
		}

		definitionID := selector.DefinitionID()

		if _, exists := seen[definitionID]; exists {
			panic("scenario: override contains duplicate scope definition")
		}

		seen[definitionID] = struct{}{}
		result[index] = selector
	}

	return result
}
