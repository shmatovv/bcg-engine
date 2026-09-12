package bcg

import (
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/provenance"
)

// Explanation is actual structured scalar execution provenance. It is never
// reconstructed from a static expression and preserves duplicate occurrences.
type Explanation struct {
	target PropertyAddress
	result Result
	root   ExplanationNode
}

func explanationFromInternal(value evaluator.Explanation) Explanation {
	return Explanation{target: addressFromInternal(value.Target()), result: resultFromInternal(value.Result()), root: explanationNodeFromInternal(value.Root())}
}

// Target returns the property that was explained.
func (e Explanation) Target() PropertyAddress { return e.target }

// Result returns the explained evaluation result.
func (e Explanation) Result() Result { return e.result }

// Root returns the root actual-execution provenance node.
func (e Explanation) Root() ExplanationNode { return e.root }

// ExplanationKind is a semantic provenance-node kind.
type ExplanationKind string

// ExplanationNode is an immutable semantic projection of evaluator provenance.
type ExplanationNode struct {
	kind                        ExplanationKind
	property                    PropertyAddress
	hasProperty                 bool
	relationID, relatedEntityID ID
	hasAggregateItem            bool
	operation                   string
	decision                    string
	state                       ResultState
	value                       Value
	hasValue                    bool
	undefined                   UndefinedInfo
	overrideTarget              PropertyAddress
	hasOverride                 bool
	scenarioID                  ID
	hasScenario                 bool
	children                    []ExplanationNode
}

// Kind returns the semantic provenance-node kind.
func (n ExplanationNode) Kind() ExplanationKind { return n.kind }

// Property returns the property identity attached to this node, when present.
func (n ExplanationNode) Property() (PropertyAddress, bool) { return n.property, n.hasProperty }

// AggregateItem returns relation and related-Entity identities for an aggregate item.
func (n ExplanationNode) AggregateItem() (ID, ID, bool) {
	return n.relationID, n.relatedEntityID, n.hasAggregateItem
}

// Operation returns the recorded operation label, when one exists.
func (n ExplanationNode) Operation() (string, bool) { return n.operation, n.operation != "" }

// Decision returns the recorded Boolean/lazy decision label.
func (n ExplanationNode) Decision() string { return n.decision }

// State returns the outcome recorded at this provenance node. It is empty
// only for manually incomplete/invalid provenance, which normal evaluation
// never produces.
func (n ExplanationNode) State() ResultState { return n.state }

// Value returns the defined node value when State is Defined.
func (n ExplanationNode) Value() (Value, bool) { return n.value, n.hasValue }

// Undefined returns the node's structured Undefined cause when present.
func (n ExplanationNode) Undefined() (UndefinedInfo, bool) { return n.undefined, n.state == Undefined }

// CalculationOverrideTarget returns the applied calculation override target.
func (n ExplanationNode) CalculationOverrideTarget() (PropertyAddress, bool) {
	return n.overrideTarget, n.hasOverride
}

// ScenarioID returns the applied Scenario identity when this node used one.
func (n ExplanationNode) ScenarioID() (ID, bool) { return n.scenarioID, n.hasScenario }

// Children returns an owned ordered copy of actual child occurrences.
func (n ExplanationNode) Children() []ExplanationNode {
	return append([]ExplanationNode(nil), n.children...)
}

