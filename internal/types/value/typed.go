package value

import (
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
)

// Typed represents a typed engine value.
//
// Typed corresponds to TypedValue in the engine domain model:
//
//	TypedValue {
//	    raw:  RawValue
//	    unit: Unit?
//	}
//
// Unit information is intentionally kept outside Raw.
//
// A Typed value may either have a Unit or be unitless.
//
// The zero value of Typed is invalid.
type Typed struct {
	raw  Raw
	unit unit.Unit
}

// NewTyped returns a unitless Typed value containing raw.
//
// NewTyped panics if raw is invalid.
func NewTyped(raw Raw) Typed {
	if !raw.IsValid() {
		panic("value: invalid raw value")
	}

	return Typed{
		raw: raw,
	}
}

// NewTypedWithUnit returns a Typed value containing raw and unit.
//
// NewTypedWithUnit panics if raw or unit is invalid.
func NewTypedWithUnit(raw Raw, u unit.Unit) Typed {
	if !raw.IsValid() {
		panic("value: invalid raw value")
	}

	if !u.IsValid() {
		panic("value: invalid unit")
	}

	return Typed{
		raw:  raw,
		unit: u,
	}
}

// Raw returns the raw value contained in v.
func (v Typed) Raw() Raw {
	return v.raw
}

// Type returns the engine type of v.
//
// Invalid is returned for the zero value of Typed.
func (v Typed) Type() Type {
	return v.raw.Type()
}

// HasUnit reports whether v has an associated Unit.
func (v Typed) HasUnit() bool {
	return v.unit.IsValid()
}

// Unit returns the Unit associated with v.
//
// Unit panics if v has no Unit.
func (v Typed) Unit() unit.Unit {
	if !v.HasUnit() {
		panic("value: typed value has no unit")
	}

	return v.unit
}

// IsValid reports whether v represents a valid Typed value.
//
// The zero value of Typed is invalid.
func (v Typed) IsValid() bool {
	return v.raw.IsValid()
}

// Equal reports whether v and other represent the same typed engine value.
//
// Two Typed values are equal only when:
//
//   - their raw values are equal;
//   - both have no Unit, or both have equal Units.
//
// Invalid Typed values are never equal.
func (v Typed) Equal(other Typed) bool {
	if !v.IsValid() || !other.IsValid() {
		return false
	}

	if !v.raw.Equal(other.raw) {
		return false
	}

	vHasUnit := v.HasUnit()
	otherHasUnit := other.HasUnit()

	if vHasUnit != otherHasUnit {
		return false
	}

	if !vHasUnit {
		return true
	}

	return v.unit.Equal(other.unit)
}
