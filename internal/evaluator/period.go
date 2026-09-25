package evaluator

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	resolverscope "github.com/shmatovv/bcg-engine/v2/internal/resolver/scope"
	resolvervalue "github.com/shmatovv/bcg-engine/v2/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
)

// PeriodSegment is one finite half-open [from, to) interval evaluated with
// the normal At evaluator at from.
type PeriodSegment struct {
	from datetime.DateTime
	to   datetime.DateTime

	result Result
}

func newPeriodSegment(
	from datetime.DateTime,
	to datetime.DateTime,
	result Result,
) PeriodSegment {
	if !from.IsValid() || !to.IsValid() || from.Cmp(to) >= 0 {
		panic("evaluator: invalid PeriodSegment bounds")
	}

	if !result.IsValid() {
		panic("evaluator: invalid PeriodSegment result")
	}

	return PeriodSegment{from: from, to: to, result: result}
}

// From is the inclusive segment boundary.
func (s PeriodSegment) From() datetime.DateTime { return s.from }

// To is the exclusive segment boundary.
func (s PeriodSegment) To() datetime.DateTime { return s.to }

// Result is the ordinary At evaluation result for From.
func (s PeriodSegment) Result() Result { return s.result }

func (s PeriodSegment) IsValid() bool {
	return s.from.IsValid() &&
		s.to.IsValid() &&
		s.from.Cmp(s.to) < 0 &&
		s.result.IsValid()
}

// PeriodResult contains all contiguous scalar results over one requested
// finite calculation period. It intentionally performs no time aggregation.
type PeriodResult struct {
	sourceIdentity model.EffectiveSourceIdentity
	target         address.PropertyAddress
	from           datetime.DateTime
	to             datetime.DateTime
	segments       []PeriodSegment
}

func newPeriodResult(
	sourceIdentity model.EffectiveSourceIdentity,
	target address.PropertyAddress,
	from datetime.DateTime,
	to datetime.DateTime,
	segments []PeriodSegment,
) PeriodResult {
	if !sourceIdentity.IsValid() || !target.IsValid() || !from.IsValid() ||
		!to.IsValid() || from.Cmp(to) >= 0 || len(segments) == 0 {
		panic("evaluator: invalid PeriodResult")
	}

	copied := append([]PeriodSegment(nil), segments...)
	for index, segment := range copied {
		if !segment.IsValid() ||
			(index == 0 && !segment.from.Equal(from)) ||
			(index == len(copied)-1 && !segment.to.Equal(to)) ||
			(index > 0 && !copied[index-1].to.Equal(segment.from)) {
			panic("evaluator: invalid PeriodResult segments")
		}
	}

	return PeriodResult{
		sourceIdentity: sourceIdentity,
		target:         target,
		from:           from,
		to:             to,
		segments:       copied,
	}
}

// ModelVersionID identifies the immutable model snapshot evaluated.
func (r PeriodResult) ModelVersionID() identity.UUID { return r.sourceIdentity.ModelVersionID() }

// SourceIdentity identifies the exact immutable model-only or Dataset-backed
// source evaluated over the requested period.
func (r PeriodResult) SourceIdentity() model.EffectiveSourceIdentity { return r.sourceIdentity }

// Target identifies the concrete scalar property evaluated.
func (r PeriodResult) Target() address.PropertyAddress { return r.target }

// From is the inclusive requested calculation-period boundary.
func (r PeriodResult) From() datetime.DateTime { return r.from }

// To is the exclusive requested calculation-period boundary.
func (r PeriodResult) To() datetime.DateTime { return r.to }

// Segments returns an owned copy in chronological order.
func (r PeriodResult) Segments() []PeriodSegment {
	return append([]PeriodSegment(nil), r.segments...)
}

func (r PeriodResult) IsValid() bool {
	if !r.sourceIdentity.IsValid() || !r.target.IsValid() || !r.from.IsValid() ||
		!r.to.IsValid() || r.from.Cmp(r.to) >= 0 || len(r.segments) == 0 {
		return false
	}

	for index, segment := range r.segments {
		if !segment.IsValid() ||
			(index == 0 && !segment.from.Equal(r.from)) ||
			(index == len(r.segments)-1 && !segment.to.Equal(r.to)) ||
			(index > 0 && !r.segments[index-1].to.Equal(segment.from)) {
			return false
		}
	}

	return true
}

