package duration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

var (
	bigTen              = big.NewInt(10)
	bigSecondsPerMinute = big.NewInt(60)
	bigMinutesPerHour   = big.NewInt(60)
)

// String returns the canonical textual representation of d.
//
// The format is a strict elapsed-time subset inspired by ISO 8601:
//
//	PT0S
//	PT1S
//	PT1.5S
//	PT1M
//	PT1H
//	PT1H30M
//	PT1H30M0.5S
//	-PT1H30M
//
// Calendar units such as days, months, and years are intentionally not used.
//
// The representation is canonical. For example:
//
//	PT00H       -> PT0S
//	PT01H       -> PT1H
//	PT1H00M     -> PT1H
//	PT0.100S    -> PT0.1S
//	-PT0S       -> PT0S
//
// String is defined for every Duration, including the zero value.
func (d Duration) String() string {
	if d.IsZero() {
		return "PT0S"
	}

	negative := d.Sign() < 0

	var absolute big.Int
	absolute.Abs(&d.nanoseconds)

	var totalSeconds big.Int
	var nanoseconds big.Int

	totalSeconds.QuoRem(
		&absolute,
		nanosecondsPerSecond,
		&nanoseconds,
	)

	var totalMinutes big.Int
	var seconds big.Int

	totalMinutes.QuoRem(
		&totalSeconds,
		bigSecondsPerMinute,
		&seconds,
	)

	var hours big.Int
	var minutes big.Int

	hours.QuoRem(
		&totalMinutes,
		bigMinutesPerHour,
		&minutes,
	)

	var builder strings.Builder

	if negative {
		builder.WriteByte('-')
	}

	builder.WriteString("PT")

	if hours.Sign() != 0 {
		builder.WriteString(hours.String())
		builder.WriteByte('H')
	}

	if minutes.Sign() != 0 {
		builder.WriteString(minutes.String())
		builder.WriteByte('M')
	}

	if seconds.Sign() != 0 || nanoseconds.Sign() != 0 {
		builder.WriteString(seconds.String())

		if nanoseconds.Sign() != 0 {
			builder.WriteByte('.')

			fraction := fmt.Sprintf(
				"%09d",
				nanoseconds.Int64(),
			)

			fraction = strings.TrimRight(fraction, "0")

			builder.WriteString(fraction)
		}

		builder.WriteByte('S')
	}

	return builder.String()
}

