// Package duration provides an exact arbitrary-range elapsed-duration type.
//
// A Duration represents a signed amount of elapsed time with nanosecond
// resolution. It does not represent a calendar period.
//
// In particular, Duration has no concepts such as months or years because
// those units do not have a fixed elapsed length without calendar context.
//
// Duration is immutable from the caller's point of view and supports values
// outside the range of time.Duration.
package duration

import (
	"fmt"
	"math/big"
	"time"
)

var (
	nanosecondsPerMicrosecond = big.NewInt(1_000)
	nanosecondsPerMillisecond = big.NewInt(1_000_000)
	nanosecondsPerSecond      = big.NewInt(1_000_000_000)
	nanosecondsPerMinute      = big.NewInt(60_000_000_000)
	nanosecondsPerHour        = big.NewInt(3_600_000_000_000)
	nanosecondsPerDay         = big.NewInt(86_400_000_000_000)
)

// Duration represents an exact signed amount of elapsed time.
//
// Internally, the value is stored as an arbitrary-precision number of
// nanoseconds.
//
// The zero value of Duration is a valid zero duration.
type Duration struct {
	nanoseconds big.Int
}

// Zero returns the zero duration.
func Zero() Duration {
	return Duration{}
}

// FromNanoseconds constructs a Duration from nanoseconds.
//
// The input is copied. Subsequent modification of nanoseconds does not affect
// the returned Duration.
//
// FromNanoseconds panics if nanoseconds is nil.
func FromNanoseconds(nanoseconds *big.Int) Duration {
	if nanoseconds == nil {
		panic("duration: nil nanoseconds")
	}

	var value big.Int
	value.Set(nanoseconds)

	return Duration{
		nanoseconds: value,
	}
}

// FromInt64Nanoseconds constructs a Duration from an int64 number of
// nanoseconds.
func FromInt64Nanoseconds(nanoseconds int64) Duration {
	var value big.Int
	value.SetInt64(nanoseconds)

	return Duration{
		nanoseconds: value,
	}
}

// FromMicroseconds constructs a Duration from an int64 number of
// microseconds.
func FromMicroseconds(microseconds int64) Duration {
	var value big.Int
	value.SetInt64(microseconds)
	value.Mul(&value, nanosecondsPerMicrosecond)

	return Duration{
		nanoseconds: value,
	}
}

// FromMilliseconds constructs a Duration from an int64 number of
// milliseconds.
func FromMilliseconds(milliseconds int64) Duration {
	var value big.Int
	value.SetInt64(milliseconds)
	value.Mul(&value, nanosecondsPerMillisecond)

	return Duration{
		nanoseconds: value,
	}
}

// FromSeconds constructs a Duration from an int64 number of seconds.
func FromSeconds(seconds int64) Duration {
	var value big.Int
	value.SetInt64(seconds)
	value.Mul(&value, nanosecondsPerSecond)

	return Duration{
		nanoseconds: value,
	}
}

// FromMinutes constructs a Duration from an int64 number of minutes.
func FromMinutes(minutes int64) Duration {
	var value big.Int
	value.SetInt64(minutes)
	value.Mul(&value, nanosecondsPerMinute)

	return Duration{
		nanoseconds: value,
	}
}

// FromHours constructs a Duration from an int64 number of hours.
func FromHours(hours int64) Duration {
	var value big.Int
	value.SetInt64(hours)
	value.Mul(&value, nanosecondsPerHour)

	return Duration{
		nanoseconds: value,
	}
}

// FromDays constructs a Duration from an int64 number of fixed 24-hour days.
//
// A day here means exactly 24 hours. It is not a calendar day and therefore
// does not account for civil-time transitions such as daylight-saving time.
func FromDays(days int64) Duration {
	var value big.Int
	value.SetInt64(days)
	value.Mul(&value, nanosecondsPerDay)

	return Duration{
		nanoseconds: value,
	}
}

// FromTimeDuration constructs a Duration from a time.Duration.
//
// The conversion is exact because time.Duration itself is represented as an
// integral number of nanoseconds.
func FromTimeDuration(value time.Duration) Duration {
	return FromInt64Nanoseconds(int64(value))
}

// Nanoseconds returns the exact number of nanoseconds represented by d.
//
// The returned big.Int is a copy. Modifying it does not modify d.
func (d Duration) Nanoseconds() *big.Int {
	return new(big.Int).Set(&d.nanoseconds)
}

// Sign returns:
//
//	-1 if d < 0
//	 0 if d = 0
//	+1 if d > 0
func (d Duration) Sign() int {
	return d.nanoseconds.Sign()
}

// IsZero reports whether d represents zero elapsed time.
func (d Duration) IsZero() bool {
	return d.nanoseconds.Sign() == 0
}

// IsPositive reports whether d is greater than zero.
func (d Duration) IsPositive() bool {
	return d.nanoseconds.Sign() > 0
}

// IsNegative reports whether d is less than zero.
func (d Duration) IsNegative() bool {
	return d.nanoseconds.Sign() < 0
}

// Equal reports whether d and other represent the same duration.
func (d Duration) Equal(other Duration) bool {
	return d.nanoseconds.Cmp(&other.nanoseconds) == 0
}

// Cmp compares d and other.
//
// It returns:
//
//	-1 if d < other
//	 0 if d = other
//	+1 if d > other
func (d Duration) Cmp(other Duration) int {
	return d.nanoseconds.Cmp(&other.nanoseconds)
}

// Neg returns -d.
func (d Duration) Neg() Duration {
	var value big.Int
	value.Neg(&d.nanoseconds)

	return Duration{
		nanoseconds: value,
	}
}

// Abs returns the absolute value of d.
func (d Duration) Abs() Duration {
	var value big.Int
	value.Abs(&d.nanoseconds)

	return Duration{
		nanoseconds: value,
	}
}

// Add returns d + other.
//
// Addition is exact and cannot overflow.
func (d Duration) Add(other Duration) Duration {
	var value big.Int
	value.Add(&d.nanoseconds, &other.nanoseconds)

	return Duration{
		nanoseconds: value,
	}
}

// Sub returns d - other.
//
// Subtraction is exact and cannot overflow.
func (d Duration) Sub(other Duration) Duration {
	var value big.Int
	value.Sub(&d.nanoseconds, &other.nanoseconds)

	return Duration{
		nanoseconds: value,
	}
}

// MulInt64 returns d multiplied by factor.
//
// Multiplication is exact and cannot overflow.
func (d Duration) MulInt64(factor int64) Duration {
	var multiplier big.Int
	multiplier.SetInt64(factor)

	var value big.Int
	value.Mul(&d.nanoseconds, &multiplier)

	return Duration{
		nanoseconds: value,
	}
}

// ToTimeDuration converts d to time.Duration.
//
// The conversion succeeds only when d is representable by time.Duration.
// Otherwise an error is returned.
func (d Duration) ToTimeDuration() (time.Duration, error) {
	if !d.nanoseconds.IsInt64() {
		return 0, fmt.Errorf(
			"duration: value %s ns is outside time.Duration range",
			d.nanoseconds.String(),
		)
	}

	return time.Duration(d.nanoseconds.Int64()), nil
}
