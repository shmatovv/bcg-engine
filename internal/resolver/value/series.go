package value

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	resolvertime "github.com/shmatovv/bcg-engine/v2/internal/resolver/time"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/series"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// SeriesOutcome is one selected whole Series snapshot or a whole-Series
// Undefined result. A source Series cannot be Null; Null belongs to samples.
type SeriesOutcome struct {
	series series.Series

	undefined enginevalue.UndefinedInfo

	source    Source
	hasSource bool
}

func newSelectedSeriesOutcome(stored series.Series, source Source) SeriesOutcome {
	if !stored.IsValid() || !source.IsValid() {
		panic("resolver/value: invalid selected Series outcome")
	}
	return SeriesOutcome{series: stored, source: source, hasSource: true}
}

func newUndefinedSeriesOutcome(kind enginevalue.UndefinedKind) SeriesOutcome {
	return SeriesOutcome{undefined: enginevalue.NewUndefinedInfo(kind)}
}

// Series returns the selected source snapshot when o is defined.
func (o SeriesOutcome) Series() (series.Series, bool) {
	if !o.hasSource {
		return series.Series{}, false
	}
	return o.series, true
}

func (o SeriesOutcome) UndefinedInfo() (enginevalue.UndefinedInfo, bool) {
	if o.hasSource {
		return enginevalue.UndefinedInfo{}, false
	}
	return o.undefined, o.undefined.IsValid()
}

func (o SeriesOutcome) Source() (Source, bool) { return o.source, o.hasSource }

func (o SeriesOutcome) IsDefined() bool   { return o.hasSource }
func (o SeriesOutcome) IsUndefined() bool { return !o.hasSource && o.undefined.IsValid() }

func (o SeriesOutcome) IsValid() bool {
	if o.hasSource {
		return o.series.IsValid() && o.source.IsValid() && !o.undefined.IsValid()
	}
	return !o.series.IsValid() && !o.source.IsValid() && o.undefined.IsValid()
}

// ResolveSeries selects one effective input Series snapshot. Series time stays
// wholly inside the selected value; EffectivePeriod only chooses that snapshot.
// Series Core v1 accepts CalculationContext.At only.
func ResolveSeries(version ModelSource, target address.PropertyAddress, context runtime.CalculationContext) (SeriesOutcome, error) {
	definition, owner, err := validateSeriesTarget(version, target, context, true)
	if err != nil {
		return SeriesOutcome{}, err
	}
	_ = definition

	if _, ok := context.TimeContext().At(); !ok {
		return SeriesOutcome{}, newError(ErrorInvalidCalculationContext, "Series resolution requires an At calculation time context")
	}

	if override, ok := context.Override(target); ok {
		if err := validateOverride(version, definition, override); err != nil {
			return SeriesOutcome{}, err
		}
		stored, ok := override.Value().Series()
		if !ok {
			return SeriesOutcome{}, newError(ErrorInvalidCalculationOverride, "Series calculation override has no Series value")
		}
		return newSelectedSeriesOutcome(stored, calculationOverrideSource(override)), nil
	}

	if outcome, found, err := ResolveSeriesScenarioOverride(version, target, context); err != nil {
		return SeriesOutcome{}, err
	} else if found {
		return outcome, nil
	}

	objectCandidates, typeCandidates := collectBaseCandidates(version, target, owner)
	objectCandidates = filterByScope(objectCandidates, context.ScopeValues())
	typeCandidates = filterByScope(typeCandidates, context.ScopeValues())
	instant, _ := context.TimeContext().At()
	return selectSeriesBaseAt(objectCandidates, typeCandidates, instant)
}

// ResolveSeriesCalculationOverride resolves only the highest-precedence
// calculation override for a Series property. It is valid for Input and
// Derived properties and is used by the Effective Evaluation Plan.
func ResolveSeriesCalculationOverride(version ModelSource, target address.PropertyAddress, context runtime.CalculationContext) (SeriesOutcome, bool, error) {
	definition, _, err := validateSeriesTarget(version, target, context, false)
	if err != nil {
		return SeriesOutcome{}, false, err
	}
	if _, ok := context.TimeContext().At(); !ok {
		return SeriesOutcome{}, false, newError(ErrorInvalidCalculationContext, "Series calculation override resolution requires an At calculation time context")
	}
	override, ok := context.Override(target)
	if !ok {
		return SeriesOutcome{}, false, nil
	}
	if err := validateOverride(version, definition, override); err != nil {
		return SeriesOutcome{}, false, err
	}
	stored, ok := override.Value().Series()
	if !ok {
		return SeriesOutcome{}, false, newError(ErrorInvalidCalculationOverride, "Series calculation override has no Series value")
	}
	return newSelectedSeriesOutcome(stored, calculationOverrideSource(override)), true, nil
}

// ResolveSeriesScenarioOverride resolves only the Scenario layer for a Series
// property. An ambiguous matching layer is a final Undefined outcome.
func ResolveSeriesScenarioOverride(version ModelSource, target address.PropertyAddress, context runtime.CalculationContext) (SeriesOutcome, bool, error) {
	_, _, err := validateSeriesTarget(version, target, context, false)
	if err != nil {
		return SeriesOutcome{}, false, err
	}
	instant, ok := context.TimeContext().At()
	if !ok {
		return SeriesOutcome{}, false, newError(ErrorInvalidCalculationContext, "Series Scenario override resolution requires an At calculation time context")
	}
	scenario, hasScenario := context.Scenario()
	if !hasScenario {
		return SeriesOutcome{}, false, nil
	}
	candidates := filterScenarioByScope(collectScenarioCandidates(scenario, target), context.ScopeValues())
	return selectSeriesScenarioAt(candidates, instant)
}

