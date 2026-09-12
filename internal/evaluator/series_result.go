package evaluator

import (
	"github.com/shmatovv/bcg-engine/internal/provenance"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/series"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// SeriesPoint is one evaluated point sample with its ordinary scalar Result.
type SeriesPoint struct {
	at     datetime.DateTime
	result Result
}

func newSeriesPoint(at datetime.DateTime, result Result) SeriesPoint {
	if !at.IsValid() || !result.IsValid() {
		panic("evaluator: invalid evaluated Series point")
	}
	return SeriesPoint{at: at, result: result}
}

func (p SeriesPoint) At() datetime.DateTime { return p.at }
func (p SeriesPoint) Result() Result        { return p.result }
func (p SeriesPoint) IsValid() bool         { return p.at.IsValid() && p.result.IsValid() }

// SeriesInterval is one evaluated half-open interval sample.
type SeriesInterval struct {
	from   datetime.DateTime
	to     datetime.DateTime
	result Result
}

func newSeriesInterval(from, to datetime.DateTime, result Result) SeriesInterval {
	if !from.IsValid() || !to.IsValid() || from.Cmp(to) >= 0 || !result.IsValid() {
		panic("evaluator: invalid evaluated Series interval")
	}
	return SeriesInterval{from: from, to: to, result: result}
}

func (i SeriesInterval) From() datetime.DateTime { return i.from }
func (i SeriesInterval) To() datetime.DateTime   { return i.to }
func (i SeriesInterval) Result() Result          { return i.result }
func (i SeriesInterval) IsValid() bool {
	return i.from.IsValid() && i.to.IsValid() && i.from.Cmp(i.to) < 0 && i.result.IsValid()
}

// SeriesResult is either an evaluated Series whose individual samples contain
// ordinary Result/provenance pairs, or a whole-Series Undefined outcome. A
// whole-Series Null is deliberately not represented in Series Core v1.
type SeriesResult struct {
	defined bool

	axisKind    series.AxisKind
	elementType value.Type
	unit        unit.Unit
	hasUnit     bool

	points    []SeriesPoint
	intervals []SeriesInterval

	undefined           value.UndefinedInfo
	undefinedProvenance provenance.Node
}

func newDefinedPointSeriesResult(elementType value.Type, u unit.Unit, hasUnit bool, points []SeriesPoint) SeriesResult {
	result := SeriesResult{defined: true, axisKind: series.AxisPoint, elementType: elementType, unit: u, hasUnit: hasUnit, points: append([]SeriesPoint(nil), points...)}
	if !result.IsValid() {
		panic("evaluator: invalid defined point Series result")
	}
	return result
}

func newDefinedIntervalSeriesResult(elementType value.Type, u unit.Unit, hasUnit bool, intervals []SeriesInterval) SeriesResult {
	result := SeriesResult{defined: true, axisKind: series.AxisInterval, elementType: elementType, unit: u, hasUnit: hasUnit, intervals: append([]SeriesInterval(nil), intervals...)}
	if !result.IsValid() {
		panic("evaluator: invalid defined interval Series result")
	}
	return result
}

func newUndefinedSeriesResult(info value.UndefinedInfo, node provenance.Node) SeriesResult {
	if !info.IsValid() || !node.IsValid() {
		panic("evaluator: invalid undefined Series result")
	}
	return SeriesResult{undefined: info, undefinedProvenance: node}
}

func (r SeriesResult) IsDefined() bool           { return r.defined }
func (r SeriesResult) IsUndefined() bool         { return !r.defined && r.undefined.IsValid() }
func (r SeriesResult) AxisKind() series.AxisKind { return r.axisKind }
func (r SeriesResult) ElementType() value.Type   { return r.elementType }
func (r SeriesResult) HasUnit() bool             { return r.hasUnit }
func (r SeriesResult) Unit() unit.Unit {
	if !r.defined || !r.hasUnit {
		panic("evaluator: Series result has no unit")
	}
	return r.unit
}

func (r SeriesResult) Points() []SeriesPoint {
	if !r.defined || r.axisKind != series.AxisPoint {
		return nil
	}
	return append([]SeriesPoint(nil), r.points...)
}

func (r SeriesResult) Intervals() []SeriesInterval {
	if !r.defined || r.axisKind != series.AxisInterval {
		return nil
	}
	return append([]SeriesInterval(nil), r.intervals...)
}

// UndefinedInfo and UndefinedProvenance are available only for a whole-Series
// Undefined result, such as MissingValue or AmbiguousValue.
func (r SeriesResult) UndefinedInfo() (value.UndefinedInfo, bool) {
	return r.undefined, r.IsUndefined()
}
func (r SeriesResult) UndefinedProvenance() (provenance.Node, bool) {
	return r.undefinedProvenance, r.IsUndefined()
}

func (r SeriesResult) IsValid() bool {
	if !r.defined {
		return !r.axisKind.IsValid() && !r.elementType.IsValid() && !r.hasUnit &&
			!r.unit.IsValid() && len(r.points) == 0 && len(r.intervals) == 0 &&
			r.undefined.IsValid() && r.undefinedProvenance.IsValid()
	}
	if !r.axisKind.IsValid() || !r.elementType.IsValid() || (r.hasUnit && !r.unit.IsValid()) || (!r.hasUnit && r.unit.IsValid()) || r.undefined.IsValid() || r.undefinedProvenance.IsValid() {
		return false
	}
	switch r.axisKind {
	case series.AxisPoint:
		if len(r.intervals) != 0 {
			return false
		}
		var previous datetime.DateTime
		for index, point := range r.points {
			if !point.IsValid() || (index > 0 && previous.Cmp(point.at) >= 0) || !resultMatchesSeriesType(point.result, r.elementType, r.unit, r.hasUnit) {
				return false
			}
			previous = point.at
		}
		return true
	case series.AxisInterval:
		if len(r.points) != 0 {
			return false
		}
		var previousTo datetime.DateTime
		for index, interval := range r.intervals {
			if !interval.IsValid() || (index > 0 && previousTo.Cmp(interval.from) > 0) || !resultMatchesSeriesType(interval.result, r.elementType, r.unit, r.hasUnit) {
				return false
			}
			previousTo = interval.to
		}
		return true
	default:
		return false
	}
}

func resultMatchesSeriesType(result Result, elementType value.Type, expectedUnit unit.Unit, hasUnit bool) bool {
	resolved := result.Value()
	if !resolved.IsDefined() {
		return true
	}
	actual := resolved.Value()
	if actual.Type() != elementType || actual.HasUnit() != hasUnit {
		return false
	}
	return !hasUnit || actual.Unit().Equal(expectedUnit)
}

func seriesAxesEqual(left, right SeriesResult) bool {
	if !left.IsDefined() || !right.IsDefined() || left.axisKind != right.axisKind {
		return false
	}
	switch left.axisKind {
	case series.AxisPoint:
		if len(left.points) != len(right.points) {
			return false
		}
		for index := range left.points {
			if !left.points[index].at.Equal(right.points[index].at) {
				return false
			}
		}
		return true
	case series.AxisInterval:
		if len(left.intervals) != len(right.intervals) {
			return false
		}
		for index := range left.intervals {
			if !left.intervals[index].from.Equal(right.intervals[index].from) || !left.intervals[index].to.Equal(right.intervals[index].to) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
