package plan

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/internal/model"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// Build constructs the target-scoped Effective Evaluation Plan for one At
// CalculationContext.
//
// The immutable Base Dependency Graph is never mutated.
//
// Build walks the Base Dependency Graph backwards from roots and removes
// upstream dependencies whenever a Calculation or Scenario override resolves
// the concrete property in the current context.
//
// The resulting Plan is permanently bound to this exact CalculationContext.
func Build(
	version model.ModelVersion,
	roots []address.PropertyAddress,
	context runtime.CalculationContext,
) (Plan, error) {
	if !version.IsCommitted() {
		return Plan{},
			newError(
				ErrorInvalidModelVersion,
				"Effective Evaluation Plan requires a committed ModelVersion",
				nil,
			)
	}

	baseGraph, err :=
		dependency.Build(
			version,
		)

	if err != nil {
		return Plan{},
			newError(
				ErrorDependencyGraphBuild,
				"failed to build Base Dependency Graph",
				err,
			)
	}

	return BuildWithDependencyGraph(
		version,
		baseGraph,
		roots,
		context,
	)
}

// BuildWithDependencyGraph constructs a Plan using a dependency graph already
// built from version. It exists for runtime execution layers that retain the
// immutable graph for one ModelVersion and must not rescan every expression
// AST on each context transition.
//
// The caller must provide the concrete dependency.Graph for the same committed
// version. This package validates all roots and context/model interactions; a
// mismatched graph is an internal invariant violation at its call site.
func BuildWithDependencyGraph(
	version model.ModelVersion,
	baseGraph dependency.Graph,
	roots []address.PropertyAddress,
	context runtime.CalculationContext,
) (Plan, error) {
	if !version.IsCommitted() {
		return Plan{},
			newError(
				ErrorInvalidModelVersion,
				"Effective Evaluation Plan requires a committed ModelVersion",
				nil,
			)
	}

	if !context.IsValid() {
		return Plan{},
			newError(
				ErrorInvalidCalculationContext,
				"CalculationContext is invalid",
				nil,
			)
	}

	if context.TimeContext().Kind() != domaintime.ContextAt {
		return Plan{},
			newError(
				ErrorUnsupportedTimeContext,
				"Effective Evaluation Plan currently requires an At time context",
				nil,
			)
	}

	if len(roots) == 0 {
		return Plan{},
			newError(
				ErrorInvalidRoot,
				"at least one evaluation root is required",
				nil,
			)
	}

	overrideLayers, err := resolvervalue.NewOverrideLayers(version, context)
	if err != nil {
		return Plan{},
			newError(
				ErrorInvalidCalculationContext,
				"CalculationContext is not valid for the committed ModelVersion",
				err,
			)
	}

	canonicalRoots, err :=
		validateAndCanonicalizeRoots(
			baseGraph,
			roots,
		)

	if err != nil {
		return Plan{},
			err
	}

	// builder owns every collection below until ownership is transferred to the
	// immutable Plan at the end of this function. The graph bounds all of
	// their cardinalities, so reserving here avoids repeated map/slice growth
	// when a Session prepares a large effective environment.
	vertexCapacity := baseGraph.Len()
	edgeCapacity := baseGraph.EdgeLen()

	builder := builder{
		version:   version,
		context:   context,
		base:      baseGraph,
		overrides: overrideLayers,

		nodes: make(
			map[address.PropertyAddress]Node,
			vertexCapacity,
		),

		state: make(
			map[address.PropertyAddress]visitState,
			vertexCapacity,
		),

		edges: make(
			map[edgeKey]struct{},
			edgeCapacity,
		),

		order: make(
			[]address.PropertyAddress,
			0,
			vertexCapacity,
		),
	}

	for _, root := range canonicalRoots {

		if err :=
			builder.visit(
				root,
			); err != nil {

			return Plan{},
				err
		}
	}

	edges :=
		builder.sortedEdges()

	result :=
		newPlan(
			version.ID(),
			context,
			canonicalRoots,
			builder.nodes,
			edges,
			builder.order,
		)

	if !result.IsValid() {
		return Plan{},
			newError(
				ErrorInvariantViolation,
				"constructed Effective Evaluation Plan is invalid",
				nil,
			)
	}

	return result,
		nil
}

type visitState uint8

const (
	visitUnseen visitState = iota
	visitActive
	visitDone
)

type edgeKey struct {
	from address.PropertyAddress
	to   address.PropertyAddress
}

type builder struct {
	version model.ModelVersion
	context runtime.CalculationContext

	base dependency.Graph

	overrides resolvervalue.OverrideLayers

	nodes map[address.PropertyAddress]Node
	state map[address.PropertyAddress]visitState
	edges map[edgeKey]struct{}

	order []address.PropertyAddress
}

