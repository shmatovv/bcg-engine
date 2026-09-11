package eval

import (
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
)

// Result is the runtime state of one declarative Rule.
//
// condition always contains the evaluator result and provenance used to decide
// the Rule state.
type Result struct {
	rule      domainrule.Rule
	state     State
	condition evaluator.Result
}

func newResult(
	modelRule domainrule.Rule,
	state State,
	condition evaluator.Result,
) Result {
	result := Result{
		rule:      modelRule,
		state:     state,
		condition: condition,
	}

	if !result.IsValid() {
		panic("rule/eval: invalid result")
	}

	return result
}

func (r Result) Rule() domainrule.Rule {
	return r.rule
}

func (r Result) State() State {
	return r.state
}

func (r Result) Condition() evaluator.Result {
	return r.condition
}

// ActiveEffect returns the declarative effect only when the Rule is Active.
//
// Returning an effect does not apply it and does not mutate the model.
func (r Result) ActiveEffect() (
	domainrule.Effect,
	bool,
) {
	if r.state != StateActive {
		return domainrule.Effect{}, false
	}

	return r.rule.Effect(), true
}

func (r Result) IsValid() bool {
	return r.rule.IsValid() &&
		r.state.IsValid() &&
		r.condition.IsValid()
}

// Report contains Rule results in deterministic Rule UUID order.
type Report struct {
	results []Result
}

func newReport(
	results []Result,
) Report {
	copied := append(
		[]Result(nil),
		results...,
	)

	report := Report{
		results: copied,
	}

	if !report.IsValid() {
		panic("rule/eval: invalid report")
	}

	return report
}

func (r Report) Len() int {
	return len(r.results)
}

func (r Report) Results() []Result {
	return append(
		[]Result(nil),
		r.results...,
	)
}

// ActiveEffects returns active declarative effects in deterministic Rule UUID
// order. No effect is executed by this method.
func (r Report) ActiveEffects() []domainrule.Effect {
	result := make(
		[]domainrule.Effect,
		0,
	)

	for _, evaluated := range r.results {
		effect, ok := evaluated.ActiveEffect()
		if !ok {
			continue
		}

		result = append(
			result,
			effect,
		)
	}

	return result
}

func (r Report) IsValid() bool {
	for _, result := range r.results {
		if !result.IsValid() {
			return false
		}
	}

	return true
}
