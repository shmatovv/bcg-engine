package model

import (
	"math/big"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestDraftValidate_BaseValue_ObjectInputAccepted(t *testing.T) {
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

	fixture.draft.AddValueBinding(
		semantic.NewValueBinding(
			semantic.ObjectPropertyTarget(
				semantic.NewPropertyAddress(
					semantic.EntityRef(fixture.entityID),
					fixture.definitionID,
				),
			),
			semantic.DefinedBindingValue(
				value.NewTyped(value.NewInt64(42)),
			),
			nil,
			domaintime.EffectivePeriod{},
			false,
		),
	)

	report := fixture.draft.Validate()
	if !report.OK() {
		t.Fatalf("valid base value rejected: %v", report.Issues())
	}
}

func TestDraftValidate_BaseValue_TypeInputAccepted(t *testing.T) {
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

	fixture.draft.AddValueBinding(
		semantic.NewValueBinding(
			semantic.TypePropertyTarget(
				schema.EntityTypeOwner(fixture.entityTypeID),
				fixture.definitionID,
			),
			semantic.DefinedBindingValue(
				value.NewTyped(value.NewInt64(7)),
			),
			nil,
			domaintime.EffectivePeriod{},
			false,
		),
	)

	report := fixture.draft.Validate()
	if !report.OK() {
		t.Fatalf("valid type base value rejected: %v", report.Issues())
	}
}

func TestDraftValidate_BaseValue_TargetObjectNotFound(t *testing.T) {
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

	fixture.draft.AddValueBinding(
		semantic.NewValueBinding(
			semantic.ObjectPropertyTarget(
				semantic.NewPropertyAddress(
					semantic.EntityRef(identity.New()),
					fixture.definitionID,
				),
			),
			semantic.DefinedBindingValue(
				value.NewTyped(value.NewInt64(1)),
			),
			nil,
			domaintime.EffectivePeriod{},
			false,
		),
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueValueBindingTargetNotFound,
	)
}

func TestDraftValidate_BaseValue_DefinitionNotFound(t *testing.T) {
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

	fixture.draft.AddValueBinding(
		semantic.NewValueBinding(
			semantic.ObjectPropertyTarget(
				semantic.NewPropertyAddress(
					semantic.EntityRef(fixture.entityID),
					identity.New(),
				),
			),
			semantic.DefinedBindingValue(
				value.NewTyped(value.NewInt64(1)),
			),
			nil,
			domaintime.EffectivePeriod{},
			false,
		),
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueValueBindingDefinitionNotFound,
	)
}

func TestDraftValidate_BaseValue_TargetMismatch(t *testing.T) {
	typeAID := identity.New()
	typeBID := identity.New()
	entityID := identity.New()
	definitionID := identity.New()

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(typeBID),
		identity.UUID{},
		false,
		"Value",
		value.Integer,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	draft := NewDraft()
	draft.AddEntityType(schema.NewEntityType(typeAID, "A", nil, nil))
	draft.AddEntityType(schema.NewEntityType(
		typeBID,
		"B",
		nil,
		[]identity.UUID{definitionID},
	))
	draft.AddPropertyDefinition(definition)
	draft.AddEntity(semantic.NewEntity(entityID, typeAID, "", false))
	draft.AddValueBinding(
		semantic.NewValueBinding(
			semantic.ObjectPropertyTarget(
				semantic.NewPropertyAddress(
					semantic.EntityRef(entityID),
					definitionID,
				),
			),
			semantic.DefinedBindingValue(
				value.NewTyped(value.NewInt64(1)),
			),
			nil,
			domaintime.EffectivePeriod{},
			false,
		),
	)

	requireValidationIssue(
		t,
		draft.Validate(),
		ValidationIssueValueBindingTargetMismatch,
	)
}

func TestDraftValidate_BaseValue_DerivedRejected(t *testing.T) {
	fixture := newBaseValueFixture(
		t,
		schema.DerivedComputation(
			ast.NewConstant(value.NewTyped(value.NewInt64(1))),
		),
		schema.TemporalScalar,
		identity.UUID{},
		false,
		value.Integer,
		unit.Unit{},
		false,
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewInt64(1)),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueBaseValueForDerivedProperty,
	)
}

func TestDraftValidate_BaseValue_TypeMismatch(t *testing.T) {
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
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewString("wrong")),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueValueTypeMismatch,
	)
}

func TestDraftValidate_BaseValue_UnitMustBeExact(t *testing.T) {
	length := unit.Base("length")
	meter := unit.NewUnit("meter", length, "length", rational.One())
	centimeter := unit.NewUnit(
		"centimeter",
		length,
		"length",
		rational.New(big.NewInt(1), big.NewInt(100)),
	)

	fixture := newBaseValueFixture(
		t,
		schema.InputComputation(),
		schema.TemporalScalar,
		identity.UUID{},
		false,
		value.Integer,
		meter,
		true,
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTypedWithUnit(
				value.NewInt64(100),
				centimeter,
			),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueValueUnitMismatch,
	)
}