// EvaluatePeriod evaluates one Scalar property over a finite Period context.
// It plans effective-time boundaries and delegates every segment to Evaluate
// with an At context, retaining the ordinary plan, lazy evaluator, and
// provenance semantics for each segment.
func EvaluatePeriod(
	version Source,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (PeriodResult, error) {
	if !version.IsCommitted() {
		return PeriodResult{}, newError(ErrorInvalidModelVersion,
			"Period evaluation requires a committed ModelVersion", nil)
	}

	if !target.IsValid() {
		return PeriodResult{}, newError(ErrorInvalidPropertyAddress,
			"target property address is invalid", nil)
	}

	if !context.IsValid() {
		return PeriodResult{}, newError(ErrorInvalidCalculationContext,
			"calculation context is invalid", nil)
	}

	from, to, isPeriod := context.TimeContext().Period()
	if !isPeriod {
		return PeriodResult{}, newError(ErrorUnsupportedTimeContext,
			"Period evaluation requires a Period time context", nil)
	}

	if err := resolvervalue.ValidateContext(version, context); err != nil {
		return PeriodResult{}, newError(ErrorResolverFailure,
			"Period calculation context validation failed", err)
	}

	graph, cached := dependencyGraph(version)
	var err error
	if !cached {
		graph, err = dependency.Build(version)
	}
	if err != nil {
		return PeriodResult{}, newError(ErrorPlanBuild,
			"failed to build Base Dependency Graph for Period evaluation", err)
	}

	if !graph.HasVertex(target) {
		return PeriodResult{}, newError(ErrorDefinitionNotAvailable,
			"target property is not available on the concrete object", nil)
	}

	definition, ok := version.PropertyDefinition(target.DefinitionID())
	if !ok || !definition.IsValid() {
		return PeriodResult{}, newError(ErrorDefinitionNotFound,
			"target property definition could not be resolved", nil)
	}
	if definition.TemporalMode() != schema.TemporalScalar {
		return PeriodResult{}, newError(ErrorUnsupportedSeries,
			"Series property Period evaluation is not implemented", nil)
	}

	boundaries, err := periodBoundaries(version, graph, target, context, from, to)
	if err != nil {
		return PeriodResult{}, err
	}

	segments := make([]PeriodSegment, 0, len(boundaries)-1)
	for index := 0; index+1 < len(boundaries); index++ {
		segmentFrom, segmentTo := boundaries[index], boundaries[index+1]
		result, err := Evaluate(version, target,
			context.WithTimeContext(domaintime.At(segmentFrom)))
		if err != nil {
			return PeriodResult{}, err
		}
		segments = append(segments, newPeriodSegment(segmentFrom, segmentTo, result))
	}

	return newPeriodResult(version.SourceIdentity(), target, from, to, segments), nil
}

func periodBoundaries(
	version Source,
	graph dependency.Graph,
	target address.PropertyAddress,
	context runtime.CalculationContext,
	from datetime.DateTime,
	to datetime.DateTime,
) ([]datetime.DateTime, error) {
	relevant := periodDependencies(graph, target)
	relevantTargets := relevantSet(relevant)
	boundaries := []datetime.DateTime{from, to}
	scopeValues := context.ScopeValues()

	for _, property := range relevant {
		owner, ok := version.ObjectType(property.Object())
		if !ok {
			return nil, newError(ErrorInvariantViolation,
				"dependency object owner could not be resolved", nil)
		}

		for _, binding := range version.ValueBindingsForTarget(
			semantic.ObjectPropertyTarget(property)) {
			if resolverscope.Matches(binding.ScopeSelectors(), scopeValues) {
				boundaries = appendBindingBoundaries(boundaries, binding, from, to)
			}
		}

		for _, binding := range version.ValueBindingsForTarget(
			semantic.TypePropertyTarget(owner, property.DefinitionID())) {
			if resolverscope.Matches(binding.ScopeSelectors(), scopeValues) {
				boundaries = appendBindingBoundaries(boundaries, binding, from, to)
			}
		}
	}

	if scenario, ok := context.Scenario(); ok {
		for _, override := range scenario.Overrides() {
			if _, relevantTarget := relevantTargets[override.Target()]; !relevantTarget ||
				!resolverscope.Matches(override.ScopeSelectors(), scopeValues) {
				continue
			}
			boundaries = appendOverrideBoundaries(boundaries, override, from, to)
		}
	}

	sort.Slice(boundaries, func(i, j int) bool {
		return boundaries[i].Cmp(boundaries[j]) < 0
	})

	return uniquePeriodBoundaries(boundaries), nil
}

func periodDependencies(
	graph dependency.Graph,
	target address.PropertyAddress,
) []address.PropertyAddress {
	seen := map[address.PropertyAddress]struct{}{target: {}}
	pending := []address.PropertyAddress{target}

	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		for _, dependencyAddress := range graph.DependenciesOf(current) {
			if _, exists := seen[dependencyAddress]; exists {
				continue
			}
			seen[dependencyAddress] = struct{}{}
			pending = append(pending, dependencyAddress)
		}
	}

	result := make([]address.PropertyAddress, 0, len(seen))
	for property := range seen {
		result = append(result, property)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].String() < result[j].String()
	})

	return result
}

func relevantSet(properties []address.PropertyAddress) map[address.PropertyAddress]struct{} {
	result := make(map[address.PropertyAddress]struct{}, len(properties))
	for _, property := range properties {
		result[property] = struct{}{}
	}
	return result
}

func appendBindingBoundaries(
	boundaries []datetime.DateTime,
	binding semantic.ValueBinding,
	from datetime.DateTime,
	to datetime.DateTime,
) []datetime.DateTime {
	period, hasPeriod := binding.EffectivePeriod()
	return appendEffectivePeriodBoundaries(boundaries, period, hasPeriod, from, to)
}

func appendOverrideBoundaries(
	boundaries []datetime.DateTime,
	override domainscenario.Override,
	from datetime.DateTime,
	to datetime.DateTime,
) []datetime.DateTime {
	period, hasPeriod := override.EffectivePeriod()
	return appendEffectivePeriodBoundaries(boundaries, period, hasPeriod, from, to)
}

func appendEffectivePeriodBoundaries(
	boundaries []datetime.DateTime,
	period domaintime.EffectivePeriod,
	hasPeriod bool,
	from datetime.DateTime,
	to datetime.DateTime,
) []datetime.DateTime {
	if !hasPeriod {
		return boundaries
	}

	if boundary, ok := period.From(); ok && boundary.Cmp(from) > 0 && boundary.Cmp(to) < 0 {
		boundaries = append(boundaries, boundary)
	}
	if boundary, ok := period.To(); ok && boundary.Cmp(from) > 0 && boundary.Cmp(to) < 0 {
		boundaries = append(boundaries, boundary)
	}

	return boundaries
}

func uniquePeriodBoundaries(boundaries []datetime.DateTime) []datetime.DateTime {
	result := boundaries[:0]
	for _, boundary := range boundaries {
		if len(result) == 0 || !result[len(result)-1].Equal(boundary) {
			result = append(result, boundary)
		}
	}
	return result
}
