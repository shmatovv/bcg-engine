package typecheck

import "github.com/shmatovv/bcg-engine/v2/internal/types/value"

// IsNumeric reports whether t belongs to the numeric domain supported
// by arithmetic expressions.
func IsNumeric(t value.Type) bool {
	switch t {
	case value.Integer, value.Decimal:
		return true
	default:
		return false
	}
}

// PromoteAdditive returns the result ValueType for addition
// and subtraction.
//
// Rules:
//
//	Integer + Integer -> Integer
//	Integer + Decimal -> Decimal
//	Decimal + Integer -> Decimal
//	Decimal + Decimal -> Decimal
//
// Unit compatibility is intentionally handled separately.
func PromoteAdditive(
	left value.Type,
	right value.Type,
) (value.Type, bool) {
	if !IsNumeric(left) || !IsNumeric(right) {
		return value.Invalid, false
	}

	if left == value.Decimal || right == value.Decimal {
		return value.Decimal, true
	}

	return value.Integer, true
}

// PromoteMultiplicative returns the result ValueType for multiplication.
//
// Rules:
//
//	Integer * Integer -> Integer
//	Integer * Decimal -> Decimal
//	Decimal * Integer -> Decimal
//	Decimal * Decimal -> Decimal
//
// Unit composition is intentionally handled separately.
func PromoteMultiplicative(
	left value.Type,
	right value.Type,
) (value.Type, bool) {
	if !IsNumeric(left) || !IsNumeric(right) {
		return value.Invalid, false
	}

	if left == value.Decimal || right == value.Decimal {
		return value.Decimal, true
	}

	return value.Integer, true
}

// PromoteDivision returns the result ValueType for division.
//
// Division always produces Decimal:
//
//	Integer / Integer -> Decimal
//	Integer / Decimal -> Decimal
//	Decimal / Integer -> Decimal
//	Decimal / Decimal -> Decimal
//
// In particular:
//
//	1 / 2 -> Decimal(0.5)
//
// This avoids integer truncation in business calculations.
//
// Unit composition is intentionally handled separately.
func PromoteDivision(
	left value.Type,
	right value.Type,
) (value.Type, bool) {
	if !IsNumeric(left) || !IsNumeric(right) {
		return value.Invalid, false
	}

	return value.Decimal, true
}
