package bcg

import (
	domainrule "github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	ruleeval "github.com/shmatovv/bcg-engine/v2/internal/rule/eval"
)

// RuleState is the semantic state of a declarative named applicability decision.
type RuleState string

const (
	RuleActive    RuleState = "Active"
	RuleInactive  RuleState = "Inactive"
	RuleUndefined RuleState = "Undefined"
)

// RuleInfo is read-only Rule metadata. Rules never carry effects or actions.
type RuleInfo struct {
	id         ID
	name       string
	metadata   map[string]string
	definition Rule
}

// ID returns the Rule UUID.
func (r RuleInfo) ID() ID { return r.id }

// Name returns the Rule display name.
func (r RuleInfo) Name() string { return r.name }

// Metadata returns an owned copy of model metadata.
func (r RuleInfo) Metadata() map[string]string {
	result := make(map[string]string, len(r.metadata))
	for k, v := range r.metadata {
		result[k] = v
	}
	return result
}

// Condition returns the immutable Boolean expression that determines the
// Rule's Active, Inactive, or Undefined state.
func (r RuleInfo) Condition() Expression { return r.definition.Condition() }

// Definition returns the complete immutable Rule declaration.
func (r RuleInfo) Definition() Rule { return r.definition }

// RuleResult retains the condition result/provenance used for its state.
type RuleResult struct {
	rule      RuleInfo
	state     RuleState
	condition Result
}

// Rule returns the evaluated Rule metadata.
func (r RuleResult) Rule() RuleInfo { return r.rule }

// RuleID returns the evaluated Rule UUID.
func (r RuleResult) RuleID() ID { return r.rule.id }

// State returns Active, Inactive, or Undefined.
func (r RuleResult) State() RuleState { return r.state }

// Condition returns the actual Boolean condition evaluation.
func (r RuleResult) Condition() Result { return r.condition }

// Undefined returns the condition cause when State is RuleUndefined.
func (r RuleResult) Undefined() (UndefinedInfo, bool) { return r.condition.Undefined() }

// RuleReport contains every Rule result in deterministic ID order.
type RuleReport struct{ results []RuleResult }

// Results returns an owned deterministic copy of every Rule result.
func (r RuleReport) Results() []RuleResult { return append([]RuleResult(nil), r.results...) }

// Len returns the number of reported Rules.
func (r RuleReport) Len() int { return len(r.results) }

// Active returns the Active subset without re-evaluation.
func (r RuleReport) Active() []RuleResult { return r.byState(RuleActive) }

// Inactive returns the Inactive subset without re-evaluation.
func (r RuleReport) Inactive() []RuleResult { return r.byState(RuleInactive) }

// Undefined returns the Undefined subset without re-evaluation.
func (r RuleReport) Undefined() []RuleResult { return r.byState(RuleUndefined) }
func (r RuleReport) byState(state RuleState) []RuleResult {
	result := make([]RuleResult, 0)
	for _, item := range r.results {
		if item.state == state {
			result = append(result, item)
		}
	}
	return result
}
func ruleInfoFromInternal(value domainrule.Rule) RuleInfo {
	return RuleInfo{id: idFromInternal(value.ID()), name: value.Name(), metadata: value.Metadata(), definition: ruleFromInternal(value)}
}
func ruleStateFromInternal(value ruleeval.State) RuleState {
	switch value {
	case ruleeval.StateActive:
		return RuleActive
	case ruleeval.StateInactive:
		return RuleInactive
	case ruleeval.StateUndefined:
		return RuleUndefined
	default:
		return ""
	}
}
func ruleResultFromInternal(value ruleeval.Result) RuleResult {
	return RuleResult{rule: ruleInfoFromInternal(value.Rule()), state: ruleStateFromInternal(value.State()), condition: resultFromInternal(value.Condition())}
}
func ruleReportFromInternal(value ruleeval.Report) RuleReport {
	items := value.Results()
	result := RuleReport{results: make([]RuleResult, len(items))}
	for i, item := range items {
		result.results[i] = ruleResultFromInternal(item)
	}
	return result
}
