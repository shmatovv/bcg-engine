package typecheck_test

import (
	"errors"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/rational"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func TestInferNegatePreservesNumericType(t *testing.T) {
	mass := unit.Base("mass")

	kilogram := unit.NewUnit(
		"kilogram",
		mass,
		"mass",
		rational.One(),
	)

	cases := []struct {
		name    string
		operand typing.Type
	}{
		{
			name: "integer",
			operand: typing.New(
				value.Integer,
			),
		},
		{
			name: "decimal",
			operand: typing.New(
				value.Decimal,
			),
		},
		{
			name: "integer with unit",
			operand: typing.NewWithUnit(
				value.Integer,
				kilogram,
			),
		},
		{
			name: "decimal with unit",
			operand: typing.NewWithUnit(
				value.Decimal,
				kilogram,
			),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := typecheck.InferNegate(
				tc.operand,
			)

			if err != nil {
				t.Fatal(err)
			}

			if got.ValueType() !=
				tc.operand.ValueType() {
				t.Fatalf(
					"value type changed: got %s want %s",
					got.ValueType(),
					tc.operand.ValueType(),
				)
			}

			gotUnit, gotHasUnit := got.Unit()
			wantUnit, wantHasUnit :=
				tc.operand.Unit()

			if gotHasUnit != wantHasUnit {
				t.Fatalf(
					"has unit changed: got %v want %v",
					gotHasUnit,
					wantHasUnit,
				)
			}

			if wantHasUnit &&
				!gotUnit.Equal(wantUnit) {
				t.Fatal("unit changed")
			}
		})
	}
}

func TestInferNegateRejectsNonNumericTypes(
	t *testing.T,
) {
	cases := []value.Type{
		value.Bool,
		value.String,
		value.Date,
		value.DateTime,
		value.Duration,
	}

	for _, valueType := range cases {
		t.Run(valueType.String(), func(t *testing.T) {
			_, err := typecheck.InferNegate(
				typing.New(valueType),
			)

			if err == nil {
				t.Fatal("expected error")
			}

			var unaryError typecheck.UnaryError

			if !errors.As(err, &unaryError) {
				t.Fatalf(
					"unexpected error type %T",
					err,
				)
			}

			if unaryError.Kind() !=
				typecheck.UnaryErrorNonNumericOperand {
				t.Fatalf(
					"unexpected error kind %s",
					unaryError.Kind(),
				)
			}
		})
	}
}
