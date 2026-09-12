package model

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
)

// RuleDependencies is the immutable structural set of concrete properties
// read by one Rule condition. It includes every structural lazy branch; the
// condition evaluator Result retained by rule/eval.Result records the path
// actually observed for a concrete CalculationContext.
type RuleDependencies struct {
	condition []address.PropertyAddress
}

func newRuleDependencies(condition []address.PropertyAddress) RuleDependencies {
	result := RuleDependencies{condition: append([]address.PropertyAddress(nil), condition...)}
	if !result.IsValid() {
		panic("model: invalid Rule dependency index entry")
	}
	return result
}

// Condition returns an owned deterministic list of properties structurally
// read by a Rule condition.
func (d RuleDependencies) Condition() []address.PropertyAddress {
	return append([]address.PropertyAddress(nil), d.condition...)
}

// IsValid reports whether the condition dependencies are valid, canonical,
// and duplicate-free.
func (d RuleDependencies) IsValid() bool {
	return validCanonicalDependencyAddresses(d.condition)
}

// RuleDependencies resolves one committed Rule's indexed structural
// condition dependencies. false means ruleID is absent from version.
func (v ModelVersion) RuleDependencies(ruleID identity.UUID) (RuleDependencies, bool) {
	dependencies, ok := v.indexes.ruleDependenciesByID[ruleID]
	return dependencies, ok
}

// RuleIDsDependingOn returns an owned deterministic list of Rules whose
// condition structurally reads propertyAddress.
func (v ModelVersion) RuleIDsDependingOn(propertyAddress address.PropertyAddress) []identity.UUID {
	if !propertyAddress.IsValid() {
		return nil
	}
	return append([]identity.UUID(nil), v.indexes.ruleIDsByDependency[propertyAddress]...)
}

func buildRuleDependencyIndexes(version ModelVersion) (map[identity.UUID]RuleDependencies, map[address.PropertyAddress][]identity.UUID, error) {
	byRule := make(map[identity.UUID]RuleDependencies, len(version.rules))
	byProperty := make(map[address.PropertyAddress][]identity.UUID)
	rules := append([]domainrule.Rule(nil), version.rules...)
	sort.Slice(rules, func(i, j int) bool { return compareDependencyUUID(rules[i].ID(), rules[j].ID()) < 0 })
	for _, modelRule := range rules {
		condition, err := dependency.ExtractStandaloneExpression(version, modelRule.Condition())
		if err != nil {
			return nil, nil, err
		}
		dependencies := newRuleDependencies(condition)
		byRule[modelRule.ID()] = dependencies
		for _, propertyAddress := range dependencies.Condition() {
			byProperty[propertyAddress] = append(byProperty[propertyAddress], modelRule.ID())
		}
	}
	for propertyAddress := range byProperty {
		ids := byProperty[propertyAddress]
		sort.Slice(ids, func(i, j int) bool { return compareDependencyUUID(ids[i], ids[j]) < 0 })
		byProperty[propertyAddress] = ids
	}
	return byRule, byProperty, nil
}
