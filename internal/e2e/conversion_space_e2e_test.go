package e2e_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
	"github.com/shmatovv/bcg-engine/v2/internal/types/rational"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func TestE2E_ConversionSpaces_DimensionDoesNotImplyAutomaticConversion(
	t *testing.T,
) {
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

	if !rub.Dimension().Equal(eur.Dimension()) {
		t.Fatal("RUB and EUR must have the same currency dimension")
	}

	if rub.CanConvertTo(eur) {
		t.Fatal(
			"RUB and EUR must not be automatically convertible",
		)
	}

	if eur.CanConvertTo(rub) {
		t.Fatal(
			"EUR and RUB must not be automatically convertible",
		)
	}
}

func TestE2E_ConversionSpaces_FixedCurrencySubunitsAreConvertible(
	t *testing.T,
) {
	currency := unit.Base("currency")

	rub := unit.NewUnit(
		"rub",
		currency,
		"rub",
		rational.One(),
	)

	kopeck := unit.NewUnit(
		"kopeck",
		currency,
		"rub",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	eur := unit.NewUnit(
		"eur",
		currency,
		"eur",
		rational.One(),
	)

	euroCent := unit.NewUnit(
		"euro_cent",
		currency,
		"eur",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	if !rub.CanConvertTo(kopeck) {
		t.Fatal("RUB must be convertible to kopeck")
	}

	if !kopeck.CanConvertTo(rub) {
		t.Fatal("kopeck must be convertible to RUB")
	}

	if !eur.CanConvertTo(euroCent) {
		t.Fatal("EUR must be convertible to euro cent")
	}

	if !euroCent.CanConvertTo(eur) {
		t.Fatal("euro cent must be convertible to EUR")
	}

	if rub.CanConvertTo(eur) {
		t.Fatal("RUB must not be automatically convertible to EUR")
	}

	if kopeck.CanConvertTo(euroCent) {
		t.Fatal(
			"kopeck must not be automatically convertible to euro cent",
		)
	}
}

func TestE2E_ConversionSpaces_PhysicalScaleConversionStillWorks(
	t *testing.T,
) {
	mass := unit.Base("mass")

	kilogram := unit.NewUnit(
		"kilogram",
		mass,
		"mass",
		rational.One(),
	)

	gram := unit.NewUnit(
		"gram",
		mass,
		"mass",
		rational.New(
			big.NewInt(1),
			big.NewInt(1000),
		),
	)

	if !kilogram.CanConvertTo(gram) {
		t.Fatal("kilogram must be convertible to gram")
	}

	if !gram.CanConvertTo(kilogram) {
		t.Fatal("gram must be convertible to kilogram")
	}
}

func TestE2E_ConversionSpaces_CompoundCurrencyUnitsPreserveBoundary(
	t *testing.T,
) {
	currency := unit.Base("currency")
	count := unit.Base("count")

	rub := unit.NewUnit(
		"rub",
		currency,
		"rub",
		rational.One(),
	)

	kopeck := unit.NewUnit(
		"kopeck",
		currency,
		"rub",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	eur := unit.NewUnit(
		"eur",
		currency,
		"eur",
		rational.One(),
	)

	item := unit.NewUnit(
		"item",
		count,
		"count",
		rational.One(),
	)

	rubPerItem, err := rub.Div(item)
	if err != nil {
		t.Fatal(err)
	}

	kopeckPerItem, err := kopeck.Div(item)
	if err != nil {
		t.Fatal(err)
	}

	eurPerItem, err := eur.Div(item)
	if err != nil {
		t.Fatal(err)
	}

	if !rubPerItem.Dimension().Equal(
		eurPerItem.Dimension(),
	) {
		t.Fatal(
			"RUB/item and EUR/item must have the same dimension",
		)
	}

	if !rubPerItem.CanConvertTo(kopeckPerItem) {
		t.Fatal(
			"RUB/item must be convertible to kopeck/item",
		)
	}

	if rubPerItem.CanConvertTo(eurPerItem) {
		t.Fatal(
			"RUB/item must not be automatically convertible to EUR/item",
		)
	}
}

func TestE2E_ConversionSpaces_CompoundCancellationPreservesConversionSemantics(
	t *testing.T,
) {
	currency := unit.Base("currency")
	count := unit.Base("count")

	rub := unit.NewUnit(
		"rub",
		currency,
		"rub",
		rational.One(),
	)

	item := unit.NewUnit(
		"item",
		count,
		"count",
		rational.One(),
	)

	rubPerItem, err := rub.Div(item)
	if err != nil {
		t.Fatal(err)
	}

	reconstructedRub := rubPerItem.Mul(item)

	if !reconstructedRub.Dimension().Equal(
		rub.Dimension(),
	) {
		t.Fatal(
			"(RUB/item)*item must have currency dimension",
		)
	}

	if !reconstructedRub.CanConvertTo(rub) {
		t.Fatal(
			"(RUB/item)*item must remain automatically convertible to RUB",
		)
	}

	if !rub.CanConvertTo(reconstructedRub) {
		t.Fatal(
			"RUB must remain automatically convertible to reconstructed RUB",
		)
	}
}

func TestE2E_ConversionSpaces_ConvertRejectsContextualCurrencyConversion(
	t *testing.T,
) {
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

	_, err := unit.Convert(
		decimal.FromInt64(100),
		rub,
		eur,
		precision.New(2, precision.HalfEven),
	)

	if err == nil {
		t.Fatal("expected contextual conversion rejection")
	}

	if !errors.Is(
		err,
		unit.ErrConversionSpaceMismatch,
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestE2E_ConversionSpaces_ConvertAllowsFixedCurrencySubunitConversion(
	t *testing.T,
) {
	currency := unit.Base("currency")

	rub := unit.NewUnit(
		"rub",
		currency,
		"rub",
		rational.One(),
	)

	kopeck := unit.NewUnit(
		"kopeck",
		currency,
		"rub",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	got, err := unit.Convert(
		decimal.FromInt64(1),
		rub,
		kopeck,
		precision.New(2, precision.HalfEven),
	)
	if err != nil {
		t.Fatal(err)
	}

	want := decimal.FromInt64(100)

	if got.Cmp(want) != 0 {
		t.Fatalf(
			"unexpected conversion: got %s want %s",
			got.String(),
			want.String(),
		)
	}
}

func TestE2E_ConversionSpaces_TypecheckerRejectsCrossSpaceAddition(
	t *testing.T,
) {
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

	_, err := typecheck.InferArithmetic(
		ast.ArithmeticAdd,
		typing.NewWithUnit(value.Decimal, rub),
		typing.NewWithUnit(value.Decimal, eur),
	)

	if err == nil {
		t.Fatal("expected RUB + EUR to be rejected")
	}
}

func TestE2E_ConversionSpaces_TypecheckerRejectsCrossSpaceComparison(
	t *testing.T,
) {
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

	_, err := typecheck.InferComparison(
		ast.ComparisonEqual,
		typing.NewWithUnit(value.Decimal, rub),
		typing.NewWithUnit(value.Decimal, eur),
	)

	if err == nil {
		t.Fatal("expected RUB == EUR to be rejected")
	}
}

func TestE2E_ConversionSpaces_TypecheckerRejectsCrossSpaceUnification(
	t *testing.T,
) {
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

	_, err := typecheck.Unify(
		typing.NewWithUnit(value.Integer, rub),
		typing.NewWithUnit(value.Decimal, eur),
	)

	if err == nil {
		t.Fatal("expected RUB and EUR unification to fail")
	}

	var unifyError typecheck.UnifyError
	if !errors.As(err, &unifyError) {
		t.Fatalf(
			"unexpected error type %T",
			err,
		)
	}

	if unifyError.Kind() !=
		typecheck.UnifyErrorIncompatibleUnits {
		t.Fatalf(
			"unexpected error kind %s",
			unifyError.Kind(),
		)
	}
}

func TestE2E_ConversionSpaces_TypecheckerAllowsSameSpaceUnification(
	t *testing.T,
) {
	currency := unit.Base("currency")

	rub := unit.NewUnit(
		"rub",
		currency,
		"rub",
		rational.One(),
	)

	kopeck := unit.NewUnit(
		"kopeck",
		currency,
		"rub",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	got, err := typecheck.Unify(
		typing.NewWithUnit(value.Integer, rub),
		typing.NewWithUnit(value.Decimal, kopeck),
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.ValueType() != value.Decimal {
		t.Fatalf(
			"unexpected value type: got %s want %s",
			got.ValueType(),
			value.Decimal,
		)
	}

	gotUnit, ok := got.Unit()
	if !ok {
		t.Fatal("expected result unit")
	}

	if !gotUnit.Equal(rub) {
		t.Fatal(
			"unification must preserve the left unit representation",
		)
	}
}
