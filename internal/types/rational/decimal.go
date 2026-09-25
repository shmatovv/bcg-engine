package rational

import (
	"math/big"

	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
)

// FromDecimal returns the exact Rational representation of value.
//
// Decimal represents a value as:
//
//	coefficient × 10^-scale
//
// Therefore the equivalent Rational is constructed as:
//
//	coefficient / 10^scale
//
// The conversion is exact and performs no rounding.
//
// For example:
//
//	Decimal("12.34") -> Rational(617/50)
//	Decimal("0.125") -> Rational(1/8)
//	Decimal("-0.1")  -> Rational(-1/10)
//
// The resulting Rational is normalized to its canonical representation.
func FromDecimal(value decimal.Decimal) Rational {
	numerator := value.Coefficient()

	var base big.Int
	base.SetInt64(10)

	var exponent big.Int
	exponent.SetUint64(value.Scale())

	var denominator big.Int
	denominator.Exp(&base, &exponent, nil)

	return New(numerator, &denominator)
}

// Decimal converts r to a Decimal according to policy.
//
// Rational values may represent numbers that have no finite decimal
// representation. Therefore, this conversion is a precision boundary:
// policy determines the target decimal scale and rounding mode.
//
// The rational value itself remains exact until this conversion is performed.
//
// For example, the Rational value 1/8 can be represented exactly as 0.125,
// while 1/3 requires rounding for any finite decimal scale.
//
// Decimal returns an error if the requested decimal operation cannot be
// completed according to the Decimal contract, such as when the requested
// scale exceeds decimal.MaxScale.
func (r Rational) Decimal(
	policy precision.Policy,
) (decimal.Decimal, error) {
	numerator := decimal.New(r.Numerator(), 0)
	denominator := decimal.New(r.Denominator(), 0)

	return numerator.Div(denominator, policy)
}
