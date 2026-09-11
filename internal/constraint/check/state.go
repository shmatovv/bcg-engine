package check

type State uint8

const (
	StateInvalid State = iota
	StateSatisfied
	StateViolated
	StateNotApplicable
	StateUndefined
)

func (s State) IsValid() bool {
	switch s {
	case StateSatisfied,
		StateViolated,
		StateNotApplicable,
		StateUndefined:
		return true
	default:
		return false
	}
}

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
