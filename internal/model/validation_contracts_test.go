package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestDraftValidate_TraitRequirementSatisfied(t *testing.T) {
	entityID := identity.New()
	traitID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	contract := schema.NewPropertyContract(
		contractID,
		"Amount",
		value.Decimal,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		false,
	)

	trait := schema.NewTrait(
		traitID,
		"Priced",
		[]identity.UUID{
			contractID,
		},
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"Amount",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Entity",
		[]identity.UUID{
			traitID,
		},
		[]identity.UUID{
			definitionID,
		},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddTrait(trait)
	draft.AddPropertyDefinition(definition)
	draft.AddEntityType(entity)

	report := draft.Validate()

	if !report.OK() {
		t.Fatalf(
			"valid trait implementation rejected: %v",
			report.Issues(),
		)
	}
}

func TestDraftValidate_TraitRequirementMissing(t *testing.T) {
	entityID := identity.New()
	traitID := identity.New()
	contractID := identity.New()

	draft := NewDraft()

	draft.AddPropertyContract(
		schema.NewPropertyContract(
			contractID,
			"Amount",
			value.Decimal,
			unit.Dimension{},
			false,
			schema.TemporalScalar,
			false,
		),
	)

	draft.AddTrait(
		schema.NewTrait(
			traitID,
			"Priced",
			[]identity.UUID{
				contractID,
			},
		),
	)

	draft.AddEntityType(
		schema.NewEntityType(
			entityID,
			"Entity",
			[]identity.UUID{
				traitID,
			},
			nil,
		),
	)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueTraitRequirementNotSatisfied,
	)
}

func TestDraftValidate_DuplicateContractImplementation(t *testing.T) {
	entityID := identity.New()
	contractID := identity.New()
	firstID := identity.New()
	secondID := identity.New()

	contract := schema.NewPropertyContract(
		contractID,
		"Amount",
		value.Decimal,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		false,
	)

	first := schema.NewPropertyDefinition(
		firstID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"AmountA",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	second := schema.NewPropertyDefinition(
		secondID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"AmountB",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Entity",
		nil,
		[]identity.UUID{
			firstID,
			secondID,
		},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddPropertyDefinition(first)
	draft.AddPropertyDefinition(second)
	draft.AddEntityType(entity)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueDuplicateContractImplementation,
	)
}

func TestDraftValidate_ContractValueTypeMismatch(t *testing.T) {
	entityID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	contract := schema.NewPropertyContract(
		contractID,
		"Amount",
		value.Integer,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		false,
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"Amount",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Entity",
		nil,
		[]identity.UUID{
			definitionID,
		},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddPropertyDefinition(definition)
	draft.AddEntityType(entity)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueContractValueTypeMismatch,
	)
}

func TestDraftValidate_ContractTemporalModeMismatch(t *testing.T) {
	entityID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	contract := schema.NewPropertyContract(
		contractID,
		"Metric",
		value.Decimal,
		unit.Dimension{},
		false,
		schema.TemporalSeries,
		false,
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"Metric",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Entity",
		nil,
		[]identity.UUID{
			definitionID,
		},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddPropertyDefinition(definition)
	draft.AddEntityType(entity)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueContractTemporalModeMismatch,
	)
}

func TestDraftValidate_ContractDimensionSatisfied(t *testing.T) {
	entityID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	length := unit.Base("length")

	meter := unit.NewUnit(
		"meter",
		length,
		"length",
		rational.One(),
	)

	contract := schema.NewPropertyContract(
		contractID,
		"Length",
		value.Decimal,
		length,
		true,
		schema.TemporalScalar,
		false,
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"Length",
		value.Decimal,
		meter,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Entity",
		nil,
		[]identity.UUID{
			definitionID,
		},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddPropertyDefinition(definition)
	draft.AddEntityType(entity)

	report := draft.Validate()

	if !report.OK() {
		t.Fatalf(
			"compatible contract dimension rejected: %v",
			report.Issues(),
		)
	}
}

func TestDraftValidate_ContractDimensionMismatch(t *testing.T) {
	entityID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	length := unit.Base("length")
	mass := unit.Base("mass")

	kilogram := unit.NewUnit(
		"kilogram",
		mass,
		"mass",
		rational.One(),
	)

	contract := schema.NewPropertyContract(
		contractID,
		"Length",
		value.Decimal,
		length,
		true,
		schema.TemporalScalar,
		false,
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"Length",
		value.Decimal,
		kilogram,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Entity",
		nil,
		[]identity.UUID{
			definitionID,
		},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddPropertyDefinition(definition)
	draft.AddEntityType(entity)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueContractDimensionMismatch,
	)
}

func TestDraftValidate_DimensionedContractRequiresUnit(t *testing.T) {
	entityID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	length := unit.Base("length")

	contract := schema.NewPropertyContract(
		contractID,
		"Length",
		value.Decimal,
		length,
		true,
		schema.TemporalScalar,
		false,
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"Length",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Entity",
		nil,
		[]identity.UUID{
			definitionID,
		},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddPropertyDefinition(definition)
	draft.AddEntityType(entity)

	report := draft.Validate()

	requireValidationIssue(
		t,
		report,
		ValidationIssueContractDimensionMismatch,
	)
}

func TestDraftValidate_ContractWithoutDimensionDoesNotConstrainUnitDimension(
	t *testing.T,
) {
	entityID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	currency := unit.Base("currency")

	rub := unit.NewUnit(
		"rub",
		currency,
		"rub",
		rational.One(),
	)

	contract := schema.NewPropertyContract(
		contractID,
		"Price",
		value.Decimal,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		false,
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityID),
		contractID,
		true,
		"Price",
		value.Decimal,
		rub,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Entity",
		nil,
		[]identity.UUID{
			definitionID,
		},
	)

	draft := NewDraft()
	draft.AddPropertyContract(contract)
	draft.AddPropertyDefinition(definition)
	draft.AddEntityType(entity)

	report := draft.Validate()

	if !report.OK() {
		t.Fatalf(
			"dimension-unconstrained contract rejected implementation: %v",
			report.Issues(),
		)
	}
}
