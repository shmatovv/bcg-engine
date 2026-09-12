package unit

import (
	"fmt"
	"sort"

	"github.com/shmatovv/bcg-engine/internal/types/rational"
)

// UnitID identifies a concrete symbolic unit.
//
// Examples:
//
//	meter
//	kilometer
//	second
//	kilogram
//
// UnitID identifies a concrete unit rather than a dimension. Different units
// may therefore have different UnitIDs while sharing the same Dimension.
type UnitID string

// UnitFactor represents one concrete symbolic unit raised to an integer
// exponent.
//
// For example:
//
//	kilometer / hour
//
// contains:
//
//	kilometer -> 1
//	hour      -> -1
//
// UnitFactor values are internal components of the canonical symbolic
// representation of a Unit.
type UnitFactor struct {
	id       UnitID
	exponent int
}

// SymbolicFactor is the portable, immutable representation of one symbolic
// unit exponent.
type SymbolicFactor struct {
	ID       UnitID
	Exponent int
}

// ConversionFactorSpec is the portable, immutable representation of one
// conversion-space exponent.
type ConversionFactorSpec struct {
	ID       ConversionSpaceID
	Exponent int
}

// Unit represents a concrete unit of measurement.
//
// A Unit consists of:
//
//   - dimension: dimensional meaning;
//   - factors: canonical symbolic composition;
//   - conversionFactors: canonical composition of exact, context-free
//     conversion spaces;
//   - scale: exact multiplicative scale within its conversion signature;
//   - valid: distinguishes a real Unit from the zero value.
//
// Dimension and conversion signature are intentionally separate.
//
// Equal Dimensions mean that values describe the same dimensional kind.
// They do not necessarily mean that values may be automatically converted.
//
// For example, RUB and EUR may both have the Currency Dimension while
// belonging to different conversion spaces because their conversion requires
// external context.
//
// Unit algebra is exact. Scale is represented by Rational and no decimal
// rounding is performed.
//
// The zero value Unit{} is invalid.
type Unit struct {
	dimension         Dimension
	factors           []UnitFactor
	conversionFactors []ConversionFactor
	scale             rational.Rational
	valid             bool
}

// DimensionlessUnit returns the canonical dimensionless unit.
//
// It is the multiplicative identity of unit algebra:
//
//	u.Mul(DimensionlessUnit()) == u
//
// DimensionlessUnit has an empty conversion signature, which is the
// multiplicative identity of conversion-space algebra.
//
// The returned Unit is valid and distinct from the invalid zero value Unit{}.
func DimensionlessUnit() Unit {
	return Unit{
		dimension:         Dimensionless(),
		conversionFactors: nil,
		scale:             rational.One(),
		valid:             true,
	}
}

// NewUnit constructs a concrete symbolic unit.
//
// id identifies the concrete symbolic unit.
//
// dimension describes its dimensional meaning.
//
// conversionSpace identifies the family within which exact, context-free
// scale conversion is allowed.
//
// scale specifies the exact multiplicative relationship to the canonical
// scale of that conversion space.
//
// Examples:
//
//	meter:
//	    dimension       = length
//	    conversionSpace = length
//	    scale           = 1
//
//	centimeter:
//	    dimension       = length
//	    conversionSpace = length
//	    scale           = 1/100
//
//	rub:
//	    dimension       = currency
//	    conversionSpace = rub
//	    scale           = 1
//
//	eur:
//	    dimension       = currency
//	    conversionSpace = eur
//	    scale           = 1
//
// RUB and EUR therefore have equal Dimensions but are not automatically
// convertible.
//
// NewUnit panics if:
//
//   - id is invalid;
//   - dimension is invalid;
//   - conversionSpace is invalid;
//   - scale is zero or negative.
func NewUnit(
	id UnitID,
	dimension Dimension,
	conversionSpace ConversionSpaceID,
	scale rational.Rational,
) Unit {
	result, err := NewUnitChecked(id, dimension, conversionSpace, scale)
	if err != nil {
		panic(err)
	}
	return result
}

