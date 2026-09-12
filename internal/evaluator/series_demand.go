package evaluator

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/provenance"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/series"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	enginevalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

// seriesDemand identifies samples by their temporal coordinate, never by an
// array position. A full demand requires exact whole-axis compatibility; a
// partial demand requires exact identity only for its selected coordinates.
//
// It is private evaluation state. Constructors copy all slices so sibling
// branches and concurrent evaluations cannot share mutable demand storage.
type seriesDemand struct {
	axisKind  series.AxisKind
	points    []datetime.DateTime
	intervals []seriesIntervalCoordinate

	full bool
}

type seriesIntervalCoordinate struct {
	from datetime.DateTime
	to   datetime.DateTime
}

// seriesPartial is one locally-demanded view of a Series result. result keeps
// its complete source axis; demand is a local mask over that axis. The mask is
// only an efficient local lookup. Temporal identity is retained in
// seriesDemand whenever demand is passed to another expression.
type seriesPartial struct {
	result SeriesResult
	demand []bool

	// fullDemand means every coordinate in result was requested as one exact
	// axis, so a binary operation must retain normal whole-axis compatibility.
	fullDemand bool
}

func newSeriesPartial(result SeriesResult, requested *seriesDemand) (seriesPartial, error) {
	if !result.IsValid() {
		return seriesPartial{}, newError(ErrorInvariantViolation, "partial Series has an invalid result", nil)
	}
	if result.IsUndefined() {
		return seriesPartial{result: result}, nil
	}

	count := seriesSampleCount(result)
	if requested == nil {
		return seriesPartial{result: result, demand: allSeriesDemand(count), fullDemand: true}, nil
	}
	if !requested.isValid() {
		return seriesPartial{}, newError(ErrorInvariantViolation, "Series demand is invalid", nil)
	}
	if requested.axisKind != result.axisKind {
		return seriesPartial{}, incompatibleSeriesAxisError()
	}
	if requested.full && !seriesAxisMatchesDemand(result, *requested) {
		return seriesPartial{}, incompatibleSeriesAxisError()
	}

	mask := make([]bool, count)
	switch result.axisKind {
	case series.AxisPoint:
		for _, at := range requested.points {
			index, found := pointIndex(result.points, at)
			if !found {
				return seriesPartial{}, incompatibleSeriesAxisError()
			}
			mask[index] = true
		}
	case series.AxisInterval:
		for _, coordinate := range requested.intervals {
			index, found := intervalIndex(result.intervals, coordinate)
			if !found {
				return seriesPartial{}, incompatibleSeriesAxisError()
			}
			mask[index] = true
		}
	default:
		return seriesPartial{}, newError(ErrorInvariantViolation, "Series result has an invalid axis", nil)
	}

	return seriesPartial{result: result, demand: mask, fullDemand: requested.full}, nil
}

func (d seriesDemand) isValid() bool {
	if !d.axisKind.IsValid() {
		return false
	}
	switch d.axisKind {
	case series.AxisPoint:
		if len(d.intervals) != 0 {
			return false
		}
		var previous datetime.DateTime
		for index, at := range d.points {
			if !at.IsValid() || (index > 0 && previous.Cmp(at) >= 0) {
				return false
			}
			previous = at
		}
		return true
	case series.AxisInterval:
		if len(d.points) != 0 {
			return false
		}
		var previousTo datetime.DateTime
		for index, coordinate := range d.intervals {
			if !coordinate.from.IsValid() || !coordinate.to.IsValid() || coordinate.from.Cmp(coordinate.to) >= 0 ||
				(index > 0 && previousTo.Cmp(coordinate.from) > 0) {
				return false
			}
			previousTo = coordinate.to
		}
		return true
	default:
		return false
	}
}

func allSeriesDemand(count int) []bool {
	result := make([]bool, count)
	for index := range result {
		result[index] = true
	}
	return result
}

func seriesSampleCount(result SeriesResult) int {
	switch result.axisKind {
	case series.AxisPoint:
		return len(result.points)
	case series.AxisInterval:
		return len(result.intervals)
	default:
		return 0
	}
}

func (p seriesPartial) isDefined() bool   { return p.result.IsDefined() }
func (p seriesPartial) isUndefined() bool { return p.result.IsUndefined() }

func (p seriesPartial) sample(index int) (Result, error) {
	if !p.result.IsDefined() || index < 0 || index >= len(p.demand) || !p.demand[index] {
		return Result{}, newError(ErrorInvariantViolation, "attempted to read an undemanded Series sample", nil)
	}
	return seriesSampleAt(p.result, index), nil
}

// demandFor returns an owned temporal request for the selected local samples.
// A full exact-axis request is retained only when all samples of an already
// full input participate. Any short-circuit selection becomes partial.
func (p seriesPartial) demandFor(mask []bool) (*seriesDemand, error) {
	if !p.result.IsDefined() || len(mask) != len(p.demand) {
		return nil, newError(ErrorInvariantViolation, "Series demand does not match its source axis", nil)
	}
	for index, requested := range mask {
		if requested && !p.demand[index] {
			return nil, newError(ErrorInvariantViolation, "Series demand selects an unevaluated sample", nil)
		}
	}

	demand := &seriesDemand{axisKind: p.result.axisKind, full: p.fullDemand && allDemanded(mask)}
	switch p.result.axisKind {
	case series.AxisPoint:
		for index, requested := range mask {
			if requested {
				demand.points = append(demand.points, p.result.points[index].at)
			}
		}
	case series.AxisInterval:
		for index, requested := range mask {
			if requested {
				interval := p.result.intervals[index]
				demand.intervals = append(demand.intervals, seriesIntervalCoordinate{from: interval.from, to: interval.to})
			}
		}
	default:
		return nil, newError(ErrorInvariantViolation, "Series result has an invalid axis", nil)
	}
	return demand, nil
}

