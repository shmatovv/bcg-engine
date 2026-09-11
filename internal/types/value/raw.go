package value

import (
	"fmt"
	"math/big"

	"github.com/shmatovv/bcg-engine/internal/types/date"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/duration"
)

// Raw represents a raw engine value.
//
// Raw corresponds to RawValue in the engine domain model.
//
// A valid Raw contains exactly one value of one of the supported engine
// types:
//
//	Bool
//	Integer
//	Decimal
//	String
//	Date
//	DateTime
//	Duration
//
// Unit information is intentionally not part of Raw. Units belong to the
// typed-value layer.
//
// Null and Undefined are also not Raw values. They belong to the resolved
// value layer.
//
// The zero value of Raw is invalid.
//
// Raw has immutable value semantics. In particular, mutable values such as
// big.Int are copied when they enter or leave Raw.
type Raw struct {
	typ Type

	boolValue     bool
	integerValue  big.Int
	decimalValue  decimal.Decimal
	stringValue   string
	dateValue     date.Date
	dateTimeValue datetime.DateTime
	durationValue duration.Duration
}

// NewBool returns a Raw containing value as a Bool.
func NewBool(value bool) Raw {
	return Raw{
		typ:       Bool,
		boolValue: value,
	}
}

// NewInteger returns a Raw containing value as an Integer.
//
// value is copied, so subsequent modifications of value do not affect the
// returned Raw.
//
// NewInteger panics if value is nil.
func NewInteger(value *big.Int) Raw {
	if value == nil {
		panic("value: nil integer")
	}

	var copied big.Int
	copied.Set(value)

	return Raw{
		typ:          Integer,
		integerValue: copied,
	}
}

// NewInt64 returns a Raw containing value as an Integer.
func NewInt64(value int64) Raw {
	var integer big.Int
	integer.SetInt64(value)

	return Raw{
		typ:          Integer,
		integerValue: integer,
	}
}

// NewUint64 returns a Raw containing value as an Integer.
func NewUint64(value uint64) Raw {
	var integer big.Int
	integer.SetUint64(value)

	return Raw{
		typ:          Integer,
		integerValue: integer,
	}
}

// NewDecimal returns a Raw containing value as a Decimal.
func NewDecimal(value decimal.Decimal) Raw {
	return Raw{
		typ:          Decimal,
		decimalValue: value,
	}
}

// NewString returns a Raw containing value as a String.
func NewString(value string) Raw {
	return Raw{
		typ:         String,
		stringValue: value,
	}
}

// NewDate returns a Raw containing value as a Date.
//
// NewDate panics if value is not a valid Date.
func NewDate(value date.Date) Raw {
	if !value.IsValid() {
		panic("value: invalid date")
	}

	return Raw{
		typ:       Date,
		dateValue: value,
	}
}

// NewDateTime returns a Raw containing value as a DateTime.
//
// NewDateTime panics if value is not a valid DateTime.
func NewDateTime(value datetime.DateTime) Raw {
	if !value.IsValid() {
		panic("value: invalid datetime")
	}

	return Raw{
		typ:           DateTime,
		dateTimeValue: value,
	}
}

// NewDuration returns a Raw containing value as a Duration.
//
// Zero and negative durations are valid.
func NewDuration(value duration.Duration) Raw {
	return Raw{
		typ:           Duration,
		durationValue: value,
	}
}

// Type returns the engine type of r.
//
// Type returns Invalid for the zero value of Raw.
func (r Raw) Type() Type {
	return r.typ
}

// IsValid reports whether r contains a valid raw engine value.
//
// The zero value of Raw is invalid.
func (r Raw) IsValid() bool {
	if !r.typ.IsValid() {
		return false
	}

	switch r.typ {
	case Bool:
		return true

	case Integer:
		return true

	case Decimal:
		return true

	case String:
		return true

	case Date:
		return r.dateValue.IsValid()

	case DateTime:
		return r.dateTimeValue.IsValid()

	case Duration:
		return true

	default:
		return false
	}
}

// Bool returns the Bool contained in r.
//
// Bool panics if r does not contain a Bool.
func (r Raw) Bool() bool {
	r.requireType(Bool)

	return r.boolValue
}

// Integer returns the Integer contained in r.
//
// The returned big.Int is a copy and may be modified by the caller without
// affecting r.
//
// Integer panics if r does not contain an Integer.
func (r Raw) Integer() *big.Int {
	r.requireType(Integer)

	return new(big.Int).Set(&r.integerValue)
}

// Decimal returns the Decimal contained in r.
//
// Decimal panics if r does not contain a Decimal.
func (r Raw) Decimal() decimal.Decimal {
	r.requireType(Decimal)

	return r.decimalValue
}

// StringValue returns the String contained in r.
//
// StringValue panics if r does not contain a String.
func (r Raw) StringValue() string {
	r.requireType(String)

	return r.stringValue
}

// Date returns the Date contained in r.
//
// Date panics if r does not contain a Date.
func (r Raw) Date() date.Date {
	r.requireType(Date)

	return r.dateValue
}

// DateTime returns the DateTime contained in r.
//
// DateTime panics if r does not contain a DateTime.
func (r Raw) DateTime() datetime.DateTime {
	r.requireType(DateTime)

	return r.dateTimeValue
}

// Duration returns the Duration contained in r.
//
// Duration panics if r does not contain a Duration.
func (r Raw) Duration() duration.Duration {
	r.requireType(Duration)

	return r.durationValue
}

// Equal reports whether r and other contain equal raw engine values.
//
// Values of different types are never equal.
//
// Two invalid Raw values are not considered equal. Equality is defined only
// for valid raw engine values.
func (r Raw) Equal(other Raw) bool {
	if !r.IsValid() || !other.IsValid() {
		return false
	}

	if r.typ != other.typ {
		return false
	}

	switch r.typ {
	case Bool:
		return r.boolValue == other.boolValue

	case Integer:
		return r.integerValue.Cmp(&other.integerValue) == 0

	case Decimal:
		return r.decimalValue.Equal(other.decimalValue)

	case String:
		return r.stringValue == other.stringValue

	case Date:
		return r.dateValue.Equal(other.dateValue)

	case DateTime:
		return r.dateTimeValue.Equal(other.dateTimeValue)

	case Duration:
		return r.durationValue.Equal(other.durationValue)

	default:
		return false
	}
}

// requireType panics unless r is valid and contains a value of expected type.
func (r Raw) requireType(expected Type) {
	if !r.IsValid() {
		panic("value: operation on invalid raw value")
	}

	if r.typ != expected {
		panic(fmt.Sprintf(
			"value: requested %s from %s raw value",
			expected,
			r.typ,
		))
	}
}
