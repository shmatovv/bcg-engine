// internal/model/value_binding_conflicts_test.go

package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestDraftValidate_BaseValue_DisjointEqualSpecificityScopesConflict(t *testing.T) {
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

	regionID := identity.New()
	channelID := identity.New()

	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(
			regionID,
			"Region",
			value.String,
		),
	)

	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(
			channelID,
			"Channel",
			value.String,
		),
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(1),
			),
		),
		[]scope.Selector{
			scope.NewSelector(
				regionID,
				value.NewTyped(
					value.NewString("EU"),
				),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(2),
			),
		),
		[]scope.Selector{
			scope.NewSelector(
				channelID,
				value.NewTyped(
					value.NewString("B2B"),
				),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueConflictingValueBindings,
	)
}

func TestDraftValidate_BaseValue_CompatibleOverlappingScopesConflict(t *testing.T) {
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

	regionID := identity.New()
	channelID := identity.New()
	tierID := identity.New()

	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(
			regionID,
			"Region",
			value.String,
		),
	)

	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(
			channelID,
			"Channel",
			value.String,
		),
	)

	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(
			tierID,
			"Tier",
			value.String,
		),
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(1),
			),
		),
		[]scope.Selector{
			scope.NewSelector(
				regionID,
				value.NewTyped(
					value.NewString("EU"),
				),
			),
			scope.NewSelector(
				channelID,
				value.NewTyped(
					value.NewString("B2B"),
				),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(2),
			),
		),
		[]scope.Selector{
			scope.NewSelector(
				regionID,
				value.NewTyped(
					value.NewString("EU"),
				),
			),
			scope.NewSelector(
				tierID,
				value.NewTyped(
					value.NewString("VIP"),
				),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	requireValidationIssue(
		t,
		fixture.draft.Validate(),
		ValidationIssueConflictingValueBindings,
	)
}

func TestDraftValidate_BaseValue_IncompatibleEqualSpecificityScopesDoNotConflict(t *testing.T) {
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

	regionID := identity.New()

	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(
			regionID,
			"Region",
			value.String,
		),
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(1),
			),
		),
		[]scope.Selector{
			scope.NewSelector(
				regionID,
				value.NewTyped(
					value.NewString("EU"),
				),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(2),
			),
		),
		[]scope.Selector{
			scope.NewSelector(
				regionID,
				value.NewTyped(
					value.NewString("US"),
				),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	report := fixture.draft.Validate()
	if !report.OK() {
		t.Fatalf(
			"mutually exclusive scope bindings rejected: %v",
			report.Issues(),
		)
	}
}

func TestDraftValidate_BaseValue_DifferentScopeSpecificityDoesNotConflict(t *testing.T) {
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

	regionID := identity.New()
	channelID := identity.New()

	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(
			regionID,
			"Region",
			value.String,
		),
	)

	fixture.draft.AddScopeDefinition(
		scope.NewDefinition(
			channelID,
			"Channel",
			value.String,
		),
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(1),
			),
		),
		[]scope.Selector{
			scope.NewSelector(
				regionID,
				value.NewTyped(
					value.NewString("EU"),
				),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	fixture.addObjectBinding(
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(2),
			),
		),
		[]scope.Selector{
			scope.NewSelector(
				regionID,
				value.NewTyped(
					value.NewString("EU"),
				),
			),
			scope.NewSelector(
				channelID,
				value.NewTyped(
					value.NewString("B2B"),
				),
			),
		},
		domaintime.EffectivePeriod{},
		false,
	)

	report := fixture.draft.Validate()
	if !report.OK() {
		t.Fatalf(
			"different scope specificity rejected: %v",
			report.Issues(),
		)
	}
}
