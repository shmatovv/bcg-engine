package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestValidationReport_ZeroValueIsSuccessful(t *testing.T) {
	var report ValidationReport

	if !report.IsValid() {
		t.Fatal("zero ValidationReport must be structurally valid")
	}

	if !report.OK() {
		t.Fatal("zero ValidationReport must represent successful validation")
	}

	if report.Len() != 0 {
		t.Fatalf("got %d issues, want 0", report.Len())
	}
}

func TestDraftValidate_EmptyDraftIsValid(t *testing.T) {
	draft := NewDraft()

	report := draft.Validate()

	if !report.OK() {
		t.Fatalf(
			"empty draft unexpectedly failed validation: %v",
			report.Issues(),
		)
	}
}

func TestDraftValidate_DuplicateEntityTypeID(t *testing.T) {
	id := identity.New()

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			id,
			"A",
			nil,
			nil,
		),
	)

	draft.AddEntityType(
		schema.NewEntityType(
			id,
			"B",
			nil,
			nil,
		),
	)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueDuplicateEntityTypeID,
	)
}

func TestDraftValidate_UnknownTrait(t *testing.T) {
	entityID := identity.New()
	missingTraitID := identity.New()

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			entityID,
			"Entity",
			[]identity.UUID{
				missingTraitID,
			},
			nil,
		),
	)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueUnknownTrait,
	)
}

func TestDraftValidate_UnknownPropertyDefinition(t *testing.T) {
	entityID := identity.New()
	missingDefinitionID := identity.New()

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			entityID,
			"Entity",
			nil,
			[]identity.UUID{
				missingDefinitionID,
			},
		),
	)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueUnknownPropertyDefinition,
	)
}

func TestDraftValidate_MissingPropertyOwner(t *testing.T) {
	missingOwnerID := identity.New()
	definitionID := identity.New()

	draft := NewDraft()

	draft.AddPropertyDefinition(
		schema.NewPropertyDefinition(
			definitionID,
			schema.EntityTypeOwner(
				missingOwnerID,
			),
			identity.UUID{},
			false,
			"Value",
			value.Decimal,
			unit.Unit{},
			false,
			schema.TemporalScalar,
			schema.InputComputation(),
		),
	)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssuePropertyOwnerNotFound,
	)
}

func TestDraftValidate_WrongPropertyOwner(t *testing.T) {
	ownerAID := identity.New()
	ownerBID := identity.New()
	definitionID := identity.New()

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(ownerBID),
		identity.UUID{},
		false,
		"Value",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			ownerAID,
			"A",
			nil,
			[]identity.UUID{
				definitionID,
			},
		),
	)

	draft.AddEntityType(
		schema.NewEntityType(
			ownerBID,
			"B",
			nil,
			nil,
		),
	)

	draft.AddPropertyDefinition(definition)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssuePropertyOwnerMismatch,
	)
}

func TestDraftValidate_OrphanPropertyDefinition(t *testing.T) {
	entityID := identity.New()
	definitionID := identity.New()

	draft := NewDraft()

	draft.AddEntityType(
		schema.NewEntityType(
			entityID,
			"Entity",
			nil,
			nil,
		),
	)

	draft.AddPropertyDefinition(
		schema.NewPropertyDefinition(
			definitionID,
			schema.EntityTypeOwner(entityID),
			identity.UUID{},
			false,
			"Value",
			value.Decimal,
			unit.Unit{},
			false,
			schema.TemporalScalar,
			schema.InputComputation(),
		),
	)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssuePropertyOwnerMismatch,
	)
}

func requireValidationIssue(
	t *testing.T,
	report ValidationReport,
	kind ValidationIssueKind,
) ValidationIssue {
	t.Helper()

	for _, issue := range report.Issues() {
		if issue.Kind() == kind {
			return issue
		}
	}

	t.Fatalf(
		"validation report does not contain %s; issues=%v",
		kind,
		report.Issues(),
	)

	return ValidationIssue{}
}
