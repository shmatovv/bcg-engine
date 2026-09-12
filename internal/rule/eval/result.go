package eval

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/types/value"
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

// RuleID identifies the immutable Rule that was evaluated.
func (r Result) RuleID() identity.UUID {
	return r.rule.ID()
}

func (r Result) State() State {
	return r.state
}

func (r Result) Condition() evaluator.Result {
	return r.condition
}

// UndefinedCause returns the evaluator's structured Undefined reason when
// the Rule is Undefined because its condition evaluated to Undefined. Null
// is also indeterminate but deliberately has no UndefinedInfo.
func (r Result) UndefinedCause() (value.UndefinedInfo, bool) {
	if r.state != StateUndefined || !r.condition.IsValid() || !r.condition.Value().IsUndefined() {
		return value.UndefinedInfo{}, false
	}
	return r.condition.Value().UndefinedInfo(), true
}

func (r Result) IsValid() bool {
	if !r.rule.IsValid() || !r.state.IsValid() || !r.condition.IsValid() {
		return false
	}
	active, indeterminate, err := evaluator.InterpretScalarBool(r.condition.Value())
	if err != nil {
		return false
	}
	if indeterminate {
		return r.state == StateUndefined
	}
	if active {
		return r.state == StateActive
	}
	return r.state == StateInactive
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

// Active returns owned results whose condition evaluated to true.
func (r Report) Active() []Result {
	return r.byState(StateActive)
}

// Inactive returns owned results whose condition evaluated to false.
func (r Report) Inactive() []Result {
	return r.byState(StateInactive)
}

// Undefined returns owned indeterminate results. Condition retains whether
// indeterminacy came from Null or a structured Undefined outcome.
func (r Report) Undefined() []Result {
	return r.byState(StateUndefined)
}

func (r Report) byState(state State) []Result {
	result := make([]Result, 0)
	for _, evaluated := range r.results {
		if evaluated.state == state {
			result = append(result, evaluated)
		}
	}
	return result
}

func (r Report) IsValid() bool {
	for index, result := range r.results {
		if !result.IsValid() {
			return false
		}
		if index > 0 && compareUUID(r.results[index-1].Rule().ID(), result.Rule().ID()) >= 0 {
			return false
		}
	}

	return true
}
