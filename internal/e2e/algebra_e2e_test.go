package e2e_test

import (
	"math/big"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
)

func TestE2E_RationalExactArithmetic(t *testing.T) {
	half := rational.New(
		big.NewInt(1),
		big.NewInt(2),
	)

	third := rational.New(
		big.NewInt(1),
		big.NewInt(3),
	)

	t.Run("one half plus one third", func(t *testing.T) {
		got := half.Add(third)

		want := rational.New(
			big.NewInt(5),
			big.NewInt(6),
		)

		if got.Cmp(want) != 0 {
			t.Fatalf("got %s want %s", got, want)
		}
	})

	t.Run("one half times one third", func(t *testing.T) {
		got := half.Mul(third)

		want := rational.New(
			big.NewInt(1),
			big.NewInt(6),
		)

		if got.Cmp(want) != 0 {
			t.Fatalf("got %s want %s", got, want)
		}
	})

	t.Run("one half divided by one third", func(t *testing.T) {
		got, err := half.Div(third)
		if err != nil {
			t.Fatal(err)
		}

		want := rational.New(
			big.NewInt(3),
			big.NewInt(2),
		)

		if got.Cmp(want) != 0 {
			t.Fatalf("got %s want %s", got, want)
		}
	})

	t.Run("exact cancellation", func(t *testing.T) {
		got := third.Mul(
			rational.FromInt64(3),
		)

		if got.Cmp(rational.One()) != 0 {
			t.Fatalf("got %s want 1", got)
		}
	})
}

func TestE2E_RationalNormalization(t *testing.T) {
	cases := []struct {
		name string
		n    int64
		d    int64
		want rational.Rational
	}{
		{
			name: "two fourths",
			n:    2,
			d:    4,
			want: rational.New(
				big.NewInt(1),
				big.NewInt(2),
			),
		},
		{
			name: "negative denominator",
			n:    1,
			d:    -2,
			want: rational.New(
				big.NewInt(-1),
				big.NewInt(2),
			),
		},
		{
			name: "both negative",
			n:    -2,
			d:    -4,
			want: rational.New(
				big.NewInt(1),
				big.NewInt(2),
			),
		},
		{
			name: "zero",
			n:    0,
			d:    999,
			want: rational.New(
				big.NewInt(0),
				big.NewInt(1),
			),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rational.New(
				big.NewInt(tc.n),
				big.NewInt(tc.d),
			)

			if got.Cmp(tc.want) != 0 {
				t.Fatalf(
					"got %s want %s",
					got,
					tc.want,
				)
			}
		})
	}
}

func TestE2E_DimensionAlgebraLaws(t *testing.T) {
	length := unit.Base("length")
	mass := unit.Base("mass")
	time := unit.Base("time")

	t.Run("multiplication commutative", func(t *testing.T) {
		left := length.Mul(mass)
		right := mass.Mul(length)

		if !left.Equal(right) {
			t.Fatal("L*M must equal M*L")
		}
	})

	t.Run("multiplication associative", func(t *testing.T) {
		left := length.Mul(mass).Mul(time)
		right := length.Mul(
			mass.Mul(time),
		)

		if !left.Equal(right) {
			t.Fatal("(L*M)*T must equal L*(M*T)")
		}
	})

	t.Run("self division dimensionless", func(t *testing.T) {
		got := length.Div(length)

		if !got.IsDimensionless() {
			t.Fatal("L/L must be dimensionless")
		}
	})

	t.Run("multiply then divide cancels", func(t *testing.T) {
		got := length.
			Mul(mass).
			Div(mass)

		if !got.Equal(length) {
			t.Fatal("(L*M)/M must equal L")
		}
	})

	t.Run("square divided by base", func(t *testing.T) {
		square := length.Mul(length)
		got := square.Div(length)

		if !got.Equal(length) {
			t.Fatal("L²/L must equal L")
		}
	})

	t.Run("cube divided by square", func(t *testing.T) {
		square := length.Mul(length)
		cube := square.Mul(length)

		got := cube.Div(square)

		if !got.Equal(length) {
			t.Fatal("L³/L² must equal L")
		}
	})

	t.Run("quotient multiplication cancellation", func(t *testing.T) {
		currency := unit.Base("currency")
		count := unit.Base("count")

		got := currency.
			Div(count).
			Mul(count)

		if !got.Equal(currency) {
			t.Fatal("(Currency/Count)*Count must equal Currency")
		}
	})
}

