package datetime

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shmatovv/bcg-engine/v2/internal/types/date"
)

const (
	minHour       = 0
	maxHour       = 23
	minMinute     = 0
	maxMinute     = 59
	minSecond     = 0
	maxSecond     = 59
	minNanosecond = 0
	maxNanosecond = 999_999_999

	minOffsetHour   = 0
	maxOffsetHour   = 23
	minOffsetMinute = 0
	maxOffsetMinute = 59
)

// DateTime represents an instant with nanosecond precision.
//
// DateTime is always stored internally in UTC. Time-zone names and daylight
// saving rules are intentionally not part of the value.
//
// A valid DateTime contains a valid calendar date and a time of day in the
// ranges:
//
//	hour:       0..23
//	minute:     0..59
//	second:     0..59
//	nanosecond: 0..999999999
//
// Leap seconds are not represented.
//
// The zero value DateTime{} is not a valid instant. It exists as the natural
// Go zero value and can be detected with IsZero.
type DateTime struct {
	date       date.Date
	hour       int
	minute     int
	second     int
	nanosecond int
}

// New returns a DateTime from UTC calendar and clock components.
//
// d must be a valid non-zero Date. The remaining components must describe a
// valid time of day.
//
// New panics if any supplied component is invalid.
//
// New interprets all supplied components as UTC. It does not perform any
// time-zone or offset conversion.
func New(
	d date.Date,
	hour int,
	minute int,
	second int,
	nanosecond int,
) DateTime {
	dt, err := newDateTime(
		d,
		hour,
		minute,
		second,
		nanosecond,
	)
	if err != nil {
		panic(err)
	}

	return dt
}

