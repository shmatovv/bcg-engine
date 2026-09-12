package check

// State is the semantic result of evaluating one Constraint.
type State uint8

const (
	StateInvalid State = iota
	StateSatisfied
	StateViolated
	StateNotApplicable
	StateUndefined
)

// IsValid reports whether s is a supported Constraint outcome.
func (s State) IsValid() bool {
	switch s {
	case StateSatisfied, StateViolated, StateNotApplicable, StateUndefined:
		return true
	default:
		return false
	}
}

// String returns s's stable semantic name.
func (s State) String() string {
	switch s {
	case StateSatisfied:
		return "Satisfied"
	case StateViolated:
		return "Violated"
	case StateNotApplicable:
		return "NotApplicable"
	case StateUndefined:
		return "Undefined"
	default:
		return "Invalid"
	}
}
