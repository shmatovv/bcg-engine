package bcg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/types/series"
	internalunit "github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// EffectivePeriod is an immutable optional-ended half-open [from,to) time
// selector. Its zero value is the meaningful fully open period. A valid
// period may have either or both open bounds. A nil
// *EffectivePeriod in a binding spec means no time selector; a non-nil period
// with both bounds open preserves the distinct portable explicit-empty form.
type EffectivePeriod struct{ value domaintime.EffectivePeriod }

// NewEffectivePeriod constructs a half-open effective-time selector. Nil
// bounds are open. When both bounds are present, from must precede to.
func NewEffectivePeriod(from, to *DateTime) (EffectivePeriod, error) {
	var fromValue, toValue DateTime
	if from != nil {
		if !from.IsValid() {
			return EffectivePeriod{}, fmt.Errorf("%w: effective period from is invalid", ErrInvalidArgument)
		}
		fromValue = *from
	}
	if to != nil {
		if !to.IsValid() {
			return EffectivePeriod{}, fmt.Errorf("%w: effective period to is invalid", ErrInvalidArgument)
		}
		toValue = *to
	}
	if from != nil && to != nil && fromValue.value.Cmp(toValue.value) >= 0 {
		return EffectivePeriod{}, fmt.Errorf("%w: effective period must satisfy from < to", ErrInvalidArgument)
	}
	return EffectivePeriod{value: domaintime.NewEffectivePeriod(fromValue.value, from != nil, toValue.value, to != nil)}, nil
}

// From returns the inclusive lower bound when one is present.
func (p EffectivePeriod) From() (DateTime, bool) {
	from, ok := p.value.From()
	return DateTime{value: from}, ok
}

// To returns the exclusive upper bound when one is present.
func (p EffectivePeriod) To() (DateTime, bool) {
	to, ok := p.value.To()
	return DateTime{value: to}, ok
}

// IsValid reports structural effective-period validity.
func (p EffectivePeriod) IsValid() bool { return p.value.IsValid() }

// Equal reports exact optional-bound equality.
func (p EffectivePeriod) Equal(other EffectivePeriod) bool { return p.value.Equal(other.value) }

// BindingTargetKind identifies whether a persistent value applies to one
// concrete object property or is a type-level default.
type BindingTargetKind uint8

const (
	// BindingTargetObjectProperty targets one Entity or Relation property.
	BindingTargetObjectProperty BindingTargetKind = iota + 1
	// BindingTargetTypeProperty targets a PropertyDefinition default for one
	// EntityType or RelationType.
	BindingTargetTypeProperty
)

// IsValid reports whether k is a supported binding target kind.
func (k BindingTargetKind) IsValid() bool {
	return k == BindingTargetObjectProperty || k == BindingTargetTypeProperty
}

// String returns the stable target-kind name.
func (k BindingTargetKind) String() string {
	switch k {
	case BindingTargetObjectProperty:
		return "ObjectProperty"
	case BindingTargetTypeProperty:
		return "TypeProperty"
	default:
		return "Invalid"
	}
}

// BindingTarget identifies the target of a persistent base ValueBinding.
// It has no mutable registry or object alias.
type BindingTarget struct{ value semantic.ValueTarget }

// ObjectPropertyBindingTarget targets a property of one concrete Entity or
// Relation.
func ObjectPropertyBindingTarget(address PropertyAddress) (BindingTarget, error) {
	value, err := address.internal()
	if err != nil {
		return BindingTarget{}, fmt.Errorf("%w: object binding target: %v", ErrInvalidArgument, err)
	}
	return BindingTarget{value: semantic.ObjectPropertyTarget(value)}, nil
}

// TypePropertyBindingTarget targets a type-level default PropertyDefinition.
func TypePropertyBindingTarget(owner PropertyOwner, definitionID ID) (BindingTarget, error) {
	ownerValue, err := owner.internal()
	if err != nil {
		return BindingTarget{}, fmt.Errorf("%w: type binding owner: %v", ErrInvalidArgument, err)
	}
	definition, err := definitionID.internal()
	if err != nil {
		return BindingTarget{}, fmt.Errorf("%w: type binding property definition ID: %v", ErrInvalidArgument, err)
	}
	return BindingTarget{value: semantic.TypePropertyTarget(ownerValue, definition)}, nil
}

