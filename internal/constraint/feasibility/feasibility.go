// Package feasibility derives model feasibility from evaluated Constraints.
package feasibility

import (
	"github.com/shmatovv/bcg-engine/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// State is the feasibility conclusion determined solely by Hard Constraints.
type State uint8

const (
	StateInvalid State = iota
	StateFeasible
	StateInfeasible
	StateUndetermined
)

// IsValid reports whether s is one of the feasibility states.
func (s State) IsValid() bool {
	return s == StateFeasible || s == StateInfeasible || s == StateUndetermined
}

// String returns s's stable semantic name.
func (s State) String() string {
	switch s {
	case StateFeasible:
		return "Feasible"
	case StateInfeasible:
		return "Infeasible"
	case StateUndetermined:
		return "Undetermined"
	default:
		return "Invalid"
	}
}

// Result is the immutable feasibility conclusion and the complete Constraint
// report from which it was derived. All Constraints are evaluated before the
// conclusion is computed, so callers retain every violation and indeterminate
// Hard witness rather than only the first one encountered.
type Result struct {
	state State

	report check.Report

	violatedHard  []identity.UUID
	undefinedHard []identity.UUID
}

// Evaluate checks every Constraint in version using context, then derives
// feasibility. It uses check.All and therefore shares the normal resolver and
// evaluator semantics; business violations and Undefined outcomes are not Go
// errors.
func Evaluate(version check.Source, context runtime.CalculationContext) (Result, error) {
	report, err := check.All(version, context)
	if err != nil {
		return Result{}, err
	}
	return Assess(report), nil
}

// EvaluateWithSession derives feasibility from Constraint checks performed
// through one shared evaluator Session.
func EvaluateWithSession(session *evaluator.Session, context runtime.CalculationContext) (Result, error) {
	report, err := check.AllWithSession(session, context)
	if err != nil {
		return Result{}, err
	}
	return Assess(report), nil
}

// Assess derives overall feasibility from a complete valid Constraint report.
//
// Semantics:
//
//	any Violated Hard -> Infeasible
//	no Violated Hard, but at least one Undefined Hard -> Undetermined
//	otherwise -> Feasible
//
// Info, Warning, and Error Constraint outcomes never change feasibility.
func Assess(report check.Report) Result {
	if !report.IsValid() {
		panic("constraint/feasibility: invalid check report")
	}

	violatedHard, undefinedHard := hardWitnesses(report)

	state := StateFeasible
	if len(violatedHard) > 0 {
		state = StateInfeasible
	} else if len(undefinedHard) > 0 {
		state = StateUndetermined
	}

	result := Result{
		state:         state,
		report:        report,
		violatedHard:  append([]identity.UUID(nil), violatedHard...),
		undefinedHard: append([]identity.UUID(nil), undefinedHard...),
	}
	if !result.IsValid() {
		panic("constraint/feasibility: constructed invalid result")
	}
	return result
}

// State returns the feasibility conclusion.
func (r Result) State() State { return r.state }

// Report returns the complete immutable report used to derive State.
func (r Result) Report() check.Report { return r.report }

// ViolatedHard returns an owned deterministic list of applicable violated
// Hard Constraint IDs.
func (r Result) ViolatedHard() []identity.UUID {
	return append([]identity.UUID(nil), r.violatedHard...)
}

// UndefinedHard returns an owned deterministic list of indeterminate Hard
// Constraint IDs. It remains complete even when a separate hard violation
// makes the overall state Infeasible.
func (r Result) UndefinedHard() []identity.UUID {
	return append([]identity.UUID(nil), r.undefinedHard...)
}

// IsValid reports whether r is internally consistent with its complete report.
func (r Result) IsValid() bool {
	if !r.state.IsValid() || !r.report.IsValid() {
		return false
	}

	violated, undefined := hardWitnesses(r.report)
	if !sameIDs(r.violatedHard, violated) || !sameIDs(r.undefinedHard, undefined) {
		return false
	}

	switch r.state {
	case StateInfeasible:
		return len(violated) > 0
	case StateUndetermined:
		return len(violated) == 0 && len(undefined) > 0
	case StateFeasible:
		return len(violated) == 0 && len(undefined) == 0
	default:
		return false
	}
}

func hardWitnesses(report check.Report) ([]identity.UUID, []identity.UUID) {
	var violated, undefined []identity.UUID
	for _, checked := range report.Results() {
		if checked.Constraint().Severity() != constraint.SeverityHard {
			continue
		}
		switch checked.State() {
		case check.StateViolated:
			violated = append(violated, checked.Constraint().ID())
		case check.StateUndefined:
			undefined = append(undefined, checked.Constraint().ID())
		}
	}
	return violated, undefined
}

func sameIDs(left, right []identity.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
