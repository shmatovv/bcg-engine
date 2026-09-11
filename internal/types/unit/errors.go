package unit

import "errors"

var (
	// ErrDimensionMismatch reports that two Units have incompatible
	// dimensional meanings.
	ErrDimensionMismatch = errors.New(
		"unit: dimension mismatch",
	)

	// ErrConversionSpaceMismatch reports that two dimensionally compatible
	// Units cannot be converted using exact, context-free static scaling.
	//
	// Such conversion may require an explicit higher-level contextual rule.
	ErrConversionSpaceMismatch = errors.New(
		"unit: automatic conversion is not allowed between conversion spaces",
	)
)

// ErrExponentOverflow identifies unrepresentable dimension or unit exponents.
// Direct unit algebra panics with this error; type inference reports failure.
var ErrExponentOverflow = errors.New("unit: dimension exponent overflow")