// Kind returns the target form.
func (t BindingTarget) Kind() BindingTargetKind {
	switch t.value.Kind() {
	case semantic.ValueTargetObjectProperty:
		return BindingTargetObjectProperty
	case semantic.ValueTargetTypeProperty:
		return BindingTargetTypeProperty
	default:
		return 0
	}
}

// ObjectProperty returns the concrete target when Kind is
// BindingTargetObjectProperty.
func (t BindingTarget) ObjectProperty() (PropertyAddress, bool) {
	value, ok := t.value.ObjectProperty()
	if !ok {
		return PropertyAddress{}, false
	}
	return addressFromInternal(value), true
}

// TypeProperty returns the type owner and PropertyDefinition when Kind is
// BindingTargetTypeProperty.
func (t BindingTarget) TypeProperty() (PropertyOwner, ID, bool) {
	owner, definitionID, ok := t.value.TypeProperty()
	if !ok {
		return PropertyOwner{}, ID{}, false
	}
	return propertyOwnerFromInternal(owner), idFromInternal(definitionID), true
}

// IsValid reports structural target validity.
func (t BindingTarget) IsValid() bool { return t.value.IsValid() }

// BindingValueKind distinguishes persistent defined, explicit Null, and
// source-Series values. Missing bindings are not a BindingValue.
type BindingValueKind uint8

const (
	// BindingDefined stores one defined scalar Value.
	BindingDefined BindingValueKind = iota + 1
	// BindingNull stores explicit known absence for a nullable scalar property.
	BindingNull
	// BindingSeries stores an immutable source Series for a Series property.
	BindingSeries
)

// IsValid reports whether k is a supported persistent binding value kind.
func (k BindingValueKind) IsValid() bool {
	return k == BindingDefined || k == BindingNull || k == BindingSeries
}

// String returns the stable binding value-kind name.
func (k BindingValueKind) String() string {
	switch k {
	case BindingDefined:
		return "Defined"
	case BindingNull:
		return "Null"
	case BindingSeries:
		return "Series"
	default:
		return "Invalid"
	}
}

// BindingValue is one immutable persisted binding value.
type BindingValue struct{ value semantic.BindingValue }

// DefinedBindingValue constructs a scalar persistent value. Model type, unit,
// temporal, and nullability semantics are checked by Draft.Validate.
func DefinedBindingValue(value Value) (BindingValue, error) {
	typed, err := value.internal()
	if err != nil {
		return BindingValue{}, fmt.Errorf("%w: defined binding value: %v", ErrInvalidArgument, err)
	}
	return BindingValue{value: semantic.DefinedBindingValue(typed)}, nil
}

// NullBindingValue constructs an explicit Null persistent value. It is not a
// zero Value and is never runtime Undefined.
func NullBindingValue() BindingValue { return BindingValue{value: semantic.NullBindingValue()} }

// SeriesBindingValue constructs a persistent source-Series value.
func SeriesBindingValue(value SourceSeries) (BindingValue, error) {
	if !value.IsValid() {
		return BindingValue{}, fmt.Errorf("%w: source Series is invalid", ErrInvalidArgument)
	}
	return BindingValue{value: semantic.SeriesBindingValue(value.value)}, nil
}

// Kind returns the stored persistent value form.
func (v BindingValue) Kind() BindingValueKind {
	switch v.value.Kind() {
	case semantic.BindingValueDefined:
		return BindingDefined
	case semantic.BindingValueNull:
		return BindingNull
	case semantic.BindingValueSeries:
		return BindingSeries
	default:
		return 0
	}
}

// Value returns the defined scalar Value when Kind is BindingDefined.
func (v BindingValue) Value() (Value, bool) {
	value, ok := v.value.Value()
	if !ok {
		return Value{}, false
	}
	return valueFromInternal(value), true
}

// Series returns the immutable source Series when Kind is BindingSeries.
func (v BindingValue) Series() (SourceSeries, bool) {
	value, ok := v.value.Series()
	if !ok {
		return SourceSeries{}, false
	}
	return sourceSeriesFromInternal(value), true
}

// IsNull reports whether v is explicit Null.
func (v BindingValue) IsNull() bool { return v.value.Kind() == semantic.BindingValueNull }

