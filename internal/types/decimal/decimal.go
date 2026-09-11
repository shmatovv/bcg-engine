package decimal

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/shmatovv/bcg-engine/internal/types/precision"
)

// MaxScale is the maximum number of fractional decimal digits supported by Decimal.
const MaxScale uint64 = 10_000

// Decimal represents an arbitrary-precision decimal number.
//
// A Decimal is represented internally by an integer coefficient and a scale.
// Its numeric value is:
//
//	coefficient * 10^(-scale)
//
// Decimal values are normalized, so unnecessary trailing fractional zeros are
// removed. The zero value of Decimal is valid and represents zero.
//
// Decimal is designed to be used as an immutable value. Its arithmetic methods
// do not modify their operands and instead return new Decimal values.
type Decimal struct {
	coefficient big.Int
	scale       uint64
}

// New creates a Decimal from coefficient and scale.
//
// The resulting Decimal is normalized by removing trailing decimal zeros.
//
// New panics if coefficient is nil or scale exceeds MaxScale.
// The supplied coefficient is copied and is not retained by the Decimal.
func New(coefficient *big.Int, scale uint64) Decimal {
	if coefficient == nil {
		panic("decimal: nil coefficient")
	}

	if scale > MaxScale {
		panic("decimal: scale out of range")
	}

	normalizedCoefficient, normalizedScale := normalize(coefficient, scale)

	return Decimal{
		coefficient: normalizedCoefficient,
		scale:       normalizedScale,
	}
}

// FromInt64 returns a Decimal representing value.
func FromInt64(value int64) Decimal {
	return New(big.NewInt(value), 0)
}

// FromUint64 returns a Decimal representing value.
func FromUint64(value uint64) Decimal {
	var coefficient big.Int
	coefficient.SetUint64(value)

	return New(&coefficient, 0)
}

// Zero returns a Decimal representing zero.
func Zero() Decimal {
	return Decimal{}
}

// One returns a Decimal representing one.
func One() Decimal {
	return FromInt64(1)
}

// Parse parses s as a decimal number.
//
// Parse accepts an optional leading '+' or '-' sign followed by decimal digits
// and an optional fractional part separated by a decimal point. Digits are
// required on both sides of the decimal point when one is present.
//
// Parse does not accept whitespace, scientific notation, NaN, infinity, or
// locale-specific decimal separators.
//
// The returned Decimal is normalized.
//
// Parse returns ErrInvalidDecimal if s is malformed and ErrScaleOutOfRange if
// its fractional scale exceeds MaxScale.
func Parse(s string) (Decimal, error) {
	if s == "" {
		return Decimal{}, fmt.Errorf("%w: empty string", ErrInvalidDecimal)
	}

	negative := false

	switch s[0] {
	case '-':
		negative = true
		s = s[1:]
	case '+':
		s = s[1:]
	}

	if s == "" {
		return Decimal{}, fmt.Errorf("%w: missing digits", ErrInvalidDecimal)
	}

	if strings.Count(s, ".") > 1 {
		return Decimal{}, fmt.Errorf(
			"%w: multiple decimal points",
			ErrInvalidDecimal,
		)
	}

	integerPart := s
	fractionalPart := ""

	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		integerPart = s[:dot]
		fractionalPart = s[dot+1:]

		if integerPart == "" || fractionalPart == "" {
			return Decimal{}, fmt.Errorf(
				"%w: digits required on both sides of decimal point",
				ErrInvalidDecimal,
			)
		}
	}

	if !isDigits(integerPart) || !isDigits(fractionalPart) {
		return Decimal{}, fmt.Errorf(
			"%w: contains non-decimal characters",
			ErrInvalidDecimal,
		)
	}

	if uint64(len(fractionalPart)) > MaxScale {
		return Decimal{}, fmt.Errorf(
			"%w: scale %d exceeds maximum %d",
			ErrScaleOutOfRange,
			len(fractionalPart),
			MaxScale,
		)
	}

	digits := integerPart + fractionalPart

	coefficient := new(big.Int)
	if _, ok := coefficient.SetString(digits, 10); !ok {
		return Decimal{}, fmt.Errorf(
			"%w: invalid coefficient",
			ErrInvalidDecimal,
		)
	}

	if negative {
		coefficient.Neg(coefficient)
	}

	return New(coefficient, uint64(len(fractionalPart))), nil
}

// MustParse parses s as a decimal number.
//
// MustParse panics if s cannot be parsed by Parse.
func MustParse(s string) Decimal {
	d, err := Parse(s)
	if err != nil {
		panic(fmt.Errorf("decimal: cannot parse %q: %w", s, err))
	}

	return d
}

