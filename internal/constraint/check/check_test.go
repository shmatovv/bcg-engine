package check_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/impact"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

type fixture struct {
	draft            *model.Draft
	typeID, entityID identity.UUID
	definitionIDs    []identity.UUID
}

func newFixture() *fixture {
	return &fixture{draft: model.NewDraft(), typeID: identity.New(), entityID: identity.New()}
}

func (f *fixture) property(typ value.Type, computation schema.Computation, nullable bool) address.PropertyAddress {
	definitionID := identity.New()
	contractID := identity.UUID{}
	if nullable {
		contractID = identity.New()
		f.draft.AddPropertyContract(schema.NewPropertyContract(contractID, "nullable", typ, unit.Dimension{}, false, schema.TemporalScalar, true))
	}
	f.definitionIDs = append(f.definitionIDs, definitionID)
	f.draft.AddPropertyDefinition(schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(f.typeID),
		contractID,
		nullable,
		"property",
		typ,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		computation,
	))
	return address.NewPropertyAddress(address.EntityRef(f.entityID), definitionID)
}

func (f *fixture) commit(t *testing.T) model.ModelVersion {
	t.Helper()
	f.draft.AddEntityType(schema.NewEntityType(f.typeID, "item", nil, f.definitionIDs))
	f.draft.AddEntity(semantic.NewEntity(f.entityID, f.typeID, "", false))
	version, err := model.Commit(
		model.NewModel(identity.New(), "constraint-check"),
		f.draft,
		model.NewCommitOptions(identity.New(), "constraint-check", precision.New(6, precision.HalfEven)),
	)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func at(t *testing.T, value string) domaintime.Context {
	t.Helper()
	instant, err := datetime.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return domaintime.At(instant)
}

func contextAt(t *testing.T, value string, overrides ...runtime.Override) runtime.CalculationContext {
	t.Helper()
	return runtime.NewCalculationContext(at(t, value), scope.Values{}, overrides)
}

func intBinding(number int64) semantic.BindingValue {
	return semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(number)))
}

func integer(number int64) ast.Expression {
	return ast.NewConstant(value.NewTyped(value.NewInt64(number)))
}
func boolean(flag bool) ast.Expression { return ast.NewConstant(value.NewTyped(value.NewBool(flag))) }
func ref(target address.PropertyAddress) ast.Expression {
	return ast.NewAbsolutePropertyReference(target)
}

func TestConstraintStatesAndApplicabilityLaziness(t *testing.T) {
	f := newFixture()
	nullable := f.property(value.Bool, schema.InputComputation(), true)
	inactiveID := identity.New()
	undefinedApplicabilityID := identity.New()
	nullAssertionID := identity.New()
	trueID := identity.New()
	falseID := identity.New()
	applicableID := identity.New()
	dangerous := ast.Greater(ast.Divide(integer(1), integer(0)), integer(10))
	f.draft.AddConstraint(constraint.New(inactiveID, "inactive", boolean(false), true, dangerous, constraint.SeverityHard, "", false))
	f.draft.AddConstraint(constraint.New(undefinedApplicabilityID, "unknown-applicability", ref(nullable), true, dangerous, constraint.SeverityHard, "", false))
	f.draft.AddConstraint(constraint.New(nullAssertionID, "null-assertion", nil, false, ref(nullable), constraint.SeverityHard, "", false))
	f.draft.AddConstraint(constraint.New(trueID, "satisfied", nil, false, boolean(true), constraint.SeverityInfo, "", false))
	f.draft.AddConstraint(constraint.New(falseID, "violated", nil, false, boolean(false), constraint.SeverityWarning, "", false))
	f.draft.AddConstraint(constraint.New(applicableID, "applicable", boolean(true), true, boolean(false), constraint.SeverityHard, "", false))
	version := f.commit(t)

	base := contextAt(t, "2026-01-01T00:00:00Z")
	inactive, err := check.One(version, inactiveID, base)
	if err != nil || inactive.State() != check.StateNotApplicable {
		t.Fatalf("inactive state = %v, %v", inactive.State(), err)
	}
	if _, evaluated := inactive.Assertion(); evaluated {
		t.Fatal("false applicability evaluated assertion")
	}
	unknown, err := check.One(version, undefinedApplicabilityID, base)
	if err != nil || unknown.State() != check.StateUndefined {
		t.Fatalf("undefined applicability state = %v, %v", unknown.State(), err)
	}
	if _, evaluated := unknown.Assertion(); evaluated {
		t.Fatal("undefined applicability evaluated assertion")
	}
	nullResult, err := check.One(version, nullAssertionID, base.WithTimeContext(base.TimeContext()))
	if err != nil || nullResult.State() != check.StateUndefined {
		t.Fatalf("missing nullable assertion state = %v, %v", nullResult.State(), err)
	}
	assertion, evaluated := nullResult.Assertion()
	if !evaluated || !assertion.Value().IsUndefined() {
		t.Fatal("missing assertion must retain its actual Undefined evaluator result")
	}

	nullContext := contextAt(t, "2026-01-01T00:00:00Z", runtime.NewOverride(nullable, semantic.NullBindingValue()))
	nullApplicability, err := check.One(version, undefinedApplicabilityID, nullContext)
	if err != nil || nullApplicability.State() != check.StateUndefined {
		t.Fatalf("null applicability state = %v, %v", nullApplicability.State(), err)
	}
	if applicability, evaluated := nullApplicability.Applicability(); !evaluated || !applicability.Value().IsNull() {
		t.Fatal("Null applicability must be indeterminate, not false")
	}
	if _, evaluated := nullApplicability.Assertion(); evaluated {
		t.Fatal("Null applicability evaluated assertion")
	}
	nullResult, err = check.One(version, nullAssertionID, nullContext)
	if err != nil || nullResult.State() != check.StateUndefined {
		t.Fatalf("null assertion state = %v, %v", nullResult.State(), err)
	}
	assertion, evaluated = nullResult.Assertion()
	if !evaluated || !assertion.Value().IsNull() {
		t.Fatal("Null assertion must stay distinct from Undefined")
	}

	applicable, err := check.One(version, applicableID, base)
	if err != nil || applicable.State() != check.StateViolated {
		t.Fatalf("true applicability state = %v, %v", applicable.State(), err)
	}
	if _, evaluated := applicable.Assertion(); !evaluated {
		t.Fatal("true applicability did not evaluate assertion")
	}

	for _, test := range []struct {
		id   identity.UUID
		want check.State
	}{{trueID, check.StateSatisfied}, {falseID, check.StateViolated}} {
		result, err := check.One(version, test.id, base)
		if err != nil || result.State() != test.want {
			t.Fatalf("constant result = %v, %v; want %v", result.State(), err, test.want)
		}
	}
}

