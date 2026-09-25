package typing

import (
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// Type describes the statically known result type of an expression.
//
// Type carries four independent pieces of static information:
//
//   - ValueType;
//   - optional Unit;
//   - TemporalMode;
//   - Nullability.
//
// Examples:
//
//	Integer
//	Decimal<RUB>
//	Decimal<RUB/item>
//	Boolean
//	Series<Decimal<RUB>>
//	Nullable<Decimal<RUB>>
//
// Nullable means that evaluation may successfully produce Null.
//
// Undefined is intentionally not represented here. Undefined is an
// evaluation outcome that may arise for reasons such as MissingValue,
// DependencyUndefined or EmptyAggregation and is not part of the logical
// value type.
type Type struct {
	valueType value.Type

	unit    unit.Unit
	hasUnit bool

	temporalMode TemporalMode
	nullable     bool
}

// New constructs a non-nullable scalar static type without a unit.
//
// Scalar/non-nullable is the default for literals and ordinary intermediate
// values. Property references use the more explicit constructors when their
// schema semantics differ.
func New(valueType value.Type) Type {
	return NewQualified(
		valueType,
		unit.Unit{},
		false,
		TemporalScalar,
		false,
	)
}

// NewWithUnit constructs a non-nullable scalar static type with a unit.
func NewWithUnit(
	valueType value.Type,
	u unit.Unit,
) Type {
	return NewQualified(
		valueType,
		u,
		true,
		TemporalScalar,
		false,
	)
}

// NewQualified constructs a fully qualified static type.
func NewQualified(
	valueType value.Type,
	u unit.Unit,
	hasUnit bool,
	temporalMode TemporalMode,
	nullable bool,
) Type {
	if !valueType.IsValid() {
		panic("typing: value type must be valid")
	}

	if hasUnit && !u.IsValid() {
		panic("typing: unit must be valid")
	}

	if !temporalMode.IsValid() {
		panic("typing: temporal mode must be valid")
	}

	if !hasUnit {
		u = unit.Unit{}
	}

	return Type{
		valueType:    valueType,
		unit:         u,
		hasUnit:      hasUnit,
		temporalMode: temporalMode,
		nullable:     nullable,
	}
}

// ValueType returns the underlying ValueType.
func (t Type) ValueType() value.Type {
	return t.valueType
}

// Unit returns the optional Unit.
func (t Type) Unit() (unit.Unit, bool) {
	return t.unit, t.hasUnit
}

// HasUnit reports whether t carries a Unit.
func (t Type) HasUnit() bool {
	return t.hasUnit
}

// TemporalMode returns the temporal shape of the expression result.
func (t Type) TemporalMode() TemporalMode {
	return t.temporalMode
}

// Nullable reports whether evaluation may successfully produce Null.
func (t Type) Nullable() bool {
	return t.nullable
}

// WithNullable returns a copy of t with the requested nullability.
func (t Type) WithNullable(nullable bool) Type {
	if !t.IsValid() {
		panic("typing: cannot modify invalid type")
	}

	t.nullable = nullable
	return t
}

// WithTemporalMode returns a copy of t with the requested temporal mode.
func (t Type) WithTemporalMode(mode TemporalMode) Type {
	if !t.IsValid() {
		panic("typing: cannot modify invalid type")
	}

	if !mode.IsValid() {
		panic("typing: temporal mode must be valid")
	}

	t.temporalMode = mode
	return t
}

// IsValid reports whether t is structurally valid.
//
// Semantic restrictions such as whether a particular ValueType may carry a
// Unit are enforced by the static type checker.
func (t Type) IsValid() bool {
	if !t.valueType.IsValid() {
		return false
	}

	if !t.temporalMode.IsValid() {
		return false
	}

	if t.hasUnit {
		return t.unit.IsValid()
	}

	return true
}