// IsValid reports structural persistent-value validity.
func (v BindingValue) IsValid() bool { return v.value.IsValid() }

// SourceSample is one immutable defined or explicit-Null source series sample.
// Undefined is deliberately unavailable in source data.
type SourceSample struct{ value series.Sample }

// DefinedSourceSample constructs a source sample. Source-series units belong
// to the Series as a whole, so value must not carry a Unit.
func DefinedSourceSample(value Value) (SourceSample, error) {
	typed, err := value.internal()
	if err != nil {
		return SourceSample{}, fmt.Errorf("%w: source sample value: %v", ErrInvalidArgument, err)
	}
	if typed.HasUnit() {
		return SourceSample{}, fmt.Errorf("%w: source sample Value must not carry a Unit; set the Series Unit instead", ErrInvalidArgument)
	}
	return SourceSample{value: series.Defined(typed.Raw())}, nil
}

// NullSourceSample constructs one explicit Null source sample.
func NullSourceSample() SourceSample { return SourceSample{value: series.Null()} }

// IsDefined reports whether s has a defined raw source value.
func (s SourceSample) IsDefined() bool { return s.value.IsDefined() }

// IsNull reports whether s is explicit Null.
func (s SourceSample) IsNull() bool { return s.value.IsNull() }

// Value returns the defined scalar sample value without a unit.
func (s SourceSample) Value() (Value, bool) {
	raw, ok := s.value.Value()
	if !ok {
		return Value{}, false
	}
	return valueFromInternal(enginevalue.NewTyped(raw)), true
}

// IsValid reports source-sample structural validity.
func (s SourceSample) IsValid() bool { return s.value.IsValid() }

// SourcePoint is one immutable point-axis source sample.
type SourcePoint struct{ value series.Point }

// NewSourcePoint constructs one point-axis sample.
func NewSourcePoint(at DateTime, sample SourceSample) (SourcePoint, error) {
	if !at.IsValid() || !sample.IsValid() {
		return SourcePoint{}, fmt.Errorf("%w: valid source point timestamp and sample are required", ErrInvalidArgument)
	}
	return SourcePoint{value: series.NewPoint(at.value, sample.value)}, nil
}

// At returns the point coordinate.
func (p SourcePoint) At() DateTime { return DateTime{value: p.value.At()} }

// Sample returns the immutable point sample.
func (p SourcePoint) Sample() SourceSample { return SourceSample{value: p.value.Sample()} }

// IsValid reports source-point structural validity.
func (p SourcePoint) IsValid() bool { return p.value.IsValid() }

// SourceInterval is one immutable half-open [from,to) interval-axis source sample.
type SourceInterval struct{ value series.Interval }

// NewSourceInterval constructs one interval-axis sample.
func NewSourceInterval(from, to DateTime, sample SourceSample) (SourceInterval, error) {
	if !from.IsValid() || !to.IsValid() || from.value.Cmp(to.value) >= 0 || !sample.IsValid() {
		return SourceInterval{}, fmt.Errorf("%w: source interval requires valid from < to and sample", ErrInvalidArgument)
	}
	return SourceInterval{value: series.NewInterval(from.value, to.value, sample.value)}, nil
}

// From returns the inclusive lower coordinate.
func (i SourceInterval) From() DateTime { return DateTime{value: i.value.From()} }

// To returns the exclusive upper coordinate.
func (i SourceInterval) To() DateTime { return DateTime{value: i.value.To()} }

// Sample returns the immutable interval sample.
func (i SourceInterval) Sample() SourceSample { return SourceSample{value: i.value.Sample()} }

// IsValid reports source-interval structural validity.
func (i SourceInterval) IsValid() bool { return i.value.IsValid() }

// SourceSeries is an immutable source Point or Interval Series. It is
// distinct from the runtime SeriesResult returned by evaluation.
type SourceSeries struct{ value series.Series }

