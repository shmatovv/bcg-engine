package value

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
)

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
