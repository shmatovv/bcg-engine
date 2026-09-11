package typing

// TemporalMode describes the temporal shape of an expression result.
//
// TemporalMode is independent from ValueType.
//
// Scalar represents one logical value.
// Series represents values indexed by series time.
//
// Static expression inference must preserve this distinction because
// implicit temporal conversion is forbidden.
type TemporalMode uint8

const (
	TemporalInvalid TemporalMode = iota
	TemporalScalar
	TemporalSeries
)

// IsValid reports whether m is a valid temporal mode.
func (m TemporalMode) IsValid() bool {
	switch m {
	case TemporalScalar, TemporalSeries:
		return true
	default:
		return false
	}
}

// String returns the canonical name of m.
func (m TemporalMode) String() string {
	switch m {
	case TemporalScalar:
		return "Scalar"
	case TemporalSeries:
		return "Series"
	default:
		return "Invalid"
	}
}