func explanationNodeFromInternal(node provenance.Node) ExplanationNode {
	result := ExplanationNode{kind: ExplanationKind(node.Kind().String()), decision: node.Decision().String()}
	if property, ok := node.Property(); ok {
		result.property, result.hasProperty = addressFromInternal(property), true
	}
	if relation, entity, ok := node.AggregateItem(); ok {
		result.relationID, result.relatedEntityID, result.hasAggregateItem = idFromInternal(relation.ID()), idFromInternal(entity.ID()), true
	}
	if operation, ok := node.Operation(); ok {
		result.operation = operation
	}
	if resolved, ok := node.Resolved(); ok {
		result.state = resultStateFromResolved(resolved)
		if resolved.IsDefined() {
			result.value, result.hasValue = valueFromInternal(resolved.Value()), true
		}
		if resolved.IsUndefined() {
			result.undefined = undefinedInfoFromInternal(resolved.UndefinedInfo())
		}
	}
	if override, ok := node.Override(); ok {
		result.overrideTarget, result.hasOverride = addressFromInternal(override.Target()), true
	}
	if scenario, _, ok := node.ScenarioOverride(); ok {
		result.scenarioID, result.hasScenario = idFromInternal(scenario.ID()), true
	}
	children := node.Children()
	result.children = make([]ExplanationNode, len(children))
	for i, child := range children {
		result.children[i] = explanationNodeFromInternal(child)
	}
	return result
}

// SeriesExplanation is actual per-sample Series provenance.
type SeriesExplanation struct {
	target       PropertyAddress
	result       SeriesResult
	points       []SeriesPointExplanation
	intervals    []SeriesIntervalExplanation
	undefined    ExplanationNode
	hasUndefined bool
}

// SeriesPointExplanation is actual provenance for one point Series sample.
type SeriesPointExplanation struct {
	at     DateTime
	result Result
	root   ExplanationNode
}

// At returns this point explanation's coordinate.
func (p SeriesPointExplanation) At() DateTime { return p.at }

// Result returns this point explanation's evaluation outcome.
func (p SeriesPointExplanation) Result() Result { return p.result }

// Root returns this point explanation's actual provenance.
func (p SeriesPointExplanation) Root() ExplanationNode { return p.root }

// SeriesIntervalExplanation is actual provenance for one interval Series sample.
type SeriesIntervalExplanation struct {
	from, to DateTime
	result   Result
	root     ExplanationNode
}

// From returns this interval explanation's inclusive lower coordinate.
func (i SeriesIntervalExplanation) From() DateTime { return i.from }

// To returns this interval explanation's exclusive upper coordinate.
func (i SeriesIntervalExplanation) To() DateTime { return i.to }

// Result returns this interval explanation's evaluation outcome.
func (i SeriesIntervalExplanation) Result() Result { return i.result }

// Root returns this interval explanation's actual provenance.
func (i SeriesIntervalExplanation) Root() ExplanationNode { return i.root }
func seriesExplanationFromInternal(value evaluator.SeriesExplanation) SeriesExplanation {
	result := SeriesExplanation{target: addressFromInternal(value.Target()), result: seriesResultFromInternal(value.Result())}
	for _, item := range value.Points() {
		result.points = append(result.points, SeriesPointExplanation{at: DateTime{value: item.At()}, result: resultFromInternal(item.Result()), root: explanationNodeFromInternal(item.Root())})
	}
	for _, item := range value.Intervals() {
		result.intervals = append(result.intervals, SeriesIntervalExplanation{from: DateTime{value: item.From()}, to: DateTime{value: item.To()}, result: resultFromInternal(item.Result()), root: explanationNodeFromInternal(item.Root())})
	}
	if _, node, ok := value.Undefined(); ok {
		result.undefined, result.hasUndefined = explanationNodeFromInternal(node), true
	}
	return result
}

// Target returns the Series property that was explained.
func (e SeriesExplanation) Target() PropertyAddress { return e.target }

// Result returns the explained Series result.
func (e SeriesExplanation) Result() SeriesResult { return e.result }

// Points returns an owned deterministic copy of point explanations.
func (e SeriesExplanation) Points() []SeriesPointExplanation {
	return append([]SeriesPointExplanation(nil), e.points...)
}

// Intervals returns an owned deterministic copy of interval explanations.
func (e SeriesExplanation) Intervals() []SeriesIntervalExplanation {
	return append([]SeriesIntervalExplanation(nil), e.intervals...)
}

// Undefined returns the terminal provenance node if the whole Series is Undefined.
func (e SeriesExplanation) Undefined() (ExplanationNode, bool) { return e.undefined, e.hasUndefined }
