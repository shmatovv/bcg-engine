package precision

// Policy defines the decimal precision and rounding behavior used by
// operations that require a finite decimal result.
//
// A Policy consists of:
//
//   - scale: the number of fractional decimal digits to retain;
//   - rounding: the rounding mode applied when discarded digits affect
//     the result.
//
// Scale zero is valid and represents integer precision.
//
// The zero value Policy{} is invalid because its rounding mode is Invalid.
type Policy struct {
	scale    uint64
	rounding Mode
}

// New returns a Policy with the specified scale and rounding mode.
//
// New panics if rounding is not a valid Mode.
func New(scale uint64, rounding Mode) Policy {
	if !rounding.IsValid() {
		panic("precision: invalid rounding mode")
	}

	return Policy{
		scale:    scale,
		rounding: rounding,
	}
}

// Scale returns the number of fractional decimal digits retained by p.
func (p Policy) Scale() uint64 {
	return p.scale
}

// Rounding returns the rounding mode used by p.
func (p Policy) Rounding() Mode {
	return p.rounding
}

// IsValid reports whether p represents a valid precision policy.
//
// Scale does not affect validity: every uint64 scale is structurally valid.
// Consumers may impose narrower scale limits according to their own numeric
// representation.
//
// The zero value Policy{} is invalid.
func (p Policy) IsValid() bool {
	return p.rounding.IsValid()
}

// Equal reports whether p and other represent the same precision policy.
//
// Invalid policies are never equal.
func (p Policy) Equal(other Policy) bool {
	if !p.IsValid() || !other.IsValid() {
		return false
	}

	return p.scale == other.scale &&
		p.rounding == other.rounding
}
