package provenance

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// Node is one immutable node in an evaluation provenance tree.
//
// A node may additionally carry exactly one of:
//   - a concrete PropertyAddress;
//   - a traversed Relation and its related Entity;
//   - an operation name;
//   - the selected ValueBinding;
//   - the selected calculation Override;
//   - the selected Scenario + ScenarioOverride.
//
// Children preserve the actual evaluation path. Branches that were not
// evaluated because of short-circuit, IF selection or an effective override
// are intentionally absent.
type Node struct {
	kind Kind

	property    address.PropertyAddress
	hasProperty bool

	relation         address.ObjectRef
	relatedEntity    address.ObjectRef
	hasAggregateItem bool

	operation string
	decision  Decision

	resolved    value.Resolved
	hasResolved bool

	binding    semantic.ValueBinding
	hasBinding bool

	override    runtime.Override
	hasOverride bool

	scenario         domainscenario.Scenario
	scenarioOverride domainscenario.Override
	hasScenario      bool

	children []Node
}

func NewProperty(
	kind Kind,
	property address.PropertyAddress,
	children ...Node,
) Node {
	if kind != KindInputProperty &&
		kind != KindDerivedProperty &&
		kind != KindReference {
		panic("provenance: invalid property node kind")
	}

	if !property.IsValid() {
		panic("provenance: property address is invalid")
	}

	return Node{
		kind:        kind,
		property:    property,
		hasProperty: true,
		children:    cloneChildren(children),
	}
}

func NewConstant() Node {
	return Node{
		kind: KindConstant,
	}
}

// NewAggregateItem records one concrete semantic-graph traversal occurrence.
// Its children are only the applicability, filter and map steps that actually
// executed for this Relation. Relation UUID identity is retained even for a
// Count with no property reads, so Explain and observed-impact consumers do
// not have to reconstruct traversal from the AST.
func NewAggregateItem(
	relation address.ObjectRef,
	relatedEntity address.ObjectRef,
	children ...Node,
) Node {
	if !relation.IsValid() || relation.Kind() != address.ObjectRefRelation {
		panic("provenance: aggregate item relation must be a relation reference")
	}
	if !relatedEntity.IsValid() || relatedEntity.Kind() != address.ObjectRefEntity {
		panic("provenance: aggregate item related entity must be an entity reference")
	}

	return Node{
		kind:             KindAggregateItem,
		relation:         relation,
		relatedEntity:    relatedEntity,
		hasAggregateItem: true,
		children:         cloneChildren(children),
	}
}

func NewOperation(
	operation string,
	children ...Node,
) Node {
	return NewOperationDecision(operation, DecisionNone, children...)
}

// NewOperationDecision records a semantic control-flow decision for one
// actual operation. Children are only paths that were actually evaluated.
func NewOperationDecision(
	operation string,
	decision Decision,
	children ...Node,
) Node {
	if operation == "" {
		panic("provenance: operation name must not be empty")
	}
	if !decision.IsAllowedForOperation(operation) {
		panic("provenance: operation decision is invalid for operation")
	}

	return Node{
		kind:      KindOperation,
		operation: operation,
		decision:  decision,
		children:  cloneChildren(children),
	}
}

func NewCalculationOverride(
	override runtime.Override,
) Node {
	if !override.IsValid() {
		panic("provenance: calculation override is invalid")
	}

	return Node{
		kind:        KindCalculationOverride,
		override:    override,
		hasOverride: true,
	}
}

// NewScenarioOverride records the exact immutable Scenario and override that
// replaced normal property resolution/evaluation.
//
// For a Derived property this node means that the base Expression was
// deliberately bypassed for the current CalculationContext.
func NewScenarioOverride(
	scenario domainscenario.Scenario,
	override domainscenario.Override,
) Node {
	if !scenario.IsValid() {
		panic("provenance: scenario is invalid")
	}

	if !override.IsValid() {
		panic("provenance: scenario override is invalid")
	}

	if scenario.ID().IsZero() ||
		override.Target().Object().ID().IsZero() {
		panic("provenance: invalid scenario override source")
	}

	return Node{
		kind:             KindScenarioOverride,
		scenario:         scenario,
		scenarioOverride: override,
		hasScenario:      true,
	}
}

func NewBaseBinding(
	kind Kind,
	binding semantic.ValueBinding,
) Node {
	if kind != KindBaseObjectBinding &&
		kind != KindBaseTypeBinding {
		panic("provenance: invalid base binding node kind")
	}

	if !binding.IsValid() {
		panic("provenance: base value binding is invalid")
	}

	return Node{
		kind:       kind,
		binding:    binding,
		hasBinding: true,
	}
}

func NewMissingValue() Node {
	return Node{
		kind: KindMissingValue,
	}
}

func NewAmbiguousValue() Node {
	return Node{
		kind: KindAmbiguousValue,
	}
}

func (n Node) Kind() Kind {
	return n.kind
}

func (n Node) Property() (
	address.PropertyAddress,
	bool,
) {
	return n.property,
		n.hasProperty
}

