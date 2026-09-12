package model

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// validateRules validates every declarative Rule stored in the Draft.
//
// Rule conditions are standalone in Core v1. They have no implicit self owner,
// so static inference starts from inference.EmptyContext().
func (d *Draft) validateRules(
	report *ValidationReport,
	indexes validationIndexes,
) {
	resolver := newValidationInferenceResolver(indexes)

	for _, modelRule := range d.rules {
		if !modelRule.IsValid() {
			continue
		}

		conditionValid := validateRuleCondition(
			report,
			modelRule.ID(),
			modelRule.Condition(),
			resolver,
		)
		if conditionValid {
			d.validateRuleDependencies(report, indexes, modelRule)
		}

	}
}

func validateRuleCondition(
	report *ValidationReport,
	ruleID identity.UUID,
	expression ast.Expression,
	resolver validationInferenceResolver,
) bool {
	if expression == nil {
		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidRuleCondition,
				ruleID,
				"rule condition must not be nil",
			),
		)
		return false
	}

	inferred, err := inference.Infer(
		expression,
		inference.EmptyContext(),
		resolver,
	)
	if err != nil {
		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidRuleCondition,
				ruleID,
				"rule condition cannot be statically inferred: "+err.Error(),
			),
		)
		return false
	}

	if inferred.ValueType() != value.Bool ||
		inferred.TemporalMode() != typing.TemporalScalar ||
		inferred.HasUnit() {
		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidRuleCondition,
				ruleID,
				"rule condition must be a unitless Scalar Bool expression",
			),
		)
		return false
	}
	return true
}

func (d *Draft) validateRuleDependencies(report *ValidationReport, indexes validationIndexes, modelRule domainrule.Rule) {
	source := newDraftDependencySource(d, indexes)
	if _, err := dependency.ExtractStandaloneExpression(source, modelRule.Condition()); err != nil {
		report.append(newSubjectValidationIssue(ValidationIssueInvalidRuleCondition, modelRule.ID(), "rule condition dependencies cannot be extracted: "+err.Error()))
	}
}