func TestE2E_UnitIdentityLaws(t *testing.T) {
	u := testUnits()

	one := unit.DimensionlessUnit()

	all := []struct {
		name string
		u    unit.Unit
	}{
		{"meter", u.meter},
		{"centimeter", u.centimeter},
		{"kilogram", u.kilogram},
		{"gram", u.gram},
		{"item", u.item},
		{"rub", u.rub},
		{"rub per item", u.rubPerItem},
	}

	for _, tc := range all {
		t.Run(tc.name+" multiply one", func(t *testing.T) {
			got := tc.u.Mul(one)

			if !got.Equal(tc.u) {
				t.Fatal("U*1 must equal U")
			}
		})

		t.Run(tc.name+" one multiply", func(t *testing.T) {
			got := one.Mul(tc.u)

			if !got.Equal(tc.u) {
				t.Fatal("1*U must equal U")
			}
		})

		t.Run(tc.name+" divide one", func(t *testing.T) {
			got, err := tc.u.Div(one)
			if err != nil {
				t.Fatal(err)
			}

			if !got.Equal(tc.u) {
				t.Fatal("U/1 must equal U")
			}
		})

		t.Run(tc.name+" divide itself", func(t *testing.T) {
			got, err := tc.u.Div(tc.u)
			if err != nil {
				t.Fatal(err)
			}

			if !got.IsDimensionless() {
				t.Fatal("U/U must be dimensionless")
			}

			if got.Scale().Cmp(rational.One()) != 0 {
				t.Fatalf(
					"U/U scale got %s want 1",
					got.Scale(),
				)
			}
		})
	}
}

func TestE2E_UnitMultiplicationAssociativity(t *testing.T) {
	u := testUnits()

	units := []unit.Unit{
		u.meter,
		u.centimeter,
		u.kilogram,
		u.gram,
		u.item,
		u.rub,
		u.rubPerItem,
		unit.DimensionlessUnit(),
	}

	for i, a := range units {
		for j, b := range units {
			for k, c := range units {
				left := a.Mul(b).Mul(c)
				right := a.Mul(b.Mul(c))

				if !left.Equal(right) {
					t.Fatalf(
						"associativity failed for [%d,%d,%d]",
						i,
						j,
						k,
					)
				}
			}
		}
	}
}

func TestE2E_UnitMultiplicationCommutativity(t *testing.T) {
	u := testUnits()

	units := []unit.Unit{
		u.meter,
		u.centimeter,
		u.kilogram,
		u.gram,
		u.item,
		u.rub,
		u.rubPerItem,
		unit.DimensionlessUnit(),
	}

	for i, a := range units {
		for j, b := range units {
			left := a.Mul(b)
			right := b.Mul(a)

			if !left.Equal(right) {
				t.Fatalf(
					"commutativity failed for [%d,%d]",
					i,
					j,
				)
			}
		}
	}
}

