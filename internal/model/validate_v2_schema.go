package model

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// validateV2Schema intentionally validates declarations only. Object-level
// cardinality, required binding presence, uniqueness, and invariant outcomes
// are runtime integrity concerns and do not invalidate a schema commit.
func (d *Draft) validateV2Schema(report *ValidationReport) {
	enums := map[identity.UUID]bool{}
	for _, e := range d.enumDefinitions {
		if !e.IsValid() {
			report.append(newSubjectValidationIssue(ValidationIssueInvalidEnumDefinition, e.ID(), "invalid enum definition"))
			continue
		}
		if enums[e.ID()] {
			report.append(newSubjectValidationIssue(ValidationIssueDuplicateEnumDefinitionID, e.ID(), "duplicate enum definition ID"))
		}
		enums[e.ID()] = true
	}
	enumKnown := func(id identity.UUID) bool { return enums[id] }
	for _, p := range d.propertyDefinitions {
		if p.ValueType() == value.Enum {
			id, ok := p.EnumDefinitionID()
			if !ok || !enumKnown(id) {
				report.append(newSubjectValidationIssue(ValidationIssueUnknownEnumDefinition, p.ID(), "property references unknown enum definition"))
			}
		}
	}
	for _, c := range d.propertyContracts {
		if c.ValueType() == value.Enum {
			id, ok := c.EnumDefinitionID()
			if !ok || !enumKnown(id) {
				report.append(newSubjectValidationIssue(ValidationIssueUnknownEnumDefinition, c.ID(), "contract references unknown enum definition"))
			}
		}
	}
	keys := map[identity.UUID]bool{}
	for _, k := range d.entityKeyDefinitions {
		if !k.IsValid() {
			report.append(newSubjectValidationIssue(ValidationIssueInvalidEntityKeyDefinition, k.ID(), "invalid entity key definition"))
			continue
		}
		if keys[k.ID()] {
			report.append(newSubjectValidationIssue(ValidationIssueDuplicateEntityKeyDefinitionID, k.ID(), "duplicate entity key definition ID"))
		}
		keys[k.ID()] = true
		et, ok := findEntityType(d, k.EntityTypeID())
		if !ok {
			report.append(newRelatedValidationIssue(ValidationIssueUnknownEntityType, k.ID(), k.EntityTypeID(), "key references unknown entity type"))
			continue
		}
		for _, pid := range k.PropertyDefinitionIDs() {
			p, ok := findPropertyDefinition(d, pid)
			if !ok {
				report.append(newRelatedValidationIssue(ValidationIssueUnknownPropertyDefinition, k.ID(), pid, "key references unknown property"))
				continue
			}
			if p.Owner() != schema.EntityTypeOwner(et.ID()) || p.Computation().Kind() != schema.ComputationInput || p.TemporalMode() != schema.TemporalScalar {
				report.append(newRelatedValidationIssue(ValidationIssueUnsupportedSchemaCombination, k.ID(), pid, "key property must be an input scalar of its entity type"))
			}
		}
	}
	seen := map[identity.UUID]bool{}
	resolver := newValidationInferenceResolver(buildValidationIndexes(d))
	for _, inv := range d.invariants {
		if !inv.IsValid() {
			report.append(newSubjectValidationIssue(ValidationIssueInvalidInvariant, inv.ID(), "invalid invariant"))
			continue
		}
		if seen[inv.ID()] {
			report.append(newSubjectValidationIssue(ValidationIssueDuplicateInvariantID, inv.ID(), "duplicate invariant ID"))
		}
		seen[inv.ID()] = true
		s := inv.Subject()
		owner := schema.PropertyOwner{}
		if s.Kind() == invariant.SubjectEntityType {
			if _, ok := findEntityType(d, s.TypeID()); !ok {
				report.append(newRelatedValidationIssue(ValidationIssueUnknownEntityType, inv.ID(), s.TypeID(), "invariant references unknown entity type"))
				continue
			}
			owner = schema.EntityTypeOwner(s.TypeID())
		} else if _, ok := findRelationType(d, s.TypeID()); !ok {
			report.append(newRelatedValidationIssue(ValidationIssueUnknownRelationType, inv.ID(), s.TypeID(), "invariant references unknown relation type"))
			continue
		} else {
			owner = schema.RelationTypeOwner(s.TypeID())
		}
		if !invariantExpressionIsBool(inv.Assertion(), owner, resolver) {
			report.append(newSubjectValidationIssue(ValidationIssueInvalidInvariant, inv.ID(), "invariant assertion must be a unitless Scalar Bool expression"))
		}
		if applicability, ok := inv.Applicability(); ok && !invariantExpressionIsBool(applicability, owner, resolver) {
			report.append(newSubjectValidationIssue(ValidationIssueInvalidInvariant, inv.ID(), "invariant applicability must be a unitless Scalar Bool expression"))
		}
	}
}
func invariantExpressionIsBool(expression ast.Expression, owner schema.PropertyOwner, resolver validationInferenceResolver) bool {
	inferred, err := inference.Infer(expression, inference.NewContext(owner), resolver)
	return err == nil && inferred.ValueType() == value.Bool && inferred.TemporalMode() == typing.TemporalScalar && !inferred.HasUnit()
}
func findEntityType(d *Draft, id identity.UUID) (schema.EntityType, bool) {
	for _, v := range d.entityTypes {
		if v.ID() == id {
			return v, true
		}
	}
	return schema.EntityType{}, false
}
func findRelationType(d *Draft, id identity.UUID) (schema.RelationType, bool) {
	for _, v := range d.relationTypes {
		if v.ID() == id {
			return v, true
		}
	}
	return schema.RelationType{}, false
}
func findPropertyDefinition(d *Draft, id identity.UUID) (schema.PropertyDefinition, bool) {
	for _, v := range d.propertyDefinitions {
		if v.ID() == id {
			return v, true
		}
	}
	return schema.PropertyDefinition{}, false
}
