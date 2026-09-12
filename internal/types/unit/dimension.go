package unit

import (
	"fmt"
	"math"
	"sort"
)

// ID identifies a base physical or logical dimension.
//
// An ID is a stable symbolic identifier such as:
//
//	length
//	time
//	mass
//	currency
//
// Valid IDs use the same restricted identifier format throughout the unit
// system: they must start with a lowercase ASCII letter and may contain
// lowercase ASCII letters, decimal digits, and underscores.
//
// ID describes a dimension, not a concrete unit. For example, "length" may
// identify the dimension shared by meter, kilometer, and centimeter.
type ID string

// Factor represents one base dimension raised to an integer exponent.
//
// For example:
//
//	length / time^2
//
// contains:
//
//	length -> 1
//	time   -> -2
//
// Factor values are internal components of a canonical Dimension.
type Factor struct {
	id       ID
	exponent int
}

// DimensionFactor is the portable, immutable representation of one base
// dimension exponent. It deliberately exposes value data rather than the
// internal canonical Factor representation so codecs do not need access to
// Dimension internals.
type DimensionFactor struct {
	ID       ID
	Exponent int
}

// Dimension represents the dimensional structure of a quantity.
//
// A Dimension is a canonical product of base dimensions raised to integer
// powers.
//
// For example:
//
//	length
//	length * time^-1
//	mass * length * time^-2
//
// Dimension contains dimensional meaning only. It does not contain concrete
// unit identity, conversion scale, or a numeric quantity.
//
// Dimension values are canonical:
//
//   - factors with equal IDs are combined;
//   - zero exponents are removed;
//   - factors are sorted lexicographically by ID;
//   - exponent arithmetic never silently overflows.
//
// The zero value is valid and represents the dimensionless dimension.
type Dimension struct {
	factors []Factor
}

