package model

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// Validate validates the complete candidate model.
//
// Draft is deliberately allowed to be temporarily incomplete or inconsistent.
// Validation therefore collects diagnostics instead of assuming that Draft
// already satisfies committed-model invariants.
//
// Pass order follows dependency depth:
//
//  1. structural validity;
//  2. stable identity uniqueness;
//  3. schema semantic combinations;
//  4. schema reference integrity;
//  5. ownership consistency;
//  6. contract and trait semantics;
//  7. semantic data structure and references;
//  8. relation endpoint applicability;
//  9. stored expression typing;
//
// 10. Constraint expression typing;
// 11. Rule condition validation;
// 12. concrete value dependency extraction and cycle detection;
// 13. base value bindings.
func (d *Draft) Validate() ValidationReport {
	return d.validate(true)
}

// validateWithoutDependencyGraph performs every ordinary authored-model
// validation pass except concrete dependency extraction. Dataset construction
// uses it to validate the combined semantic data without constructing a graph
// that belongs to the later immutable EvaluationSource.
func (d *Draft) validateWithoutDependencyGraph() ValidationReport {
	return d.validate(false)
}

func (d *Draft) validate(includeDependencyGraph bool) ValidationReport {
	d.requireNonNil()

	var report ValidationReport

	d.validateStructural(
		&report,
	)

	d.validateDuplicateIDs(
		&report,
	)

	d.validateSchemaSemantics(
		&report,
	)

	d.validateReferences(
		&report,
	)

	d.validateOwnership(
		&report,
	)

	d.validateContractsAndTraits(
		&report,
	)

	indexes :=
		buildValidationIndexes(d)

	d.validateSemanticData(
		&report,
		indexes,
	)

	d.validateRelationEndpoints(
		&report,
		indexes,
	)

	d.validateExpressions(
		&report,
		indexes,
	)

	d.validateConstraints(
		&report,
		indexes,
	)

	d.validateRules(
		&report,
		indexes,
	)

	if includeDependencyGraph {
		d.validateDependencyGraph(
			&report,
			indexes,
		)
	}

	d.validateValueBindings(
		&report,
		indexes,
	)

	report.sortDeterministically()

	return report
}

func (d *Draft) validateStructural(
	report *ValidationReport,
) {
	for _, value := range d.entityTypes {
		if value.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidEntityType,
				value.ID(),
				"entity type is structurally invalid",
			),
		)
	}

	for _, value := range d.relationTypes {
		if value.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidRelationType,
				value.ID(),
				"relation type is structurally invalid",
			),
		)
	}

	for _, value := range d.traits {
		if value.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidTrait,
				value.ID(),
				"trait is structurally invalid",
			),
		)
	}

	for _, value := range d.propertyContracts {
		if value.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidPropertyContract,
				value.ID(),
				"property contract is structurally invalid",
			),
		)
	}

	for _, value := range d.propertyDefinitions {
		if value.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidPropertyDefinition,
				value.ID(),
				"property definition is structurally invalid",
			),
		)
	}

	for _, value := range d.scopeDefinitions {
		if value.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidScopeDefinition,
				value.ID(),
				"scope definition is structurally invalid",
			),
		)
	}

	for _, value := range d.constraints {
		if value.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidConstraint,
				value.ID(),
				"constraint is structurally invalid",
			),
		)
	}

	for _, value := range d.rules {
		if value.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidRule,
				value.ID(),
				"rule is structurally invalid",
			),
		)
	}
}

func (d *Draft) validateDuplicateIDs(
	report *ValidationReport,
) {
	entityTypeIDs := make(
		map[identity.UUID]struct{},
	)

	for _, value := range d.entityTypes {
		validateOneID(
			report,
			entityTypeIDs,
			value.ID(),
			ValidationIssueDuplicateEntityTypeID,
			"entity type",
		)
	}

	relationTypeIDs := make(
		map[identity.UUID]struct{},
	)

	for _, value := range d.relationTypes {
		validateOneID(
			report,
			relationTypeIDs,
			value.ID(),
			ValidationIssueDuplicateRelationTypeID,
			"relation type",
		)
	}

	traitIDs := make(
		map[identity.UUID]struct{},
	)

	for _, value := range d.traits {
		validateOneID(
			report,
			traitIDs,
			value.ID(),
			ValidationIssueDuplicateTraitID,
			"trait",
		)
	}

	contractIDs := make(
		map[identity.UUID]struct{},
	)

	for _, value := range d.propertyContracts {
		validateOneID(
			report,
			contractIDs,
			value.ID(),
			ValidationIssueDuplicatePropertyContractID,
			"property contract",
		)
	}

	definitionIDs := make(
		map[identity.UUID]struct{},
	)

	for _, value := range d.propertyDefinitions {
		validateOneID(
			report,
			definitionIDs,
			value.ID(),
			ValidationIssueDuplicatePropertyDefinitionID,
			"property definition",
		)
	}

	scopeDefinitionIDs := make(
		map[identity.UUID]struct{},
	)

	for _, value := range d.scopeDefinitions {
		validateOneID(
			report,
			scopeDefinitionIDs,
			value.ID(),
			ValidationIssueDuplicateScopeDefinitionID,
			"scope definition",
		)
	}

	constraintIDs := make(
		map[identity.UUID]struct{},
	)

	for _, value := range d.constraints {
		validateOneID(
			report,
			constraintIDs,
			value.ID(),
			ValidationIssueDuplicateConstraintID,
			"constraint",
		)
	}

	ruleIDs := make(
		map[identity.UUID]struct{},
	)

	for _, value := range d.rules {
		validateOneID(
			report,
			ruleIDs,
			value.ID(),
			ValidationIssueDuplicateRuleID,
			"rule",
		)
	}
}

func validateOneID(
	report *ValidationReport,
	seen map[identity.UUID]struct{},
	id identity.UUID,
	kind ValidationIssueKind,
	subject string,
) {
	if id.IsZero() {
		return
	}

	if _, exists :=
		seen[id]; exists {

		report.append(
			newSubjectValidationIssue(
				kind,
				id,
				fmt.Sprintf(
					"duplicate %s id",
					subject,
				),
			),
		)

		return
	}

	seen[id] =
		struct{}{}
}