// NewPointSourceSeries constructs a canonical point source Series. points
// must already be strictly increasing by coordinate; the input slice is
// defensively copied.
func NewPointSourceSeries(elementType ValueType, unit *Unit, points []SourcePoint) (SourceSeries, error) {
	typeValue, err := elementType.internal()
	if err != nil {
		return SourceSeries{}, err
	}
	internalPoints := make([]series.Point, len(points))
	for index, point := range points {
		if !point.IsValid() || (index > 0 && internalPoints[index-1].At().Cmp(point.value.At()) >= 0) {
			return SourceSeries{}, fmt.Errorf("%w: point source Series points must be valid and strictly ordered", ErrInvalidArgument)
		}
		if sample, ok := point.Sample().Value(); ok && sample.Type() != elementType {
			return SourceSeries{}, fmt.Errorf("%w: point source Series sample type does not match element type", ErrInvalidArgument)
		}
		internalPoints[index] = point.value
	}
	unitValue, hasUnit, err := sourceSeriesUnit(unit)
	if err != nil {
		return SourceSeries{}, err
	}
	result := series.NewPointSeries(typeValue, unitValue, hasUnit, internalPoints)
	return SourceSeries{value: result}, nil
}

// NewIntervalSourceSeries constructs a canonical interval source Series.
// Intervals must be ordered and non-overlapping; adjacent half-open intervals
// are valid. The input slice is defensively copied.
func NewIntervalSourceSeries(elementType ValueType, unit *Unit, intervals []SourceInterval) (SourceSeries, error) {
	typeValue, err := elementType.internal()
	if err != nil {
		return SourceSeries{}, err
	}
	internalIntervals := make([]series.Interval, len(intervals))
	for index, interval := range intervals {
		if !interval.IsValid() || (index > 0 && internalIntervals[index-1].To().Cmp(interval.value.From()) > 0) {
			return SourceSeries{}, fmt.Errorf("%w: interval source Series intervals must be valid, ordered, and non-overlapping", ErrInvalidArgument)
		}
		if sample, ok := interval.Sample().Value(); ok && sample.Type() != elementType {
			return SourceSeries{}, fmt.Errorf("%w: interval source Series sample type does not match element type", ErrInvalidArgument)
		}
		internalIntervals[index] = interval.value
	}
	unitValue, hasUnit, err := sourceSeriesUnit(unit)
	if err != nil {
		return SourceSeries{}, err
	}
	result := series.NewIntervalSeries(typeValue, unitValue, hasUnit, internalIntervals)
	return SourceSeries{value: result}, nil
}

func sourceSeriesUnit(value *Unit) (internalunit.Unit, bool, error) {
	if value == nil {
		return internalunit.Unit{}, false, nil
	}
	if !value.IsValid() {
		return internalunit.Unit{}, false, fmt.Errorf("%w: source Series Unit is invalid", ErrInvalidArgument)
	}
	return value.value, true, nil
}

func sourceSeriesFromInternal(value series.Series) SourceSeries { return SourceSeries{value: value} }

// IsValid reports source-Series structural validity.
func (s SourceSeries) IsValid() bool { return s.value.IsValid() }

// Axis returns PointSeries or IntervalSeries.
func (s SourceSeries) Axis() SeriesAxis {
	switch s.value.AxisKind() {
	case series.AxisPoint:
		return PointSeries
	case series.AxisInterval:
		return IntervalSeries
	default:
		return ""
	}
}

// ElementType returns the required scalar sample type.
func (s SourceSeries) ElementType() ValueType { return valueTypeFromInternal(s.value.ElementType()) }

// Unit returns the optional unit shared by every defined source sample.
func (s SourceSeries) Unit() (Unit, bool) {
	if !s.IsValid() || !s.value.HasUnit() {
		return Unit{}, false
	}
	return Unit{value: s.value.Unit()}, true
}

// Points returns an owned ordered point slice for a Point Series.
func (s SourceSeries) Points() []SourcePoint {
	items := s.value.Points()
	result := make([]SourcePoint, len(items))
	for index, item := range items {
		result[index] = SourcePoint{value: item}
	}
	return result
}

// Intervals returns an owned ordered interval slice for an Interval Series.
func (s SourceSeries) Intervals() []SourceInterval {
	items := s.value.Intervals()
	result := make([]SourceInterval, len(items))
	for index, item := range items {
		result[index] = SourceInterval{value: item}
	}
	return result
}

// Equal reports exact source-Series equality, including coordinates, samples,
// type, unit, and axis.
func (s SourceSeries) Equal(other SourceSeries) bool { return s.value.Equal(other.value) }

