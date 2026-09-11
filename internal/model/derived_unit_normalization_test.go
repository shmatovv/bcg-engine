// internal/model/derived_unit_normalization_test.go

package model

import (
	"math/big"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestDraftValidate_DerivedExpressionAllowsConvertibleDecimalUnit(
	t *testing.T,
) {
	entityTypeID := identity.New()
	definitionID := identity.New()

	length := unit.Base("length")

	meter := unit.NewUnit(
		"meter",
		length,
		"length",
		rational.One(),
	)

	centimeter := unit.NewUnit(
		"centimeter",
		length,
		"length",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	squareMeter := meter.Mul(meter)

	expression := ast.Multiply(
		ast.NewConstant(
			value.NewTypedWithUnit(
				value.NewDecimal(decimal.One()),
				meter,
			),
		),
		ast.NewConstant(
			value.NewTypedWithUnit(
				value.NewDecimal(decimal.One()),
				centimeter,
			),
		),
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityTypeID),
		identity.UUID{},
		false,
		"Area",
		value.Decimal,
		squareMeter,
		true,
		schema.TemporalScalar,
		schema.DerivedComputation(expression),
	)

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			entityTypeID,
			"Item",
			nil,
			[]identity.UUID{
				definitionID,
			},
		),
	)

	draft.AddPropertyDefinition(definition)

	report := draft.Validate()

	if !report.OK() {
		t.Fatalf(
			"convertible derived unit rejected: %v",
			report.Issues(),
		)
	}
}

func TestDraftValidate_DerivedExpressionRejectsDifferentConversionSpace(
	t *testing.T,
) {
	entityTypeID := identity.New()
	definitionID := identity.New()

	currency := unit.Base("currency")

	rub := unit.NewUnit(
		"rub",
		currency,
		"rub",
		rational.One(),
	)

	eur := unit.NewUnit(
		"eur",
		currency,
		"eur",
		rational.One(),
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityTypeID),
		identity.UUID{},
		false,
		"Price",
		value.Decimal,
		eur,
		true,
		schema.TemporalScalar,
		schema.DerivedComputation(
			ast.NewConstant(
				value.NewTypedWithUnit(
					value.NewDecimal(decimal.One()),
					rub,
				),
			),
		),
	)

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			entityTypeID,
			"Item",
			nil,
			[]identity.UUID{
				definitionID,
			},
		),
	)

	draft.AddPropertyDefinition(definition)

	requireValidationIssue(
		t,
		draft.Validate(),
		ValidationIssueInvalidDerivedExpression,
	)
}

func TestDraftValidate_DerivedIntegerAllowsIntegralUnitNormalization(
	t *testing.T,
) {
	entityTypeID := identity.New()
	definitionID := identity.New()

	length := unit.Base("length")

	meter := unit.NewUnit(
		"meter",
		length,
		"length",
		rational.One(),
	)

	centimeter := unit.NewUnit(
		"centimeter",
		length,
		"length",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityTypeID),
		identity.UUID{},
		false,
		"Length",
		value.Integer,
		centimeter,
		true,
		schema.TemporalScalar,
		schema.DerivedComputation(
			ast.NewConstant(
				value.NewTypedWithUnit(
					value.NewInt64(2),
					meter,
				),
			),
		),
	)

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			entityTypeID,
			"Item",
			nil,
			[]identity.UUID{
				definitionID,
			},
		),
	)

	draft.AddPropertyDefinition(definition)

	report := draft.Validate()

	if !report.OK() {
		t.Fatalf(
			"integral integer unit normalization rejected: %v",
			report.Issues(),
		)
	}
}

func TestDraftValidate_DerivedIntegerRejectsFractionalUnitNormalization(
	t *testing.T,
) {
	entityTypeID := identity.New()
	definitionID := identity.New()

	length := unit.Base("length")

	meter := unit.NewUnit(
		"meter",
		length,
		"length",
		rational.One(),
	)

	centimeter := unit.NewUnit(
		"centimeter",
		length,
		"length",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityTypeID),
		identity.UUID{},
		false,
		"Length",
		value.Integer,
		meter,
		true,
		schema.TemporalScalar,
		schema.DerivedComputation(
			ast.NewConstant(
				value.NewTypedWithUnit(
					value.NewInt64(50),
					centimeter,
				),
			),
		),
	)

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			entityTypeID,
			"Item",
			nil,
			[]identity.UUID{
				definitionID,
			},
		),
	)

	draft.AddPropertyDefinition(definition)

	requireValidationIssue(
		t,
		draft.Validate(),
		ValidationIssueInvalidDerivedExpression,
	)
}
