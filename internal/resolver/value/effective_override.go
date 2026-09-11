package value

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// ResolveEffectiveOverride resolves only the value-replacement layers that
// bypass normal property computation.
//
// Precedence:
//
//  1. Calculation override
//  2. Scenario override
//
// The function is valid for both Input and Derived Scalar properties.
//
// found=false means normal property semantics must continue:
//   - Input -> base Object/Type ValueBinding resolution;
//   - Derived -> evaluate its committed Expression.
//
// found=true means the returned Outcome is final for the property at the
// override layer. This includes Undefined(AmbiguousValue) produced by an
// ambiguous Scenario layer; the base expression/bindings must not be used as
// fallback in that case.
func ResolveEffectiveOverride(
	version model.ModelVersion,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Outcome, bool, error) {
	calculationOutcome,
		calculationFound,
		err :=
		ResolveCalculationOverride(
			version,
			target,
			context,
		)

	if err != nil {
		return Outcome{}, false, err
	}

	if calculationFound {
		return calculationOutcome, true, nil
	}

	scenarioOutcome,
		scenarioFound,
		err :=
		ResolveScenarioOverride(
			version,
			target,
			context,
		)

	if err != nil {
		return Outcome{}, false, err
	}

	if scenarioFound {
		return scenarioOutcome, true, nil
	}

	return Outcome{}, false, nil
}
