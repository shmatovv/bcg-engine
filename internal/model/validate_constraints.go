package model

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// validateConstraints validates every declarative Constraint stored in the
// Draft.
//
// Constraint expressions are standalone in Core v1. They have no implicit
// self owner, so static inference starts from inference.EmptyContext().
func (d *Draft) validateConstraints(
	report *ValidationReport,
	indexes validationIndexes,
) {
	resolver := newValidationInferenceResolver(indexes)

	for _, modelConstraint := range d.constraints {
		if !modelConstraint.IsValid() {
			continue
		}

		if applicability, hasApplicability :=
			modelConstraint.Applicability(); hasApplicability {
			validateConstraintBooleanExpression(
				report,
				modelConstraint.ID(),
				applicability,
				resolver,
				ValidationIssueInvalidConstraintApplicability,
				"constraint applicability",
			)
		}

		validateConstraintBooleanExpression(
			report,
			modelConstraint.ID(),
			modelConstraint.Assertion(),
			resolver,
			ValidationIssueInvalidConstraintAssertion,
			"constraint assertion",
		)
	}
}

func validateConstraintBooleanExpression(
	report *ValidationReport,
	constraintID identity.UUID,
	expression ast.Expression,
	resolver validationInferenceResolver,
	kind ValidationIssueKind,
	label string,
) {
	if expression == nil {
		report.append(
			newSubjectValidationIssue(
				kind,
				constraintID,
				label+" must not be nil",
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
				kind,
				constraintID,
				label+" cannot be statically inferred: "+err.Error(),
			),
		)
		return
	}

	if inferred.ValueType() != value.Bool ||
		inferred.TemporalMode() != typing.TemporalScalar ||
		inferred.HasUnit() {
		report.append(
			newSubjectValidationIssue(
				kind,
				constraintID,
				label+" must be a unitless Scalar Bool expression",
			),
		)
	}
}