// ValueBindingSpec describes one persistent base value candidate. A nil
// EffectivePeriod means no selector; use a non-nil NewEffectivePeriod(nil,
// nil) to author the distinct explicit unbounded wire form.
type ValueBindingSpec struct {
	Target          BindingTarget
	Value           BindingValue
	ScopeSelectors  []ScopeSelector
	EffectivePeriod *EffectivePeriod
}

// ValueBinding is an immutable persistent base candidate. It has no generated
// standalone ID: Key identifies its actual target-plus-selector identity.
type ValueBinding struct{ value semantic.ValueBinding }

// NewValueBinding constructs a persistent base binding. Cross-object,
// type/unit, nullability, temporal, and ambiguity semantics remain in
// Draft.Validate and Draft.Commit.
func NewValueBinding(spec ValueBindingSpec) (ValueBinding, error) {
	if !spec.Target.IsValid() || !spec.Value.IsValid() {
		return ValueBinding{}, fmt.Errorf("%w: valid binding Target and Value are required", ErrInvalidArgument)
	}
	selectors, publicSelectors, err := canonicalBindingSelectors(spec.ScopeSelectors)
	if err != nil {
		return ValueBinding{}, err
	}
	period, hasPeriod, err := bindingEffectivePeriod(spec.EffectivePeriod)
	if err != nil {
		return ValueBinding{}, err
	}
	_ = publicSelectors // retained by the public key through the domain binding.
	return ValueBinding{value: semantic.NewValueBinding(spec.Target.value, spec.Value.value, selectors, period, hasPeriod)}, nil
}

func valueBindingFromInternal(value semantic.ValueBinding) ValueBinding {
	return ValueBinding{value: value}
}

// Target returns the immutable binding target.
func (b ValueBinding) Target() BindingTarget { return BindingTarget{value: b.value.Target()} }

// Value returns the immutable persisted value.
func (b ValueBinding) Value() BindingValue { return BindingValue{value: b.value.Value()} }

// ScopeSelectors returns an owned canonical selector slice.
func (b ValueBinding) ScopeSelectors() []ScopeSelector {
	return publicSelectorsFromInternal(b.value.ScopeSelectors())
}

// EffectivePeriod returns the optional time selector.
func (b ValueBinding) EffectivePeriod() (EffectivePeriod, bool) {
	value, ok := b.value.EffectivePeriod()
	return EffectivePeriod{value: value}, ok
}

// Key returns the stable compound identity of b. Binding values themselves do
// not participate in this key, so a value can be replaced without inventing a
// synthetic UUID.
func (b ValueBinding) Key() ValueBindingKey {
	period, hasPeriod := b.EffectivePeriod()
	return ValueBindingKey{target: b.Target(), selectors: b.ScopeSelectors(), period: period, hasPeriod: hasPeriod}
}

// IsValid reports structural ValueBinding validity.
func (b ValueBinding) IsValid() bool { return b.value.IsValid() }

// ValueBindingKey is the actual stable compound identity of one base binding:
// target, complete scope selector set, and effective-period selector.
type ValueBindingKey struct {
	target    BindingTarget
	selectors []ScopeSelector
	period    EffectivePeriod
	hasPeriod bool
}

// Target returns the persistent target portion of k.
func (k ValueBindingKey) Target() BindingTarget { return k.target }

// ScopeSelectors returns an owned canonical copy.
func (k ValueBindingKey) ScopeSelectors() []ScopeSelector {
	return append([]ScopeSelector(nil), k.selectors...)
}

// EffectivePeriod returns the optional effective-time selector.
func (k ValueBindingKey) EffectivePeriod() (EffectivePeriod, bool) { return k.period, k.hasPeriod }

// IsValid reports whether k can identify a binding.
func (k ValueBindingKey) IsValid() bool {
	if !k.target.IsValid() || (k.hasPeriod && !k.period.IsValid()) {
		return false
	}
	_, _, err := canonicalBindingSelectors(k.selectors)
	return err == nil
}

// Equal reports exact compound-key equality. Scope selector ordering is
// canonicalized and therefore does not affect equality.
func (k ValueBindingKey) Equal(other ValueBindingKey) bool {
	if !k.IsValid() || !other.IsValid() || !bindingTargetsEqual(k.target, other.target) || k.hasPeriod != other.hasPeriod || len(k.selectors) != len(other.selectors) {
		return false
	}
	if k.hasPeriod && !k.period.Equal(other.period) {
		return false
	}
	for index := range k.selectors {
		left, right := k.selectors[index], other.selectors[index]
		if !left.DefinitionID().Equal(right.DefinitionID()) || !valuesEqual(left.Value(), right.Value()) {
			return false
		}
	}
	return true
}

