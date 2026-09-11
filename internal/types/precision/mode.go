package precision

// Mode specifies how a value is rounded when discarded digits affect the result.
type Mode int

const (
	// Invalid represents an invalid rounding mode.
	Invalid Mode = iota

	// Truncate rounds toward zero.
	Truncate

	// Floor rounds toward negative infinity.
	Floor

	// Ceil rounds toward positive infinity.
	Ceil

	// HalfUp rounds to the nearest value, with ties rounded away from zero.
	HalfUp

	// HalfEven rounds to the nearest value, with ties rounded toward the
	// value whose last retained digit is even.
	HalfEven

	last
)

// IsValid reports whether m is a valid rounding mode.
func (m Mode) IsValid() bool {
	return m > Invalid && m < last
}

// String returns the name of m.
//
// String returns "Invalid" for values that do not represent a valid Mode.
func (m Mode) String() string {
	switch m {
	case Truncate:
		return "Truncate"
	case Floor:
		return "Floor"
	case Ceil:
		return "Ceil"
	case HalfUp:
		return "HalfUp"
	case HalfEven:
		return "HalfEven"
	default:
		return "Invalid"
	}
}
