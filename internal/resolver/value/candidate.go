package value

import (
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	resolverscope "github.com/shmatovv/bcg-engine/internal/resolver/scope"
	resolvertime "github.com/shmatovv/bcg-engine/internal/resolver/time"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	enginevalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

type candidate struct {
	binding semantic.ValueBinding
	source  Source
}

func newCandidate(
	binding semantic.ValueBinding,
	sourceKind SourceKind,
) candidate {
	specificity :=
		resolverscope.Specificity(
			binding.ScopeSelectors(),
		)

	return candidate{
		binding: binding,
		source: baseBindingSource(
			sourceKind,
			binding,
			specificity,
		),
	}
}

type scenarioCandidate struct {
	override domainscenario.Override
	source   Source
}

func newScenarioCandidate(
	scenario domainscenario.Scenario,
	override domainscenario.Override,
) scenarioCandidate {
	specificity :=
		resolverscope.Specificity(
			override.ScopeSelectors(),
		)

	return scenarioCandidate{
		override: override,
		source: scenarioOverrideSource(
			scenario,
			override,
			specificity,
		),
	}
}

func filterByScope(
	candidates []candidate,
	values scope.Values,
) []candidate {
	result := make(
		[]candidate,
		0,
		len(candidates),
	)

	for _, candidate := range candidates {

		if resolverscope.Matches(
			candidate.binding.ScopeSelectors(),
			values,
		) {
			result = append(
				result,
				candidate,
			)
		}
	}

	return result
}

func filterScenarioByScope(
	candidates []scenarioCandidate,
	values scope.Values,
) []scenarioCandidate {
	result := make(
		[]scenarioCandidate,
		0,
		len(candidates),
	)

	for _, candidate := range candidates {

		if resolverscope.Matches(
			candidate.override.ScopeSelectors(),
			values,
		) {
			result = append(
				result,
				candidate,
			)
		}
	}

	return result
}

func selectAt(
	scenarioCandidates []scenarioCandidate,
	objectCandidates []candidate,
	typeCandidates []candidate,
	instant datetime.DateTime,
) Outcome {
	if outcome, found :=
		selectScenarioLayerAt(
			scenarioCandidates,
			instant,
		); found {

		return outcome
	}

	if outcome, found :=
		selectLayerAt(
			objectCandidates,
			instant,
		); found {

		return outcome
	}

	if outcome, found :=
		selectLayerAt(
			typeCandidates,
			instant,
		); found {

		return outcome
	}

	return newUndefinedOutcome(
		enginevalue.UndefinedMissingValue,
	)
}

func selectScenarioLayerAt(
	candidates []scenarioCandidate,
	instant datetime.DateTime,
) (Outcome, bool) {
	maxSpecificity := -1

	selected := make(
		[]scenarioCandidate,
		0,
		1,
	)

	for _, candidate := range candidates {

		period,
			hasPeriod :=
			candidate.
				override.
				EffectivePeriod()

		if !resolvertime.ContainsAt(
			period,
			hasPeriod,
			instant,
		) {
			continue
		}

		specificity :=
			candidate.
				source.
				ScopeSpecificity()

		switch {
		case specificity >
			maxSpecificity:

			maxSpecificity =
				specificity

			selected =
				selected[:0]

			selected = append(
				selected,
				candidate,
			)

		case specificity ==
			maxSpecificity:

			selected = append(
				selected,
				candidate,
			)
		}
	}

	if len(selected) == 0 {
		return Outcome{},
			false
	}

	if len(selected) > 1 {
		return newUndefinedOutcome(
			enginevalue.UndefinedAmbiguousValue,
		), true
	}

	return newSelectedOutcome(
		resolvedFromBindingValue(
			selected[0].
				override.
				Value(),
		),
		selected[0].source,
	), true
}

func selectLayerAt(
	candidates []candidate,
	instant datetime.DateTime,
) (Outcome, bool) {
	maxSpecificity := -1

	selected := make(
		[]candidate,
		0,
		1,
	)

	for _, candidate := range candidates {

		period,
			hasPeriod :=
			candidate.
				binding.
				EffectivePeriod()

		if !resolvertime.ContainsAt(
			period,
			hasPeriod,
			instant,
		) {
			continue
		}

		specificity :=
			candidate.
				source.
				ScopeSpecificity()

		switch {
		case specificity >
			maxSpecificity:

			maxSpecificity =
				specificity

			selected =
				selected[:0]

			selected = append(
				selected,
				candidate,
			)

		case specificity ==
			maxSpecificity:

			selected = append(
				selected,
				candidate,
			)
		}
	}

	if len(selected) == 0 {
		return Outcome{},
			false
	}

	if len(selected) > 1 {
		return newUndefinedOutcome(
			enginevalue.UndefinedAmbiguousValue,
		), true
	}

	return newSelectedOutcome(
		resolvedFromBindingValue(
			selected[0].
				binding.
				Value(),
		),
		selected[0].source,
	), true
}

func resolvedFromBindingValue(
	bindingValue semantic.BindingValue,
) enginevalue.Resolved {
	switch bindingValue.Kind() {
	case semantic.BindingValueDefined:
		defined, ok :=
			bindingValue.Value()

		if !ok ||
			!defined.IsValid() {

			panic(
				"resolver/value: invalid committed defined binding value",
			)
		}

		return enginevalue.Defined(
			defined,
		)

	case semantic.BindingValueNull:
		return enginevalue.Null()

	default:
		panic(
			"resolver/value: invalid committed binding value",
		)
	}
}