func allDemanded(mask []bool) bool {
	for _, demanded := range mask {
		if !demanded {
			return false
		}
	}
	return true
}

// compatibleFor verifies that every requested coordinate from left has an
// exact coordinate in right. For a full request it deliberately retains the
// v1 whole-axis equality rule, including empty-vs-non-empty incompatibility.
func compatibleFor(left, right seriesPartial, requested []bool) bool {
	if !left.isDefined() || !right.isDefined() || len(requested) != len(left.demand) || left.result.axisKind != right.result.axisKind {
		return false
	}
	if left.fullDemand && allDemanded(requested) {
		return seriesAxesEqual(left.result, right.result)
	}
	for index, needed := range requested {
		if !needed || !left.demand[index] {
			continue
		}
		if _, found := right.indexFor(left.coordinate(index)); !found {
			return false
		}
	}
	return true
}

func (p seriesPartial) sampleFor(other seriesPartial, index int) (Result, error) {
	if !p.result.IsDefined() || !other.result.IsDefined() || index < 0 || index >= len(p.demand) || !p.demand[index] {
		return Result{}, newError(ErrorInvariantViolation, "Series sample lookup is invalid", nil)
	}
	otherIndex, found := other.indexFor(p.coordinate(index))
	if !found || !other.demand[otherIndex] {
		return Result{}, incompatibleSeriesAxisError()
	}
	return other.sample(otherIndex)
}

func (p seriesPartial) coordinate(index int) seriesIntervalCoordinate {
	if p.result.axisKind == series.AxisInterval {
		interval := p.result.intervals[index]
		return seriesIntervalCoordinate{from: interval.from, to: interval.to}
	}
	return seriesIntervalCoordinate{from: p.result.points[index].at}
}

func (p seriesPartial) indexFor(coordinate seriesIntervalCoordinate) (int, bool) {
	switch p.result.axisKind {
	case series.AxisPoint:
		return pointIndex(p.result.points, coordinate.from)
	case series.AxisInterval:
		return intervalIndex(p.result.intervals, coordinate)
	default:
		return 0, false
	}
}

func seriesAxisMatchesDemand(result SeriesResult, demand seriesDemand) bool {
	if result.axisKind != demand.axisKind || seriesSampleCount(result) != demandCount(demand) {
		return false
	}
	switch result.axisKind {
	case series.AxisPoint:
		for index, point := range result.points {
			if !point.at.Equal(demand.points[index]) {
				return false
			}
		}
		return true
	case series.AxisInterval:
		for index, interval := range result.intervals {
			coordinate := demand.intervals[index]
			if !interval.from.Equal(coordinate.from) || !interval.to.Equal(coordinate.to) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func demandCount(demand seriesDemand) int {
	if demand.axisKind == series.AxisPoint {
		return len(demand.points)
	}
	return len(demand.intervals)
}

func pointIndex(points []SeriesPoint, at datetime.DateTime) (int, bool) {
	index := sort.Search(len(points), func(index int) bool {
		return points[index].at.Cmp(at) >= 0
	})
	return index, index < len(points) && points[index].at.Equal(at)
}

func intervalIndex(intervals []SeriesInterval, coordinate seriesIntervalCoordinate) (int, bool) {
	index := sort.Search(len(intervals), func(index int) bool {
		return intervals[index].from.Cmp(coordinate.from) >= 0
	})
	return index, index < len(intervals) && intervals[index].from.Equal(coordinate.from) && intervals[index].to.Equal(coordinate.to)
}

func incompatibleSeriesAxisError() Error {
	return newError(ErrorIncompatibleSeriesAxis, "Series operands require exactly matching demanded Point timestamps or Interval bounds", nil)
}

func (p seriesPartial) withValues(elementType enginevalue.Type, u unit.Unit, hasUnit bool, values []Result) (seriesPartial, error) {
	if !p.result.IsDefined() || len(values) != len(p.demand) {
		return seriesPartial{}, newError(ErrorInvariantViolation, "partial Series values do not match demand", nil)
	}
	for index := range values {
		if !p.demand[index] {
			values[index] = unevaluatedSeriesSample()
		}
	}
	result, err := makeSeriesResult(p.result, elementType, u, hasUnit, values)
	if err != nil {
		return seriesPartial{}, err
	}
	return seriesPartial{result: result, demand: append([]bool(nil), p.demand...), fullDemand: p.fullDemand}, nil
}

func unevaluatedSeriesSample() Result {
	return newResult(
		enginevalue.Undefined(enginevalue.NewUndefinedInfo(enginevalue.UndefinedDependencyUndefined)),
		provenance.NewOperation("UnevaluatedSeriesSample"),
	)
}
