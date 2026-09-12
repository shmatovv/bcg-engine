package bcg

import (
	"fmt"
	"math/big"

	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	internalrational "github.com/shmatovv/bcg-engine/internal/types/rational"
	internalunit "github.com/shmatovv/bcg-engine/internal/types/unit"
	internalvalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

// ValueType describes the scalar type that later schema authoring assigns to
// a property. It is distinct from Value: a ValueType is metadata such as
// DecimalType, while a Value is one defined value such as DecimalValue(12.5).
// The zero value is invalid.
type ValueType uint8

const (
	// BoolType describes Boolean values.
	BoolType ValueType = iota + 1
	// IntegerType describes arbitrary-size integer values.
	IntegerType
	// DecimalType describes exact arbitrary-precision decimal values.
	DecimalType
	// StringType describes textual values.
	StringType
	// DateType describes calendar-date values.
	DateType
	// DateTimeType describes UTC instant values.
	DateTimeType
	// DurationType describes exact elapsed-duration values.
	DurationType
)

// IsValid reports whether t is a supported BCG v1 scalar value type.
func (t ValueType) IsValid() bool {
	return t >= BoolType && t <= DurationType
}

// String returns the stable scalar type name, or "Invalid".
func (t ValueType) String() string {
	switch t {
	case BoolType:
		return "Bool"
	case IntegerType:
		return "Integer"
	case DecimalType:
		return "Decimal"
	case StringType:
		return "String"
	case DateType:
		return "Date"
	case DateTimeType:
		return "DateTime"
	case DurationType:
		return "Duration"
	default:
		return "Invalid"
	}
}

func (t ValueType) internal() (internalvalue.Type, error) {
	switch t {
	case BoolType:
		return internalvalue.Bool, nil
	case IntegerType:
		return internalvalue.Integer, nil
	case DecimalType:
		return internalvalue.Decimal, nil
	case StringType:
		return internalvalue.String, nil
	case DateType:
		return internalvalue.Date, nil
	case DateTimeType:
		return internalvalue.DateTime, nil
	case DurationType:
		return internalvalue.Duration, nil
	default:
		return internalvalue.Invalid, fmt.Errorf("%w: value type is invalid", ErrInvalidArgument)
	}
}

func valueTypeFromInternal(valueType internalvalue.Type) ValueType {
	switch valueType {
	case internalvalue.Bool:
		return BoolType
	case internalvalue.Integer:
		return IntegerType
	case internalvalue.Decimal:
		return DecimalType
	case internalvalue.String:
		return StringType
	case internalvalue.Date:
		return DateType
	case internalvalue.DateTime:
		return DateTimeType
	case internalvalue.Duration:
		return DurationType
	default:
		return 0
	}
}

// TemporalMode describes whether a property has one scalar value or a source
// series of scalar samples. It is independent from ValueType. The zero value
// is invalid.
type TemporalMode uint8

const (
	// TemporalScalar describes a single scalar value.
	TemporalScalar TemporalMode = iota + 1
	// TemporalSeries describes a point or interval series of scalar samples.
	TemporalSeries
)

// IsValid reports whether m is a supported BCG v1 temporal mode.
func (m TemporalMode) IsValid() bool {
	return m == TemporalScalar || m == TemporalSeries
}

// String returns the stable temporal-mode name, or "Invalid".
func (m TemporalMode) String() string {
	switch m {
	case TemporalScalar:
		return "Scalar"
	case TemporalSeries:
		return "Series"
	default:
		return "Invalid"
	}
}

func (m TemporalMode) internal() (schema.TemporalMode, error) {
	switch m {
	case TemporalScalar:
		return schema.TemporalScalar, nil
	case TemporalSeries:
		return schema.TemporalSeries, nil
	default:
		return schema.TemporalInvalid, fmt.Errorf("%w: temporal mode is invalid", ErrInvalidArgument)
	}
}

func temporalModeFromInternal(mode schema.TemporalMode) (TemporalMode, error) {
	switch mode {
	case schema.TemporalScalar:
		return TemporalScalar, nil
	case schema.TemporalSeries:
		return TemporalSeries, nil
	default:
		return 0, fmt.Errorf("%w: temporal mode is invalid", ErrInvalidArgument)
	}
}

// DimensionID identifies one domain-defined base dimension. It is a symbolic
// identifier, not a business-unit catalog entry.
type DimensionID string

// IsValid reports whether id has the portable BCG identifier form.
func (id DimensionID) IsValid() bool { return internalunit.ID(id).IsValid() }

// UnitID identifies one domain-defined concrete unit symbol.
type UnitID string

// IsValid reports whether id has the portable BCG identifier form.
func (id UnitID) IsValid() bool { return internalunit.UnitID(id).IsValid() }

// ConversionSpaceID identifies a family of units with exact, context-free
// conversions. Equal Dimensions alone do not make units convertible.
type ConversionSpaceID string

// IsValid reports whether id has the portable BCG identifier form.
func (id ConversionSpaceID) IsValid() bool {
	return internalunit.ConversionSpaceID(id).IsValid()
}

// DimensionFactor is one base dimension raised to a non-zero integer power.
// NewDimension combines duplicate IDs, removes zero powers, and sorts factors
// canonically.
type DimensionFactor struct {
	ID       DimensionID
	Exponent int
}

// IsValid reports structural validity before canonicalization.
func (f DimensionFactor) IsValid() bool { return f.ID.IsValid() }

// Dimension is the canonical dimensional meaning of a quantity. It has no
// concrete unit symbol or conversion scale. Its zero value is the meaningful
// dimensionless dimension.
type Dimension struct{ value internalunit.Dimension }

// DimensionlessDimension returns the canonical dimensionless dimension.
func DimensionlessDimension() Dimension { return Dimension{value: internalunit.Dimensionless()} }

// NewBaseDimension creates the base Dimension identified by id.
func NewBaseDimension(id DimensionID) (Dimension, error) {
	return NewDimension([]DimensionFactor{{ID: id, Exponent: 1}})
}

// NewDimension creates a canonical composed Dimension. It combines repeated
// factors, removes zero exponents, and orders factors lexicographically.
func NewDimension(factors []DimensionFactor) (Dimension, error) {
	internalFactors := make([]internalunit.DimensionFactor, len(factors))
	for index, factor := range factors {
		internalFactors[index] = internalunit.DimensionFactor{
			ID:       internalunit.ID(factor.ID),
			Exponent: factor.Exponent,
		}
	}
	dimension, err := internalunit.NewDimensionChecked(internalFactors)
	if err != nil {
		return Dimension{}, fmt.Errorf("%w: dimension: %v", ErrInvalidArgument, err)
	}
	return Dimension{value: dimension}, nil
}

// IsValid reports whether d is canonical and valid. The zero value is valid
// and dimensionless.
func (d Dimension) IsValid() bool { return d.value.IsValid() }

// IsDimensionless reports whether d has no base-dimension factors.
func (d Dimension) IsDimensionless() bool { return d.value.IsDimensionless() }

// Equal reports whether d and other have exactly the same canonical meaning.
func (d Dimension) Equal(other Dimension) bool { return d.value.Equal(other.value) }

// Factors returns an owned canonical factor list.
func (d Dimension) Factors() []DimensionFactor {
	factors := d.value.Factors()
	result := make([]DimensionFactor, len(factors))
	for index, factor := range factors {
		result[index] = DimensionFactor{ID: DimensionID(factor.ID), Exponent: factor.Exponent}
	}
	return result
}

// Rational is an exact arbitrary-precision rational number. It is used for
// Unit conversion scales and never accepts or converts through float64. Its
// zero value is the meaningful rational zero.
type Rational struct{ value internalrational.Rational }

// NewRational creates a normalized exact ratio from canonical base-10 integer
// text. denominator must be positive and non-zero.
func NewRational(numerator, denominator string) (Rational, error) {
	if numerator == "" || !isCanonicalInteger(numerator) {
		return Rational{}, fmt.Errorf("%w: rational numerator must be canonical base-10 text", ErrInvalidArgument)
	}
	if denominator == "" || denominator[0] == '-' || !isCanonicalInteger(denominator) || denominator == "0" {
		return Rational{}, fmt.Errorf("%w: rational denominator must be positive canonical base-10 text", ErrInvalidArgument)
	}
	n, ok := new(big.Int).SetString(numerator, 10)
	if !ok {
		return Rational{}, fmt.Errorf("%w: rational numerator is invalid", ErrInvalidArgument)
	}
	d, ok := new(big.Int).SetString(denominator, 10)
	if !ok {
		return Rational{}, fmt.Errorf("%w: rational denominator is invalid", ErrInvalidArgument)
	}
	return Rational{value: internalrational.New(n, d)}, nil
}

// RationalFromInt64 returns the exact rational representation of value.
func RationalFromInt64(value int64) Rational {
	return Rational{value: internalrational.FromInt64(value)}
}

// OneRational returns the exact rational value one.
func OneRational() Rational { return Rational{value: internalrational.One()} }

// Numerator returns canonical base-10 numerator text.
func (r Rational) Numerator() string { return r.value.Numerator().String() }

// Denominator returns canonical positive base-10 denominator text.
func (r Rational) Denominator() string { return r.value.Denominator().String() }

// String returns the canonical numerator/denominator representation.
func (r Rational) String() string { return r.Numerator() + "/" + r.Denominator() }

// Sign reports whether r is negative, zero, or positive.
func (r Rational) Sign() int { return r.value.Sign() }

// IsZero reports whether r is exactly zero.
func (r Rational) IsZero() bool { return r.value.IsZero() }

// Equal reports exact rational equality.
func (r Rational) Equal(other Rational) bool { return r.value.Equal(other.value) }

// ConversionFactor is one conversion space raised to a non-zero integer
// power in a composed Unit's canonical conversion signature.
type ConversionFactor struct {
	ID       ConversionSpaceID
	Exponent int
}

// IsValid reports structural validity before canonical composed-unit checks.
func (f ConversionFactor) IsValid() bool { return f.ID.IsValid() && f.Exponent != 0 }

// DimensionlessUnit returns the valid canonical unit with no dimension,
// symbolic factors, or conversion signature.
func DimensionlessUnit() Unit { return Unit{value: internalunit.DimensionlessUnit()} }

// NewUnit constructs one symbolic Unit. The scale is exact within its named
// conversion space; BCG supplies no built-in business-unit catalog.
func NewUnit(
	id UnitID,
	dimension Dimension,
	conversionSpace ConversionSpaceID,
	scale Rational,
) (Unit, error) {
	result, err := internalunit.NewUnitChecked(
		internalunit.UnitID(id),
		dimension.value,
		internalunit.ConversionSpaceID(conversionSpace),
		scale.value,
	)
	if err != nil {
		return Unit{}, fmt.Errorf("%w: unit: %v", ErrInvalidArgument, err)
	}
	return Unit{value: result}, nil
}

// NewComposedUnit constructs a Unit from its complete canonical portable
// representation. Factors and conversion factors must already be strictly
// ordered by ID with non-zero exponents, exactly as .bcg v1 represents them.
func NewComposedUnit(
	dimension Dimension,
	factors []UnitFactor,
	conversionFactors []ConversionFactor,
	scale Rational,
) (Unit, error) {
	internalFactors := make([]internalunit.SymbolicFactor, len(factors))
	for index, factor := range factors {
		internalFactors[index] = internalunit.SymbolicFactor{ID: internalunit.UnitID(factor.ID), Exponent: factor.Exponent}
	}
	internalConversions := make([]internalunit.ConversionFactorSpec, len(conversionFactors))
	for index, factor := range conversionFactors {
		internalConversions[index] = internalunit.ConversionFactorSpec{ID: internalunit.ConversionSpaceID(factor.ID), Exponent: factor.Exponent}
	}
	result, err := internalunit.NewComposedUnitChecked(dimension.value, internalFactors, internalConversions, scale.value)
	if err != nil {
		return Unit{}, fmt.Errorf("%w: composed unit: %v", ErrInvalidArgument, err)
	}
	return Unit{value: result}, nil
}
