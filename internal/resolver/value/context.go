package value

import (
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	scenariovalidation "github.com/shmatovv/bcg-engine/v2/internal/scenario"
)

// ValidateContext verifies runtime context data that depends on a concrete
// committed ModelVersion.
//
// CalculationContext.IsValid checks only structural runtime invariants.
// ValidateContext additionally verifies:
//
//   - ScopeDefinition references;
//   - scope value types;
//   - selected Scenario compatibility with ModelVersion;
//   - every ScenarioOverride target/value/scope/effective period;
//   - every calculation override target, temporal mode, type, unit and nullability.
func ValidateContext(
	version ModelSource,
	context runtime.CalculationContext,
) error {
	if !version.IsCommitted() {
		return newError(
			ErrorInvalidModelVersion,
			"runtime context validation requires a committed ModelVersion",
		)
	}

	if !context.IsValid() {
		return newError(
			ErrorInvalidCalculationContext,
			"calculation context is invalid",
		)
	}

	if err :=
		validateScopeContext(
			version,
			context,
		); err != nil {

		return err
	}

	if selected, ok :=
		context.Scenario(); ok {

		report :=
			scenariovalidation.Validate(
				version,
				selected,
			)

		if !report.OK() {
			return newError(
				ErrorInvalidScenario,
				report.Error(),
			)
		}
	}

	// Validate overrides even when their targets are not reached by evaluation.
	for _, override := range context.Overrides() {
		target := override.Target()
		owner, err := resolveObjectOwner(version, target.Object())
		if err != nil {
			return err
		}
		definition, ok := version.PropertyDefinition(target.DefinitionID())
		if !ok {
			return newError(ErrorDefinitionNotFound, "calculation override definition does not exist")
		}
		if definition.Owner() != owner {
			return newError(ErrorDefinitionNotAvailable, "calculation override definition is not available on its target")
		}
		if err := validateOverride(version, definition, override); err != nil {
			return err
		}
	}
	return nil
}
