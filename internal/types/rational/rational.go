package rational

import (
	"errors"
	"fmt"
	"math/big"
)

// ErrDivisionByZero is returned when an operation attempts to divide by
// the zero rational value.
var ErrDivisionByZero = errors.New("rational: division by zero")

// Rational represents an arbitrary-precision rational number.
//
// A Rational is stored as a numerator and denominator and is always kept
// in canonical form:
//
//   - the denominator is positive;
//   - numerator and denominator are reduced by their greatest common divisor;
//   - zero is represented canonically as the zero value Rational{}.
//
// The zero value is valid and represents 0/1.
//
// Rational values are immutable from the caller's perspective. Methods that
// expose the numerator or denominator return copies of the underlying big.Int
// values.
type Rational struct {
	numerator   big.Int
	denominator big.Int
}

// New returns the canonical rational value numerator/denominator.
//
// New normalizes the sign so that the denominator is positive and reduces
// numerator and denominator by their greatest common divisor.
//
// New panics if numerator or denominator is nil, or if denominator is zero.
//
// The input big.Int values are copied and are not retained by the returned
// Rational.
func New(numerator, denominator *big.Int) Rational {
	if numerator == nil {
		panic("rational: nil numerator")
	}

	if denominator == nil {
		panic("rational: nil denominator")
	}

	if denominator.Sign() == 0 {
		panic("rational: zero denominator")
	}

	return normalize(numerator, denominator)
}

// FromInt64 returns the exact Rational representation of value.
func FromInt64(value int64) Rational {
	var numerator big.Int
	numerator.SetInt64(value)

	return newCanonical(&numerator, big.NewInt(1))
}

// FromUint64 returns the exact Rational representation of value.
func FromUint64(value uint64) Rational {
	var numerator big.Int
	numerator.SetUint64(value)

	return newCanonical(&numerator, big.NewInt(1))
}

// Zero returns the rational value zero.
//
// The zero value of Rational is also a valid representation of zero.
func Zero() Rational {
	return Rational{}
}

// One returns the rational value one.
func One() Rational {
	return FromInt64(1)
}

// Numerator returns a copy of r's numerator.
//
// Modifying the returned big.Int does not modify r.
func (r Rational) Numerator() *big.Int {
	var numerator big.Int
	numerator.Set(&r.numerator)

	return &numerator
}

// Denominator returns a copy of r's denominator.
//
// For the zero value of Rational, Denominator returns 1.
//
// Modifying the returned big.Int does not modify r.
func (r Rational) Denominator() *big.Int {
	if r.denominator.Sign() == 0 {
		return big.NewInt(1)
	}

	var denominator big.Int
	denominator.Set(&r.denominator)

	return &denominator
}

// Sign returns:
//
//   - -1 if r < 0;
//   - 0 if r == 0;
//   - +1 if r > 0.
func (r Rational) Sign() int {
	return r.numerator.Sign()
}

// IsZero reports whether r is zero.
func (r Rational) IsZero() bool {
	return r.numerator.Sign() == 0
}

// Equal reports whether r and other represent the same rational value.
//
// Rational values are canonical, but Equal also handles zero values whose
// denominator is represented implicitly as 1.
func (r Rational) Equal(other Rational) bool {
	if r.IsZero() && other.IsZero() {
		return true
	}

	return r.numerator.Cmp(&other.numerator) == 0 &&
		r.effectiveDenominator().Cmp(other.effectiveDenominator()) == 0
}

// Cmp compares r and other and returns:
//
//   - -1 if r < other;
//   - 0 if r == other;
//   - +1 if r > other.
func (r Rational) Cmp(other Rational) int {
	if r.IsZero() {
		return -other.Sign()
	}

	if other.IsZero() {
		return r.Sign()
	}

	var left big.Int
	left.Mul(&r.numerator, other.effectiveDenominator())

	var right big.Int
	right.Mul(&other.numerator, r.effectiveDenominator())

	return left.Cmp(&right)
}

// Neg returns -r.
func (r Rational) Neg() Rational {
	if r.IsZero() {
		return Zero()
	}

	var numerator big.Int
	numerator.Neg(&r.numerator)

	return newCanonical(&numerator, r.effectiveDenominator())
}