func bindingTargetsEqual(left, right BindingTarget) bool {
	if left.Kind() != right.Kind() {
		return false
	}
	switch left.Kind() {
	case BindingTargetObjectProperty:
		leftAddress, leftOK := left.ObjectProperty()
		rightAddress, rightOK := right.ObjectProperty()
		return leftOK && rightOK && leftAddress.ObjectKind() == rightAddress.ObjectKind() && leftAddress.ObjectID().Equal(rightAddress.ObjectID()) && leftAddress.DefinitionID().Equal(rightAddress.DefinitionID())
	case BindingTargetTypeProperty:
		leftOwner, leftDefinition, leftOK := left.TypeProperty()
		rightOwner, rightDefinition, rightOK := right.TypeProperty()
		return leftOK && rightOK && leftOwner.Kind() == rightOwner.Kind() && leftOwner.ID().Equal(rightOwner.ID()) && leftDefinition.Equal(rightDefinition)
	default:
		return false
	}
}

func valuesEqual(left, right Value) bool {
	leftValue, leftOK := left.internal()
	rightValue, rightOK := right.internal()
	return leftOK == nil && rightOK == nil && leftValue.Equal(rightValue)
}

func canonicalBindingSelectors(input []ScopeSelector) ([]scope.Selector, []ScopeSelector, error) {
	result := append([]ScopeSelector(nil), input...)
	sort.Slice(result, func(i, j int) bool { return result[i].DefinitionID().String() < result[j].DefinitionID().String() })
	internal := make([]scope.Selector, len(result))
	for index, selector := range result {
		if index > 0 && selector.DefinitionID().Equal(result[index-1].DefinitionID()) {
			return nil, nil, fmt.Errorf("%w: duplicate scope definition %s", ErrInvalidArgument, selector.DefinitionID())
		}
		value, err := selector.internal()
		if err != nil {
			return nil, nil, err
		}
		internal[index] = value
	}
	return internal, result, nil
}

func publicSelectorsFromInternal(input []scope.Selector) []ScopeSelector {
	result := make([]ScopeSelector, len(input))
	for index, selector := range input {
		result[index] = ScopeSelector{definitionID: idFromInternal(selector.DefinitionID()), value: valueFromInternal(selector.Value())}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DefinitionID().String() < result[j].DefinitionID().String() })
	return result
}

func bindingEffectivePeriod(period *EffectivePeriod) (domaintime.EffectivePeriod, bool, error) {
	if period == nil {
		return domaintime.EffectivePeriod{}, false, nil
	}
	if !period.IsValid() {
		return domaintime.EffectivePeriod{}, false, fmt.Errorf("%w: effective period is invalid", ErrInvalidArgument)
	}
	return period.value, true, nil
}

// AddValueBinding appends a binding. It may make the Draft invalid, for
// example by creating an equal-precedence conflict; validation remains
// authoritative.
func (d *Draft) AddValueBinding(value ValueBinding) error {
	if !d.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft and ValueBinding are required", ErrInvalidArgument)
	}
	d.value.AddValueBinding(value.value)
	return nil
}

// SetValueBinding replaces an existing binding with the same compound key, or
// adds it when the key is absent. It rejects duplicate matching keys rather
// than choosing by insertion order.
func (d *Draft) SetValueBinding(value ValueBinding) error {
	if !d.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft and ValueBinding are required", ErrInvalidArgument)
	}
	key := value.Key()
	indexes := d.valueBindingIndexes(key)
	switch len(indexes) {
	case 0:
		d.value.AddValueBinding(value.value)
		return nil
	case 1:
		if !d.value.ReplaceValueBinding(indexes[0], value.value) {
			return fmt.Errorf("%w: binding replacement failed", ErrInvalidArgument)
		}
		return nil
	default:
		return fmt.Errorf("%w: binding key is ambiguous", ErrInvalidArgument)
	}
}

