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

	if !context.IsValid() {
		return Plan{},
			newError(
				ErrorInvalidCalculationContext,
				"CalculationContext is invalid",
				nil,
			)
	}

	if context.
		TimeContext().
		Kind() !=
		domaintime.ContextAt {

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

	if err :=
		resolvervalue.ValidateContext(
			version,
			context,
		); err != nil {

		return Plan{},
			newError(
				ErrorInvalidCalculationContext,
				"CalculationContext is not valid for the committed ModelVersion",
				err,
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

	canonicalRoots, err :=
		validateAndCanonicalizeRoots(
			baseGraph,
			roots,
		)

	if err != nil {
		return Plan{},
			err
	}

	builder := builder{
		version: version,
		context: context,
		base:    baseGraph,

		nodes: make(
			map[address.PropertyAddress]Node,
		),

		state: make(
			map[address.PropertyAddress]visitState,
		),

		edges: make(
			map[edgeKey]struct{},
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

	nodes map[address.PropertyAddress]Node
	state map[address.PropertyAddress]visitState
	edges map[edgeKey]struct{}

	order []address.PropertyAddress
}

func (b *builder) visit(
	target address.PropertyAddress,
) error {
	switch b.state[target] {
	case visitDone:
		return nil

	case visitActive:
		return newError(
			ErrorCycle,
			"cycle encountered while building Effective Evaluation Plan",
			nil,
		)
	}

	b.state[target] =
		visitActive

	node,
		cutDependencies,
		err :=
		b.resolveNode(
			target,
		)

	if err != nil {
		return err
	}

	b.nodes[target] =
		node

	if !cutDependencies {
		for _, dependencyAddress := range b.base.
			DependenciesOf(
				target,
			) {

			if err :=
				b.visit(
					dependencyAddress,
				); err != nil {

				return err
			}

			b.edges[edgeKey{
				from: dependencyAddress,
				to:   target,
			}] = struct{}{}
		}
	}

	b.state[target] =
		visitDone

	// Post-order traversal gives deterministic dependency-first static order.
	b.order =
		append(
			b.order,
			target,
		)

	return nil
}

func (b *builder) resolveNode(
	target address.PropertyAddress,
) (Node, bool, error) {
	calculationOutcome,
		calculationFound,
		err :=
		resolvervalue.
			ResolveCalculationOverride(
				b.version,
				target,
				b.context,
			)

	if err != nil {
		return Node{},
			false,
			newError(
				ErrorOverrideResolution,
				"failed to resolve Calculation override while building plan",
				err,
			)
	}

	if calculationFound {
		return newOverrideNode(
			target,
			NodeCalculationOverride,
			calculationOutcome,
		), true, nil
	}

	scenarioOutcome,
		scenarioFound,
		err :=
		resolvervalue.
			ResolveScenarioOverride(
				b.version,
				target,
				b.context,
			)

	if err != nil {
		return Node{},
			false,
			newError(
				ErrorOverrideResolution,
				"failed to resolve Scenario override while building plan",
				err,
			)
	}

	if scenarioFound {
		return newOverrideNode(
			target,
			NodeScenarioOverride,
			scenarioOutcome,
		), true, nil
	}

	definition, ok :=
		b.version.
			PropertyDefinition(
				target.DefinitionID(),
			)

	if !ok ||
		!definition.IsValid() {

		return Node{},
			false,
			newError(
				ErrorDefinitionNotFound,
				"plan vertex PropertyDefinition could not be resolved from committed ModelVersion",
				nil,
			)
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
