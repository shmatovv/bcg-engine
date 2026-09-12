package impact

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/plan"
	"github.com/shmatovv/bcg-engine/internal/provenance"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// AnalyzeStructural reports every concrete property that may downstream-read
// source in version's immutable value dependency graph. It does not evaluate
// values, so Scope, effective time, Null and Undefined do not alter the
// result. Source itself is deliberately excluded.
func AnalyzeStructural(version model.ModelVersion, source address.PropertyAddress) (Result, error) {
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
	graph, err := baseGraph(version, source)
	if err != nil {
		return Result{}, err
	}
	effective, err := plan.Build(version, graph.Vertices(), context)
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
	effective, err := AnalyzeEffective(version, source, context)
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
	// Scalar roots share one ordinary evaluation state. This preserves the
	// evaluator's semantics and avoids evaluating common upstream subgraphs
	// once for every observed downstream target.
	if len(scalarTargets) > 0 {
		observedPlan, err := plan.Build(version, scalarTargets, context)
		if err != nil {
			return Result{}, newError(ErrorEffectivePlanBuild, "failed to build observed scalar evaluation plan", err)
		}
		evaluated, err := evaluator.EvaluatePlan(version, observedPlan)
		if err != nil {
			return Result{}, newError(ErrorEvaluation, "ordinary scalar evaluation failed while observing dependencies", err)
		}
		for _, target := range scalarTargets {
			result, ok := evaluated.Result(target)
			if !ok || !result.IsValid() {
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

func baseGraph(version model.ModelVersion, source address.PropertyAddress) (dependency.Graph, error) {
	if !version.IsCommitted() {
		return dependency.Graph{}, newError(ErrorInvalidModelVersion, "Impact requires a committed ModelVersion", nil)
	}
	if !source.IsValid() {
		return dependency.Graph{}, newError(ErrorInvalidSource, "Impact source property address is invalid", nil)
	}
	graph, err := dependency.Build(version)
	if err != nil {
		return dependency.Graph{}, newError(ErrorDependencyGraphBuild, "failed to build value dependency graph", err)
	}
	if !graph.HasVertex(source) {
		return dependency.Graph{}, newError(ErrorSourceNotFound, "Impact source is not a property in the committed ModelVersion", nil)
	}
	return graph, nil
}

func traverse(version model.ModelVersion, source address.PropertyAddress, mode Mode, dependents func(address.PropertyAddress) []address.PropertyAddress) Result {
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
	return newResult(version.ID(), source, mode, depths, edges)
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
