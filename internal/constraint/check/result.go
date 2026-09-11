package check

import (
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
)

type Result struct {
	constraint constraint.Constraint
	state      State

	applicability    evaluator.Result
	hasApplicability bool

	assertion    evaluator.Result
	hasAssertion bool
}

func newResult(
	modelConstraint constraint.Constraint,
	state State,
	applicability evaluator.Result,
	hasApplicability bool,
	assertion evaluator.Result,
	hasAssertion bool,
) Result {
	result := Result{
		constraint:       modelConstraint,
		state:            state,
		applicability:    applicability,
		hasApplicability: hasApplicability,
		assertion:        assertion,
		hasAssertion:     hasAssertion,
	}

	if !result.IsValid() {
		panic("constraint/check: invalid result")
	}

	return result
}

func (r Result) Constraint() constraint.Constraint {
	return r.constraint
}

func (r Result) State() State {
	return r.state
}

func (r Result) Applicability() (evaluator.Result, bool) {
	return r.applicability, r.hasApplicability
}

func (r Result) Assertion() (evaluator.Result, bool) {
	return r.assertion, r.hasAssertion
}

func (r Result) IsValid() bool {
	if !r.constraint.IsValid() ||
		!r.state.IsValid() {
		return false
	}

	if r.hasApplicability {
		if !r.applicability.IsValid() {
			return false
		}
	} else if r.applicability.IsValid() {
		return false
	}

	if r.hasAssertion {
		if !r.assertion.IsValid() {
			return false
		}
	} else if r.assertion.IsValid() {
		return false
	}

	_, constraintHasApplicability := r.constraint.Applicability()
	if constraintHasApplicability != r.hasApplicability {
		return false
	}

	switch r.state {
	case StateNotApplicable:
		return r.hasApplicability && !r.hasAssertion
	case StateSatisfied, StateViolated:
		return r.hasAssertion
	case StateUndefined:
		return r.hasApplicability || r.hasAssertion
	default:
		return false
	}
}

type Report struct {
	results []Result
}

func newReport(results []Result) Report {
	copied := append([]Result(nil), results...)

	report := Report{
		results: copied,
	}

	if !report.IsValid() {
		panic("constraint/check: invalid report")
	}

	return report
}

func (r Report) Len() int {
	return len(r.results)
}

func (r Report) Results() []Result {
	return append([]Result(nil), r.results...)
}

func (r Report) IsValid() bool {
	for _, result := range r.results {
		if !result.IsValid() {
			return false
		}
	}

	return true
}
