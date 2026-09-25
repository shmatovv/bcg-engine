package impact

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/plan"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
)

// AnalyzeStructural reports every concrete property that may downstream-read
// source in version's immutable value dependency graph. It does not evaluate
// values, so Scope, effective time, Null and Undefined do not alter the
// result. Source itself is deliberately excluded.
func AnalyzeStructural(version model.ModelVersion, source address.PropertyAddress) (Result, error) {
	return AnalyzeStructuralSource(version, source)
}

// AnalyzeStructuralSource is the combined-source variant used by immutable
// runtime snapshots. The long-standing AnalyzeStructural API remains
// ModelVersion-only for compatibility with existing internal consumers.
func AnalyzeStructuralSource(version evaluator.Source, source address.PropertyAddress) (Result, error) {
	graph, err := baseGraph(version, source)
	if err != nil {
		return Result{}, err
	}
	return traverse(version, source, ModeStructural, graph.DependentsOf), nil
}

// AnalyzeEffective reports downstream dependencies in one At
// CalculationContext after ordinary Effective Evaluation Plan override
// pruning. A calculation or Scenario override removes only that property's
// incoming edges: its downstream dependents remain affected when the override
// value itself changes. Lazy expression branches remain structural in this
// mode; use AnalyzeObserved to inspect what the ordinary evaluator actually
// executed.
func AnalyzeEffective(version model.ModelVersion, source address.PropertyAddress, context runtime.CalculationContext) (Result, error) {
	return AnalyzeEffectiveSource(version, source, context)
}

func AnalyzeEffectiveSource(version evaluator.Source, source address.PropertyAddress, context runtime.CalculationContext) (Result, error) {
	graph, err := baseGraph(version, source)
	if err != nil {
		return Result{}, err
	}
	effective, err := plan.BuildWithDependencyGraph(version, graph, graph.Vertices(), context)
	if err != nil {
		return Result{}, newError(ErrorEffectivePlanBuild, "failed to build complete Effective Evaluation Plan", err)
	}
	outgoing := make(map[address.PropertyAddress][]address.PropertyAddress)
	for _, edge := range effective.Edges() {
		outgoing[edge.From()] = append(outgoing[edge.From()], edge.To())
	}
	return traverse(version, source, ModeEffective, func(node address.PropertyAddress) []address.PropertyAddress {
		return append([]address.PropertyAddress(nil), outgoing[node]...)
	}), nil
}

// AnalyzeObserved reports only downstream property reads actually recorded by
// normal evaluator provenance for this CalculationContext. It first limits
// work to the effective downstream closure, then evaluates those possible
// dependents through Explain/ExplainSeries; it never implements a second AST
// evaluator. Thus lazy branches and override-bypassed expressions do not
// appear. For Series this is a property-level union of independently recorded
// sample paths; no sample is matched to another by array position.
func AnalyzeObserved(version model.ModelVersion, source address.PropertyAddress, context runtime.CalculationContext) (Result, error) {
	return AnalyzeObservedSource(version, source, context)
}

func AnalyzeObservedSource(version evaluator.Source, source address.PropertyAddress, context runtime.CalculationContext) (Result, error) {
	return analyzeObserved(
		version,
		source,
		context,
		func(targets []address.PropertyAddress) ([]evaluator.Result, error) {
			graph, graphErr := baseGraph(version, source)
			if graphErr != nil {
				return nil, graphErr
			}
			observedPlan, err := plan.BuildWithDependencyGraph(version, graph, targets, context)
			if err != nil {
				return nil, newError(ErrorEffectivePlanBuild, "failed to build observed scalar evaluation plan", err)
			}
			evaluated, err := evaluator.EvaluatePlan(version, observedPlan)
			if err != nil {
				return nil, newError(ErrorEvaluation, "ordinary scalar evaluation failed while observing dependencies", err)
			}
			results := make([]evaluator.Result, len(targets))
			for index, target := range targets {
				result, ok := evaluated.Result(target)
				if !ok || !result.IsValid() {
					return nil, newError(ErrorInvariantViolation, "observed scalar evaluation omitted a requested root", nil)
				}
				results[index] = result
			}
			return results, nil
		},
	)
}

// AnalyzeObservedWithSession derives observed impact from actual provenance
// produced through a shared evaluator Session. It has the same semantic output
// as AnalyzeObserved while reusing the Session's scalar property cache.
func AnalyzeObservedWithSession(session *evaluator.Session, source address.PropertyAddress, context runtime.CalculationContext) (Result, error) {
	if session == nil {
		return Result{}, newError(ErrorInvalidModelVersion, "evaluation Session is nil", nil)
	}
	version := session.Source()
	return analyzeObserved(
		version,
		source,
		context,
		func(targets []address.PropertyAddress) ([]evaluator.Result, error) {
			results, err := session.EvaluateMany(targets, context)
			if err != nil {
				return nil, newError(ErrorEvaluation, "Session scalar evaluation failed while observing dependencies", err)
			}
			return results, nil
		},
	)
}

