package date

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalText returns the canonical YYYY-MM-DD representation of d.
//
// MarshalText returns an error if d is the zero Date because the zero value
// does not represent a valid calendar date.
func (d Date) MarshalText() ([]byte, error) {
	if d.IsZero() {
		return nil, fmt.Errorf(
			"date: cannot marshal zero date",
		)
	}

	return []byte(d.String()), nil
}

// UnmarshalText parses text as a Date.
//
// The input must use canonical YYYY-MM-DD format and represent a valid
// calendar date.
//
// The receiver is modified only if parsing succeeds.
//
// UnmarshalText panics if called on a nil receiver.
func (d *Date) UnmarshalText(text []byte) error {
	if d == nil {
		panic("date: UnmarshalText called on nil Date receiver")
	}

	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

// MarshalJSON encodes d as a JSON string containing its canonical
// YYYY-MM-DD representation.
//
// MarshalJSON returns an error if d is the zero Date.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return nil, fmt.Errorf(
			"date: cannot marshal zero date",
		)
	}

	return json.Marshal(d.String())
}

// UnmarshalJSON decodes a Date from a JSON string.
//
// JSON null and all non-string JSON values are rejected. The string must use
// canonical YYYY-MM-DD format and represent a valid calendar date.
//
// The receiver is modified only if decoding and parsing succeed.
//
// UnmarshalJSON panics if called on a nil receiver.
func (d *Date) UnmarshalJSON(data []byte) error {
	if d == nil {
		panic("date: UnmarshalJSON called on nil Date receiver")
	}

	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf(
			"date: invalid date: null",
		)
	}

	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf(
			"date: invalid date: expected JSON string",
		)
	}

	parsed, err := Parse(value)
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}
