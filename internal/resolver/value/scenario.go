package value

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// ResolveScenarioOverride resolves only the Scenario layer for one concrete
// Scalar property.
//
// It is valid for both Input and Derived properties.
//
// Calculation overrides are intentionally not considered here because they
// belong to the higher-precedence layer and must be checked by the caller
// first.
func ResolveScenarioOverride(
	version model.ModelVersion,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Outcome, bool, error) {
	if !version.IsCommitted() {
		return Outcome{},
			false,
			newError(
				ErrorInvalidModelVersion,
				"Scenario override resolution requires a committed ModelVersion",
			)
	}

	if !target.IsValid() {
		return Outcome{},
			false,
			newError(
				ErrorInvalidPropertyAddress,
				"target property address is invalid",
			)
	}

	if err :=
		ValidateContext(
			version,
			context,
		); err != nil {

		return Outcome{},
			false,
			err
	}

	selectedScenario,
		hasScenario :=
		context.Scenario()

	if !hasScenario {
		return Outcome{},
			false,
			nil
	}

	owner, err :=
		resolveObjectOwner(
			version,
			target.Object(),
		)

	if err != nil {
		return Outcome{},
			false,
			err
	}

	definition, ok :=
		version.PropertyDefinition(
			target.DefinitionID(),
		)

	if !ok ||
		!definition.IsValid() {

		return Outcome{},
			false,
			newError(
				ErrorDefinitionNotFound,
				"target property definition could not be resolved",
			)
	}

	if definition.Owner() != owner {
		return Outcome{},
			false,
			newError(
				ErrorDefinitionNotAvailable,
				"target property definition is not available on the concrete object type",
			)
	}

	if definition.TemporalMode() !=
		schema.TemporalScalar {

		return Outcome{},
			false,
			newError(
				ErrorUnsupportedSeriesProperty,
				"Series Scenario override resolution is not implemented yet",
			)
	}

	instant, ok :=
		context.
			TimeContext().
			At()

	if !ok {
		return Outcome{},
			false,
			newError(
				ErrorInvalidCalculationContext,
				"single Scenario override resolution requires an At time context",
			)
	}

	candidates :=
		collectScenarioCandidates(
			selectedScenario,
			target,
		)

	candidates =
		filterScenarioByScope(
			candidates,
			context.ScopeValues(),
		)

	outcome, found :=
		selectScenarioLayerAt(
			candidates,
			instant,
		)

	return outcome,
		found,
		nil
}

func collectScenarioCandidates(
	selectedScenario domainscenario.Scenario,
	target address.PropertyAddress,
) []scenarioCandidate {
	result := make(
		[]scenarioCandidate,
		0,
	)

	for _, override := range selectedScenario.Overrides() {

		if override.Target() !=
			target {

			continue
		}

		result = append(
			result,
			newScenarioCandidate(
				selectedScenario,
				override,
			),
		)
	}

	return result
}
