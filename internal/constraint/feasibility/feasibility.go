package feasibility

import (
	"github.com/shmatovv/bcg-engine/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

type State uint8

const (
	StateInvalid State = iota
	StateFeasible
	StateInfeasible
	StateUndetermined
)

func (s State) IsValid() bool {
	return s == StateFeasible ||
		s == StateInfeasible ||
		s == StateUndetermined
}

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

type Result struct {
	state State

	violatedHard  []identity.UUID
	undefinedHard []identity.UUID
}

// Assess computes overall feasibility.
//
// Semantics:
//
//	any Violated Hard -> Infeasible
//	no Violated Hard, but at least one Undefined Hard -> Undetermined
//	otherwise -> Feasible
func Assess(report check.Report) Result {
	if !report.IsValid() {
		panic("constraint/feasibility: invalid check report")
	}

	violatedHard := make([]identity.UUID, 0)
	undefinedHard := make([]identity.UUID, 0)

	for _, checked := range report.Results() {
		modelConstraint := checked.Constraint()

		if modelConstraint.Severity() != constraint.SeverityHard {
			continue
		}

		switch checked.State() {
		case check.StateViolated:
			violatedHard = append(
				violatedHard,
				modelConstraint.ID(),
			)
		case check.StateUndefined:
			undefinedHard = append(
				undefinedHard,
				modelConstraint.ID(),
			)
		}
	}

	state := StateFeasible

	if len(violatedHard) > 0 {
		state = StateInfeasible
	} else if len(undefinedHard) > 0 {
		state = StateUndetermined
	}

	return Result{
		state:         state,
		violatedHard:  violatedHard,
		undefinedHard: undefinedHard,
	}
}

func (r Result) State() State {
	return r.state
}

func (r Result) ViolatedHard() []identity.UUID {
	return append([]identity.UUID(nil), r.violatedHard...)
}

func (r Result) UndefinedHard() []identity.UUID {
	return append([]identity.UUID(nil), r.undefinedHard...)
}

func (r Result) IsValid() bool {
	if !r.state.IsValid() {
		return false
	}

	switch r.state {
	case StateInfeasible:
		return len(r.violatedHard) > 0
	case StateUndetermined:
		return len(r.violatedHard) == 0 &&
			len(r.undefinedHard) > 0
	case StateFeasible:
		return len(r.violatedHard) == 0 &&
			len(r.undefinedHard) == 0
	default:
		return false
	}
}
