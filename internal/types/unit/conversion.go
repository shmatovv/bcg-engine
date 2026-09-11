package unit

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
)

// Convert converts value expressed in from into the equivalent value
// expressed in to using only exact, context-free unit scaling.
//
// Automatic conversion requires:
//
//   - valid source and target Units;
//   - equal Dimensions;
//   - equal conversion signatures.
//
// Equal Dimension alone is intentionally insufficient.
//
// Examples:
//
//	kg -> g           allowed
//	m -> cm           allowed
//	RUB -> kopeck     allowed
//	RUB -> EUR        rejected
//
// Contextual conversions such as currency exchange are intentionally outside
// this function and require an explicit higher-level rule.
//
// Unit scales are exact Rational values. Conversion is performed exactly:
//
//	result = value * from.scale / to.scale
//
// Rounding occurs only once, when the exact Rational result is converted back
// to Decimal according to policy.
func Convert(
	value decimal.Decimal,
	from Unit,
	to Unit,
	policy precision.Policy,
) (decimal.Decimal, error) {
	if !from.IsValid() || !to.IsValid() {
		return decimal.Decimal{}, fmt.Errorf(
			"unit: conversion requires valid units",
		)
	}

	if !from.dimension.Equal(to.dimension) {
		return decimal.Decimal{}, fmt.Errorf(
			"%w",
			ErrDimensionMismatch,
		)
	}

	if !from.CanConvertTo(to) {
		return decimal.Decimal{}, fmt.Errorf(
			"%w",
			ErrConversionSpaceMismatch,
		)
	}

	valueRational := rational.FromDecimal(value)

	scaled := valueRational.Mul(from.scale)

	result, err := scaled.Div(to.scale)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf(
			"unit: conversion scale division: %w",
			err,
		)
	}

	converted, err := result.Decimal(policy)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf(
			"unit: conversion result: %w",
			err,
		)
	}

	return converted, nil
}