func analyzeObserved(
	version evaluator.Source,
	source address.PropertyAddress,
	context runtime.CalculationContext,
	evaluateScalars func([]address.PropertyAddress) ([]evaluator.Result, error),
) (Result, error) {
	effective, err := AnalyzeEffectiveSource(version, source, context)
	if err != nil {
		return Result{}, err
	}
	actual := make(map[edgeKey]struct{})
	scalarTargets := make([]address.PropertyAddress, 0, effective.Len())
	seriesTargets := make([]address.PropertyAddress, 0)
	for _, candidate := range effective.Affected() {
		definition, ok := version.PropertyDefinition(candidate.Target().DefinitionID())
		if !ok || !definition.IsValid() {
			return Result{}, newError(ErrorInvariantViolation, "effective graph contains an unavailable property definition", nil)
		}
		if definition.TemporalMode() == schema.TemporalSeries {
			seriesTargets = append(seriesTargets, candidate.Target())
		} else {
			scalarTargets = append(scalarTargets, candidate.Target())
		}
	}
	// Scalar roots share either one ordinary evaluation state or one Session
	// cache generation. Both paths use ordinary evaluator provenance.
	if len(scalarTargets) > 0 {
		results, err := evaluateScalars(scalarTargets)
		if err != nil {
			return Result{}, err
		}
		if len(results) != len(scalarTargets) {
			return Result{}, newError(ErrorInvariantViolation, "observed scalar evaluator returned a wrong result count", nil)
		}
		for _, result := range results {
			if !result.IsValid() {
				return Result{}, newError(ErrorInvariantViolation, "observed scalar evaluation omitted a requested root", nil)
			}
			collectProvenanceEdges(result.Provenance(), address.PropertyAddress{}, actual)
		}
	}
	// Series evaluator demand is root-local by design. Each complete Series
	// root is therefore evaluated through its established public pipeline.
	for _, target := range seriesTargets {
		explanation, err := evaluator.ExplainSeries(version, target, context)
		if err != nil {
			return Result{}, newError(ErrorEvaluation, "ordinary Series evaluation failed while observing dependencies", err)
		}
		collectSeriesEdges(explanation, actual)
	}
	return traverse(version, source, ModeObserved, func(node address.PropertyAddress) []address.PropertyAddress {
		result := make([]address.PropertyAddress, 0)
		for edge := range actual {
			if edge.from == node {
				result = append(result, edge.to)
			}
		}
		sortAddresses(result)
		return result
	}), nil
}

func baseGraph(version evaluator.Source, source address.PropertyAddress) (dependency.Graph, error) {
	if !version.IsCommitted() {
		return dependency.Graph{}, newError(ErrorInvalidModelVersion, "Impact requires a committed ModelVersion", nil)
	}
	if !source.IsValid() {
		return dependency.Graph{}, newError(ErrorInvalidSource, "Impact source property address is invalid", nil)
	}
	var graph dependency.Graph
	var err error
	if cached, ok := version.(interface{ DependencyGraph() dependency.Graph }); ok {
		graph = cached.DependencyGraph()
	} else {
		graph, err = dependency.Build(version)
	}
	if err != nil {
		return dependency.Graph{}, newError(ErrorDependencyGraphBuild, "failed to build value dependency graph", err)
	}
	if !graph.HasVertex(source) {
		return dependency.Graph{}, newError(ErrorSourceNotFound, "Impact source is not a property in the committed ModelVersion", nil)
	}
	return graph, nil
}

func traverse(version evaluator.Source, source address.PropertyAddress, mode Mode, dependents func(address.PropertyAddress) []address.PropertyAddress) Result {
	depths := map[address.PropertyAddress]int{source: 0}
	edges := make(map[edgeKey]struct{})
	queue := []address.PropertyAddress{source}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		for _, dependent := range dependents(current) {
			edges[edgeKey{from: current, to: dependent}] = struct{}{}
			if _, seen := depths[dependent]; seen {
				continue
			}
			depths[dependent] = depths[current] + 1
			queue = append(queue, dependent)
		}
	}
	return newResult(version.SourceIdentity(), source, mode, depths, edges)
}

func collectSeriesEdges(explanation evaluator.SeriesExplanation, edges map[edgeKey]struct{}) {
	if _, node, undefined := explanation.Undefined(); undefined {
		collectProvenanceEdges(node, address.PropertyAddress{}, edges)
		return
	}
	for _, point := range explanation.Points() {
		collectProvenanceEdges(point.Root(), address.PropertyAddress{}, edges)
	}
	for _, interval := range explanation.Intervals() {
		collectProvenanceEdges(interval.Root(), address.PropertyAddress{}, edges)
	}
}

// collectProvenanceEdges derives direct property reads from recorded execution
// provenance. owner changes only at an actual property evaluation root.
// A Reference P beneath owner Q therefore records P -> Q, while P's own
// nested derivation is traversed under owner P. This preserves actual lazy
// paths and never infers dependencies from static expressions.
func collectProvenanceEdges(node provenance.Node, owner address.PropertyAddress, edges map[edgeKey]struct{}) {
	if property, ok := node.Property(); ok {
		switch node.Kind() {
		case provenance.KindInputProperty, provenance.KindDerivedProperty:
			owner = property
		case provenance.KindReference:
			if owner.IsValid() && property != owner {
				edges[edgeKey{from: property, to: owner}] = struct{}{}
			}
		}
	}
	for _, child := range node.Children() {
		collectProvenanceEdges(child, owner, edges)
	}
}

func sortAddresses(values []address.PropertyAddress) {
	sort.Slice(values, func(i, j int) bool { return lessAddress(values[i], values[j]) })
}
