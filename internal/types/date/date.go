package date

import (
	"fmt"
	"strconv"
)

// Date represents a calendar date without a time of day or time zone.
//
// A valid Date uses the proleptic Gregorian calendar and has a year in the
// range 1 through 9999.
//
// The zero value Date{} is not a valid calendar date. It exists as the
// natural Go zero value and can be detected with IsZero.
type Date struct {
	year  int
	month int
	day   int
}

// New returns the calendar date identified by year, month, and day.
//
// The year must be in the range 1 through 9999, the month must be in the
// range 1 through 12, and the day must exist in the specified month.
//
// New panics if the supplied components do not form a valid calendar date.
func New(year, month, day int) Date {
	date, err := newDate(year, month, day)
	if err != nil {
		panic(err)
	}

	return date
}

// Parse parses s as a calendar date in canonical YYYY-MM-DD format.
//
// Parsing is strict. Exactly four year digits, two month digits, two day
// digits, and the two '-' separators are required.
//
// Parse also validates that the resulting date exists in the proleptic
// Gregorian calendar.
//
// Examples of valid input:
//
//	2026-08-26
//	2000-02-29
//	0001-01-01
//
// Examples of invalid input:
//
//	2026-8-26
//	2026-08-1
//	2023-02-29
//	0000-01-01
func Parse(s string) (Date, error) {
	if len(s) != 10 {
		return Date{}, fmt.Errorf(
			"date: invalid date %q: expected YYYY-MM-DD",
			s,
		)
	}

	if s[4] != '-' || s[7] != '-' {
		return Date{}, fmt.Errorf(
			"date: invalid date %q: expected YYYY-MM-DD",
			s,
		)
	}

	if !isASCIIDigits(s[0:4]) ||
		!isASCIIDigits(s[5:7]) ||
		!isASCIIDigits(s[8:10]) {
		return Date{}, fmt.Errorf(
			"date: invalid date %q: expected decimal digits",
			s,
		)
	}

	year, err := strconv.Atoi(s[0:4])
	if err != nil {
		return Date{}, fmt.Errorf(
			"date: invalid date %q: invalid year",
			s,
		)
	}

	month, err := strconv.Atoi(s[5:7])
	if err != nil {
		return Date{}, fmt.Errorf(
			"date: invalid date %q: invalid month",
			s,
		)
	}

	day, err := strconv.Atoi(s[8:10])
	if err != nil {
		return Date{}, fmt.Errorf(
			"date: invalid date %q: invalid day",
			s,
		)
	}

	date, err := newDate(year, month, day)
	if err != nil {
		return Date{}, fmt.Errorf(
			"date: invalid date %q: %w",
			s,
			err,
		)
	}

	return date, nil
}

// Year returns the year component of d.
//
// For the zero Date, Year returns 0.
func (d Date) Year() int {
	return d.year
}

// Month returns the month component of d as an integer in the range 1
// through 12.
//
// For the zero Date, Month returns 0.
func (d Date) Month() int {
	return d.month
}

// Day returns the day-of-month component of d.
//
// For the zero Date, Day returns 0.
func (d Date) Day() int {
	return d.day
}

// IsZero reports whether d is the zero Date.
//
// The zero Date is not a valid calendar date.
func (d Date) IsZero() bool {
	return d.year == 0 &&
		d.month == 0 &&
		d.day == 0
}

// IsValid reports whether d represents a valid calendar date.
//
// Because all Date values are created through validated construction paths,
// every non-zero Date is valid.
func (d Date) IsValid() bool {
	return !d.IsZero()
}

// Equal reports whether d and other represent the same calendar date.
//
// Two zero Date values compare equal.
func (d Date) Equal(other Date) bool {
	return d.year == other.year &&
		d.month == other.month &&
		d.day == other.day
}

// Cmp compares d and other chronologically and returns:
//
//   - -1 if d is before other;
//   - 0 if d and other are equal;
//   - +1 if d is after other.
//
// Comparison is lexicographic by year, month, and day.
//
// The zero Date sorts before every valid Date.
func (d Date) Cmp(other Date) int {
	switch {
	case d.year < other.year:
		return -1
	case d.year > other.year:
		return 1
	case d.month < other.month:
		return -1
	case d.month > other.month:
		return 1
	case d.day < other.day:
		return -1
	case d.day > other.day:
		return 1
	default:
		return 0
	}
}

// String returns the canonical YYYY-MM-DD representation of d.
//
// String returns an empty string for the zero Date because the zero value
// does not represent a valid calendar date.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}

	return fmt.Sprintf(
		"%04d-%02d-%02d",
		d.year,
		d.month,
		d.day,
	)
}

// newDate validates and constructs a Date.
//
// All construction paths for valid Date values pass through this function so
// that calendar invariants are enforced consistently.
func newDate(year, month, day int) (Date, error) {
	if year < 1 || year > 9999 {
		return Date{}, fmt.Errorf(
			"year %d out of range [1, 9999]",
			year,
		)
	}

	if month < 1 || month > 12 {
		return Date{}, fmt.Errorf(
			"month %d out of range [1, 12]",
			month,
		)
	}

	maxDay := daysInMonth(year, month)

	if day < 1 || day > maxDay {
		return Date{}, fmt.Errorf(
			"day %d out of range [1, %d] for %04d-%02d",
			day,
			maxDay,
			year,
			month,
		)
	}

	return Date{
		year:  year,
		month: month,
		day:   day,
	}, nil
}

// daysInMonth returns the number of days in month for the specified year.
//
// year and month are expected to have already been validated.
func daysInMonth(year, month int) int {
	switch month {
	case 2:
		if isLeapYear(year) {
			return 29
		}

		return 28

	case 4, 6, 9, 11:
		return 30

	default:
		return 31
	}
}

// isLeapYear reports whether year is a leap year in the proleptic Gregorian
// calendar.
func isLeapYear(year int) bool {
	return year%400 == 0 ||
		(year%4 == 0 && year%100 != 0)
}

// isASCIIDigits reports whether s is non-empty and contains only ASCII
// decimal digits.
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
