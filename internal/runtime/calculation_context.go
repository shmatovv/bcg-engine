package runtime

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
)

// CalculationContext contains transient runtime dimensions that influence one
// calculation without mutating ModelVersion.
//
// Core v1 keeps the dimensions separate:
//
//   - Time is exactly one At or Period context;
//   - Scope contains independent selected scope dimensions;
//   - at most one immutable Scenario may be selected;
//   - Calculation Overrides are concrete runtime input values.
//
// The selected Scenario remains a separate immutable domain object. Keeping
// the object itself in the context makes calculation input self-contained and
// avoids hidden runtime lookup by scenario name/ID.
//
// The zero value is invalid because TimeContext is required.
type CalculationContext struct {
	timeContext domaintime.Context
	scopeValues scope.Values

	scenario    domainscenario.Scenario
	hasScenario bool

	overrides []Override
}

// NewCalculationContext constructs a runtime calculation context without a
// Scenario.
//
// Overrides must target distinct PropertyAddresses. Duplicate targets are a
// programming error because calculation overrides have one highest-precedence
// value per concrete property.
func NewCalculationContext(
	timeContext domaintime.Context,
	scopeValues scope.Values,
	overrides []Override,
) CalculationContext {
	if !timeContext.IsValid() {
		panic("runtime: calculation time context is invalid")
	}

	if !scopeValues.IsValid() {
		panic("runtime: calculation scope values are invalid")
	}

	copiedOverrides := cloneOverrides(overrides)

	return CalculationContext{
		timeContext: timeContext,
		scopeValues: scopeValues,
		overrides:   copiedOverrides,
	}
}

// WithScenario returns a copy of c selecting scenario.
//
// The Scenario must be structurally valid. Compatibility with the concrete
// committed ModelVersion is checked by the scenario/resolution layer because
// CalculationContext intentionally does not own model data.
func (c CalculationContext) WithScenario(
	scenario domainscenario.Scenario,
) CalculationContext {
	c.requireValid()

	if !scenario.IsValid() {
		panic("runtime: scenario is invalid")
	}

	c.scenario = scenario
	c.hasScenario = true

	return c
}

// WithoutScenario returns a copy of c with no selected Scenario.
func (c CalculationContext) WithoutScenario() CalculationContext {
	c.requireValid()

	c.scenario = domainscenario.Scenario{}
	c.hasScenario = false

	return c
}

// TimeContext returns the temporal calculation dimension.
func (c CalculationContext) TimeContext() domaintime.Context {
	return c.timeContext
}

// WithTimeContext returns a copy of c using timeContext while preserving its
// Scope values, selected Scenario, and calculation overrides.
//
// This supports temporal orchestration that derives independent At contexts
// from one Period context. It follows the direct construction contract and
// panics when c or timeContext is invalid.
func (c CalculationContext) WithTimeContext(
	timeContext domaintime.Context,
) CalculationContext {
	c.requireValid()

	if !timeContext.IsValid() {
		panic("runtime: calculation time context is invalid")
	}

	c.timeContext = timeContext

	return c
}

// ScopeValues returns the immutable selected scope dimensions.
func (c CalculationContext) ScopeValues() scope.Values {
	return scope.NewValues(
		c.scopeValues.Selectors(),
	)
}

// Scenario returns the optional immutable Scenario selected for this
// calculation.
func (c CalculationContext) Scenario() (
	domainscenario.Scenario,
	bool,
) {
	return c.scenario,
		c.hasScenario
}

// ScenarioID returns the optional selected Scenario identity.
//
// This is a convenience view over Scenario and does not perform any external
// lookup.
func (c CalculationContext) ScenarioID() (
	identity.UUID,
	bool,
) {
	if !c.hasScenario {
		return identity.UUID{}, false
	}

	return c.scenario.ID(), true
}

// Overrides returns calculation-local overrides in deterministic target order.
func (c CalculationContext) Overrides() []Override {
	if len(c.overrides) == 0 {
		return nil
	}

	result := make(
		[]Override,
		len(c.overrides),
	)

	copy(result, c.overrides)

	return result
}

// Override resolves a calculation override for target.
func (c CalculationContext) Override(
	target address.PropertyAddress,
) (Override, bool) {
	if !target.IsValid() {
		return Override{}, false
	}

	index := sort.Search(
		len(c.overrides),
		func(index int) bool {
			return comparePropertyAddress(
				c.overrides[index].Target(),
				target,
			) >= 0
		},
	)

	if index >= len(c.overrides) ||
		comparePropertyAddress(
			c.overrides[index].Target(),
			target,
		) != 0 {
		return Override{}, false
	}

	return c.overrides[index], true
}

// IsValid reports whether c satisfies runtime context invariants.
func (c CalculationContext) IsValid() bool {
	if !c.timeContext.IsValid() ||
		!c.scopeValues.IsValid() {
		return false
	}

	if c.hasScenario {
		if !c.scenario.IsValid() {
			return false
		}
	} else if c.scenario.IsValid() {
		return false
	}

	for index, override := range c.overrides {
		if !override.IsValid() {
			return false
		}

		if index > 0 &&
			comparePropertyAddress(
				c.overrides[index-1].Target(),
				override.Target(),
			) >= 0 {
			return false
		}
	}

	return true
}

func (c CalculationContext) requireValid() {
	if !c.IsValid() {
		panic("runtime: invalid calculation context")
	}
}

func cloneOverrides(
	overrides []Override,
) []Override {
	if len(overrides) == 0 {
		return nil
	}

	result := make(
		[]Override,
		len(overrides),
	)

	copy(result, overrides)

	for _, override := range result {
		if !override.IsValid() {
			panic("runtime: calculation override is invalid")
		}
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			return comparePropertyAddress(
				result[i].Target(),
				result[j].Target(),
			) < 0
		},
	)

	for index := 1; index < len(result); index++ {
		if result[index-1].Target() == result[index].Target() {
			panic("runtime: duplicate calculation override target")
		}
	}

	return result
}

func comparePropertyAddress(
	left address.PropertyAddress,
	right address.PropertyAddress,
) int {
	if left.Object().Kind() < right.Object().Kind() {
		return -1
	}

	if left.Object().Kind() > right.Object().Kind() {
		return 1
	}

	if comparison := compareUUID(
		left.Object().ID(),
		right.Object().ID(),
	); comparison != 0 {
		return comparison
	}

	return compareUUID(
		left.DefinitionID(),
		right.DefinitionID(),
	)
}

func compareUUID(
	left identity.UUID,
	right identity.UUID,
) int {
	leftBytes := left.Bytes()
	rightBytes := right.Bytes()

	for index := range leftBytes {
		switch {
		case leftBytes[index] < rightBytes[index]:
			return -1
		case leftBytes[index] > rightBytes[index]:
			return 1
		}
	}

	return 0
}
