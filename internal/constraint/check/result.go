package check

import (
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
)

// Result is one immutable Constraint evaluation, including only the
// applicability/assertion evaluator Results that were actually executed.
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

// Constraint is the immutable model Constraint that was evaluated.
func (r Result) Constraint() constraint.Constraint {
	return r.constraint
}

// State is the semantic outcome of this Constraint evaluation.
func (r Result) State() State {
	return r.state
}

// Applicability returns the actual applicability result when the Constraint
// declares one. Its absence means the Constraint is unconditionally applicable.
func (r Result) Applicability() (evaluator.Result, bool) {
	return r.applicability, r.hasApplicability
}

// Assertion returns the actual assertion result only when it was evaluated.
// It is absent for NotApplicable or applicability-Undefined Constraints.
func (r Result) Assertion() (evaluator.Result, bool) {
	return r.assertion, r.hasAssertion
}

func (r Result) IsValid() bool {
	if !r.constraint.IsValid() || !r.state.IsValid() {
		return false
	}

	_, constraintHasApplicability := r.constraint.Applicability()
	if constraintHasApplicability != r.hasApplicability {
		return false
	}

	applicable := true
	applicabilityIndeterminate := false
	if r.hasApplicability {
		if !r.applicability.IsValid() {
			return false
		}
		var err error
		applicable, applicabilityIndeterminate, err = boolState(r.applicability.Value())
		if err != nil {
			return false
		}
	} else if r.applicability.IsValid() {
		return false
	}

	assertionSatisfied := false
	assertionIndeterminate := false
	if r.hasAssertion {
		if !r.assertion.IsValid() {
			return false
		}
		var err error
		assertionSatisfied, assertionIndeterminate, err = boolState(r.assertion.Value())
		if err != nil {
			return false
		}
	} else if r.assertion.IsValid() {
		return false
	}

	switch r.state {
	case StateNotApplicable:
		return r.hasApplicability && !applicabilityIndeterminate && !applicable && !r.hasAssertion
	case StateSatisfied:
		return !applicabilityIndeterminate && applicable && r.hasAssertion && !assertionIndeterminate && assertionSatisfied
	case StateViolated:
		return !applicabilityIndeterminate && applicable && r.hasAssertion && !assertionIndeterminate && !assertionSatisfied
	case StateUndefined:
		if r.hasAssertion {
			return !applicabilityIndeterminate && applicable && assertionIndeterminate
		}
		return r.hasApplicability && applicabilityIndeterminate
	default:
		return false
	}
}

// Report is a deterministic immutable collection of all requested Constraint
// results.
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

// Len returns the number of evaluated Constraints.
func (r Report) Len() int {
	return len(r.results)
}

// Results returns an owned deterministic copy of Constraint results.
func (r Report) Results() []Result {
	return append([]Result(nil), r.results...)
}

// IsValid reports whether r contains valid, uniquely identified results in
// deterministic Constraint UUID order.
func (r Report) IsValid() bool {
	for index, result := range r.results {
		if !result.IsValid() {
			return false
		}
		if index > 0 && compareUUID(r.results[index-1].Constraint().ID(), result.Constraint().ID()) >= 0 {
			return false
		}
	}
	return true
}