// NewUnitChecked constructs a concrete symbolic Unit without panicking for
// invalid input. It is the error-returning counterpart to NewUnit for public
// and other recoverable data boundaries.
func NewUnitChecked(
	id UnitID,
	dimension Dimension,
	conversionSpace ConversionSpaceID,
	scale rational.Rational,
) (Unit, error) {
	if !id.IsValid() {
		return Unit{}, fmt.Errorf("unit: invalid unit id %q", id)
	}

	if !dimension.IsValid() {
		return Unit{}, fmt.Errorf("unit: invalid dimension")
	}

	if !conversionSpace.IsValid() {
		return Unit{}, fmt.Errorf(
			"unit: invalid conversion space %q",
			conversionSpace,
		)
	}

	if scale.Sign() <= 0 {
		return Unit{}, fmt.Errorf("unit: scale must be positive")
	}

	return Unit{
		dimension: dimension,
		factors: []UnitFactor{
			{
				id:       id,
				exponent: 1,
			},
		},
		conversionFactors: []ConversionFactor{
			{
				id:       conversionSpace,
				exponent: 1,
			},
		},
		scale: scale,
		valid: true,
	}, nil
}

// NewComposedUnit constructs a canonical Unit from its complete portable
// representation. It is intended for data boundaries such as model codecs;
// ordinary model code should normally use NewUnit and unit algebra.
//
// The inputs must already describe a canonical unit: IDs are valid, factors
// and conversion factors are strictly ordered and non-zero, the dimension is
// valid, and scale is positive. This deliberately rejects silent repair of
// malformed serialized units.
func NewComposedUnit(
	dimension Dimension,
	factors []SymbolicFactor,
	conversionFactors []ConversionFactorSpec,
	scale rational.Rational,
) Unit {
	result, err := NewComposedUnitChecked(dimension, factors, conversionFactors, scale)
	if err != nil {
		panic(err)
	}
	return result
}

// NewComposedUnitChecked constructs a canonical portable Unit without
// panicking for invalid input. Unlike unit algebra, this boundary accepts only
// an already canonical factor and conversion-factor representation.
func NewComposedUnitChecked(
	dimension Dimension,
	factors []SymbolicFactor,
	conversionFactors []ConversionFactorSpec,
	scale rational.Rational,
) (Unit, error) {
	internalFactors := make([]UnitFactor, len(factors))
	for index, factor := range factors {
		internalFactors[index] = UnitFactor{id: factor.ID, exponent: factor.Exponent}
	}
	internalConversions := make([]ConversionFactor, len(conversionFactors))
	for index, factor := range conversionFactors {
		internalConversions[index] = ConversionFactor{id: factor.ID, exponent: factor.Exponent}
	}
	result := Unit{
		dimension:         dimension,
		factors:           internalFactors,
		conversionFactors: internalConversions,
		scale:             scale,
		valid:             true,
	}
	if !result.IsValid() {
		return Unit{}, fmt.Errorf("unit: invalid composed unit")
	}
	return result, nil
}

// IsValid reports whether id is a valid unit identifier.
//
// A valid identifier:
//
//   - is non-empty;
//   - starts with a lowercase ASCII letter;
//   - contains only lowercase ASCII letters, decimal digits, and underscores.
func (id UnitID) IsValid() bool {
	value := string(id)

	if len(value) == 0 {
		return false
	}

	if value[0] < 'a' || value[0] > 'z' {
		return false
	}

	for i := 1; i < len(value); i++ {
		c := value[i]

		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '_':
		default:
			return false
		}
	}

	return true
}

// Dimension returns the dimensional meaning of u.
func (u Unit) Dimension() Dimension {
	return u.dimension
}

// Scale returns the exact multiplicative scale of u within its conversion
// signature.
func (u Unit) Scale() rational.Rational {
	return u.scale
}

// Factors returns a defensive portable copy of the canonical symbolic unit
// composition.
func (u Unit) Factors() []SymbolicFactor {
	if len(u.factors) == 0 {
		return nil
	}
	result := make([]SymbolicFactor, len(u.factors))
	for index, factor := range u.factors {
		result[index] = SymbolicFactor{ID: factor.id, Exponent: factor.exponent}
	}
	return result
}

// ConversionFactors returns a defensive portable copy of the canonical
// conversion-space composition.
func (u Unit) ConversionFactors() []ConversionFactorSpec {
	if len(u.conversionFactors) == 0 {
		return nil
	}
	result := make([]ConversionFactorSpec, len(u.conversionFactors))
	for index, factor := range u.conversionFactors {
		result[index] = ConversionFactorSpec{ID: factor.id, Exponent: factor.exponent}
	}
	return result
}

