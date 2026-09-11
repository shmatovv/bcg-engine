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