func (b *builder) visit(
	root address.PropertyAddress,
) error {
	// Effective-plan construction can traverse a concrete property chain much
	// deeper than any single AST. Keep the DFS explicitly on the heap rather
	// than on the goroutine stack: a valid, deeply chained ModelVersion must
	// produce a deterministic plan (and later a controlled evaluator depth
	// error when applicable), never terminate the host process.
	type frame struct {
		target       address.PropertyAddress
		dependencies []address.PropertyAddress
		next         int
	}

	push := func(target address.PropertyAddress) (frame, bool, error) {
		switch b.state[target] {
		case visitDone:
			return frame{}, false, nil

		case visitActive:
			return frame{}, false, newError(
				ErrorCycle,
				"cycle encountered while building Effective Evaluation Plan",
				nil,
			)
		}

		b.state[target] = visitActive
		node, cutDependencies, err := b.resolveNode(target)
		if err != nil {
			return frame{}, false, err
		}
		b.nodes[target] = node

		result := frame{target: target}
		if !cutDependencies {
			result.dependencies = b.base.DependenciesOf(target)
		}
		return result, true, nil
	}

	initial, pushed, err := push(root)
	if err != nil || !pushed {
		return err
	}
	stack := []frame{initial}

	for len(stack) > 0 {
		last := len(stack) - 1
		current := &stack[last]
		if current.next < len(current.dependencies) {
			dependencyAddress := current.dependencies[current.next]
			current.next++

			next, pushed, err := push(dependencyAddress)
			if err != nil {
				return err
			}
			// The base graph owns the direct relation; whether the child was
			// already complete or has just been scheduled does not change this
			// effective edge. An active child was rejected by push above.
			b.edges[edgeKey{from: dependencyAddress, to: current.target}] = struct{}{}
			if pushed {
				stack = append(stack, next)
				continue
			}
			continue
		}

		b.state[current.target] = visitDone
		// Post-order traversal gives deterministic dependency-first order.
		b.order = append(b.order, current.target)
		stack = stack[:last]
	}

	return nil
}

func (b *builder) resolveNode(
	target address.PropertyAddress,
) (Node, bool, error) {
	definition, ok :=
		b.version.
			PropertyDefinition(
				target.DefinitionID(),
			)

	if !ok || !definition.IsValid() {
		return Node{}, false, newError(
			ErrorDefinitionNotFound,
			"plan vertex PropertyDefinition could not be resolved from committed ModelVersion",
			nil,
		)
	}

	if definition.TemporalMode() == schema.TemporalSeries {
		return b.resolveSeriesNode(target, definition)
	}

	overrideOutcome, overrideFound, err := b.overrides.ResolveScalar(target, definition)
	if err != nil {
		return Node{},
			false,
			newError(
				ErrorOverrideResolution,
				"failed to resolve effective override while building plan",
				err,
			)
	}

	if overrideFound {
		overrideKind := NodeScenarioOverride
		if _, calculationOverride := b.context.Override(target); calculationOverride {
			overrideKind = NodeCalculationOverride
		}

		return newOverrideNode(
			target,
			overrideKind,
			overrideOutcome,
		), true, nil
	}

	switch definition.
		Computation().
		Kind() {

	case schema.ComputationInput:
		return newBaseNode(
			target,
			NodeInput,
		), false, nil

	case schema.ComputationDerived:
		return newBaseNode(
			target,
			NodeDerivedExpression,
		), false, nil

	default:
		return Node{},
			false,
			newError(
				ErrorInvalidComputation,
				"plan vertex has invalid computation kind",
				nil,
			)
	}
}

func (b *builder) resolveSeriesNode(
	target address.PropertyAddress,
	definition schema.PropertyDefinition,
) (Node, bool, error) {
	override, overrideFound, err := b.overrides.ResolveSeries(target, definition)
	if err != nil {
		return Node{}, false, newError(ErrorOverrideResolution, "failed to resolve effective Series override while building plan", err)
	}
	if overrideFound {
		if !override.IsValid() {
			return Node{}, false, newError(ErrorInvariantViolation, "effective Series override outcome is invalid", nil)
		}

		overrideKind := NodeScenarioOverride
		if _, calculationOverride := b.context.Override(target); calculationOverride {
			overrideKind = NodeCalculationOverride
		}

		return newSeriesOverrideNode(target, overrideKind), true, nil
	}

	switch definition.Computation().Kind() {
	case schema.ComputationInput:
		return newBaseNode(target, NodeInput), false, nil
	case schema.ComputationDerived:
		return newBaseNode(target, NodeDerivedExpression), false, nil
	default:
		return Node{}, false, newError(ErrorInvalidComputation, "Series plan vertex has invalid computation kind", nil)
	}
}

func (b *builder) sortedEdges() []Edge {
	result := make(
		[]Edge,
		0,
		len(b.edges),
	)

	for key := range b.edges {

		result =
			append(
				result,
				newEdge(
					key.from,
					key.to,
				),
			)
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			if comparison :=
				comparePropertyAddress(
					result[i].from,
					result[j].from,
				); comparison != 0 {

				return comparison < 0
			}

			return comparePropertyAddress(
				result[i].to,
				result[j].to,
			) < 0
		},
	)

	return result
}

func validateAndCanonicalizeRoots(
	base dependency.Graph,
	roots []address.PropertyAddress,
) ([]address.PropertyAddress, error) {
	result := append(
		[]address.PropertyAddress(nil),
		roots...,
	)

	for _, root := range result {

		if !root.IsValid() {
			return nil,
				newError(
					ErrorInvalidRoot,
					"evaluation root PropertyAddress is invalid",
					nil,
				)
		}

		if !base.HasVertex(
			root,
		) {
			return nil,
				newError(
					ErrorRootNotFound,
					"evaluation root does not exist in the committed concrete dependency graph",
					nil,
				)
		}
	}

	sortPropertyAddresses(
		result,
	)

	unique :=
		result[:0]

	for _, root := range result {

		if len(unique) > 0 &&
			unique[len(unique)-1] == root {

			continue
		}

		unique =
			append(
				unique,
				root,
			)
	}

	return unique,
		nil
}