// IsDimensionless reports whether u has dimensionless dimensional meaning.
//
// An invalid Unit is not considered dimensionless.
func (u Unit) IsDimensionless() bool {
	return u.IsValid() && u.dimension.IsDimensionless()
}

// CanConvertTo reports whether u may be converted to other using only exact,
// context-free static unit scaling.
//
// Automatic conversion requires:
//
//   - both Units to be valid;
//   - equal Dimensions;
//   - equal canonical conversion signatures.
//
// Equal Dimension alone is intentionally insufficient.
//
// Examples:
//
//	kg -> g           true
//	m -> cm           true
//	RUB -> kopeck     true
//	RUB -> EUR        false
//	RUB/item -> EUR/item
//	                  false
func (u Unit) CanConvertTo(other Unit) bool {
	if !u.IsValid() || !other.IsValid() {
		return false
	}

	if !u.dimension.Equal(other.dimension) {
		return false
	}

	return equalConversionFactors(
		u.conversionFactors,
		other.conversionFactors,
	)
}

// IsValid reports whether u represents a valid canonical Unit.
//
// The zero value Unit{} is invalid.
//
// A valid Unit:
//
//   - has been explicitly constructed;
//   - contains a valid Dimension;
//   - has a strictly positive scale;
//   - contains valid, non-zero, strictly ordered symbolic factors;
//   - contains valid, non-zero, strictly ordered conversion factors.
//
// A dimensionless unit may legitimately contain no symbolic or conversion
// factors.
func (u Unit) IsValid() bool {
	if !u.valid {
		return false
	}

	if !u.dimension.IsValid() {
		return false
	}

	if u.scale.Sign() <= 0 {
		return false
	}

	var previousUnitID UnitID

	for i, factor := range u.factors {
		if !factor.id.IsValid() {
			return false
		}

		if factor.exponent == 0 {
			return false
		}

		if i > 0 && previousUnitID >= factor.id {
			return false
		}

		previousUnitID = factor.id
	}

	var previousConversionSpace ConversionSpaceID

	for i, factor := range u.conversionFactors {
		if !factor.id.IsValid() {
			return false
		}

		if factor.exponent == 0 {
			return false
		}

		if i > 0 &&
			previousConversionSpace >= factor.id {
			return false
		}

		previousConversionSpace = factor.id
	}

	return true
}

// Equal reports whether u and other represent the same Unit.
//
// Equality compares:
//
//   - validity;
//   - dimension;
//   - exact scale;
//   - canonical symbolic factors;
//   - canonical conversion signature.
//
// Equal represents Unit identity, not merely dimensional compatibility or
// convertibility.
func (u Unit) Equal(other Unit) bool {
	if u.valid != other.valid {
		return false
	}

	if !u.valid {
		return true
	}

	if !u.dimension.Equal(other.dimension) {
		return false
	}

	if !u.scale.Equal(other.scale) {
		return false
	}

	if len(u.factors) != len(other.factors) {
		return false
	}

	for i := range u.factors {
		if u.factors[i].id != other.factors[i].id {
			return false
		}

		if u.factors[i].exponent !=
			other.factors[i].exponent {
			return false
		}
	}

	if !equalConversionFactors(
		u.conversionFactors,
		other.conversionFactors,
	) {
		return false
	}

	return true
}

// Mul returns the compound Unit produced by multiplying u by other.
//
// Both operands must be valid.
//
// Multiplication combines:
//
//   - Dimensions;
//   - symbolic factors;
//   - conversion signatures;
//   - exact scales.
//
// Mul panics if either operand is invalid or exponent arithmetic overflows.
func (u Unit) Mul(other Unit) Unit {
	u.requireValid()
	other.requireValid()

	return Unit{
		dimension: u.dimension.Mul(other.dimension),
		factors: mergeUnitFactors(
			u.factors,
			other.factors,
			1,
		),
		conversionFactors: mergeConversionFactors(
			u.conversionFactors,
			other.conversionFactors,
			1,
		),
		scale: u.scale.Mul(other.scale),
		valid: true,
	}
}