// Parse parses text as a Duration.
//
// Accepted syntax:
//
//	[sign]PT[hoursH][minutesM][seconds[.fraction]S]
//
// where:
//
//	sign     = optional '-'
//	hours    = one or more decimal digits
//	minutes  = decimal integer in [0, 59]
//	seconds  = decimal integer in [0, 59]
//	fraction = 1 to 9 decimal digits
//
// Components must appear in H, M, S order and may appear at most once.
// At least one of H, M, or S must be present.
//
// Parsing is strict. Whitespace, a leading '+', lowercase designators,
// calendar units, scientific notation, and more than nine fractional digits
// are rejected.
//
// Non-canonical but semantically valid representations are accepted and
// normalized by String. For example:
//
//	PT00H       -> PT0S
//	PT01H       -> PT1H
//	PT1H00M     -> PT1H
//	PT0.100S    -> PT0.1S
//	-PT0S       -> PT0S
//
// Minutes and seconds are components and therefore must be in [0, 59].
// For example, PT60M and PT60S are rejected.
//
// Duration parsing is exact and does not use floating point.
func Parse(text string) (Duration, error) {
	original := text

	if text == "" {
		return Duration{}, invalidDurationError(original)
	}

	negative := false

	if text[0] == '-' {
		negative = true
		text = text[1:]

		if text == "" {
			return Duration{}, invalidDurationError(original)
		}
	}

	if len(text) < 3 || text[0] != 'P' || text[1] != 'T' {
		return Duration{}, invalidDurationError(original)
	}

	body := text[2:]

	if body == "" {
		return Duration{}, invalidDurationError(original)
	}

	var total big.Int

	seenHour := false
	seenMinute := false
	seenSecond := false

	position := 0

	for position < len(body) {
		start := position

		for position < len(body) && isDecimalDigit(body[position]) {
			position++
		}

		if start == position {
			return Duration{}, invalidDurationError(original)
		}

		integerPart := body[start:position]

		if position >= len(body) {
			return Duration{}, invalidDurationError(original)
		}

		switch body[position] {
		case 'H':
			if seenHour || seenMinute || seenSecond {
				return Duration{}, invalidDurationError(original)
			}

			hours, ok := new(big.Int).SetString(integerPart, 10)
			if !ok {
				return Duration{}, invalidDurationError(original)
			}

			var component big.Int
			component.Mul(hours, nanosecondsPerHour)

			total.Add(&total, &component)

			seenHour = true
			position++

		case 'M':
			if seenMinute || seenSecond {
				return Duration{}, invalidDurationError(original)
			}

			minutes, ok := new(big.Int).SetString(integerPart, 10)
			if !ok {
				return Duration{}, invalidDurationError(original)
			}

			if minutes.Cmp(bigSecondsPerMinute) >= 0 {
				return Duration{}, fmt.Errorf(
					"duration: minutes out of range in %q",
					original,
				)
			}

			var component big.Int
			component.Mul(minutes, nanosecondsPerMinute)

			total.Add(&total, &component)

			seenMinute = true
			position++

		case 'S':
			if seenSecond {
				return Duration{}, invalidDurationError(original)
			}

			seconds, ok := new(big.Int).SetString(integerPart, 10)
			if !ok {
				return Duration{}, invalidDurationError(original)
			}

			if seconds.Cmp(bigSecondsPerMinute) >= 0 {
				return Duration{}, fmt.Errorf(
					"duration: seconds out of range in %q",
					original,
				)
			}

			var component big.Int
			component.Mul(seconds, nanosecondsPerSecond)

			total.Add(&total, &component)

			seenSecond = true
			position++

		case '.':
			if seenSecond {
				return Duration{}, invalidDurationError(original)
			}

			seconds, ok := new(big.Int).SetString(integerPart, 10)
			if !ok {
				return Duration{}, invalidDurationError(original)
			}

			if seconds.Cmp(bigSecondsPerMinute) >= 0 {
				return Duration{}, fmt.Errorf(
					"duration: seconds out of range in %q",
					original,
				)
			}

			position++

			fractionStart := position

			for position < len(body) && isDecimalDigit(body[position]) {
				position++
			}

			fractionLength := position - fractionStart

			if fractionLength < 1 || fractionLength > 9 {
				return Duration{}, fmt.Errorf(
					"duration: invalid fractional seconds in %q",
					original,
				)
			}

			if position >= len(body) || body[position] != 'S' {
				return Duration{}, invalidDurationError(original)
			}

			fractionText := body[fractionStart:position]

			fraction, ok := new(big.Int).SetString(fractionText, 10)
			if !ok {
				return Duration{}, invalidDurationError(original)
			}

			if fractionLength < 9 {
				var exponent big.Int
				exponent.SetInt64(int64(9 - fractionLength))

				var multiplier big.Int
				multiplier.Exp(bigTen, &exponent, nil)

				fraction.Mul(fraction, &multiplier)
			}

			var secondComponent big.Int
			secondComponent.Mul(
				seconds,
				nanosecondsPerSecond,
			)

			total.Add(&total, &secondComponent)
			total.Add(&total, fraction)

			seenSecond = true
			position++

		default:
			return Duration{}, invalidDurationError(original)
		}
	}

	if !seenHour && !seenMinute && !seenSecond {
		return Duration{}, invalidDurationError(original)
	}

	if negative {
		total.Neg(&total)
	}

	return FromNanoseconds(&total), nil
}

// MustParse parses text as a Duration.
//
// It panics if text is not a valid Duration representation.
func MustParse(text string) Duration {
	value, err := Parse(text)
	if err != nil {
		panic(err)
	}

	return value
}

// MarshalText returns the canonical textual representation of d.
//
// Every Duration, including zero, has a valid textual representation.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText parses text as a Duration.
//
// The receiver is modified only if parsing succeeds.
//
// UnmarshalText panics if called on a nil receiver.
func (d *Duration) UnmarshalText(text []byte) error {
	if d == nil {
		panic("duration: UnmarshalText called on nil Duration receiver")
	}

	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

// MarshalJSON encodes d as a JSON string containing its canonical textual
// representation.
//
// For example:
//
//	"PT0S"
//	"PT1H30M"
//	"-PT0.5S"
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON decodes a Duration from a JSON string.
//
// JSON null and all non-string JSON values are rejected.
//
// The receiver is modified only if decoding and parsing succeed.
//
// UnmarshalJSON panics if called on a nil receiver.
func (d *Duration) UnmarshalJSON(data []byte) error {
	if d == nil {
		panic("duration: UnmarshalJSON called on nil Duration receiver")
	}

	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf(
			"duration: invalid duration: null",
		)
	}

	var text string

	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf(
			"duration: invalid duration: expected JSON string",
		)
	}

	parsed, err := Parse(text)
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

// isDecimalDigit reports whether c is an ASCII decimal digit.
func isDecimalDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// invalidDurationError returns the standard parse error for an invalid
// Duration representation.
func invalidDurationError(text string) error {
	return fmt.Errorf(
		"duration: invalid duration %q",
		text,
	)
}
