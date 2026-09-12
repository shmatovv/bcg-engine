package series

import (
	"fmt"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func at(t *testing.T, text string) datetime.DateTime {
	t.Helper()
	result, err := datetime.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func requirePanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestPointSeriesOwnershipAndAxis(t *testing.T) {
	points := []Point{
		NewPoint(at(t, "2026-01-01T00:00:00Z"), Defined(value.NewInt64(1))),
		NewPoint(at(t, "2026-01-02T00:00:00Z"), Null()),
	}
	stored := NewPointSeries(value.Integer, unitless(), false, points)
	points[0] = Point{}
	if !stored.IsValid() || len(stored.Points()) != 2 || !stored.Points()[1].Sample().IsNull() {
		t.Fatal("Point Series did not own its source slice")
	}
	returned := stored.Points()
	returned[0] = Point{}
	if !stored.IsValid() || len(stored.Points()) != 2 {
		t.Fatal("Point Series exposed mutable backing storage")
	}
	if !stored.AxisEqual(NewPointSeries(value.Integer, unitless(), false, stored.Points())) {
		t.Fatal("equal Point axes did not match")
	}
	if !NewPointSeries(value.Integer, unitless(), false, nil).IsValid() {
		t.Fatal("empty Point Series is invalid")
	}
	requirePanic(t, func() {
		NewPointSeries(value.Integer, unitless(), false, []Point{
			NewPoint(at(t, "2026-01-02T00:00:00Z"), Defined(value.NewInt64(1))),
			NewPoint(at(t, "2026-01-01T00:00:00Z"), Defined(value.NewInt64(2))),
		})
	})
	requirePanic(t, func() {
		NewPointSeries(value.Integer, unitless(), false, []Point{
			NewPoint(at(t, "2026-01-01T00:00:00Z"), Defined(value.NewInt64(1))),
			NewPoint(at(t, "2026-01-01T00:00:00Z"), Defined(value.NewInt64(2))),
		})
	})
}

func TestIntervalSeriesAxis(t *testing.T) {
	intervals := []Interval{
		NewInterval(at(t, "2026-01-01T00:00:00Z"), at(t, "2026-02-01T00:00:00Z"), Defined(value.NewInt64(1))),
		NewInterval(at(t, "2026-03-01T00:00:00Z"), at(t, "2026-04-01T00:00:00Z"), Defined(value.NewInt64(2))),
	}
	stored := NewIntervalSeries(value.Integer, unitless(), false, intervals)
	if !stored.IsValid() || len(stored.Intervals()) != 2 {
		t.Fatal("valid gapped Interval Series rejected")
	}
	if !intervals[0].Contains(at(t, "2026-01-01T00:00:00Z")) ||
		intervals[0].Contains(at(t, "2026-02-01T00:00:00Z")) {
		t.Fatal("Interval does not use half-open [from, to) semantics")
	}
	if !NewIntervalSeries(value.Integer, unitless(), false, nil).IsValid() {
		t.Fatal("empty Interval Series is invalid")
	}
	requirePanic(t, func() {
		NewIntervalSeries(value.Integer, unitless(), false, []Interval{
			NewInterval(at(t, "2026-01-01T00:00:00Z"), at(t, "2026-03-01T00:00:00Z"), Defined(value.NewInt64(1))),
			NewInterval(at(t, "2026-02-01T00:00:00Z"), at(t, "2026-04-01T00:00:00Z"), Defined(value.NewInt64(2))),
		})
	})
}

func FuzzPointSeriesAxisInvariants(f *testing.F) {
	f.Add(uint8(0), int64(1))
	f.Add(uint8(3), int64(-9))
	f.Fuzz(func(t *testing.T, count uint8, seed int64) {
		n := int(count % 28)
		points := make([]Point, n)
		for index := range points {
			points[index] = NewPoint(
				at(t, fmt.Sprintf("2026-01-%02dT00:00:00Z", index+1)),
				Defined(value.NewInt64(seed+int64(index))),
			)
		}
		left := NewPointSeries(value.Integer, unitless(), false, points)
		right := NewPointSeries(value.Integer, unitless(), false, left.Points())
		if !left.IsValid() || !right.IsValid() || !left.AxisEqual(right) {
			t.Fatal("valid Point Series did not preserve exact axis identity")
		}
		if n > 0 {
			shorter := NewPointSeries(value.Integer, unitless(), false, left.Points()[1:])
			if left.AxisEqual(shorter) {
				t.Fatal("different Point axes compared equal")
			}
		}
	})
}

func unitless() unit.Unit { return unit.Unit{} }
