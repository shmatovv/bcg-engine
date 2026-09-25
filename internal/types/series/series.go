// Package series contains immutable source time-series values.
//
// A Series carries its element ValueType and optional Unit once for the whole
// axis. Samples contain only a raw value or Null; source series deliberately
// cannot contain runtime Undefined outcomes.
package series

import (
	"errors"

	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

var (
	ErrElementTypeMismatch = errors.New("series: element value type does not match series type")
	ErrUnitMismatch        = errors.New("series: unit does not match expected unit")
	ErrNullNotAllowed      = errors.New("series: Null sample is not allowed")
)

// AxisKind identifies the temporal axis of a Series.
type AxisKind uint8

const (
	AxisInvalid AxisKind = iota
	AxisPoint
	AxisInterval
)

func (k AxisKind) IsValid() bool { return k == AxisPoint || k == AxisInterval }

func (k AxisKind) String() string {
	switch k {
	case AxisPoint:
		return "Point"
	case AxisInterval:
		return "Interval"
	default:
		return "Invalid"
	}
}

// Sample is one stored source sample. It is either Defined(Raw) or Null.
// Undefined is intentionally an evaluation-only outcome and cannot enter a
// source Series.
type Sample struct {
	value value.Raw
	null  bool
}

// Defined constructs a source sample with one raw value.
func Defined(raw value.Raw) Sample {
	if !raw.IsValid() {
		panic("series: sample value is invalid")
	}
	return Sample{value: raw}
}

// Null constructs an explicit Null source sample.
func Null() Sample { return Sample{null: true} }

// IsDefined reports whether s contains a raw source value.
func (s Sample) IsDefined() bool { return !s.null && s.value.IsValid() }

// IsNull reports whether s is an explicit Null source sample.
func (s Sample) IsNull() bool { return s.null }

// Value returns the source value and true for a Defined sample.
func (s Sample) Value() (value.Raw, bool) {
	if !s.IsDefined() {
		return value.Raw{}, false
	}
	return s.value, true
}

func (s Sample) IsValid() bool {
	return s.IsNull() || s.IsDefined()
}

// Point is one point-axis sample at a unique instant.
type Point struct {
	at     datetime.DateTime
	sample Sample
}

func NewPoint(at datetime.DateTime, sample Sample) Point {
	if !at.IsValid() {
		panic("series: point timestamp is invalid")
	}
	if !sample.IsValid() {
		panic("series: point sample is invalid")
	}
	return Point{at: at, sample: sample}
}

func (p Point) At() datetime.DateTime { return p.at }
func (p Point) Sample() Sample        { return p.sample }
func (p Point) IsValid() bool         { return p.at.IsValid() && p.sample.IsValid() }

// Interval is one half-open [from, to) interval-axis sample.
type Interval struct {
	from   datetime.DateTime
	to     datetime.DateTime
	sample Sample
}

func NewInterval(from, to datetime.DateTime, sample Sample) Interval {
	if !from.IsValid() || !to.IsValid() || from.Cmp(to) >= 0 {
		panic("series: interval must satisfy valid from < to")
	}
	if !sample.IsValid() {
		panic("series: interval sample is invalid")
	}
	return Interval{from: from, to: to, sample: sample}
}

func (i Interval) From() datetime.DateTime { return i.from }
func (i Interval) To() datetime.DateTime   { return i.to }
func (i Interval) Sample() Sample          { return i.sample }

// Contains reports whether at belongs to this half-open interval [from, to).
// An invalid instant is never contained.
func (i Interval) Contains(at datetime.DateTime) bool {
	return at.IsValid() && i.from.Cmp(at) <= 0 && at.Cmp(i.to) < 0
}

func (i Interval) IsValid() bool {
	return i.from.IsValid() && i.to.IsValid() && i.from.Cmp(i.to) < 0 && i.sample.IsValid()
}

// Series is an immutable Point or Interval source series. Empty axes are
// valid and retain their element type/unit metadata.
type Series struct {
	axisKind AxisKind

	elementType value.Type
	unit        unit.Unit
	hasUnit     bool

	points    []Point
	intervals []Interval
}

func NewPointSeries(elementType value.Type, u unit.Unit, hasUnit bool, points []Point) Series {
	result := Series{axisKind: AxisPoint, elementType: elementType, unit: u, hasUnit: hasUnit, points: append([]Point(nil), points...)}
	result.requireValid()
	return result
}

func NewIntervalSeries(elementType value.Type, u unit.Unit, hasUnit bool, intervals []Interval) Series {
	result := Series{axisKind: AxisInterval, elementType: elementType, unit: u, hasUnit: hasUnit, intervals: append([]Interval(nil), intervals...)}
	result.requireValid()
	return result
}

func (s Series) AxisKind() AxisKind      { return s.axisKind }
func (s Series) ElementType() value.Type { return s.elementType }
func (s Series) HasUnit() bool           { return s.hasUnit }
func (s Series) Unit() unit.Unit {
	if !s.hasUnit {
		panic("series: series has no unit")
	}
	return s.unit
}

// Points returns an owned ordered copy for a Point Series, otherwise nil.
func (s Series) Points() []Point {
	if s.axisKind != AxisPoint {
		return nil
	}
	return append([]Point(nil), s.points...)
}

// Intervals returns an owned ordered copy for an Interval Series, otherwise nil.
func (s Series) Intervals() []Interval {
	if s.axisKind != AxisInterval {
		return nil
	}
	return append([]Interval(nil), s.intervals...)
}

// Equal reports whether s and other carry exactly the same immutable source
// series data. It deliberately compares samples as well as axis metadata;
// AxisEqual is the narrower operation for expression compatibility checks.
func (s Series) Equal(other Series) bool {
	if !s.IsValid() || !other.IsValid() ||
		s.axisKind != other.axisKind ||
		s.elementType != other.elementType ||
		s.hasUnit != other.hasUnit {
		return false
	}

	if s.hasUnit && !s.unit.Equal(other.unit) {
		return false
	}

	equalSample := func(left, right Sample) bool {
		if left.IsNull() || right.IsNull() {
			return left.IsNull() && right.IsNull()
		}
		leftValue, leftOK := left.Value()
		rightValue, rightOK := right.Value()
		return leftOK && rightOK && leftValue.Equal(rightValue)
	}

	switch s.axisKind {
	case AxisPoint:
		if len(s.points) != len(other.points) {
			return false
		}
		for index := range s.points {
			left, right := s.points[index], other.points[index]
			if !left.at.Equal(right.at) || !equalSample(left.sample, right.sample) {
				return false
			}
		}
		return true

	case AxisInterval:
		if len(s.intervals) != len(other.intervals) {
			return false
		}
		for index := range s.intervals {
			left, right := s.intervals[index], other.intervals[index]
			if !left.from.Equal(right.from) || !left.to.Equal(right.to) ||
				!equalSample(left.sample, right.sample) {
				return false
			}
		}
		return true

	default:
		return false
	}
}

// AxisEqual reports exact axis equality. It does not compare element values,
// types, or units.
func (s Series) AxisEqual(other Series) bool {
	if !s.IsValid() || !other.IsValid() || s.axisKind != other.axisKind {
		return false
	}
	switch s.axisKind {
	case AxisPoint:
		if len(s.points) != len(other.points) {
			return false
		}
		for index := range s.points {
			if !s.points[index].at.Equal(other.points[index].at) {
				return false
			}
		}
		return true
	case AxisInterval:
		if len(s.intervals) != len(other.intervals) {
			return false
		}
		for index := range s.intervals {
			left, right := s.intervals[index], other.intervals[index]
			if !left.from.Equal(right.from) || !left.to.Equal(right.to) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// ValidateAgainst verifies that this source series can be stored for a
// property with the requested element type, unit and nullability.
func (s Series) ValidateAgainst(elementType value.Type, expectedUnit unit.Unit, hasUnit, nullable bool) error {
	if !s.IsValid() || !elementType.IsValid() || (hasUnit && !expectedUnit.IsValid()) {
		return ErrElementTypeMismatch
	}
	if s.elementType != elementType {
		return ErrElementTypeMismatch
	}
	if s.hasUnit != hasUnit || (hasUnit && !s.unit.Equal(expectedUnit)) {
		return ErrUnitMismatch
	}
	return s.forEachSample(func(sample Sample) error {
		if sample.IsNull() && !nullable {
			return ErrNullNotAllowed
		}
		return nil
	})
}

func (s Series) IsValid() bool {
	if !s.axisKind.IsValid() || !s.elementType.IsValid() || (s.hasUnit && !s.unit.IsValid()) {
		return false
	}
	if !s.hasUnit && s.unit.IsValid() {
		return false
	}
	switch s.axisKind {
	case AxisPoint:
		if len(s.intervals) != 0 {
			return false
		}
		var previous datetime.DateTime
		for index, point := range s.points {
			if !point.IsValid() || (index > 0 && previous.Cmp(point.at) >= 0) {
				return false
			}
			if point.sample.IsDefined() && point.sample.value.Type() != s.elementType {
				return false
			}
			previous = point.at
		}
		return true
	case AxisInterval:
		if len(s.points) != 0 {
			return false
		}
		var previousTo datetime.DateTime
		for index, interval := range s.intervals {
			if !interval.IsValid() || (index > 0 && previousTo.Cmp(interval.from) > 0) {
				return false
			}
			if interval.sample.IsDefined() && interval.sample.value.Type() != s.elementType {
				return false
			}
			previousTo = interval.to
		}
		return true
	default:
		return false
	}
}

func (s Series) requireValid() {
	if !s.IsValid() {
		panic("series: invalid series")
	}
}

func (s Series) forEachSample(fn func(Sample) error) error {
	switch s.axisKind {
	case AxisPoint:
		for _, point := range s.points {
			if err := fn(point.sample); err != nil {
				return err
			}
		}
	case AxisInterval:
		for _, interval := range s.intervals {
			if err := fn(interval.sample); err != nil {
				return err
			}
		}
	}
	return nil
}
