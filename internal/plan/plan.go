package plan

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// NodeKind identifies how one concrete property must be evaluated in this
// Effective Evaluation Plan.
type NodeKind uint8

const (
	NodeInvalid NodeKind = iota

	NodeInput
	NodeDerivedExpression
	NodeCalculationOverride
	NodeScenarioOverride
)

func (k NodeKind) IsValid() bool {
	switch k {
	case NodeInput,
		NodeDerivedExpression,
		NodeCalculationOverride,
		NodeScenarioOverride:
		return true

	default:
		return false
	}
}

func (k NodeKind) String() string {
	switch k {
	case NodeInput:
		return "Input"

	case NodeDerivedExpression:
		return "DerivedExpression"

	case NodeCalculationOverride:
		return "CalculationOverride"

	case NodeScenarioOverride:
		return "ScenarioOverride"

	default:
		return "Invalid"
	}
}

// Node describes the effective evaluation strategy for one concrete property.
//
// Override nodes carry the already-resolved override-layer Outcome.
//
// A Scenario outcome may itself be Undefined(AmbiguousValue). That is still
// a final Scenario-layer result and therefore cuts upstream base
// dependencies.
type Node struct {
	target address.PropertyAddress
	kind   NodeKind

	outcome    resolvervalue.Outcome
	hasOutcome bool
}

func newBaseNode(
	target address.PropertyAddress,
	kind NodeKind,
) Node {
	if !target.IsValid() {
		panic("plan: node target is invalid")
	}

	if kind != NodeInput &&
		kind != NodeDerivedExpression {
		panic("plan: invalid base node kind")
	}

	return Node{
		target: target,
		kind:   kind,
	}
}

func newOverrideNode(
	target address.PropertyAddress,
	kind NodeKind,
	outcome resolvervalue.Outcome,
) Node {
	if !target.IsValid() {
		panic("plan: node target is invalid")
	}

	if kind != NodeCalculationOverride &&
		kind != NodeScenarioOverride {
		panic("plan: invalid override node kind")
	}

	if !outcome.IsValid() {
		panic("plan: override outcome is invalid")
	}

	return Node{
		target:     target,
		kind:       kind,
		outcome:    outcome,
		hasOutcome: true,
	}
}

func (n Node) Target() address.PropertyAddress {
	return n.target
}

func (n Node) Kind() NodeKind {
	return n.kind
}

// Outcome returns the resolved replacement value carried by an override node.
//
// false is returned for ordinary Input and DerivedExpression nodes.
func (n Node) Outcome() (
	resolvervalue.Outcome,
	bool,
) {
	return n.outcome,
		n.hasOutcome
}

// CutsDependencies reports whether this node replaces the property's ordinary
// computation and therefore removes all of its base incoming dependencies
// from the Effective Evaluation Plan.
func (n Node) CutsDependencies() bool {
	return n.kind == NodeCalculationOverride ||
		n.kind == NodeScenarioOverride
}

func (n Node) IsValid() bool {
	if !n.target.IsValid() ||
		!n.kind.IsValid() {
		return false
	}

	switch n.kind {
	case NodeInput,
		NodeDerivedExpression:

		return !n.hasOutcome &&
			!n.outcome.IsValid()

	case NodeCalculationOverride,
		NodeScenarioOverride:

		return n.hasOutcome &&
			n.outcome.IsValid()

	default:
		return false
	}
}

// Edge represents one effective direct dependency.
//
// Direction:
//
//	dependency -> dependent
type Edge struct {
	from address.PropertyAddress
	to   address.PropertyAddress
}

func newEdge(
	from address.PropertyAddress,
	to address.PropertyAddress,
) Edge {
	if !from.IsValid() ||
		!to.IsValid() {
		panic("plan: edge addresses must be valid")
	}

	return Edge{
		from: from,
		to:   to,
	}
}

func (e Edge) From() address.PropertyAddress {
	return e.from
}

func (e Edge) To() address.PropertyAddress {
	return e.to
}

func (e Edge) IsValid() bool {
	return e.from.IsValid() &&
		e.to.IsValid()
}

// Plan is an immutable target-scoped Effective Evaluation Plan.
//
// A Plan is bound not only to one ModelVersion but also to the exact
// CalculationContext from which it was built. This is important because
// Scenario, Scope, Time and calculation overrides may change the effective
// dependency structure.
//
// It contains only properties reachable from Roots after override pruning.
//
// Order is deterministic and dependency-first:
//
//	dependency always appears before dependent.
//
// Evaluator does not have to eagerly execute every node in Order. Runtime
// short-circuit semantics may legitimately leave statically reachable nodes
// unevaluated.
type Plan struct {
	modelVersionID identity.UUID

	context runtime.CalculationContext

	roots []address.PropertyAddress

	nodes map[address.PropertyAddress]Node

	edges []Edge

	order []address.PropertyAddress

	incoming map[address.PropertyAddress][]address.PropertyAddress
}

