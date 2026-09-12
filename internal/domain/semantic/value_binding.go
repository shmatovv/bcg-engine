package semantic

import (
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/types/series"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// BindingValueKind distinguishes a concrete TypedValue from explicit Null.
//
// Missing ValueBinding is neither of these states.
type BindingValueKind uint8

const (
	BindingValueInvalid BindingValueKind = iota
	BindingValueDefined
	BindingValueNull
	BindingValueSeries
)

type BindingValue struct {
	kind   BindingValueKind
	value  value.Typed
	series series.Series
}

// SeriesBindingValue constructs one immutable source Series binding value.
// A Series is a temporal structure around ordinary element values; it is not
// a ValueType and cannot contain runtime Undefined samples.
func SeriesBindingValue(v series.Series) BindingValue {
	if !v.IsValid() {
		panic("semantic: series binding value must be valid")
	}

	return BindingValue{
		kind:   BindingValueSeries,
		series: v,
	}
}

func DefinedBindingValue(v value.Typed) BindingValue {
	if !v.IsValid() {
		panic("semantic: defined binding value must be valid")
	}

	return BindingValue{
		kind:  BindingValueDefined,
		value: v,
	}
}

func NullBindingValue() BindingValue {
	return BindingValue{
		kind: BindingValueNull,
	}
}

func (v BindingValue) Kind() BindingValueKind {
	return v.kind
}

func (v BindingValue) Value() (value.Typed, bool) {
	if v.kind != BindingValueDefined {
		return value.Typed{}, false
	}

	return v.value, true
}

// Series returns the immutable source Series carried by a Series binding.
func (v BindingValue) Series() (series.Series, bool) {
	if v.kind != BindingValueSeries {
		return series.Series{}, false
	}

	return v.series, true
}

func (v BindingValue) IsValid() bool {
	switch v.kind {
	case BindingValueDefined:
		return v.value.IsValid()

	case BindingValueNull:
		return true

	case BindingValueSeries:
		return v.series.IsValid() && !v.value.IsValid()

	default:
		return false
	}
}

// Equal reports semantic equality of immutable binding values. It is useful
// to runtime layers that must compare two calculation environments without
// relying on pointer identity or serialization.
func (v BindingValue) Equal(other BindingValue) bool {
	if !v.IsValid() || !other.IsValid() || v.kind != other.kind {
		return false
	}

	switch v.kind {
	case BindingValueDefined:
		return v.value.Equal(other.value)
	case BindingValueNull:
		return true
	case BindingValueSeries:
		return v.series.Equal(other.series)
	default:
		return false
	}
}

// ValueBinding stores a base value candidate.
//
// Resolution decides whether this binding is applicable for a concrete
// object, scope and effective time.
type ValueBinding struct {
	target ValueTarget
	value  BindingValue

	scopeSelectors []scope.Selector

	effectivePeriod    domaintime.EffectivePeriod
	hasEffectivePeriod bool
}

func NewValueBinding(
	target ValueTarget,
	bindingValue BindingValue,
	scopeSelectors []scope.Selector,
	effectivePeriod domaintime.EffectivePeriod,
	hasEffectivePeriod bool,
) ValueBinding {
	if !target.IsValid() {
		panic("semantic: value binding target is invalid")
	}

	if !bindingValue.IsValid() {
		panic("semantic: value binding value is invalid")
	}

	selectors := make([]scope.Selector, len(scopeSelectors))

	for i, selector := range scopeSelectors {
		if !selector.IsValid() {
			panic("semantic: value binding scope selector is invalid")
		}

		selectors[i] = selector
	}

	if hasEffectivePeriod && !effectivePeriod.IsValid() {
		panic("semantic: value binding effective period is invalid")
	}

	if !hasEffectivePeriod {
		effectivePeriod = domaintime.EffectivePeriod{}
	}

	return ValueBinding{
		target:             target,
		value:              bindingValue,
		scopeSelectors:     selectors,
		effectivePeriod:    effectivePeriod,
		hasEffectivePeriod: hasEffectivePeriod,
	}
}

func (b ValueBinding) Target() ValueTarget {
	return b.target
}

func (b ValueBinding) Value() BindingValue {
	return b.value
}

func (b ValueBinding) ScopeSelectors() []scope.Selector {
	result := make([]scope.Selector, len(b.scopeSelectors))
	copy(result, b.scopeSelectors)

	return result
}

func (b ValueBinding) EffectivePeriod() (
	domaintime.EffectivePeriod,
	bool,
) {
	return b.effectivePeriod, b.hasEffectivePeriod
}

func (b ValueBinding) IsValid() bool {
	if !b.target.IsValid() || !b.value.IsValid() {
		return false
	}

	for _, selector := range b.scopeSelectors {
		if !selector.IsValid() {
			return false
		}
	}

	if b.hasEffectivePeriod {
		return b.effectivePeriod.IsValid()
	}

	return true
}