// Abs returns the absolute value of r.
func (r Rational) Abs() Rational {
	if r.Sign() >= 0 {
		return r
	}

	var numerator big.Int
	numerator.Abs(&r.numerator)

	return newCanonical(&numerator, r.effectiveDenominator())
}

// Add returns the exact sum r + other.
//
// No rounding is performed.
func (r Rational) Add(other Rational) Rational {
	if r.IsZero() {
		return other
	}

	if other.IsZero() {
		return r
	}

	aDenominator := r.effectiveDenominator()
	bDenominator := other.effectiveDenominator()

	var gcd big.Int
	gcd.GCD(nil, nil, aDenominator, bDenominator)

	var aReducedDenominator big.Int
	aReducedDenominator.Quo(aDenominator, &gcd)

	var bReducedDenominator big.Int
	bReducedDenominator.Quo(bDenominator, &gcd)

	var left big.Int
	left.Mul(&r.numerator, &bReducedDenominator)

	var right big.Int
	right.Mul(&other.numerator, &aReducedDenominator)

	var numerator big.Int
	numerator.Add(&left, &right)

	var denominator big.Int
	denominator.Mul(&aReducedDenominator, bDenominator)

	return normalize(&numerator, &denominator)
}

// Sub returns the exact difference r - other.
//
// No rounding is performed.
func (r Rational) Sub(other Rational) Rational {
	if other.IsZero() {
		return r
	}

	if r.IsZero() {
		return other.Neg()
	}

	aDenominator := r.effectiveDenominator()
	bDenominator := other.effectiveDenominator()

	var gcd big.Int
	gcd.GCD(nil, nil, aDenominator, bDenominator)

	var aReducedDenominator big.Int
	aReducedDenominator.Quo(aDenominator, &gcd)

	var bReducedDenominator big.Int
	bReducedDenominator.Quo(bDenominator, &gcd)

	var left big.Int
	left.Mul(&r.numerator, &bReducedDenominator)

	var right big.Int
	right.Mul(&other.numerator, &aReducedDenominator)

	var numerator big.Int
	numerator.Sub(&left, &right)

	var denominator big.Int
	denominator.Mul(&aReducedDenominator, bDenominator)

	return normalize(&numerator, &denominator)
}

// Mul returns the exact product r * other.
//
// No rounding is performed.
//
// Before multiplication, common factors are cancelled across the two
// fractions to limit unnecessary growth of intermediate big.Int values.
func (r Rational) Mul(other Rational) Rational {
	if r.IsZero() || other.IsZero() {
		return Zero()
	}

	var aNumerator big.Int
	aNumerator.Set(&r.numerator)

	var aDenominator big.Int
	aDenominator.Set(r.effectiveDenominator())

	var bNumerator big.Int
	bNumerator.Set(&other.numerator)

	var bDenominator big.Int
	bDenominator.Set(other.effectiveDenominator())

	var absANumerator big.Int
	absANumerator.Abs(&aNumerator)

	var absBNumerator big.Int
	absBNumerator.Abs(&bNumerator)

	var gcdLeft big.Int
	gcdLeft.GCD(nil, nil, &absANumerator, &bDenominator)

	var gcdRight big.Int
	gcdRight.GCD(nil, nil, &absBNumerator, &aDenominator)

	aNumerator.Quo(&aNumerator, &gcdLeft)
	bDenominator.Quo(&bDenominator, &gcdLeft)

	bNumerator.Quo(&bNumerator, &gcdRight)
	aDenominator.Quo(&aDenominator, &gcdRight)

	var numerator big.Int
	numerator.Mul(&aNumerator, &bNumerator)

	var denominator big.Int
	denominator.Mul(&aDenominator, &bDenominator)

	return newCanonical(&numerator, &denominator)
}

