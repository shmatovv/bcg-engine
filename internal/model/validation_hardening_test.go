package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestDraftValidate_IncompatibleTraitImplementationDoesNotSatisfyTrait(t *testing.T) {
	entityTypeID := identity.New()
	traitID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	contract := schema.NewPropertyContract(
		contractID,
		"Price",
		value.Integer,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		false,
	)
	trait := schema.NewTrait(traitID, "Priced", []identity.UUID{contractID})
	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityTypeID),
		contractID,
		true,
		"Price",
		value.String,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)
	entityType := schema.NewEntityType(
		entityTypeID,
		"Item",
		[]identity.UUID{traitID},
		[]identity.UUID{definitionID},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddTrait(trait)
	draft.AddPropertyDefinition(definition)
	draft.AddEntityType(entityType)

	report := draft.Validate()
	requireValidationIssue(t, report, ValidationIssueContractValueTypeMismatch)
	requireValidationIssue(t, report, ValidationIssueTraitRequirementNotSatisfied)
}

func TestDraftValidate_NonNumericPropertyWithUnitIsUnsupported(t *testing.T) {
	entityTypeID := identity.New()
	definitionID := identity.New()
	mass := unit.Base("mass")
	kg := unit.NewUnit("kg", mass, "mass", rational.One())

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityTypeID),
		identity.UUID{},
		false,
		"Flag",
		value.Bool,
		kg,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	draft := NewDraft()
	draft.AddEntityType(schema.NewEntityType(
		entityTypeID,
		"Item",
		nil,
		[]identity.UUID{definitionID},
	))
	draft.AddPropertyDefinition(definition)

	requireValidationIssue(
		t,
		draft.Validate(),
		ValidationIssueUnsupportedSchemaCombination,
	)
}

func TestDraftValidate_DerivedExpressionMustMatchDefinition(t *testing.T) {
	entityTypeID := identity.New()
	definitionID := identity.New()

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityTypeID),
		identity.UUID{},
		false,
		"Amount",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.DerivedComputation(
			ast.NewConstant(value.NewTyped(value.NewString("wrong"))),
		),
	)

	draft := NewDraft()
	draft.AddEntityType(schema.NewEntityType(
		entityTypeID,
		"Item",
		nil,
		[]identity.UUID{definitionID},
	))
	draft.AddPropertyDefinition(definition)

	requireValidationIssue(
		t,
		draft.Validate(),
		ValidationIssueInvalidDerivedExpression,
	)
}

func TestDraftValidate_RelationApplicabilityMustBeScalarBool(t *testing.T) {
	entityTypeID := identity.New()
	relationTypeID := identity.New()
	fromID := identity.New()
	toID := identity.New()
	relationID := identity.New()

	draft := NewDraft()
	draft.AddEntityType(schema.NewEntityType(entityTypeID, "Item", nil, nil))
	draft.AddRelationType(schema.NewRelationType(
		relationTypeID,
		"Link",
		schema.TypeConstraint{},
		false,
		schema.TypeConstraint{},
		false,
		nil,
	))
	draft.AddEntity(semantic.NewEntity(fromID, entityTypeID, "", false))
	draft.AddEntity(semantic.NewEntity(toID, entityTypeID, "", false))
	draft.AddRelation(semantic.NewRelation(
		relationID,
		relationTypeID,
		fromID,
		toID,
		ast.NewConstant(value.NewTyped(value.NewInt64(1))),
		true,
	))

	requireValidationIssue(
		t,
		draft.Validate(),
		ValidationIssueInvalidRelationApplicability,
	)
}

func TestDraftValidate_ScopeSelectorMustResolveAndMatchType(t *testing.T) {
	fixture := newBaseValueFixture(
		t,
		schema.InputComputation(),
		schema.TemporalScalar,
		identity.UUID{},
		false,
		value.Integer,
		unit.Unit{},
		false,
	)

	scopeID := identity.New()
	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(scopeID, "Region", value.String),
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(1))),
		[]scope.Selector{
			scope.NewSelector(scopeID, value.NewTyped(value.NewInt64(42))),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueScopeSelectorTypeMismatch,
	)
}

func TestDraftValidate_UnknownScopeDefinition(t *testing.T) {
	fixture := newBaseValueFixture(
		t,
		schema.InputComputation(),
		schema.TemporalScalar,
		identity.UUID{},
		false,
		value.Integer,
		unit.Unit{},
		false,
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(1))),
		[]scope.Selector{
			scope.NewSelector(
				identity.New(),
				value.NewTyped(value.NewString("EU")),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueUnknownScopeDefinition,
	)
}

func TestDraftValidate_ZeroValueEntityTypeDoesNotPanic(t *testing.T) {
	draft := NewDraft()
	draft.AddEntityType(schema.EntityType{})

	assertValidationDoesNotPanic(t, draft)

	requireValidationIssue(
		t,
		draft.Validate(),
		ValidationIssueInvalidEntityType,
	)
}

func TestDraftValidate_ZeroValueRelationTypeDoesNotPanic(t *testing.T) {
	draft := NewDraft()
	draft.AddRelationType(schema.RelationType{})

	assertValidationDoesNotPanic(t, draft)

	requireValidationIssue(
		t,
		draft.Validate(),
		ValidationIssueInvalidRelationType,
	)
}

func TestDraftValidate_ZeroValueSchemaObjectsDoNotPanic(t *testing.T) {
	draft := NewDraft()

	draft.AddEntityType(schema.EntityType{})
	draft.AddRelationType(schema.RelationType{})
	draft.AddTrait(schema.Trait{})
	draft.AddPropertyContract(schema.PropertyContract{})
	draft.AddPropertyDefinition(schema.PropertyDefinition{})
	draft.AddScopeDefinition(scope.Definition{})

	assertValidationDoesNotPanic(t, draft)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueInvalidEntityType,
	)
	requireValidationIssue(
		t,
		report,
		ValidationIssueInvalidRelationType,
	)
	requireValidationIssue(
		t,
		report,
		ValidationIssueInvalidTrait,
	)
	requireValidationIssue(
		t,
		report,
		ValidationIssueInvalidPropertyContract,
	)
	requireValidationIssue(
		t,
		report,
		ValidationIssueInvalidPropertyDefinition,
	)
	requireValidationIssue(
		t,
		report,
		ValidationIssueInvalidScopeDefinition,
	)
}

func assertValidationDoesNotPanic(
	t *testing.T,
	draft *Draft,
) {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf(
				"Draft.Validate panicked: %v",
				recovered,
			)
		}
	}()

	_ = draft.Validate()
}
