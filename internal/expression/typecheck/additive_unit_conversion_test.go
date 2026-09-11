package typecheck

import (
	"math/big"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestInferArithmetic_IntegerAddWithSameUnitRemainsInteger(t *testing.T) {
	meter, _ := additiveConversionTestUnits()

	got, err := InferArithmetic(
		ast.ArithmeticAdd,
		typing.NewWithUnit(value.Integer, meter),
		typing.NewWithUnit(value.Integer, meter),
	)
	if err != nil {
		t.Fatal(err)
	}

	requireAdditiveConversionType(
		t,
		got,
		value.Integer,
		meter,
	)
}

func TestInferArithmetic_IntegerAddWithUnitConversionPromotesToDecimal(t *testing.T) {
	meter, centimeter := additiveConversionTestUnits()

	got, err := InferArithmetic(
		ast.ArithmeticAdd,
		typing.NewWithUnit(value.Integer, meter),
		typing.NewWithUnit(value.Integer, centimeter),
	)
	if err != nil {
		t.Fatal(err)
	}

	requireAdditiveConversionType(
		t,
		got,
		value.Decimal,
		meter,
	)
}

func TestInferArithmetic_IntegerSubtractWithUnitConversionPromotesToDecimal(t *testing.T) {
	meter, centimeter := additiveConversionTestUnits()

	got, err := InferArithmetic(
		ast.ArithmeticSubtract,
		typing.NewWithUnit(value.Integer, meter),
		typing.NewWithUnit(value.Integer, centimeter),
	)
	if err != nil {
		t.Fatal(err)
	}

	requireAdditiveConversionType(
		t,
		got,
		value.Decimal,
		meter,
	)
}

func TestInferArithmetic_IntegerAddWithReverseUnitConversionPromotesToDecimal(t *testing.T) {
	meter, centimeter := additiveConversionTestUnits()

	got, err := InferArithmetic(
		ast.ArithmeticAdd,
		typing.NewWithUnit(value.Integer, centimeter),
		typing.NewWithUnit(value.Integer, meter),
	)
	if err != nil {
		t.Fatal(err)
	}

	requireAdditiveConversionType(
		t,
		got,
		value.Decimal,
		centimeter,
	)
}

func TestUnify_IntegerSameUnitsRemainInteger(t *testing.T) {
	meter, _ := additiveConversionTestUnits()

	got, err := Unify(
		typing.NewWithUnit(value.Integer, meter),
		typing.NewWithUnit(value.Integer, meter),
	)
	if err != nil {
		t.Fatal(err)
	}

	requireAdditiveConversionType(
		t,
		got,
		value.Integer,
		meter,
	)
}

func TestUnify_IntegerDifferentConvertibleUnitsPromotesToDecimal(t *testing.T) {
	meter, centimeter := additiveConversionTestUnits()

	got, err := Unify(
		typing.NewWithUnit(value.Integer, meter),
		typing.NewWithUnit(value.Integer, centimeter),
	)
	if err != nil {
		t.Fatal(err)
	}

	requireAdditiveConversionType(
		t,
		got,
		value.Decimal,
		meter,
	)
}

func TestUnify_IntegerDifferentConvertibleUnitsPreservesQualifiers(t *testing.T) {
	meter, centimeter := additiveConversionTestUnits()

	left := typing.NewWithUnit(value.Integer, meter).
		WithTemporalMode(typing.TemporalSeries)

	right := typing.NewWithUnit(value.Integer, centimeter).
		WithTemporalMode(typing.TemporalSeries).
		WithNullable(true)

	got, err := Unify(left, right)
	if err != nil {
		t.Fatal(err)
	}

	requireAdditiveConversionType(
		t,
		got,
		value.Decimal,
		meter,
	)

	if got.TemporalMode() != typing.TemporalSeries {
		t.Fatalf(
			"temporal mode = %s, want Series",
			got.TemporalMode(),
		)
	}

	if !got.Nullable() {
		t.Fatal("unified type must remain nullable")
	}
}

func additiveConversionTestUnits() (unit.Unit, unit.Unit) {
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

	return meter, centimeter
}

func requireAdditiveConversionType(
	t *testing.T,
	got typing.Type,
	wantValueType value.Type,
	wantUnit unit.Unit,
) {
	t.Helper()

	if got.ValueType() != wantValueType {
		t.Fatalf(
			"value type = %s, want %s",
			got.ValueType(),
			wantValueType,
		)
	}

	gotUnit, ok := got.Unit()
	if !ok {
		t.Fatal("result must have a unit")
	}

	if !gotUnit.Equal(wantUnit) {
		t.Fatal("result unit does not match expected left unit")
	}
}