// Div returns the compound Unit produced by dividing u by other.
//
// Both operands must be valid.
//
// Division combines:
//
//   - Dimensions;
//   - symbolic factors;
//   - conversion signatures;
//   - exact scales.
//
// Div returns an error if Rational scale division cannot be performed.
//
// Div panics if either operand is invalid or exponent arithmetic overflows.
func (u Unit) Div(other Unit) (Unit, error) {
	u.requireValid()
	other.requireValid()

	scale, err := u.scale.Div(other.scale)
	if err != nil {
		return Unit{}, err
	}

	return Unit{
		dimension: u.dimension.Div(other.dimension),
		factors: mergeUnitFactors(
			u.factors,
			other.factors,
			-1,
		),
		conversionFactors: mergeConversionFactors(
			u.conversionFactors,
			other.conversionFactors,
			-1,
		),
		scale: scale,
		valid: true,
	}, nil
}

// requireValid panics unless u is a valid Unit.
func (u Unit) requireValid() {
	if !u.IsValid() {
		panic("unit: invalid unit")
	}
}

// mergeUnitFactors combines two canonical symbolic factor sets.
//
// rightSign controls whether right is multiplied into or divided out of left:
//
//	+1 -> multiplication
//	-1 -> division
//
// Equal UnitIDs are combined, zero exponents are removed, and the result is
// sorted lexicographically by UnitID.
func mergeUnitFactors(
	left []UnitFactor,
	right []UnitFactor,
	rightSign int,
) []UnitFactor {
	if rightSign != 1 && rightSign != -1 {
		panic("unit: invalid unit factor sign")
	}

	exponents := make(
		map[UnitID]int,
		len(left)+len(right),
	)

	for _, factor := range left {
		exponents[factor.id] = addExponent(
			exponents[factor.id],
			factor.exponent,
		)
	}

	for _, factor := range right {
		exponent := factor.exponent

		if rightSign == -1 {
			exponent = negateExponent(exponent)
		}

		exponents[factor.id] = addExponent(
			exponents[factor.id],
			exponent,
		)
	}

	ids := make(
		[]UnitID,
		0,
		len(exponents),
	)

	for id, exponent := range exponents {
		if exponent != 0 {
			ids = append(ids, id)
		}
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	factors := make(
		[]UnitFactor,
		0,
		len(ids),
	)

	for _, id := range ids {
		factors = append(
			factors,
			UnitFactor{
				id:       id,
				exponent: exponents[id],
			},
		)
	}

	return factors
}

// mergeConversionFactors combines two canonical conversion signatures.
//
// rightSign controls whether right is multiplied into or divided out of left:
//
//	+1 -> multiplication
//	-1 -> division
//
// Equal ConversionSpaceIDs are combined, zero exponents are removed, and the
// result is sorted lexicographically by ConversionSpaceID.
func mergeConversionFactors(
	left []ConversionFactor,
	right []ConversionFactor,
	rightSign int,
) []ConversionFactor {
	if rightSign != 1 && rightSign != -1 {
		panic("unit: invalid conversion factor sign")
	}

	exponents := make(
		map[ConversionSpaceID]int,
		len(left)+len(right),
	)

	for _, factor := range left {
		exponents[factor.id] = addExponent(
			exponents[factor.id],
			factor.exponent,
		)
	}

	for _, factor := range right {
		exponent := factor.exponent

		if rightSign == -1 {
			exponent = negateExponent(exponent)
		}

		exponents[factor.id] = addExponent(
			exponents[factor.id],
			exponent,
		)
	}

	ids := make(
		[]ConversionSpaceID,
		0,
		len(exponents),
	)

	for id, exponent := range exponents {
		if exponent != 0 {
			ids = append(ids, id)
		}
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	factors := make(
		[]ConversionFactor,
		0,
		len(ids),
	)

	for _, id := range ids {
		factors = append(
			factors,
			ConversionFactor{
				id:       id,
				exponent: exponents[id],
			},
		)
	}

	return factors
}

// equalConversionFactors reports whether two canonical conversion signatures
// are identical.
func equalConversionFactors(
	left []ConversionFactor,
	right []ConversionFactor,
) bool {
	if len(left) != len(right) {
		return false
	}

	for i := range left {
		if left[i].id != right[i].id {
			return false
		}

		if left[i].exponent != right[i].exponent {
			return false
		}
	}

	return true
}
