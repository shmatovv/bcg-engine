package bcg

import (
	"fmt"
	"math/big"

	"github.com/shmatovv/bcg-engine/internal/types/date"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/duration"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	enginevalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

// ValueKind classifies a defined engine Value.
type ValueKind string

const (
	ValueBool     ValueKind = "Bool"
	ValueInteger  ValueKind = "Integer"
	ValueDecimal  ValueKind = "Decimal"
	ValueString   ValueKind = "String"
	ValueDate     ValueKind = "Date"
	ValueDateTime ValueKind = "DateTime"
	ValueDuration ValueKind = "Duration"
)

// Decimal is an exact arbitrary-precision decimal. It never uses float64.
type Decimal struct{ value decimal.Decimal }

// ParseDecimal parses an exact decimal string.
func ParseDecimal(text string) (Decimal, error) {
	parsed, err := decimal.Parse(text)
	if err != nil {
		return Decimal{}, fmt.Errorf("%w: decimal: %v", ErrInvalidArgument, err)
	}
	return Decimal{value: parsed}, nil
}

// MustParseDecimal parses an exact decimal string or panics.
func MustParseDecimal(text string) Decimal {
	result, err := ParseDecimal(text)
	if err != nil {
		panic(err)
	}
	return result
}

// String returns canonical non-scientific decimal text.
func (d Decimal) String() string { return d.value.String() }

// Equal reports numeric equality.
func (d Decimal) Equal(other Decimal) bool { return d.value.Equal(other.value) }

// IsZero reports whether d is zero.
func (d Decimal) IsZero() bool { return d.value.IsZero() }

// Date is a calendar date without time-of-day or timezone.
type Date struct{ value date.Date }

// ParseDate parses a strict YYYY-MM-DD date.
func ParseDate(text string) (Date, error) {
	parsed, err := date.Parse(text)
	if err != nil {
		return Date{}, fmt.Errorf("%w: date: %v", ErrInvalidArgument, err)
	}
	return Date{value: parsed}, nil
}

// String returns canonical YYYY-MM-DD text, or empty text for an invalid zero Date.
func (d Date) String() string { return d.value.String() }

// IsValid reports whether d is a calendar date.
func (d Date) IsValid() bool { return d.value.IsValid() }

// DateTime is a nanosecond UTC instant. Parsing applies a supplied numeric
// offset and retains the engine's normalized instant semantics.
type DateTime struct{ value datetime.DateTime }

// ParseDateTime parses a strict RFC-3339-like engine DateTime.
func ParseDateTime(text string) (DateTime, error) {
	parsed, err := datetime.Parse(text)
	if err != nil {
		return DateTime{}, fmt.Errorf("%w: datetime: %v", ErrInvalidArgument, err)
	}
	return DateTime{value: parsed}, nil
}

// String returns canonical UTC DateTime text.
func (d DateTime) String() string { return d.value.String() }

// IsValid reports whether d is a valid DateTime.
func (d DateTime) IsValid() bool { return d.value.IsValid() }

// Equal reports instant equality.
func (d DateTime) Equal(other DateTime) bool { return d.value.Equal(other.value) }

// Duration is the engine's arbitrary-size duration value.
type Duration struct{ value duration.Duration }

// ParseDuration parses the engine's lossless duration notation.
func ParseDuration(text string) (Duration, error) {
	parsed, err := duration.Parse(text)
	if err != nil {
		return Duration{}, fmt.Errorf("%w: duration: %v", ErrInvalidArgument, err)
	}
	return Duration{value: parsed}, nil
}

// String returns canonical duration text.
func (d Duration) String() string { return d.value.String() }

// Unit is an immutable concrete unit. It exposes comparison and a defensive
// semantic description, not mutable unit-registry state.
type Unit struct{ value unit.Unit }

// IsValid reports whether u is a concrete unit.
func (u Unit) IsValid() bool { return u.value.IsValid() }

// Equal reports exact unit equality, including conversion-space semantics.
func (u Unit) Equal(other Unit) bool { return u.value.Equal(other.value) }

// IsDimensionless reports whether u has dimensionless meaning.
func (u Unit) IsDimensionless() bool { return u.value.IsDimensionless() }

// UnitFactor describes one symbolic factor in a Unit composition.
type UnitFactor struct {
	// ID is the canonical symbolic unit identifier.
	ID string
	// Exponent is the non-zero integer power of ID.
	Exponent int
}

// Factors returns a defensive canonical factor description.
func (u Unit) Factors() []UnitFactor {
	factors := u.value.Factors()
	result := make([]UnitFactor, len(factors))
	for index, factor := range factors {
		result[index] = UnitFactor{ID: string(factor.ID), Exponent: factor.Exponent}
	}
	return result
}

// Value is an immutable defined typed value. Null and Undefined are result
// states, not Values.
type Value struct{ value enginevalue.Typed }

// Bool returns a defined Boolean Value.
func Bool(input bool) Value {
	return valueFromInternal(enginevalue.NewTyped(enginevalue.NewBool(input)))
}

// Integer parses an arbitrary-size base-10 integer.
func Integer(text string) (Value, error) {
	parsed := new(big.Int)
	if text == "" || !isCanonicalInteger(text) {
		return Value{}, fmt.Errorf("%w: integer must be canonical base-10 text", ErrInvalidArgument)
	}
	if _, ok := parsed.SetString(text, 10); !ok {
		return Value{}, fmt.Errorf("%w: integer is invalid", ErrInvalidArgument)
	}
	return valueFromInternal(enginevalue.NewTyped(enginevalue.NewInteger(parsed))), nil
}

// Int64 returns a defined Integer Value.
func Int64(input int64) Value {
	return valueFromInternal(enginevalue.NewTyped(enginevalue.NewInt64(input)))
}

// DecimalValue returns a defined exact Decimal Value.
func DecimalValue(input Decimal) Value {
	return valueFromInternal(enginevalue.NewTyped(enginevalue.NewDecimal(input.value)))
}

// String returns a defined String Value.
func String(input string) Value {
	return valueFromInternal(enginevalue.NewTyped(enginevalue.NewString(input)))
}

// DateValue returns a defined Date Value. It returns an error for an invalid Date.
func DateValue(input Date) (Value, error) {
	if !input.IsValid() {
		return Value{}, fmt.Errorf("%w: Date is invalid", ErrInvalidArgument)
	}
	return valueFromInternal(enginevalue.NewTyped(enginevalue.NewDate(input.value))), nil
}

// DateTimeValue returns a defined DateTime Value. It returns an error for an invalid DateTime.
func DateTimeValue(input DateTime) (Value, error) {
	if !input.IsValid() {
		return Value{}, fmt.Errorf("%w: DateTime is invalid", ErrInvalidArgument)
	}
	return valueFromInternal(enginevalue.NewTyped(enginevalue.NewDateTime(input.value))), nil
}

// DurationValue returns a defined Duration Value.
func DurationValue(input Duration) Value {
	return valueFromInternal(enginevalue.NewTyped(enginevalue.NewDuration(input.value)))
}

// WithUnit returns a copy of v carrying u. The unit's compatibility with a
// target property is validated by normal model resolution.
func (v Value) WithUnit(u Unit) (Value, error) {
	if !v.IsValid() || !u.IsValid() {
		return Value{}, fmt.Errorf("%w: valid Value and Unit are required", ErrInvalidArgument)
	}
	return valueFromInternal(enginevalue.NewTypedWithUnit(v.value.Raw(), u.value)), nil
}

// IsValid reports whether v is a defined typed value.
func (v Value) IsValid() bool { return v.value.IsValid() }

// Kind returns the stable value kind, or an empty string for an invalid Value.
func (v Value) Kind() ValueKind {
	switch v.value.Type() {
	case enginevalue.Bool:
		return ValueBool
	case enginevalue.Integer:
		return ValueInteger
	case enginevalue.Decimal:
		return ValueDecimal
	case enginevalue.String:
		return ValueString
	case enginevalue.Date:
		return ValueDate
	case enginevalue.DateTime:
		return ValueDateTime
	case enginevalue.Duration:
		return ValueDuration
	default:
		return ""
	}
}

// Unit returns v's optional unit.
func (v Value) Unit() (Unit, bool) {
	if !v.IsValid() || !v.value.HasUnit() {
		return Unit{}, false
	}
	return Unit{value: v.value.Unit()}, true
}

// Bool returns the Boolean payload when v is Boolean.
func (v Value) Bool() (bool, bool) {
	if v.Kind() != ValueBool {
		return false, false
	}
	return v.value.Raw().Bool(), true
}

// IntegerString returns the exact base-10 integer text.
func (v Value) IntegerString() (string, bool) {
	if v.Kind() != ValueInteger {
		return "", false
	}
	return v.value.Raw().Integer().String(), true
}

// Decimal returns the exact Decimal payload when v is Decimal.
func (v Value) Decimal() (Decimal, bool) {
	if v.Kind() != ValueDecimal {
		return Decimal{}, false
	}
	return Decimal{value: v.value.Raw().Decimal()}, true
}

// StringValue returns the String payload when v is String.
func (v Value) StringValue() (string, bool) {
	if v.Kind() != ValueString {
		return "", false
	}
	return v.value.Raw().StringValue(), true
}

// Date returns the Date payload when v is Date.
func (v Value) Date() (Date, bool) {
	if v.Kind() != ValueDate {
		return Date{}, false
	}
	return Date{value: v.value.Raw().Date()}, true
}

// DateTime returns the DateTime payload when v is DateTime.
func (v Value) DateTime() (DateTime, bool) {
	if v.Kind() != ValueDateTime {
		return DateTime{}, false
	}
	return DateTime{value: v.value.Raw().DateTime()}, true
}

// Duration returns the Duration payload when v is Duration.
func (v Value) Duration() (Duration, bool) {
	if v.Kind() != ValueDuration {
		return Duration{}, false
	}
	return Duration{value: v.value.Raw().Duration()}, true
}

func valueFromInternal(input enginevalue.Typed) Value { return Value{value: input} }

func (v Value) internal() (enginevalue.Typed, error) {
	if !v.IsValid() {
		return enginevalue.Typed{}, fmt.Errorf("%w: Value is invalid", ErrInvalidArgument)
	}
	return v.value, nil
}

func isCanonicalInteger(text string) bool {
	if text == "0" {
		return true
	}
	start := 0
	if text[0] == '-' {
		if len(text) == 1 || text[1] == '0' {
			return false
		}
		start = 1
	} else if text[0] == '0' {
		return false
	}
	for index := start; index < len(text); index++ {
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	return true
}