// Parse parses s as a DateTime.
//
// Accepted forms are:
//
//	YYYY-MM-DDTHH:MM:SSZ
//	YYYY-MM-DDTHH:MM:SS.fractionZ
//	YYYY-MM-DDTHH:MM:SS+HH:MM
//	YYYY-MM-DDTHH:MM:SS.fraction+HH:MM
//	YYYY-MM-DDTHH:MM:SS-HH:MM
//	YYYY-MM-DDTHH:MM:SS.fraction-HH:MM
//
// The date and time syntax is strict. The 'T' and 'Z' characters must be
// uppercase. A fractional second, when present, must contain between one and
// nine ASCII decimal digits.
//
// Numeric offsets are applied during parsing and the resulting DateTime is
// normalized to UTC.
//
// For example:
//
//	2026-08-26T16:44:00+03:00
//
// is normalized to:
//
//	2026-08-26T13:44:00Z
//
// Parse rejects values whose UTC normalization would fall outside the
// supported date range 0001-01-01 through 9999-12-31.
//
// Leap seconds are rejected.
func Parse(s string) (DateTime, error) {
	if len(s) < 20 {
		return DateTime{}, invalidFormatError(s)
	}

	if s[4] != '-' ||
		s[7] != '-' ||
		s[10] != 'T' ||
		s[13] != ':' ||
		s[16] != ':' {
		return DateTime{}, invalidFormatError(s)
	}

	if !isASCIIDigits(s[0:4]) ||
		!isASCIIDigits(s[5:7]) ||
		!isASCIIDigits(s[8:10]) ||
		!isASCIIDigits(s[11:13]) ||
		!isASCIIDigits(s[14:16]) ||
		!isASCIIDigits(s[17:19]) {
		return DateTime{}, invalidFormatError(s)
	}

	d, err := date.Parse(s[0:10])
	if err != nil {
		return DateTime{}, fmt.Errorf(
			"datetime: invalid datetime %q: invalid date: %w",
			s,
			err,
		)
	}

	hour, err := parseTwoDigits(s[11:13])
	if err != nil {
		return DateTime{}, invalidFormatError(s)
	}

	minute, err := parseTwoDigits(s[14:16])
	if err != nil {
		return DateTime{}, invalidFormatError(s)
	}

	second, err := parseTwoDigits(s[17:19])
	if err != nil {
		return DateTime{}, invalidFormatError(s)
	}

	if hour < minHour || hour > maxHour {
		return DateTime{}, fmt.Errorf(
			"datetime: invalid datetime %q: hour %d out of range [0, 23]",
			s,
			hour,
		)
	}

	if minute < minMinute || minute > maxMinute {
		return DateTime{}, fmt.Errorf(
			"datetime: invalid datetime %q: minute %d out of range [0, 59]",
			s,
			minute,
		)
	}

	if second < minSecond || second > maxSecond {
		return DateTime{}, fmt.Errorf(
			"datetime: invalid datetime %q: second %d out of range [0, 59]",
			s,
			second,
		)
	}

	position := 19
	nanosecond := 0

	if position < len(s) && s[position] == '.' {
		position++

		fractionStart := position

		for position < len(s) &&
			s[position] >= '0' &&
			s[position] <= '9' {
			position++
		}

		fraction := s[fractionStart:position]

		if len(fraction) == 0 || len(fraction) > 9 {
			return DateTime{}, fmt.Errorf(
				"datetime: invalid datetime %q: fractional second must contain 1 through 9 digits",
				s,
			)
		}

		nanosecond, err = parseFraction(fraction)
		if err != nil {
			return DateTime{}, fmt.Errorf(
				"datetime: invalid datetime %q: invalid fractional second",
				s,
			)
		}
	}

	if position >= len(s) {
		return DateTime{}, fmt.Errorf(
			"datetime: invalid datetime %q: UTC designator or numeric offset required",
			s,
		)
	}

	var offsetSeconds int

	switch s[position] {
	case 'Z':
		position++

	case '+', '-':
		sign := 1

		if s[position] == '-' {
			sign = -1
		}

		position++

		if len(s)-position != 5 {
			return DateTime{}, fmt.Errorf(
				"datetime: invalid datetime %q: expected numeric offset HH:MM",
				s,
			)
		}

		if s[position+2] != ':' {
			return DateTime{}, fmt.Errorf(
				"datetime: invalid datetime %q: expected numeric offset HH:MM",
				s,
			)
		}

		if !isASCIIDigits(s[position:position+2]) ||
			!isASCIIDigits(s[position+3:position+5]) {
			return DateTime{}, fmt.Errorf(
				"datetime: invalid datetime %q: invalid numeric offset",
				s,
			)
		}

		offsetHour, err := parseTwoDigits(
			s[position : position+2],
		)
		if err != nil {
			return DateTime{}, invalidFormatError(s)
		}

		offsetMinute, err := parseTwoDigits(
			s[position+3 : position+5],
		)
		if err != nil {
			return DateTime{}, invalidFormatError(s)
		}

		if offsetHour < minOffsetHour ||
			offsetHour > maxOffsetHour {
			return DateTime{}, fmt.Errorf(
				"datetime: invalid datetime %q: offset hour %d out of range [0, 23]",
				s,
				offsetHour,
			)
		}

		if offsetMinute < minOffsetMinute ||
			offsetMinute > maxOffsetMinute {
			return DateTime{}, fmt.Errorf(
				"datetime: invalid datetime %q: offset minute %d out of range [0, 59]",
				s,
				offsetMinute,
			)
		}

		offsetSeconds = sign * (offsetHour*60*60 +
			offsetMinute*60)

		position += 5

	default:
		return DateTime{}, fmt.Errorf(
			"datetime: invalid datetime %q: UTC designator or numeric offset required",
			s,
		)
	}

	if position != len(s) {
		return DateTime{}, invalidFormatError(s)
	}

	return normalizeToUTC(
		d,
		hour,
		minute,
		second,
		nanosecond,
		offsetSeconds,
	)
}

// Date returns the UTC calendar date component of dt.
//
// For the zero DateTime, Date returns the zero date.Date.
func (dt DateTime) Date() date.Date {
	return dt.date
}

