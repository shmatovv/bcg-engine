package eval

// State is the deterministic runtime state of one Rule condition.
type State uint8

const (
	StateInvalid State = iota
	StateActive
	StateInactive
	StateUndefined
)

func (s State) IsValid() bool {
	switch s {
	case StateActive,
		StateInactive,
		StateUndefined:
		return true
	default:
		return false
	}
}

func (s State) String() string {
	switch s {
	case StateActive:
		return "Active"
	case StateInactive:
		return "Inactive"
	case StateUndefined:
		return "Undefined"
	default:
		return "Invalid"
	}
}