// Div returns the exact quotient r / other.
//
// No rounding is performed.
//
// Div returns ErrDivisionByZero if other is zero.
//
// Before division, common factors are cancelled to limit unnecessary growth
// of intermediate big.Int values.
func (r Rational) Div(other Rational) (Rational, error) {
	if other.IsZero() {
		return Rational{}, ErrDivisionByZero
	}

	if r.IsZero() {
		return Zero(), nil
	}

	var aNumerator big.Int
	aNumerator.Set(&r.numerator)

	var aDenominator big.Int
	aDenominator.Set(r.effectiveDenominator())

	var bNumerator big.Int
	bNumerator.Set(&other.numerator)

	var bDenominator big.Int
	bDenominator.Set(other.effectiveDenominator())

	var absANumerator big.Int
	absANumerator.Abs(&aNumerator)

	var absBNumerator big.Int
	absBNumerator.Abs(&bNumerator)

	var gcdNumerators big.Int
	gcdNumerators.GCD(nil, nil, &absANumerator, &absBNumerator)

	var gcdDenominators big.Int
	gcdDenominators.GCD(nil, nil, &aDenominator, &bDenominator)

	aNumerator.Quo(&aNumerator, &gcdNumerators)
	bNumerator.Quo(&bNumerator, &gcdNumerators)

	aDenominator.Quo(&aDenominator, &gcdDenominators)
	bDenominator.Quo(&bDenominator, &gcdDenominators)

	var numerator big.Int
	numerator.Mul(&aNumerator, &bDenominator)

	var denominator big.Int
	denominator.Mul(&aDenominator, &bNumerator)

	if denominator.Sign() < 0 {
		numerator.Neg(&numerator)
		denominator.Neg(&denominator)
	}

	return newCanonical(&numerator, &denominator), nil
}

// Inv returns the multiplicative inverse 1/r.
//
// Inv returns ErrDivisionByZero if r is zero.
func (r Rational) Inv() (Rational, error) {
	if r.IsZero() {
		return Rational{}, ErrDivisionByZero
	}

	var numerator big.Int
	numerator.Set(r.effectiveDenominator())

	var denominator big.Int
	denominator.Set(&r.numerator)

	if denominator.Sign() < 0 {
		numerator.Neg(&numerator)
		denominator.Neg(&denominator)
	}

	return newCanonical(&numerator, &denominator), nil
}

// String returns the canonical textual representation of r.
//
// Integer values are formatted without a denominator:
//
//	3
//	-7
//
// Non-integer values are formatted as numerator/denominator:
//
//	1/3
//	-5/7
//
// Zero is formatted as "0".
func (r Rational) String() string {
	if r.IsZero() {
		return "0"
	}

	denominator := r.effectiveDenominator()

	if denominator.Cmp(big.NewInt(1)) == 0 {
		return r.numerator.String()
	}

	return fmt.Sprintf(
		"%s/%s",
		r.numerator.String(),
		denominator.String(),
	)
}

// normalize returns numerator/denominator in canonical form.
//
// The denominator is made positive and the fraction is reduced by its
// greatest common divisor. Zero is canonicalized to Rational{}.
//
// numerator and denominator must be non-nil, and denominator must be non-zero.
// These conditions are guaranteed by callers.
func normalize(numerator, denominator *big.Int) Rational {
	if numerator.Sign() == 0 {
		return Zero()
	}

	var normalizedNumerator big.Int
	normalizedNumerator.Set(numerator)

	var normalizedDenominator big.Int
	normalizedDenominator.Set(denominator)

	if normalizedDenominator.Sign() < 0 {
		normalizedNumerator.Neg(&normalizedNumerator)
		normalizedDenominator.Neg(&normalizedDenominator)
	}

	var absNumerator big.Int
	absNumerator.Abs(&normalizedNumerator)

	var gcd big.Int
	gcd.GCD(
		nil,
		nil,
		&absNumerator,
		&normalizedDenominator,
	)

	normalizedNumerator.Quo(&normalizedNumerator, &gcd)
	normalizedDenominator.Quo(&normalizedDenominator, &gcd)

	return newCanonical(
		&normalizedNumerator,
		&normalizedDenominator,
	)
}

// newCanonical constructs a Rational from an already canonical numerator
// and denominator.
//
// For zero, the denominator is intentionally left unset so that Rational{}
// remains the canonical zero representation.
func newCanonical(numerator, denominator *big.Int) Rational {
	var result Rational
	result.numerator.Set(numerator)

	if numerator.Sign() != 0 {
		result.denominator.Set(denominator)
	}

	return result
}

// effectiveDenominator returns the mathematical denominator of r.
//
// Rational{} stores zero without an explicit denominator; in that case the
// effective denominator is 1.
func (r Rational) effectiveDenominator() *big.Int {
	if r.denominator.Sign() == 0 {
		return big.NewInt(1)
	}

	return &r.denominator
}