func newPlan(
	modelVersionID identity.UUID,
	context runtime.CalculationContext,
	roots []address.PropertyAddress,
	nodes map[address.PropertyAddress]Node,
	edges []Edge,
	order []address.PropertyAddress,
) Plan {
	if modelVersionID.IsZero() {
		panic("plan: ModelVersion id must not be zero")
	}

	if !context.IsValid() {
		panic("plan: CalculationContext is invalid")
	}

	if len(roots) == 0 {
		panic("plan: roots must not be empty")
	}

	copiedRoots := append(
		[]address.PropertyAddress(nil),
		roots...,
	)

	copiedEdges := append(
		[]Edge(nil),
		edges...,
	)

	copiedOrder := append(
		[]address.PropertyAddress(nil),
		order...,
	)

	copiedNodes := make(
		map[address.PropertyAddress]Node,
		len(nodes),
	)

	incoming := make(
		map[address.PropertyAddress][]address.PropertyAddress,
	)

	for target, node := range nodes {
		if target != node.Target() ||
			!node.IsValid() {
			panic("plan: invalid node map")
		}

		copiedNodes[target] = node
	}

	for _, edge := range copiedEdges {
		if !edge.IsValid() {
			panic("plan: invalid edge")
		}

		incoming[edge.to] = append(
			incoming[edge.to],
			edge.from,
		)
	}

	for target := range incoming {
		sortPropertyAddresses(
			incoming[target],
		)
	}

	return Plan{
		modelVersionID: modelVersionID,
		context:        context,
		roots:          copiedRoots,
		nodes:          copiedNodes,
		edges:          copiedEdges,
		order:          copiedOrder,
		incoming:       incoming,
	}
}

func (p Plan) ModelVersionID() identity.UUID {
	return p.modelVersionID
}

// Context returns the exact immutable CalculationContext for which this plan
// was built.
//
// CalculationContext exposes no mutating operations, so returning it by value
// preserves the Plan's logical immutability.
func (p Plan) Context() runtime.CalculationContext {
	return p.context
}

func (p Plan) Len() int {
	return len(p.nodes)
}

func (p Plan) EdgeLen() int {
	return len(p.edges)
}

func (p Plan) Roots() []address.PropertyAddress {
	return append(
		[]address.PropertyAddress(nil),
		p.roots...,
	)
}

func (p Plan) Node(
	target address.PropertyAddress,
) (Node, bool) {
	node, ok := p.nodes[target]

	return node, ok
}

// Nodes returns nodes in deterministic dependency-first plan order.
func (p Plan) Nodes() []Node {
	result := make(
		[]Node,
		0,
		len(p.nodes),
	)

	for _, target := range p.order {
		result = append(
			result,
			p.nodes[target],
		)
	}

	return result
}

func (p Plan) Edges() []Edge {
	return append(
		[]Edge(nil),
		p.edges...,
	)
}

// Order returns deterministic dependency-first static plan order.
//
// Runtime evaluation may execute only a subset because If/AND/OR/Coalesce
// preserve their short-circuit semantics.
func (p Plan) Order() []address.PropertyAddress {
	return append(
		[]address.PropertyAddress(nil),
		p.order...,
	)
}

func (p Plan) DependenciesOf(
	target address.PropertyAddress,
) []address.PropertyAddress {
	return append(
		[]address.PropertyAddress(nil),
		p.incoming[target]...,
	)
}

func (p Plan) IsValid() bool {
	if p.modelVersionID.IsZero() ||
		!p.context.IsValid() ||
		len(p.roots) == 0 ||
		len(p.nodes) == 0 {
		return false
	}

	if len(p.order) !=
		len(p.nodes) {
		return false
	}

	seen := make(
		map[address.PropertyAddress]struct{},
		len(p.order),
	)

	position := make(
		map[address.PropertyAddress]int,
		len(p.order),
	)

	for index, target := range p.order {
		node, ok :=
			p.nodes[target]

		if !ok ||
			!node.IsValid() ||
			node.Target() != target {
			return false
		}

		if _, duplicate :=
			seen[target]; duplicate {
			return false
		}

		seen[target] =
			struct{}{}

		position[target] =
			index
	}

	for _, root := range p.roots {
		if !root.IsValid() {
			return false
		}

		if _, ok :=
			p.nodes[root]; !ok {
			return false
		}
	}

	for _, edge := range p.edges {
		if !edge.IsValid() {
			return false
		}

		fromPosition,
			fromOK :=
			position[edge.from]

		toPosition,
			toOK :=
			position[edge.to]

		if !fromOK ||
			!toOK ||
			fromPosition >= toPosition {
			return false
		}

		if p.nodes[edge.to].CutsDependencies() {
			return false
		}
	}

	return true
}

func sortPropertyAddresses(
	values []address.PropertyAddress,
) {
	sort.Slice(
		values,
		func(i, j int) bool {
			return comparePropertyAddress(
				values[i],
				values[j],
			) < 0
		},
	)
}

func comparePropertyAddress(
	left address.PropertyAddress,
	right address.PropertyAddress,
) int {
	if left.Object().Kind() <
		right.Object().Kind() {
		return -1
	}

	if left.Object().Kind() >
		right.Object().Kind() {
		return 1
	}

	if comparison :=
		compareUUID(
			left.Object().ID(),
			right.Object().ID(),
		); comparison != 0 {
		return comparison
	}

	return compareUUID(
		left.DefinitionID(),
		right.DefinitionID(),
	)
}

func compareUUID(
	left identity.UUID,
	right identity.UUID,
) int {
	leftBytes :=
		left.Bytes()

	rightBytes :=
		right.Bytes()

	for index := range leftBytes {

		switch {
		case leftBytes[index] <
			rightBytes[index]:

			return -1

		case leftBytes[index] >
			rightBytes[index]:

			return 1
		}
	}

	return 0
}