// String returns the canonical decimal representation of d.
//
// The returned representation does not use scientific notation and contains
// no unnecessary trailing fractional zeros.
func (d Decimal) String() string {
	if d.coefficient.Sign() == 0 {
		return "0"
	}

	negative := d.coefficient.Sign() < 0

	var abs big.Int
	abs.Abs(&d.coefficient)

	digits := abs.String()
	scale := int(d.scale)

	if scale == 0 {
		if negative {
			return "-" + digits
		}

		return digits
	}

	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}

	point := len(digits) - scale
	result := digits[:point] + "." + digits[point:]

	if negative {
		result = "-" + result
	}

	return result
}

// Scale returns the normalized number of fractional decimal digits in d.
func (d Decimal) Scale() uint64 {
	return d.scale
}

// Coefficient returns a copy of the normalized integer coefficient of d.
//
// Modifying the returned big.Int does not modify d.
func (d Decimal) Coefficient() *big.Int {
	var coefficient big.Int
	coefficient.Set(&d.coefficient)

	return &coefficient
}

// Sign returns -1 if d is negative, 0 if d is zero, and +1 if d is positive.
func (d Decimal) Sign() int {
	return d.coefficient.Sign()
}

// IsZero reports whether d represents zero.
func (d Decimal) IsZero() bool {
	return d.coefficient.Sign() == 0
}

// Equal reports whether d and other represent the same numeric value.
func (d Decimal) Equal(other Decimal) bool {
	return d.Cmp(other) == 0
}

// Cmp compares d and other.
//
// It returns -1 if d < other, 0 if d == other, and +1 if d > other.
func (d Decimal) Cmp(other Decimal) int {
	left, right, _ := align(d, other)

	return left.Cmp(&right)
}

// Neg returns the negation of d.
func (d Decimal) Neg() Decimal {
	var coefficient big.Int
	coefficient.Neg(&d.coefficient)

	return New(&coefficient, d.scale)
}

// Abs returns the absolute value of d.
func (d Decimal) Abs() Decimal {
	var coefficient big.Int
	coefficient.Abs(&d.coefficient)

	return New(&coefficient, d.scale)
}

// Add returns the exact sum of d and other.
func (d Decimal) Add(other Decimal) Decimal {
	left, right, scale := align(d, other)

	var coefficient big.Int
	coefficient.Add(&left, &right)

	return New(&coefficient, scale)
}

// Sub returns the exact difference d - other.
func (d Decimal) Sub(other Decimal) Decimal {
	left, right, scale := align(d, other)

	var coefficient big.Int
	coefficient.Sub(&left, &right)

	return New(&coefficient, scale)
}

// Mul returns the exact product of d and other.
//
// Mul returns ErrScaleOutOfRange if the scale required by the product exceeds
// MaxScale.
func (d Decimal) Mul(other Decimal) (Decimal, error) {
	var coefficient big.Int
	coefficient.Mul(&d.coefficient, &other.coefficient)
	// Operand scales are bounded; normalize before checking the result limit.
	normalized, scale := normalize(&coefficient, d.scale+other.scale)
	if scale > MaxScale {
		return Decimal{}, fmt.Errorf(
			"%w: multiplication requires scale %d",
			ErrScaleOutOfRange,
			scale,
		)
	}

	return Decimal{coefficient: normalized, scale: scale}, nil
}

// Div divides d by other according to policy.
//
// The scale of the result is determined by policy. If the exact result cannot
// be represented at that scale, the configured rounding mode is applied.
//
// Div returns ErrDivisionByZero if other is zero and ErrScaleOutOfRange if the
// requested scale exceeds MaxScale.
//
// Div panics if policy contains an invalid rounding mode.
func (d Decimal) Div(
	other Decimal,
	policy precision.Policy,
) (Decimal, error) {
	validatePolicy(policy)

	if other.IsZero() {
		return Decimal{}, ErrDivisionByZero
	}

	if policy.Scale() > MaxScale {
		return Decimal{}, fmt.Errorf(
			"%w: requested scale %d exceeds maximum %d",
			ErrScaleOutOfRange,
			policy.Scale(),
			MaxScale,
		)
	}

	targetScale := policy.Scale()

	var numerator big.Int
	numerator.Set(&d.coefficient)

	var denominator big.Int
	denominator.Set(&other.coefficient)

	if d.scale <= targetScale {
		power := other.scale + (targetScale - d.scale)

		factor := pow10(power)
		numerator.Mul(&numerator, &factor)
	} else {
		difference := d.scale - targetScale

		if other.scale >= difference {
			power := other.scale - difference

			factor := pow10(power)
			numerator.Mul(&numerator, &factor)
		} else {
			power := difference - other.scale

			factor := pow10(power)
			denominator.Mul(&denominator, &factor)
		}
	}

	var quotient, remainder big.Int
	quotient.QuoRem(&numerator, &denominator, &remainder)

	if remainder.Sign() != 0 {
		roundQuotient(
			&quotient,
			&remainder,
			&denominator,
			policy.Rounding(),
		)
	}

	return New(&quotient, targetScale), nil
}

