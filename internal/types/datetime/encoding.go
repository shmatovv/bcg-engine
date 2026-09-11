package datetime

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalText returns the canonical UTC representation of dt.
//
// The returned representation is identical to String:
//
//	YYYY-MM-DDTHH:MM:SSZ
//
// or, when fractional seconds are present:
//
//	YYYY-MM-DDTHH:MM:SS.fractionZ
//
// MarshalText returns an error if dt is the zero DateTime because the zero
// value does not represent a valid instant.
func (dt DateTime) MarshalText() ([]byte, error) {
	if dt.IsZero() {
		return nil, fmt.Errorf(
			"datetime: cannot marshal zero datetime",
		)
	}

	return []byte(dt.String()), nil
}

// UnmarshalText parses text as a DateTime.
//
// The input must use one of the formats accepted by Parse. Numeric offsets
// are normalized to UTC.
//
// The receiver is modified only if parsing succeeds.
//
// UnmarshalText panics if called on a nil receiver.
func (dt *DateTime) UnmarshalText(text []byte) error {
	if dt == nil {
		panic("datetime: UnmarshalText called on nil DateTime receiver")
	}

	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}

	*dt = parsed

	return nil
}

// MarshalJSON encodes dt as a JSON string containing its canonical UTC
// representation.
//
// For example:
//
//	"2026-08-26T13:44:00Z"
//	"2026-08-26T13:44:00.123456789Z"
//
// MarshalJSON returns an error if dt is the zero DateTime.
func (dt DateTime) MarshalJSON() ([]byte, error) {
	if dt.IsZero() {
		return nil, fmt.Errorf(
			"datetime: cannot marshal zero datetime",
		)
	}

	return json.Marshal(dt.String())
}

// UnmarshalJSON decodes a DateTime from a JSON string.
//
// The JSON value must be a string containing one of the representations
// accepted by Parse. Numeric offsets are normalized to UTC.
//
// JSON null and all non-string JSON values are rejected.
//
// The receiver is modified only if decoding and parsing succeed.
//
// UnmarshalJSON panics if called on a nil receiver.
func (dt *DateTime) UnmarshalJSON(data []byte) error {
	if dt == nil {
		panic("datetime: UnmarshalJSON called on nil DateTime receiver")
	}

	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf(
			"datetime: invalid datetime: null",
		)
	}

	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf(
			"datetime: invalid datetime: expected JSON string",
		)
	}

	parsed, err := Parse(value)
	if err != nil {
		return err
	}

	*dt = parsed

	return nil
}
