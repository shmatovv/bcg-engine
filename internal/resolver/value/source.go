package value

import (
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
)

// SourceKind identifies where one successfully selected resolved value came
// from.
type SourceKind uint8

const (
	SourceInvalid SourceKind = iota
	SourceCalculationOverride
	SourceScenarioOverride
	SourceBaseObjectBinding
	SourceBaseTypeBinding
)

func (k SourceKind) IsValid() bool {
	return k == SourceCalculationOverride ||
		k == SourceScenarioOverride ||
		k == SourceBaseObjectBinding ||
		k == SourceBaseTypeBinding
}

func (k SourceKind) String() string {
	switch k {
	case SourceCalculationOverride:
		return "CalculationOverride"

	case SourceScenarioOverride:
		return "ScenarioOverride"

	case SourceBaseObjectBinding:
		return "BaseObjectBinding"

	case SourceBaseTypeBinding:
		return "BaseTypeBinding"

	default:
		return "Invalid"
	}
}

// Source captures the selected resolution source.
//
// Immutable source objects are stored directly so provenance can later explain
// exactly which Scenario/override/binding won without re-running resolution.
type Source struct {
	kind SourceKind

	override    runtime.Override
	hasOverride bool

	scenario         domainscenario.Scenario
	scenarioOverride domainscenario.Override
	hasScenario      bool

	binding    semantic.ValueBinding
	hasBinding bool

	scopeSpecificity int
}

func calculationOverrideSource(
	override runtime.Override,
) Source {
	return Source{
		kind:             SourceCalculationOverride,
		override:         override,
		hasOverride:      true,
		scopeSpecificity: 0,
	}
}

func scenarioOverrideSource(
	scenario domainscenario.Scenario,
	override domainscenario.Override,
	scopeSpecificity int,
) Source {
	if !scenario.IsValid() {
		panic(
			"resolver/value: invalid Scenario override source Scenario",
		)
	}

	if !override.IsValid() {
		panic(
			"resolver/value: invalid Scenario override source override",
		)
	}

	if scopeSpecificity < 0 {
		panic(
			"resolver/value: invalid Scenario override scope specificity",
		)
	}

	return Source{
		kind:             SourceScenarioOverride,
		scenario:         scenario,
		scenarioOverride: override,
		hasScenario:      true,
		scopeSpecificity: scopeSpecificity,
	}
}

func baseBindingSource(
	kind SourceKind,
	binding semantic.ValueBinding,
	scopeSpecificity int,
) Source {
	if kind != SourceBaseObjectBinding &&
		kind != SourceBaseTypeBinding {

		panic(
			"resolver/value: invalid base binding source kind",
		)
	}

	return Source{
		kind:             kind,
		binding:          binding,
		hasBinding:       true,
		scopeSpecificity: scopeSpecificity,
	}
}

func (s Source) Kind() SourceKind {
	return s.kind
}

func (s Source) Override() (
	runtime.Override,
	bool,
) {
	return s.override,
		s.hasOverride
}

func (s Source) ScenarioOverride() (
	domainscenario.Scenario,
	domainscenario.Override,
	bool,
) {
	return s.scenario,
		s.scenarioOverride,
		s.hasScenario
}

func (s Source) Binding() (
	semantic.ValueBinding,
	bool,
) {
	return s.binding,
		s.hasBinding
}

func (s Source) ScopeSpecificity() int {
	return s.scopeSpecificity
}

func (s Source) IsValid() bool {
	switch s.kind {
	case SourceCalculationOverride:
		return s.hasOverride &&
			!s.hasScenario &&
			!s.hasBinding &&
			s.override.IsValid() &&
			s.scopeSpecificity == 0

	case SourceScenarioOverride:
		return !s.hasOverride &&
			s.hasScenario &&
			!s.hasBinding &&
			s.scenario.IsValid() &&
			s.scenarioOverride.IsValid() &&
			s.scopeSpecificity >= 0

	case SourceBaseObjectBinding,
		SourceBaseTypeBinding:

		return !s.hasOverride &&
			!s.hasScenario &&
			s.hasBinding &&
			s.binding.IsValid() &&
			s.scopeSpecificity >= 0

	default:
		return false
	}
}
