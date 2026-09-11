package decimal

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalText returns the canonical textual representation of d.
func (d Decimal) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText parses text as a Decimal.
//
// The receiver is modified only if parsing succeeds.
//
// UnmarshalText panics if called on a nil receiver.
func (d *Decimal) UnmarshalText(text []byte) error {
	if d == nil {
		panic("decimal: UnmarshalText called on nil receiver")
	}

	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

// MarshalJSON encodes d as a JSON string containing its canonical decimal
// representation.
//
// Decimal values are encoded as strings rather than JSON numbers to preserve
// arbitrary precision across JSON implementations.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON decodes a Decimal from a JSON string.
//
// JSON null, JSON numbers, and all other non-string JSON values are rejected.
// The string must contain a decimal representation accepted by Parse.
//
// The receiver is modified only if decoding and parsing succeed.
//
// UnmarshalJSON panics if called on a nil receiver.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	if d == nil {
		panic("decimal: UnmarshalJSON called on nil receiver")
	}

	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf("%w: null", ErrInvalidDecimal)
	}

	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("%w: expected JSON string", ErrInvalidDecimal)
	}

	parsed, err := Parse(value)
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}
