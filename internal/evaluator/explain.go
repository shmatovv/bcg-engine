package evaluator

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/provenance"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// Explanation is an immutable, structured account of one scalar property
// evaluation. Root is the actual provenance tree produced by the ordinary
// evaluator; it is never reconstructed from a static expression or graph.
//
// The tree intentionally preserves separate evaluation occurrences. For
// example, x + x has two Reference nodes because both operand occurrences
// were evaluated. This keeps operation operands and their actual paths
// unambiguous while the underlying immutable provenance children are shared
// by value where possible.
type Explanation struct {
	target address.PropertyAddress
	result Result
}

// Explain evaluates one scalar property through the established evaluation
// pipeline and returns its actual structured provenance. Engine failures stay
// errors; Null and Undefined are retained in Result and Root.
func Explain(
	version Source,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Explanation, error) {
	result, err := Evaluate(version, target, context)
	if err != nil {
		return Explanation{}, err
	}
	if !scalarExplanationTraceValid(target, result) {
		return Explanation{}, newError(
			ErrorInvariantViolation,
			"scalar evaluation produced an incomplete provenance trace",
			nil,
		)
	}
	return Explanation{target: target, result: result}, nil
}

// ExplainResult turns an already evaluated scalar Result into an Explanation
// without re-running resolution or expression evaluation.
func ExplainResult(target address.PropertyAddress, result Result) Explanation {
	if !target.IsValid() {
		panic("evaluator: explanation target is invalid")
	}
	if !result.IsValid() || !scalarExplanationTraceValid(target, result) {
		panic("evaluator: result cannot form a complete scalar explanation")
	}
	return Explanation{target: target, result: result}
}

func (e Explanation) Target() address.PropertyAddress { return e.target }
func (e Explanation) Result() Result                  { return e.result }
func (e Explanation) Root() provenance.Node           { return e.result.Provenance() }

func (e Explanation) IsValid() bool {
	return e.target.IsValid() && e.result.IsValid() && scalarExplanationTraceValid(e.target, e.result)
}

// SeriesExplanation is an immutable, per-sample explanation for one Series
// property. A defined Series keeps a normal scalar Result/provenance tree at
// each Point or Interval coordinate. A whole-Series Undefined has one root
// provenance node and no samples.
type SeriesExplanation struct {
	target address.PropertyAddress
	result SeriesResult
}