func TestConstraintUsesOrdinaryResolutionDerivedOverridesAndProvenance(t *testing.T) {
	f := newFixture()
	a := f.property(value.Integer, schema.InputComputation(), false)
	b := f.property(value.Integer, schema.DerivedComputation(ast.Multiply(ref(a), integer(2))), false)
	c := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(b), integer(10))), false)
	limit := f.property(value.Integer, schema.InputComputation(), false)
	constraintID := identity.New()
	f.draft.AddConstraint(constraint.New(constraintID, "limit", nil, false, ast.LessOrEqual(ref(c), ref(limit)), constraint.SeverityHard, "", false))
	version := f.commit(t)

	base := contextAt(t, "2026-01-01T00:00:00Z", runtime.NewOverride(a, intBinding(5)), runtime.NewOverride(limit, intBinding(20)))
	result, err := check.One(version, constraintID, base)
	if err != nil || result.State() != check.StateSatisfied {
		t.Fatalf("derived base state = %v, %v", result.State(), err)
	}
	assertion, evaluated := result.Assertion()
	if !evaluated || !provenanceContains(assertion.Provenance(), a) || !provenanceContains(assertion.Provenance(), b) || !provenanceContains(assertion.Provenance(), c) {
		t.Fatal("constraint assertion lost actual derived dependency provenance")
	}

	scenarioOverride := domainscenario.NewOverride(b, intBinding(3), nil, domaintime.EffectivePeriod{}, false)
	scenario := domainscenario.New(identity.New(), "override-b", version.ID(), []domainscenario.Override{scenarioOverride})
	result, err = check.One(version, constraintID, base.WithScenario(scenario))
	if err != nil || result.State() != check.StateSatisfied {
		t.Fatalf("scenario derived override state = %v, %v", result.State(), err)
	}
	assertion, _ = result.Assertion()
	if provenanceContains(assertion.Provenance(), a) {
		t.Fatal("scenario override of derived B evaluated bypassed upstream A")
	}

	calculation := contextAt(t, "2026-01-01T00:00:00Z", runtime.NewOverride(a, intBinding(5)), runtime.NewOverride(b, intBinding(100)), runtime.NewOverride(limit, intBinding(20))).WithScenario(scenario)
	result, err = check.One(version, constraintID, calculation)
	if err != nil || result.State() != check.StateViolated {
		t.Fatalf("calculation override precedence state = %v, %v", result.State(), err)
	}
	assertion, _ = result.Assertion()
	if provenanceContains(assertion.Provenance(), a) {
		t.Fatal("calculation override of derived B evaluated bypassed upstream A")
	}

	// Constraint checking shares immutable model/context data safely with the
	// existing Explain and Observed Impact pipelines.
	var group sync.WaitGroup
	for index := 0; index < 16; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := check.One(version, constraintID, base); err != nil {
				t.Error(err)
			}
			if _, err := evaluator.Explain(version, c, base); err != nil {
				t.Error(err)
			}
			if _, err := impact.AnalyzeObserved(version, a, base); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
}