// Round rounds d according to policy.
//
// If d has more fractional digits than the scale requested by policy, the
// configured rounding mode is applied. If d already has a scale less than or
// equal to the requested scale, d is returned unchanged.
//
// Round returns ErrScaleOutOfRange if the requested scale exceeds MaxScale.
//
// Round panics if policy contains an invalid rounding mode.
func (d Decimal) Round(policy precision.Policy) (Decimal, error) {
	validatePolicy(policy)

	if policy.Scale() > MaxScale {
		return Decimal{}, fmt.Errorf(
			"%w: requested scale %d exceeds maximum %d",
			ErrScaleOutOfRange,
			policy.Scale(),
			MaxScale,
		)
	}

	targetScale := policy.Scale()

	if d.scale <= targetScale {
		return d, nil
	}

	drop := d.scale - targetScale
	divisor := pow10(drop)

	var quotient, remainder big.Int
	quotient.QuoRem(&d.coefficient, &divisor, &remainder)

	if remainder.Sign() != 0 {
		roundQuotient(
			&quotient,
			&remainder,
			&divisor,
			policy.Rounding(),
		)
	}

	return New(&quotient, targetScale), nil
}

func normalize(
	coefficient *big.Int,
	scale uint64,
) (big.Int, uint64) {
	var result big.Int
	result.Set(coefficient)

	if result.Sign() == 0 {
		return result, 0
	}

	var ten big.Int
	ten.SetInt64(10)

	var remainder big.Int

	for scale > 0 {
		remainder.Mod(&result, &ten)

		if remainder.Sign() != 0 {
			break
		}

		result.Div(&result, &ten)
		scale--
	}

	return result, scale
}

func align(a, b Decimal) (big.Int, big.Int, uint64) {
	var left, right big.Int

	left.Set(&a.coefficient)
	right.Set(&b.coefficient)

	if a.scale == b.scale {
		return left, right, a.scale
	}

	if a.scale < b.scale {
		factor := pow10(b.scale - a.scale)
		left.Mul(&left, &factor)

		return left, right, b.scale
	}

	factor := pow10(a.scale - b.scale)
	right.Mul(&right, &factor)

	return left, right, a.scale
}

func pow10(n uint64) big.Int {
	var result big.Int
	var base big.Int
	var exponent big.Int

	base.SetInt64(10)
	exponent.SetUint64(n)

	result.Exp(&base, &exponent, nil)

	return result
}

func roundQuotient(
	quotient, remainder, divisor *big.Int,
	mode precision.Mode,
) {
	sign := remainder.Sign() * divisor.Sign()

	switch mode {
	case precision.Truncate:
		return

	case precision.Floor:
		if sign < 0 {
			var one big.Int
			one.SetInt64(1)
			quotient.Sub(quotient, &one)
		}

	case precision.Ceil:
		if sign > 0 {
			var one big.Int
			one.SetInt64(1)
			quotient.Add(quotient, &one)
		}

	case precision.HalfUp:
		if shouldRoundHalfUp(remainder, divisor) {
			roundAwayFromZero(quotient, sign)
		}

	case precision.HalfEven:
		if shouldRoundHalfEven(quotient, remainder, divisor) {
			roundAwayFromZero(quotient, sign)
		}

	default:
		panic("decimal: invalid rounding mode")
	}
}

func shouldRoundHalfUp(
	remainder, divisor *big.Int,
) bool {
	var absRemainder big.Int
	absRemainder.Abs(remainder)

	var absDivisor big.Int
	absDivisor.Abs(divisor)

	var doubled big.Int
	doubled.Lsh(&absRemainder, 1)

	return doubled.Cmp(&absDivisor) >= 0
}

func shouldRoundHalfEven(
	quotient, remainder, divisor *big.Int,
) bool {
	var absRemainder big.Int
	absRemainder.Abs(remainder)

	var absDivisor big.Int
	absDivisor.Abs(divisor)

	var doubled big.Int
	doubled.Lsh(&absRemainder, 1)

	switch doubled.Cmp(&absDivisor) {
	case 1:
		return true
	case -1:
		return false
	default:
		return quotient.Bit(0) == 1
	}
}

func roundAwayFromZero(value *big.Int, sign int) {
	var one big.Int
	one.SetInt64(1)

	switch {
	case sign > 0:
		value.Add(value, &one)
	case sign < 0:
		value.Sub(value, &one)
	}
}

func validatePolicy(policy precision.Policy) {
	if !policy.IsValid() {
		panic("decimal: invalid precision policy")
	}
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}
