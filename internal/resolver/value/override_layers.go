package value

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// OverrideLayers resolves calculation and Scenario replacement layers for a
// single already-validated calculation environment.
//
// Effective-plan construction visits many concrete properties in the same
// immutable environment. Revalidating all scope selectors, Scenario
// overrides, and calculation overrides for every vertex is both redundant and
// needlessly expensive. Constructing OverrideLayers performs that validation
// once; its methods retain the ordinary target/definition checks needed for
// each concrete property.
//
// This is an internal execution helper. It does not cache business results and
// does not change resolver precedence or outcome semantics.
type OverrideLayers struct {
	version model.ModelVersion
	context runtime.CalculationContext
}

// NewOverrideLayers validates one immutable ModelVersion and calculation
// environment before it is reused while constructing an Effective Plan.
func NewOverrideLayers(
	version model.ModelVersion,
	context runtime.CalculationContext,
) (OverrideLayers, error) {
	if err := ValidateContext(version, context); err != nil {
		return OverrideLayers{}, err
	}

	return OverrideLayers{
		version: version,
		context: context,
	}, nil
}

// ResolveScalar resolves the calculation then Scenario override layers for a
// scalar property. found reports a final replacement, including an ambiguous
// Scenario outcome that intentionally cuts ordinary dependencies.
func (r OverrideLayers) ResolveScalar(
	target address.PropertyAddress,
	definition schema.PropertyDefinition,
) (Outcome, bool, error) {
	if err := r.validateTarget(target, definition, schema.TemporalScalar); err != nil {
		return Outcome{}, false, err
	}

	if override, ok := r.context.Override(target); ok {
		// NewOverrideLayers has already validated every override against this
		// exact committed ModelVersion and context.
		return newSelectedOutcome(
			resolvedFromBindingValue(override.Value()),
			calculationOverrideSource(override),
		), true, nil
	}

	scenario, hasScenario := r.context.Scenario()
	if !hasScenario {
		return Outcome{}, false, nil
	}

	instant, ok := r.context.TimeContext().At()
	if !ok {
		return Outcome{}, false, newError(
			ErrorInvalidCalculationContext,
			"scalar Scenario override resolution requires an At time context",
		)
	}

	candidates := filterScenarioByScope(
		collectScenarioCandidates(scenario, target),
		r.context.ScopeValues(),
	)

	outcome, found := selectScenarioLayerAt(candidates, instant)
	return outcome, found, nil
}

// ResolveSeries resolves the calculation then Scenario override layers for a
// Series property. The outcome is intentionally whole-Series, matching the
// existing Series resolver contract.
func (r OverrideLayers) ResolveSeries(
	target address.PropertyAddress,
	definition schema.PropertyDefinition,
) (SeriesOutcome, bool, error) {
	if err := r.validateTarget(target, definition, schema.TemporalSeries); err != nil {
		return SeriesOutcome{}, false, err
	}

	instant, ok := r.context.TimeContext().At()
	if !ok {
		return SeriesOutcome{}, false, newError(
			ErrorInvalidCalculationContext,
			"Series override resolution requires an At calculation time context",
		)
	}

	if override, ok := r.context.Override(target); ok {
		stored, ok := override.Value().Series()
		if !ok {
			return SeriesOutcome{}, false, newError(
				ErrorInvalidCalculationOverride,
				"Series calculation override has no Series value",
			)
		}

		return newSelectedSeriesOutcome(
			stored,
			calculationOverrideSource(override),
		), true, nil
	}

	scenario, hasScenario := r.context.Scenario()
	if !hasScenario {
		return SeriesOutcome{}, false, nil
	}

	candidates := filterScenarioByScope(
		collectScenarioCandidates(scenario, target),
		r.context.ScopeValues(),
	)

	return selectSeriesScenarioAt(candidates, instant)
}

func (r OverrideLayers) validateTarget(
	target address.PropertyAddress,
	definition schema.PropertyDefinition,
	mode schema.TemporalMode,
) error {
	if !r.version.IsCommitted() || !r.context.IsValid() {
		return newError(
			ErrorInvalidCalculationContext,
			"override layers are not valid for resolution",
		)
	}

	if !target.IsValid() {
		return newError(
			ErrorInvalidPropertyAddress,
			"target property address is invalid",
		)
	}

	if !definition.IsValid() || definition.ID() != target.DefinitionID() {
		return newError(
			ErrorDefinitionNotFound,
			"target property definition could not be resolved",
		)
	}

	owner, err := resolveObjectOwner(r.version, target.Object())
	if err != nil {
		return err
	}

	if definition.Owner() != owner {
		return newError(
			ErrorDefinitionNotAvailable,
			"target property definition is not available on the concrete object type",
		)
	}

	if definition.TemporalMode() != mode {
		return newError(
			ErrorUnsupportedSeriesProperty,
			"target property has an incompatible temporal mode for override resolution",
		)
	}

	return nil
}
