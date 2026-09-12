package decimal

import (
	"errors"
	"math/big"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/types/precision"
)

func TestMulChecksNormalizedScale(t *testing.T) {
	a, b := New(big.NewInt(2), MaxScale), New(big.NewInt(5), 1)
	got, err := a.Mul(b)
	if err != nil || !got.Equal(New(big.NewInt(1), MaxScale)) {
		t.Fatalf("representable product rejected: %v", err)
	}
	if _, err := a.Mul(New(big.NewInt(3), 1)); !errors.Is(err, ErrScaleOutOfRange) {
		t.Fatalf("unrepresentable product: %v", err)
	}
}

func TestRoundingSignsAndTies(t *testing.T) {
	for _, tc := range []struct {
		mode               precision.Mode
		positive, negative string
	}{
		{precision.Truncate, "2", "-2"},
		{precision.Floor, "2", "-3"},
		{precision.Ceil, "3", "-2"},
		{precision.HalfUp, "3", "-3"},
		{precision.HalfEven, "2", "-2"},
	} {
		for _, sign := range []int64{-1, 1} {
			want := tc.positive
			if sign < 0 {
				want = tc.negative
			}
			policy := precision.New(0, tc.mode)
			for _, denominator := range []int64{-2, 2} {
				got, err := FromInt64(sign*5*(denominator/2)).Div(FromInt64(denominator), policy)
				if err != nil || got.String() != want {
					t.Fatalf("%s sign %d divisor %d: %s, %v", tc.mode, sign, denominator, got, err)
				}
			}
			got, err := New(big.NewInt(sign*25), 1).Round(policy)
			if err != nil || got.String() != want {
				t.Fatalf("Round %s: %s, %v", tc.mode, got, err)
			}
		}
	}
	for _, input := range []string{"3.5", "-3.5"} {
		got, _ := MustParse(input).Round(precision.New(0, precision.HalfEven))
		if got.Abs().String() != "4" {
			t.Fatalf("odd tie %s: %s", input, got)
		}
	}
}

func TestDecimalOwnership(t *testing.T) {
	coefficient, _ := new(big.Int).SetString("123456789012345678901234567890123456789", 10)
	d := New(coefficient, 7)
	want := d.String()
	coefficient.SetInt64(1)
	d.Coefficient().SetInt64(2)
	_ = d.Neg().Add(d).Sub(d).Abs()
	if d.String() != want {
		t.Fatal("mutable coefficient alias")
	}
}

// TestExactArithmeticMatchesBigRat uses math/big.Rat as an independent
// rational oracle. It deliberately covers only exact operations: division is
// governed by the Engine's explicit rounding policy and has dedicated tie and
// sign tests above.
func TestExactArithmeticMatchesBigRat(t *testing.T) {
	state := uint64(0x9e3779b97f4a7c15)
	next := func() int64 {
		state = state*6364136223846793005 + 1442695040888963407
		return int64(state >> 1)
	}

	for index := 0; index < 1_000; index++ {
		left := New(big.NewInt(next()%2_000_000_001-1_000_000_000), uint64(index%19))
		right := New(big.NewInt(next()%2_000_000_001-1_000_000_000), uint64((index*7)%19))

		leftRat := decimalRat(left)
		rightRat := decimalRat(right)

		assertDecimalRatEqual(t, "add", left.Add(right), new(big.Rat).Add(leftRat, rightRat))
		assertDecimalRatEqual(t, "sub", left.Sub(right), new(big.Rat).Sub(leftRat, rightRat))

		product, err := left.Mul(right)
		if err != nil {
			t.Fatalf("exact product %d: %v", index, err)
		}
		assertDecimalRatEqual(t, "mul", product, new(big.Rat).Mul(leftRat, rightRat))
		if sign := left.Cmp(right); sign != leftRat.Cmp(rightRat) {
			t.Fatalf("comparison %d: got %d, want %d", index, sign, leftRat.Cmp(rightRat))
		}
	}
}

func decimalRat(value Decimal) *big.Rat {
	denominator := new(big.Int).Exp(big.NewInt(10), new(big.Int).SetUint64(value.Scale()), nil)
	return new(big.Rat).SetFrac(value.Coefficient(), denominator)
}

func assertDecimalRatEqual(t *testing.T, operation string, got Decimal, want *big.Rat) {
	t.Helper()
	if decimalRat(got).Cmp(want) != 0 {
		t.Fatalf("%s: got %s, want %s", operation, got, want)
	}
}

func FuzzParseRoundTrip(f *testing.F) {
	for _, s := range []string{"0", "-0.00", "-2.5", "123456789012345678901234567890.12345", "+1.00", "bad"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		d, err := Parse(s)
		if err != nil {
			return
		}
		again, err := Parse(d.String())
		if err != nil || !d.Equal(again) {
			t.Fatalf("round trip %q", s)
		}
		if !d.Add(d.Neg()).IsZero() {
			t.Fatalf("additive inverse %q", s)
		}
	})
}