// Hour returns the UTC hour component of dt.
//
// For the zero DateTime, Hour returns 0.
func (dt DateTime) Hour() int {
	return dt.hour
}

// Minute returns the UTC minute component of dt.
//
// For the zero DateTime, Minute returns 0.
func (dt DateTime) Minute() int {
	return dt.minute
}

// Second returns the UTC second component of dt.
//
// For the zero DateTime, Second returns 0.
func (dt DateTime) Second() int {
	return dt.second
}

// Nanosecond returns the fractional-second component of dt.
//
// The returned value is in the range 0 through 999999999 for every valid
// DateTime.
//
// For the zero DateTime, Nanosecond returns 0.
func (dt DateTime) Nanosecond() int {
	return dt.nanosecond
}

// IsZero reports whether dt is the zero DateTime.
//
// The zero DateTime is not a valid instant.
func (dt DateTime) IsZero() bool {
	return dt.date.IsZero() &&
		dt.hour == 0 &&
		dt.minute == 0 &&
		dt.second == 0 &&
		dt.nanosecond == 0
}

// IsValid reports whether dt represents a valid instant.
//
// Because all DateTime values are created through validated construction
// paths, every non-zero DateTime is valid.
func (dt DateTime) IsValid() bool {
	return !dt.IsZero()
}

// Equal reports whether dt and other represent the same instant.
//
// Because all valid DateTime values are normalized to UTC, equality is
// equivalent to equality of all stored components.
//
// Two zero DateTime values compare equal.
func (dt DateTime) Equal(other DateTime) bool {
	return dt.date.Equal(other.date) &&
		dt.hour == other.hour &&
		dt.minute == other.minute &&
		dt.second == other.second &&
		dt.nanosecond == other.nanosecond
}

// Cmp compares dt and other chronologically and returns:
//
//   - -1 if dt is before other;
//   - 0 if dt and other represent the same instant;
//   - +1 if dt is after other.
//
// The zero DateTime sorts before every valid DateTime.
func (dt DateTime) Cmp(other DateTime) int {
	if cmp := dt.date.Cmp(other.date); cmp != 0 {
		return cmp
	}

	switch {
	case dt.hour < other.hour:
		return -1
	case dt.hour > other.hour:
		return 1
	case dt.minute < other.minute:
		return -1
	case dt.minute > other.minute:
		return 1
	case dt.second < other.second:
		return -1
	case dt.second > other.second:
		return 1
	case dt.nanosecond < other.nanosecond:
		return -1
	case dt.nanosecond > other.nanosecond:
		return 1
	default:
		return 0
	}
}

// String returns the canonical UTC representation of dt.
//
// The canonical form always uses the uppercase UTC designator Z:
//
//	YYYY-MM-DDTHH:MM:SSZ
//
// If nanoseconds are present, String emits the shortest exact fractional
// representation by removing trailing zeroes:
//
//	2026-08-26T13:44:00.1Z
//	2026-08-26T13:44:00.123456Z
//	2026-08-26T13:44:00.123456789Z
//
// String returns an empty string for the zero DateTime.
func (dt DateTime) String() string {
	if dt.IsZero() {
		return ""
	}

	base := fmt.Sprintf(
		"%sT%02d:%02d:%02d",
		dt.date.String(),
		dt.hour,
		dt.minute,
		dt.second,
	)

	if dt.nanosecond == 0 {
		return base + "Z"
	}

	fraction := fmt.Sprintf(
		"%09d",
		dt.nanosecond,
	)

	fraction = strings.TrimRight(fraction, "0")

	return base + "." + fraction + "Z"
}