// ReplaceValueBinding replaces exactly the binding identified by key. The
// replacement must retain that compound identity; remove then add to change a
// target, selector set, or effective period.
func (d *Draft) ReplaceValueBinding(key ValueBindingKey, value ValueBinding) error {
	if !d.IsValid() || !key.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft, ValueBindingKey, and ValueBinding are required", ErrInvalidArgument)
	}
	if !key.Equal(value.Key()) {
		return fmt.Errorf("%w: replacement ValueBinding must retain its compound key", ErrInvalidArgument)
	}
	indexes := d.valueBindingIndexes(key)
	if len(indexes) != 1 {
		return bindingKeyLookupError(indexes)
	}
	if !d.value.ReplaceValueBinding(indexes[0], value.value) {
		return fmt.Errorf("%w: binding replacement failed", ErrInvalidArgument)
	}
	return nil
}

// RemoveValueBinding removes exactly the binding identified by key without
// silently choosing between duplicate invalid candidates.
func (d *Draft) RemoveValueBinding(key ValueBindingKey) error {
	if !d.IsValid() || !key.IsValid() {
		return fmt.Errorf("%w: valid Draft and ValueBindingKey are required", ErrInvalidArgument)
	}
	indexes := d.valueBindingIndexes(key)
	if len(indexes) != 1 {
		return bindingKeyLookupError(indexes)
	}
	if !d.value.RemoveValueBinding(indexes[0]) {
		return fmt.Errorf("%w: binding removal failed", ErrInvalidArgument)
	}
	return nil
}

func (d *Draft) valueBindingIndexes(key ValueBindingKey) []int {
	items := d.value.ValueBindings()
	result := make([]int, 0, 1)
	for index, item := range items {
		if valueBindingFromInternal(item).Key().Equal(key) {
			result = append(result, index)
		}
	}
	return result
}

func bindingKeyLookupError(indexes []int) error {
	if len(indexes) == 0 {
		return fmt.Errorf("%w: ValueBinding was not found", ErrInvalidArgument)
	}
	return fmt.Errorf("%w: ValueBinding key is ambiguous", ErrInvalidArgument)
}

// ValueBindings returns owned deterministic persistent binding declarations.
func (d *Draft) ValueBindings() []ValueBinding {
	if !d.IsValid() {
		return nil
	}
	items := d.value.ValueBindings()
	result := make([]ValueBinding, len(items))
	for index, item := range items {
		result[index] = valueBindingFromInternal(item)
	}
	sort.SliceStable(result, func(i, j int) bool { return valueBindingSortKey(result[i]) < valueBindingSortKey(result[j]) })
	return result
}

func valueBindingSortKey(value ValueBinding) string {
	key := value.Key()
	result := key.target.Kind().String()
	if address, ok := key.target.ObjectProperty(); ok {
		result += ":" + string(address.ObjectKind()) + ":" + address.ObjectID().String() + ":" + address.DefinitionID().String()
	}
	if owner, definitionID, ok := key.target.TypeProperty(); ok {
		result += ":" + owner.Kind().String() + ":" + owner.ID().String() + ":" + definitionID.String()
	}
	for _, selector := range key.selectors {
		result += ":" + selector.DefinitionID().String() + "=" + publicValueSortKey(selector.Value())
	}
	if period, ok := key.EffectivePeriod(); ok {
		from, hasFrom := period.From()
		to, hasTo := period.To()
		result += ":period=" + strings.Join([]string{boolDateTime(hasFrom, from), boolDateTime(hasTo, to)}, "/")
	}
	return result
}

func boolDateTime(present bool, value DateTime) string {
	if !present {
		return "-"
	}
	return value.String()
}

func publicValueSortKey(value Value) string {
	switch value.Kind() {
	case ValueBool:
		item, _ := value.Bool()
		if item {
			return "Bool:true"
		}
		return "Bool:false"
	case ValueInteger:
		item, _ := value.IntegerString()
		return "Integer:" + item
	case ValueDecimal:
		item, _ := value.Decimal()
		return "Decimal:" + item.String()
	case ValueString:
		item, _ := value.StringValue()
		return "String:" + item
	case ValueDate:
		item, _ := value.Date()
		return "Date:" + item.String()
	case ValueDateTime:
		item, _ := value.DateTime()
		return "DateTime:" + item.String()
	case ValueDuration:
		item, _ := value.Duration()
		return "Duration:" + item.String()
	default:
		return "Invalid"
	}
}