func TestConstraintScopeAndEffectiveTimeUseResolverSemantics(t *testing.T) {
	f := newFixture()
	amount := f.property(value.Integer, schema.InputComputation(), false)
	scopeID := identity.New()
	f.draft.AddScopeDefinition(scope.NewDefinition(scopeID, "region", value.String))
	eu := scope.NewSelector(scopeID, value.NewTyped(value.NewString("EU")))
	us := scope.NewSelector(scopeID, value.NewTyped(value.NewString("US")))
	from, err := datetime.Parse("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	middle, err := datetime.Parse("2026-02-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(amount), intBinding(12), []scope.Selector{eu}, domaintime.NewEffectivePeriod(from, true, middle, true), true))
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(amount), intBinding(8), []scope.Selector{eu}, domaintime.NewEffectivePeriod(middle, true, datetime.DateTime{}, false), true))
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(amount), intBinding(4), []scope.Selector{us}, domaintime.EffectivePeriod{}, false))
	constraintID := identity.New()
	f.draft.AddConstraint(constraint.New(constraintID, "minimum", nil, false, ast.GreaterOrEqual(ref(amount), integer(10)), constraint.SeverityHard, "", false))
	version := f.commit(t)

	context := runtime.NewCalculationContext(domaintime.At(from), scope.NewValues([]scope.Selector{eu}), nil)
	result, err := check.One(version, constraintID, context)
	if err != nil || result.State() != check.StateSatisfied {
		t.Fatalf("EU start = %v, %v", result.State(), err)
	}
	context = context.WithTimeContext(domaintime.At(middle))
	result, err = check.One(version, constraintID, context)
	if err != nil || result.State() != check.StateViolated {
		t.Fatalf("EU exact end boundary = %v, %v", result.State(), err)
	}
	context = runtime.NewCalculationContext(domaintime.At(from), scope.NewValues([]scope.Selector{us}), nil)
	result, err = check.One(version, constraintID, context)
	if err != nil || result.State() != check.StateViolated {
		t.Fatalf("US scope = %v, %v", result.State(), err)
	}
}

func TestConstraintReportDeterminismOwnershipConcurrencyAndPeriodRejection(t *testing.T) {
	f := newFixture()
	for index := 0; index < 32; index++ {
		f.draft.AddConstraint(constraint.New(identity.New(), "constant", nil, false, boolean(index%2 == 0), constraint.SeverityInfo, "", false))
	}
	version := f.commit(t)
	context := contextAt(t, "2026-01-01T00:00:00Z")
	report, err := check.All(version, context)
	if err != nil || report.Len() != 32 || !report.IsValid() {
		t.Fatalf("report = %d, %v", report.Len(), err)
	}
	results := report.Results()
	results[0] = check.Result{}
	if !report.IsValid() || report.Len() != 32 {
		t.Fatal("report exposes mutable backing slice")
	}
	for index := 0; index < 50; index++ {
		again, err := check.All(version, context)
		if err != nil {
			t.Fatal(err)
		}
		got, want := again.Results(), report.Results()
		for item := range want {
			if got[item].Constraint().ID() != want[item].Constraint().ID() || got[item].State() != want[item].State() {
				t.Fatal("nondeterministic report")
			}
		}
	}
	var group sync.WaitGroup
	for index := 0; index < 24; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := check.All(version, context); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()

	from, err := datetime.Parse("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	to, err := datetime.Parse("2026-02-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	_, err = check.All(version, runtime.NewCalculationContext(domaintime.Period(from, to), scope.Values{}, nil))
	var constraintError check.Error
	if !errors.As(err, &constraintError) || constraintError.Kind() != check.ErrorUnsupportedTimeContext {
		t.Fatalf("Period error = %v", err)
	}
}

func provenanceContains(node provenance.Node, target address.PropertyAddress) bool {
	if property, ok := node.Property(); ok && property == target {
		return true
	}
	for _, child := range node.Children() {
		if provenanceContains(child, target) {
			return true
		}
	}
	return false
}
