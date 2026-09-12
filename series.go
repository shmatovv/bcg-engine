package bcg

import (
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/types/series"
)

// SeriesAxis identifies Point or Interval coordinates.
type SeriesAxis string

const (
	PointSeries    SeriesAxis = "Point"
	IntervalSeries SeriesAxis = "Interval"
)

// SeriesPoint is one evaluated point sample.
type SeriesPoint struct {
	at     DateTime
	result Result
}

// At returns this point sample's coordinate.
func (p SeriesPoint) At() DateTime { return p.at }

// Result returns this point sample's state and value.
func (p SeriesPoint) Result() Result { return p.result }

// SeriesInterval is one evaluated half-open interval sample.
type SeriesInterval struct {
	from, to DateTime
	result   Result
}

// From returns this interval sample's inclusive lower coordinate.
func (i SeriesInterval) From() DateTime { return i.from }

// To returns this interval sample's exclusive upper coordinate.
func (i SeriesInterval) To() DateTime { return i.to }

// Result returns this interval sample's state and value.
func (i SeriesInterval) Result() Result { return i.result }

// SeriesResult is a defined point/interval series or a whole-Series Undefined outcome.
type SeriesResult struct{ value evaluator.SeriesResult }

func seriesResultFromInternal(value evaluator.SeriesResult) SeriesResult {
	return SeriesResult{value: value}
}

// IsDefined reports whether the Series evaluation completed with samples.
func (r SeriesResult) IsDefined() bool { return r.value.IsDefined() }

// IsUndefined reports whether evaluating the Series itself was indeterminate.
func (r SeriesResult) IsUndefined() bool { return r.value.IsUndefined() }

// Axis returns PointSeries or IntervalSeries for a defined result.
func (r SeriesResult) Axis() SeriesAxis {
	if !r.IsDefined() {
		return ""
	}
	if r.value.AxisKind() == series.AxisPoint {
		return PointSeries
	}
	return IntervalSeries
}

// ElementKind returns the scalar type of each Series sample.
func (r SeriesResult) ElementKind() ValueKind { return valueKindFromInternal(r.value.ElementType()) }

// Unit returns the optional shared unit of defined samples.
func (r SeriesResult) Unit() (Unit, bool) {
	if !r.IsDefined() || !r.value.HasUnit() {
		return Unit{}, false
	}
	return Unit{value: r.value.Unit()}, true
}

// Undefined returns the whole-Series cause when IsUndefined reports true.
func (r SeriesResult) Undefined() (UndefinedInfo, bool) {
	info, ok := r.value.UndefinedInfo()
	if !ok {
		return UndefinedInfo{}, false
	}
	return undefinedInfoFromInternal(info), true
}

// Points returns an owned deterministic copy of point samples.
func (r SeriesResult) Points() []SeriesPoint {
	items := r.value.Points()
	result := make([]SeriesPoint, len(items))
	for i, item := range items {
		result[i] = SeriesPoint{at: DateTime{value: item.At()}, result: resultFromInternal(item.Result())}
	}
	return result
}

// Intervals returns an owned deterministic copy of interval samples.
func (r SeriesResult) Intervals() []SeriesInterval {
	items := r.value.Intervals()
	result := make([]SeriesInterval, len(items))
	for i, item := range items {
		result[i] = SeriesInterval{from: DateTime{value: item.From()}, to: DateTime{value: item.To()}, result: resultFromInternal(item.Result())}
	}
	return result
}