func TestE2E_UnitBusinessCancellationLaws(t *testing.T) {
	u := testUnits()

	t.Run("rub per item times item", func(t *testing.T) {
		got := u.rubPerItem.Mul(u.item)

		if !got.Equal(u.rub) {
			t.Fatal("RUB/item * item must equal RUB")
		}
	})

	t.Run("rub per kg times kg", func(t *testing.T) {
		rubPerKg, err := u.rub.Div(u.kilogram)
		if err != nil {
			t.Fatal(err)
		}

		got := rubPerKg.Mul(u.kilogram)

		if !got.Equal(u.rub) {
			t.Fatal("RUB/kg * kg must equal RUB")
		}
	})

	t.Run("square meter divided by meter", func(t *testing.T) {
		squareMeter := u.meter.Mul(u.meter)

		got, err := squareMeter.Div(u.meter)
		if err != nil {
			t.Fatal(err)
		}

		if !got.Equal(u.meter) {
			t.Fatal("m²/m must equal m")
		}
	})

	t.Run("cubic meter divided by square meter", func(t *testing.T) {
		squareMeter := u.meter.Mul(u.meter)
		cubicMeter := squareMeter.Mul(u.meter)

		got, err := cubicMeter.Div(squareMeter)
		if err != nil {
			t.Fatal(err)
		}

		if !got.Equal(u.meter) {
			t.Fatal("m³/m² must equal m")
		}
	})
}

func TestE2E_UnitScaleCompositionIsExact(t *testing.T) {
	u := testUnits()

	t.Run("centimeter squared", func(t *testing.T) {
		squareCentimeter :=
			u.centimeter.Mul(u.centimeter)

		wantScale := rational.New(
			big.NewInt(1),
			big.NewInt(10000),
		)

		if squareCentimeter.Scale().Cmp(wantScale) != 0 {
			t.Fatalf(
				"cm² scale got %s want %s",
				squareCentimeter.Scale(),
				wantScale,
			)
		}
	})

	t.Run("gram per kilogram", func(t *testing.T) {
		got, err := u.gram.Div(u.kilogram)
		if err != nil {
			t.Fatal(err)
		}

		wantScale := rational.New(
			big.NewInt(1),
			big.NewInt(1000),
		)

		if got.Scale().Cmp(wantScale) != 0 {
			t.Fatalf(
				"g/kg scale got %s want %s",
				got.Scale(),
				wantScale,
			)
		}

		if !got.IsDimensionless() {
			t.Fatal("g/kg must be dimensionless")
		}
	})

	t.Run("kilogram per gram", func(t *testing.T) {
		got, err := u.kilogram.Div(u.gram)
		if err != nil {
			t.Fatal(err)
		}

		wantScale := rational.FromInt64(1000)

		if got.Scale().Cmp(wantScale) != 0 {
			t.Fatalf(
				"kg/g scale got %s want %s",
				got.Scale(),
				wantScale,
			)
		}
	})
}

func TestE2E_UnitDivisionRoundTrip(t *testing.T) {
	u := testUnits()

	units := []unit.Unit{
		u.meter,
		u.centimeter,
		u.kilogram,
		u.gram,
		u.item,
		u.rub,
	}

	for i, a := range units {
		for j, b := range units {
			quotient, err := a.Div(b)
			if err != nil {
				t.Fatalf(
					"division [%d,%d]: %v",
					i,
					j,
					err,
				)
			}

			got := quotient.Mul(b)

			if !got.Equal(a) {
				t.Fatalf(
					"(A/B)*B != A for [%d,%d]",
					i,
					j,
				)
			}
		}
	}
}

func TestE2E_CompatibleUnitsPreserveDimensions(t *testing.T) {
	u := testUnits()

	if !u.meter.Dimension().Equal(
		u.centimeter.Dimension(),
	) {
		t.Fatal("meter and centimeter must share dimension")
	}

	if !u.kilogram.Dimension().Equal(
		u.gram.Dimension(),
	) {
		t.Fatal("kilogram and gram must share dimension")
	}

	if u.meter.Dimension().Equal(
		u.kilogram.Dimension(),
	) {
		t.Fatal("length and mass must not share dimension")
	}

	if u.rub.Dimension().Equal(
		u.item.Dimension(),
	) {
		t.Fatal("currency and count must not share dimension")
	}
}
