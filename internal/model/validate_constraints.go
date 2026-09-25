package model

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
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

		applicabilityValid := true
		if applicability, hasApplicability := modelConstraint.Applicability(); hasApplicability {
			applicabilityValid = validateConstraintBooleanExpression(
				report,
				modelConstraint.ID(),
				applicability,
				resolver,
				ValidationIssueInvalidConstraintApplicability,
				"constraint applicability",
			)
		}

		assertionValid := validateConstraintBooleanExpression(
			report,
			modelConstraint.ID(),
			modelConstraint.Assertion(),
			resolver,
			ValidationIssueInvalidConstraintAssertion,
			"constraint assertion",
		)
		if applicabilityValid && assertionValid {
			d.validateConstraintDependencies(report, indexes, modelConstraint)
		}
	}
}

func validateConstraintBooleanExpression(
	report *ValidationReport,
	constraintID identity.UUID,
	expression ast.Expression,
	resolver validationInferenceResolver,
	kind ValidationIssueKind,
	label string,
) bool {
	if expression == nil {
		report.append(
			newSubjectValidationIssue(
				kind,
				constraintID,
				label+" must not be nil",
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
				kind,
				constraintID,
				label+" cannot be statically inferred: "+err.Error(),
			),
		)
		return false
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
		return false
	}
	return true
}

func (d *Draft) validateConstraintDependencies(report *ValidationReport, indexes validationIndexes, modelConstraint constraint.Constraint) {
	source := newDraftDependencySource(d, indexes)
	if applicability, hasApplicability := modelConstraint.Applicability(); hasApplicability {
		if _, err := dependency.ExtractStandaloneExpression(source, applicability); err != nil {
			report.append(newSubjectValidationIssue(ValidationIssueInvalidConstraintApplicability, modelConstraint.ID(), "constraint applicability dependencies cannot be extracted: "+err.Error()))
		}
	}
	if _, err := dependency.ExtractStandaloneExpression(source, modelConstraint.Assertion()); err != nil {
		report.append(newSubjectValidationIssue(ValidationIssueInvalidConstraintAssertion, modelConstraint.ID(), "constraint assertion dependencies cannot be extracted: "+err.Error()))
	}
}