// ResolveEffectiveSeriesOverride resolves Calculation then Scenario override
// layers for a Series property, including final ambiguity outcomes.
func ResolveEffectiveSeriesOverride(version ModelSource, target address.PropertyAddress, context runtime.CalculationContext) (SeriesOutcome, bool, error) {
	if outcome, found, err := ResolveSeriesCalculationOverride(version, target, context); err != nil || found {
		return outcome, found, err
	}
	return ResolveSeriesScenarioOverride(version, target, context)
}

func validateSeriesTarget(version ModelSource, target address.PropertyAddress, context runtime.CalculationContext, inputOnly bool) (schema.PropertyDefinition, schema.PropertyOwner, error) {
	if !version.IsCommitted() {
		return schema.PropertyDefinition{}, schema.PropertyOwner{}, newError(ErrorInvalidModelVersion, "Series resolution requires a committed ModelVersion")
	}
	if !target.IsValid() {
		return schema.PropertyDefinition{}, schema.PropertyOwner{}, newError(ErrorInvalidPropertyAddress, "Series target property address is invalid")
	}
	if err := ValidateContext(version, context); err != nil {
		return schema.PropertyDefinition{}, schema.PropertyOwner{}, err
	}
	owner, err := resolveObjectOwner(version, target.Object())
	if err != nil {
		return schema.PropertyDefinition{}, schema.PropertyOwner{}, err
	}
	definition, ok := version.PropertyDefinition(target.DefinitionID())
	if !ok || !definition.IsValid() {
		return schema.PropertyDefinition{}, schema.PropertyOwner{}, newError(ErrorDefinitionNotFound, "Series target property definition could not be resolved")
	}
	if definition.Owner() != owner {
		return schema.PropertyDefinition{}, schema.PropertyOwner{}, newError(ErrorDefinitionNotAvailable, "Series property definition is not available on the target object type")
	}
	if inputOnly && definition.Computation().Kind() != schema.ComputationInput {
		return schema.PropertyDefinition{}, schema.PropertyOwner{}, newError(ErrorNotInputProperty, "Series base resolver can only resolve Input properties")
	}
	if definition.TemporalMode() != schema.TemporalSeries {
		return schema.PropertyDefinition{}, schema.PropertyOwner{}, newError(ErrorUnsupportedSeriesProperty, "target property is Scalar, not Series")
	}
	return definition, owner, nil
}

func selectSeriesBaseAt(objectCandidates, typeCandidates []candidate, instant datetime.DateTime) (SeriesOutcome, error) {
	if outcome, found, err := selectSeriesBindingLayerAt(objectCandidates, instant); err != nil || found {
		return outcome, err
	}
	if outcome, found, err := selectSeriesBindingLayerAt(typeCandidates, instant); err != nil || found {
		return outcome, err
	}
	return newUndefinedSeriesOutcome(enginevalue.UndefinedMissingValue), nil
}

func selectSeriesBindingLayerAt(candidates []candidate, instant datetime.DateTime) (SeriesOutcome, bool, error) {
	maxSpecificity := -1
	selected := candidate{}
	selectedCount := 0
	for _, candidate := range candidates {
		period, hasPeriod := candidate.binding.EffectivePeriod()
		if !resolvertime.ContainsAt(period, hasPeriod, instant) {
			continue
		}
		specificity := candidate.source.ScopeSpecificity()
		switch {
		case specificity > maxSpecificity:
			maxSpecificity, selected, selectedCount = specificity, candidate, 1
		case specificity == maxSpecificity:
			selectedCount++
		}
	}
	if selectedCount == 0 {
		return SeriesOutcome{}, false, nil
	}
	if selectedCount > 1 {
		return newUndefinedSeriesOutcome(enginevalue.UndefinedAmbiguousValue), true, nil
	}
	stored, ok := selected.binding.Value().Series()
	if !ok {
		return SeriesOutcome{}, false, newError(ErrorInvalidCalculationContext, "committed Series binding contains no Series value")
	}
	return newSelectedSeriesOutcome(stored, selected.source), true, nil
}

func selectSeriesScenarioAt(candidates []scenarioCandidate, instant datetime.DateTime) (SeriesOutcome, bool, error) {
	maxSpecificity := -1
	selected := scenarioCandidate{}
	selectedCount := 0
	for _, candidate := range candidates {
		period, hasPeriod := candidate.override.EffectivePeriod()
		if !resolvertime.ContainsAt(period, hasPeriod, instant) {
			continue
		}
		specificity := candidate.source.ScopeSpecificity()
		switch {
		case specificity > maxSpecificity:
			maxSpecificity, selected, selectedCount = specificity, candidate, 1
		case specificity == maxSpecificity:
			selectedCount++
		}
	}
	if selectedCount == 0 {
		return SeriesOutcome{}, false, nil
	}
	if selectedCount > 1 {
		return newUndefinedSeriesOutcome(enginevalue.UndefinedAmbiguousValue), true, nil
	}
	stored, ok := selected.override.Value().Series()
	if !ok {
		return SeriesOutcome{}, false, newError(ErrorInvalidCalculationContext, "committed Series Scenario override contains no Series value")
	}
	return newSelectedSeriesOutcome(stored, selected.source), true, nil
}
