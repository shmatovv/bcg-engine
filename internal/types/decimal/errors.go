package decimal

import "errors"

var (
	// ErrInvalidDecimal indicates that a textual representation is not a valid Decimal.
	ErrInvalidDecimal = errors.New("invalid decimal")

	// ErrDivisionByZero indicates an attempt to divide by zero.
	ErrDivisionByZero = errors.New("division by zero")

	// ErrScaleOutOfRange indicates that a requested or resulting scale exceeds MaxScale.
	ErrScaleOutOfRange = errors.New("decimal scale out of range")
)
