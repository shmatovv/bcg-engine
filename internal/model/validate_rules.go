package model

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// validateRules validates every declarative Rule stored in the Draft.
//
// Rule conditions are standalone in Core v1. They have no implicit self owner,
// so static inference starts from inference.EmptyContext().
//
// Effects are declarative references. They are validated without applying or
// mutating their targets.
func (d *Draft) validateRules(
	report *ValidationReport,
	indexes validationIndexes,
) {
	resolver := newValidationInferenceResolver(indexes)

	for _, modelRule := range d.rules {
		if !modelRule.IsValid() {
			continue
		}

		validateRuleCondition(
			report,
			modelRule.ID(),
			modelRule.Condition(),
			resolver,
		)

		validateRuleEffect(
			report,
			modelRule.ID(),
			modelRule.Effect(),
			indexes,
		)
	}
}

func validateRuleCondition(
	report *ValidationReport,
	ruleID identity.UUID,
	expression ast.Expression,
	resolver validationInferenceResolver,
) {
	if expression == nil {
		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidRuleCondition,
				ruleID,
				"rule condition must not be nil",
			),
		)
		return
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
		return
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
	}
}

func validateRuleEffect(
	report *ValidationReport,
	ruleID identity.UUID,
	effect domainrule.Effect,
	indexes validationIndexes,
) {
	if !effect.IsValid() {
		return
	}

	switch effect.Kind() {
	case domainrule.EffectConstraintActivation:
		constraintID, ok := effect.ConstraintID()
		if !ok {
			report.append(
				newSubjectValidationIssue(
					ValidationIssueRuleEffectTargetNotFound,
					ruleID,
					"constraint activation effect has no valid Constraint target",
				),
			)
			return
		}

		modelConstraint, exists := indexes.constraintByID[constraintID]
		if !exists || !modelConstraint.IsValid() {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueRuleEffectTargetNotFound,
					ruleID,
					constraintID,
					"rule effect references a Constraint that does not exist in the model",
				),
			)
		}

	default:
		report.append(
			newSubjectValidationIssue(
				ValidationIssueRuleEffectTargetNotFound,
				ruleID,
				"rule effect kind is unsupported",
			),
		)
	}
}