func TestDraftValidate_BaseValue_NullAllowedByContract(t *testing.T) {
	contractID := identity.New()
	fixture := newBaseValueFixture(
		t,
		schema.InputComputation(),
		schema.TemporalScalar,
		contractID,
		true,
		value.Integer,
		unit.Unit{},
		false,
	)

	fixture.draft.AddPropertyContract(
		schema.NewPropertyContract(
			contractID,
			"NullableValue",
			value.Integer,
			unit.Dimension{},
			false,
			schema.TemporalScalar,
			true,
		),
	)

	fixture.addObjectBinding(
		semantic.NullBindingValue(),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	report := fixture.draft.Validate()
	if !report.OK() {
		t.Fatalf("nullable contract rejected Null: %v", report.Issues())
	}
}

func TestDraftValidate_BaseValue_NullRejectedWithoutNullableContract(t *testing.T) {
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
		semantic.NullBindingValue(),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueNullNotAllowed,
	)
}

func TestDraftValidate_BaseValue_DefinedSeriesRejected(t *testing.T) {
	fixture := newBaseValueFixture(
		t,
		schema.InputComputation(),
		schema.TemporalSeries,
		identity.UUID{},
		false,
		value.Integer,
		unit.Unit{},
		false,
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewInt64(1)),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueUnsupportedSeriesBaseValue,
	)
}

func TestDraftValidate_BaseValue_InvalidEffectivePeriod(t *testing.T) {
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

	from := mustDateTime(t, "2026-08-28T00:00:00Z")
	to := mustDateTime(t, "2026-08-27T00:00:00Z")

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewInt64(1)),
		),
		nil,
		domaintime.NewEffectivePeriod(
			from,
			true,
			to,
			true,
		),
		true,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueInvalidEffectivePeriod,
	)
}

func TestDraftValidate_BaseValue_DuplicateScopeSelector(t *testing.T) {
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
	selectors := []scope.Selector{
		scope.NewSelector(scopeID, value.NewTyped(value.NewString("EU"))),
		scope.NewSelector(scopeID, value.NewTyped(value.NewString("US"))),
	}

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewInt64(1)),
		),
		selectors,
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueDuplicateScopeSelector,
	)
}

func TestDraftValidate_BaseValue_ConflictingBindings(t *testing.T) {
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
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewInt64(1)),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)
	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewInt64(2)),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueConflictingValueBindings,
	)
}

func TestDraftValidate_BaseValue_NonOverlappingPeriodsDoNotConflict(t *testing.T) {
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

	t0 := mustDateTime(t, "2026-08-27T00:00:00Z")
	t1 := mustDateTime(t, "2026-08-28T00:00:00Z")
	t2 := mustDateTime(t, "2026-08-29T00:00:00Z")

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewInt64(1)),
		),
		nil,
		domaintime.NewEffectivePeriod(t0, true, t1, true),
		true,
	)
	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(value.NewInt64(2)),
		),
		nil,
		domaintime.NewEffectivePeriod(t1, true, t2, true),
		true,
	)

	report := fixture.draft.Validate()
	if !report.OK() {
		t.Fatalf("adjacent half-open periods rejected: %v", report.Issues())
	}
}

type baseValueFixture struct {
	draft        *Draft
	entityTypeID identity.UUID
	entityID     identity.UUID
	definitionID identity.UUID
}

func newBaseValueFixture(
	t *testing.T,
	computation schema.Computation,
	temporalMode schema.TemporalMode,
	contractID identity.UUID,
	hasContract bool,
	valueType value.Type,
	propertyUnit unit.Unit,
	hasUnit bool,
) baseValueFixture {
	t.Helper()

	entityTypeID := identity.New()
	entityID := identity.New()
	definitionID := identity.New()

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(entityTypeID),
		contractID,
		hasContract,
		"Value",
		valueType,
		propertyUnit,
		hasUnit,
		temporalMode,
		computation,
	)

	draft := NewDraft()
	draft.AddEntityType(
		schema.NewEntityType(
			entityTypeID,
			"EntityType",
			nil,
			[]identity.UUID{definitionID},
		),
	)
	draft.AddPropertyDefinition(definition)
	draft.AddEntity(
		semantic.NewEntity(
			entityID,
			entityTypeID,
			"",
			false,
		),
	)

	return baseValueFixture{
		draft:        draft,
		entityTypeID: entityTypeID,
		entityID:     entityID,
		definitionID: definitionID,
	}
}

func (f baseValueFixture) addObjectBinding(
	bindingValue semantic.BindingValue,
	selectors []scope.Selector,
	period domaintime.EffectivePeriod,
	hasPeriod bool,
) {
	f.draft.AddValueBinding(
		semantic.NewValueBinding(
			semantic.ObjectPropertyTarget(
				semantic.NewPropertyAddress(
					semantic.EntityRef(f.entityID),
					f.definitionID,
				),
			),
			bindingValue,
			selectors,
			period,
			hasPeriod,
		),
	)
}

func mustDateTime(
	t *testing.T,
	s string,
) datetime.DateTime {
	t.Helper()

	result, err := datetime.Parse(s)
	if err != nil {
		t.Fatalf("parse datetime %q: %v", s, err)
	}

	return result
}