// newDateTime validates and constructs an already-normalized UTC DateTime.
//
// All direct construction paths for DateTime values pass through this
// function so that the representation invariants are enforced consistently.
func newDateTime(
	d date.Date,
	hour int,
	minute int,
	second int,
	nanosecond int,
) (DateTime, error) {
	if d.IsZero() {
		return DateTime{}, fmt.Errorf(
			"datetime: date must be valid and non-zero",
		)
	}

	if hour < minHour || hour > maxHour {
		return DateTime{}, fmt.Errorf(
			"datetime: hour %d out of range [0, 23]",
			hour,
		)
	}

	if minute < minMinute || minute > maxMinute {
		return DateTime{}, fmt.Errorf(
			"datetime: minute %d out of range [0, 59]",
			minute,
		)
	}

	if second < minSecond || second > maxSecond {
		return DateTime{}, fmt.Errorf(
			"datetime: second %d out of range [0, 59]",
			second,
		)
	}

	if nanosecond < minNanosecond ||
		nanosecond > maxNanosecond {
		return DateTime{}, fmt.Errorf(
			"datetime: nanosecond %d out of range [0, 999999999]",
			nanosecond,
		)
	}

	return DateTime{
		date:       d,
		hour:       hour,
		minute:     minute,
		second:     second,
		nanosecond: nanosecond,
	}, nil
}

// normalizeToUTC applies offsetSeconds to the supplied local representation
// and returns the corresponding UTC DateTime.
//
// offsetSeconds follows the conventional numeric-offset meaning:
//
//	local = UTC + offset
//
// Therefore:
//
//	UTC = local - offset
//
// time.Date is used only as a deterministic Gregorian calendar arithmetic
// primitive. No local time zone, named time zone, DST rule, or external
// time-zone database participates in the conversion.
func normalizeToUTC(
	d date.Date,
	hour int,
	minute int,
	second int,
	nanosecond int,
	offsetSeconds int,
) (DateTime, error) {
	local := time.Date(
		d.Year(),
		time.Month(d.Month()),
		d.Day(),
		hour,
		minute,
		second,
		nanosecond,
		time.UTC,
	)

	utc := local.Add(
		-time.Duration(offsetSeconds) * time.Second,
	)

	year := utc.Year()

	if year < 1 || year > 9999 {
		return DateTime{}, fmt.Errorf(
			"datetime: UTC normalization produces year %d outside supported range [1, 9999]",
			year,
		)
	}

	utcDate := date.New(
		year,
		int(utc.Month()),
		utc.Day(),
	)

	return newDateTime(
		utcDate,
		utc.Hour(),
		utc.Minute(),
		utc.Second(),
		utc.Nanosecond(),
	)
}

// parseTwoDigits parses exactly two ASCII decimal digits.
func parseTwoDigits(s string) (int, error) {
	if len(s) != 2 || !isASCIIDigits(s) {
		return 0, fmt.Errorf(
			"datetime: expected two decimal digits",
		)
	}

	value, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf(
			"datetime: invalid decimal digits: %w",
			err,
		)
	}

	return value, nil
}

// parseFraction converts one through nine fractional-second digits into
// nanoseconds.
//
// For example:
//
//	"1"         -> 100000000
//	"01"        -> 10000000
//	"123456"    -> 123456000
//	"123456789" -> 123456789
func parseFraction(s string) (int, error) {
	if len(s) < 1 || len(s) > 9 || !isASCIIDigits(s) {
		return 0, fmt.Errorf(
			"datetime: invalid fractional second",
		)
	}

	value, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf(
			"datetime: invalid fractional second: %w",
			err,
		)
	}

	for i := len(s); i < 9; i++ {
		value *= 10
	}

	return value, nil
}

// isASCIIDigits reports whether s is non-empty and consists exclusively of
// ASCII decimal digits.
func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}

	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

// invalidFormatError returns the common error used for structurally invalid
// DateTime input.
func invalidFormatError(s string) error {
	return fmt.Errorf(
		"datetime: invalid datetime %q: expected YYYY-MM-DDTHH:MM:SS[.fraction](Z|+HH:MM|-HH:MM)",
		s,
	)
}