// ExplainSeries evaluates one Series property at CalculationContext.At through
// the normal Series evaluator. Period x Series remains unsupported by that
// evaluator and therefore returns its existing engine error.
func ExplainSeries(
	version Source,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (SeriesExplanation, error) {
	result, err := EvaluateSeries(version, target, context)
	if err != nil {
		return SeriesExplanation{}, err
	}
	if !seriesExplanationTraceValid(target, result) {
		return SeriesExplanation{}, newError(
			ErrorInvariantViolation,
			"Series evaluation produced an incomplete provenance trace",
			nil,
		)
	}
	return SeriesExplanation{target: target, result: result}, nil
}

// ExplainSeriesResult projects an already evaluated Series result without
// re-running it. It preserves source/override snapshots and per-sample lazy
// provenance exactly as recorded by the evaluator.
func ExplainSeriesResult(target address.PropertyAddress, result SeriesResult) SeriesExplanation {
	if !target.IsValid() {
		panic("evaluator: Series explanation target is invalid")
	}
	if !result.IsValid() || !seriesExplanationTraceValid(target, result) {
		panic("evaluator: Series result cannot form a complete explanation")
	}
	return SeriesExplanation{target: target, result: result}
}

func (e SeriesExplanation) Target() address.PropertyAddress { return e.target }
func (e SeriesExplanation) Result() SeriesResult            { return e.result }

// ExplainedSeriesPoint is one immutable Point coordinate and the ordinary
// scalar explanation captured for exactly that coordinate.
type ExplainedSeriesPoint struct {
	at     datetime.DateTime
	result Result
}

func (p ExplainedSeriesPoint) At() datetime.DateTime { return p.at }
func (p ExplainedSeriesPoint) Result() Result        { return p.result }
func (p ExplainedSeriesPoint) Root() provenance.Node { return p.result.Provenance() }
func (p ExplainedSeriesPoint) IsValid() bool {
	return p.at.IsValid() && p.result.IsValid()
}

// ExplainedSeriesInterval is one immutable half-open [from,to) coordinate and
// the ordinary scalar explanation captured for exactly that coordinate.
type ExplainedSeriesInterval struct {
	from   datetime.DateTime
	to     datetime.DateTime
	result Result
}

func (i ExplainedSeriesInterval) From() datetime.DateTime { return i.from }
func (i ExplainedSeriesInterval) To() datetime.DateTime   { return i.to }
func (i ExplainedSeriesInterval) Result() Result          { return i.result }
func (i ExplainedSeriesInterval) Root() provenance.Node   { return i.result.Provenance() }
func (i ExplainedSeriesInterval) IsValid() bool {
	return i.from.IsValid() && i.to.IsValid() && i.from.Cmp(i.to) < 0 && i.result.IsValid()
}

// Points returns an owned ordered copy for a Point Series explanation.
func (e SeriesExplanation) Points() []ExplainedSeriesPoint {
	if !e.result.IsDefined() {
		return nil
	}
	points := e.result.Points()
	if points == nil {
		return nil
	}
	result := make([]ExplainedSeriesPoint, len(points))
	for index, point := range points {
		result[index] = ExplainedSeriesPoint{at: point.At(), result: point.Result()}
	}
	return result
}

// Intervals returns an owned ordered copy for an Interval Series explanation.
func (e SeriesExplanation) Intervals() []ExplainedSeriesInterval {
	if !e.result.IsDefined() {
		return nil
	}
	intervals := e.result.Intervals()
	if intervals == nil {
		return nil
	}
	result := make([]ExplainedSeriesInterval, len(intervals))
	for index, interval := range intervals {
		result[index] = ExplainedSeriesInterval{from: interval.From(), to: interval.To(), result: interval.Result()}
	}
	return result
}

// Undefined returns the whole-Series outcome and its actual provenance. It is
// false for a defined Series, where Undefined belongs to individual samples.
func (e SeriesExplanation) Undefined() (value.UndefinedInfo, provenance.Node, bool) {
	info, ok := e.result.UndefinedInfo()
	if !ok {
		return value.UndefinedInfo{}, provenance.Node{}, false
	}
	node, ok := e.result.UndefinedProvenance()
	return info, node, ok
}

func (e SeriesExplanation) IsValid() bool {
	return e.target.IsValid() && e.result.IsValid() && seriesExplanationTraceValid(e.target, e.result)
}

func scalarExplanationTraceValid(target address.PropertyAddress, result Result) bool {
	if !result.IsValid() {
		return false
	}
	root := result.Provenance()
	property, ok := root.Property()
	if !ok || property != target || (root.Kind() != provenance.KindInputProperty && root.Kind() != provenance.KindDerivedProperty) {
		return false
	}
	return traceMatchesResult(root, result.Value())
}

func seriesExplanationTraceValid(target address.PropertyAddress, result SeriesResult) bool {
	if result.IsUndefined() {
		info, ok := result.UndefinedInfo()
		if !ok {
			return false
		}
		node, ok := result.UndefinedProvenance()
		return ok && traceMatchesResult(node, value.Undefined(info)) && traceRootMatchesTarget(node, target)
	}
	if !result.IsDefined() {
		return false
	}
	for _, point := range result.Points() {
		if !scalarExplanationTraceValid(target, point.Result()) {
			return false
		}
	}
	for _, interval := range result.Intervals() {
		if !scalarExplanationTraceValid(target, interval.Result()) {
			return false
		}
	}
	return true
}

func traceRootMatchesTarget(node provenance.Node, target address.PropertyAddress) bool {
	property, ok := node.Property()
	return ok && property == target && (node.Kind() == provenance.KindInputProperty || node.Kind() == provenance.KindDerivedProperty)
}

func traceMatchesResult(node provenance.Node, expected value.Resolved) bool {
	if !node.IsValid() {
		return false
	}
	recorded, ok := node.Resolved()
	if !ok || !recorded.Equal(expected) {
		return false
	}
	for _, child := range node.Children() {
		if !traceComplete(child) {
			return false
		}
	}
	return true
}

func traceComplete(node provenance.Node) bool {
	if !node.IsValid() {
		return false
	}
	if resolved, ok := node.Resolved(); !ok || !resolved.IsValid() {
		return false
	}
	for _, child := range node.Children() {
		if !traceComplete(child) {
			return false
		}
	}
	return true
}