// AggregateItem returns the concrete Relation and opposite-endpoint Entity
// represented by a KindAggregateItem provenance node.
func (n Node) AggregateItem() (
	address.ObjectRef,
	address.ObjectRef,
	bool,
) {
	return n.relation, n.relatedEntity, n.hasAggregateItem
}

func (n Node) Operation() (
	string,
	bool,
) {
	if n.operation == "" {
		return "", false
	}

	return n.operation, true
}

// Decision returns an operation's actual control-flow decision. Non-operation
// nodes and ordinary strict operations return DecisionNone.
func (n Node) Decision() Decision { return n.decision }

// WithResolved returns a copy recording the exact state produced at this
// provenance step. The evaluator attaches it as each Result is created, so
// Explain never has to replay evaluation.
func (n Node) WithResolved(resolved value.Resolved) Node {
	if !n.IsValid() {
		panic("provenance: cannot attach a result to an invalid node")
	}
	if !resolved.IsValid() {
		panic("provenance: resolved result is invalid")
	}
	n.resolved = resolved
	n.hasResolved = true
	return n
}

// Resolved returns the actual result recorded for this step. Nodes manually
// constructed outside the evaluator may deliberately have no recorded state.
func (n Node) Resolved() (value.Resolved, bool) { return n.resolved, n.hasResolved }

func (n Node) Binding() (
	semantic.ValueBinding,
	bool,
) {
	return n.binding,
		n.hasBinding
}

func (n Node) Override() (
	runtime.Override,
	bool,
) {
	return n.override,
		n.hasOverride
}

// ScenarioOverride returns the exact Scenario and override represented by a
// KindScenarioOverride node.
func (n Node) ScenarioOverride() (
	domainscenario.Scenario,
	domainscenario.Override,
	bool,
) {
	return n.scenario,
		n.scenarioOverride,
		n.hasScenario
}

func (n Node) Children() []Node {
	return cloneChildren(
		n.children,
	)
}

func (n Node) IsValid() bool {
	if !n.kind.IsValid() {
		return false
	}
	if n.hasResolved && !n.resolved.IsValid() {
		return false
	}

	for _, child := range n.children {
		if !child.IsValid() {
			return false
		}
	}

	switch n.kind {
	case KindInputProperty,
		KindDerivedProperty,
		KindReference:

		return n.hasProperty &&
			n.property.IsValid() &&
			!n.hasAggregateItem &&
			n.operation == "" &&
			n.decision == DecisionNone &&
			!n.hasBinding &&
			!n.hasOverride &&
			!n.hasScenario

	case KindConstant:
		return !n.hasProperty &&
			!n.hasAggregateItem &&
			n.operation == "" &&
			n.decision == DecisionNone &&
			!n.hasBinding &&
			!n.hasOverride &&
			!n.hasScenario &&
			len(n.children) == 0

	case KindOperation:
		return !n.hasProperty &&
			!n.hasAggregateItem &&
			n.operation != "" &&
			n.decision.IsAllowedForOperation(n.operation) &&
			!n.hasBinding &&
			!n.hasOverride &&
			!n.hasScenario

	case KindCalculationOverride:
		return !n.hasProperty &&
			!n.hasAggregateItem &&
			n.operation == "" &&
			n.decision == DecisionNone &&
			!n.hasBinding &&
			n.hasOverride &&
			n.override.IsValid() &&
			!n.hasScenario &&
			len(n.children) == 0

	case KindScenarioOverride:
		return !n.hasProperty &&
			!n.hasAggregateItem &&
			n.operation == "" &&
			n.decision == DecisionNone &&
			!n.hasBinding &&
			!n.hasOverride &&
			n.hasScenario &&
			n.scenario.IsValid() &&
			n.scenarioOverride.IsValid() &&
			len(n.children) == 0

	case KindBaseObjectBinding,
		KindBaseTypeBinding:

		return !n.hasProperty &&
			!n.hasAggregateItem &&
			n.operation == "" &&
			n.decision == DecisionNone &&
			n.hasBinding &&
			n.binding.IsValid() &&
			!n.hasOverride &&
			!n.hasScenario &&
			len(n.children) == 0

	case KindMissingValue,
		KindAmbiguousValue:

		return !n.hasProperty &&
			!n.hasAggregateItem &&
			n.operation == "" &&
			n.decision == DecisionNone &&
			!n.hasBinding &&
			!n.hasOverride &&
			!n.hasScenario &&
			len(n.children) == 0

	case KindAggregateItem:
		return !n.hasProperty &&
			n.operation == "" &&
			n.decision == DecisionNone &&
			!n.hasBinding &&
			!n.hasOverride &&
			!n.hasScenario &&
			n.hasAggregateItem &&
			n.relation.IsValid() &&
			n.relation.Kind() == address.ObjectRefRelation &&
			n.relatedEntity.IsValid() &&
			n.relatedEntity.Kind() == address.ObjectRefEntity

	default:
		return false
	}
}

func cloneChildren(
	source []Node,
) []Node {
	if len(source) == 0 {
		return nil
	}

	result := make(
		[]Node,
		len(source),
	)

	copy(
		result,
		source,
	)

	for _, child := range result {
		if !child.IsValid() {
			panic(
				"provenance: child node is invalid",
			)
		}
	}

	return result
}
