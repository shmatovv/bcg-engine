package constraint

// Severity identifies the business importance of one Constraint.
//
// Only Hard participates in Feasibility. Info, Warning and Error remain
// reportable constraint violations but do not make a calculation infeasible.
type Severity uint8

const (
	SeverityInvalid Severity = iota
	SeverityInfo
	SeverityWarning
	SeverityError
	SeverityHard
)

func (s Severity) IsValid() bool {
	switch s {
	case SeverityInfo,
		SeverityWarning,
		SeverityError,
		SeverityHard:
		return true
	default:
		return false
	}
}

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "Info"
	case SeverityWarning:
		return "Warning"
	case SeverityError:
		return "Error"
	case SeverityHard:
		return "Hard"
	default:
		return "Invalid"
	}
}