// IsValid reports whether id is a valid dimension identifier.
//
// A valid identifier:
//
//   - is non-empty;
//   - starts with a lowercase ASCII letter;
//   - contains only lowercase ASCII letters, decimal digits, and underscores.
func (id ID) IsValid() bool {
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

// Dimensionless returns the dimensionless dimension.
//
// Dimensionless is the multiplicative identity of dimension algebra.
//
// The returned value is equal to the zero value Dimension{}.
func Dimensionless() Dimension {
	return Dimension{}
}

// NewDimension constructs a canonical Dimension from portable factors.
//
// Repeated IDs are combined, zero exponents are removed, and the result is
// sorted exactly like dimensions produced by unit algebra. It panics for an
// invalid identifier or exponent overflow, matching the construction
// contract of Base and the arithmetic operations.
func NewDimension(factors []DimensionFactor) Dimension {
	internal := make([]Factor, len(factors))
	for index, factor := range factors {
		internal[index] = Factor{id: factor.ID, exponent: factor.Exponent}
	}
	result, err := newDimensionChecked(internal)
	if err != nil {
		panic(err)
	}
	return result
}

// NewDimensionChecked constructs a canonical Dimension without panicking for
// invalid portable input. It is the error-returning counterpart to
// NewDimension for public and other recoverable data boundaries.
func NewDimensionChecked(factors []DimensionFactor) (Dimension, error) {
	internal := make([]Factor, len(factors))
	for index, factor := range factors {
		internal[index] = Factor{id: factor.ID, exponent: factor.Exponent}
	}
	return newDimensionChecked(internal)
}

// Base returns the base dimension identified by id.
//
// Base panics if id is invalid.
func Base(id ID) Dimension {
	if !id.IsValid() {
		panic("unit: invalid dimension id")
	}

	return Dimension{
		factors: []Factor{
			{
				id:       id,
				exponent: 1,
			},
		},
	}
}

// IsValid reports whether d is a canonical Dimension.
//
// The zero value is valid and represents the dimensionless dimension.
func (d Dimension) IsValid() bool {
	var previous ID

	for i, factor := range d.factors {
		if !factor.id.IsValid() {
			return false
		}

		if factor.exponent == 0 {
			return false
		}

		if i > 0 && previous >= factor.id {
			return false
		}

		previous = factor.id
	}

	return true
}

// IsDimensionless reports whether d represents the dimensionless dimension.
func (d Dimension) IsDimensionless() bool {
	return len(d.factors) == 0
}

// Factors returns a defensive portable copy of d's canonical factors.
// The returned order is canonical lexicographic ID order.
func (d Dimension) Factors() []DimensionFactor {
	if len(d.factors) == 0 {
		return nil
	}
	result := make([]DimensionFactor, len(d.factors))
	for index, factor := range d.factors {
		result[index] = DimensionFactor{ID: factor.id, Exponent: factor.exponent}
	}
	return result
}

// Equal reports whether d and other represent the same dimension.
func (d Dimension) Equal(other Dimension) bool {
	if len(d.factors) != len(other.factors) {
		return false
	}

	for i := range d.factors {
		if d.factors[i].id != other.factors[i].id {
			return false
		}

		if d.factors[i].exponent != other.factors[i].exponent {
			return false
		}
	}

	return true
}

// Mul returns the dimension produced by multiplying d by other.
//
// Exponents of matching base dimensions are added.
//
// Mul panics if exponent arithmetic overflows int.
func (d Dimension) Mul(other Dimension) Dimension {
	factors := make([]Factor, 0, len(d.factors)+len(other.factors))

	factors = append(factors, d.factors...)
	factors = append(factors, other.factors...)

	return newDimension(factors)
}

// Div returns the dimension produced by dividing d by other.
//
// Exponents of other are negated before the dimensions are combined.
//
// Div panics if exponent arithmetic overflows int.
func (d Dimension) Div(other Dimension) Dimension {
	factors := make([]Factor, 0, len(d.factors)+len(other.factors))

	factors = append(factors, d.factors...)

	for _, factor := range other.factors {
		factors = append(factors, Factor{
			id:       factor.id,
			exponent: negateExponent(factor.exponent),
		})
	}

	return newDimension(factors)
}

// newDimension constructs a canonical Dimension from factors.
//
// It validates IDs, combines equal factors, removes zero exponents, and sorts
// the result lexicographically by ID.
//
// newDimension panics if an ID is invalid or exponent arithmetic overflows.
func newDimension(factors []Factor) Dimension {
	result, err := newDimensionChecked(factors)
	if err != nil {
		panic(err)
	}
	return result
}

func newDimensionChecked(factors []Factor) (Dimension, error) {
	if len(factors) == 0 {
		return Dimensionless(), nil
	}

	powers := make(map[ID]int, len(factors))

	for _, factor := range factors {
		if !factor.id.IsValid() {
			return Dimension{}, fmt.Errorf("unit: invalid dimension id %q", factor.id)
		}

		exponent, err := addExponentChecked(powers[factor.id], factor.exponent)
		if err != nil {
			return Dimension{}, err
		}
		powers[factor.id] = exponent
	}

	result := make([]Factor, 0, len(powers))

	for id, exponent := range powers {
		if exponent == 0 {
			continue
		}

		result = append(result, Factor{
			id:       id,
			exponent: exponent,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].id < result[j].id
	})

	return Dimension{
		factors: result,
	}, nil
}

// addExponent returns a+b.
//
// It panics rather than allowing integer overflow to silently change the
// mathematical meaning of a dimension.
func addExponent(a, b int) int {
	result, err := addExponentChecked(a, b)
	if err != nil {
		panic(err)
	}
	return result
}

func addExponentChecked(a, b int) (int, error) {
	if b > 0 && a > math.MaxInt-b {
		return 0, ErrExponentOverflow
	}

	if b < 0 && a < math.MinInt-b {
		return 0, ErrExponentOverflow
	}

	return a + b, nil
}

// negateExponent returns -value.
//
// It panics for math.MinInt because its positive counterpart cannot be
// represented by int.
func negateExponent(value int) int {
	if value == math.MinInt {
		panic(ErrExponentOverflow)
	}

	return -value
}
