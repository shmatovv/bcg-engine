package eval_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/impact"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/provenance"
	ruleeval "github.com/shmatovv/bcg-engine/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

type ruleFixture struct {
	draft         *model.Draft
	typeID        identity.UUID
	entityID      identity.UUID
	definitionIDs []identity.UUID
}

func newRuleFixture() *ruleFixture {
	return &ruleFixture{draft: model.NewDraft(), typeID: identity.New(), entityID: identity.New()}
}

func (f *ruleFixture) property(typ value.Type, computation schema.Computation, nullable bool) address.PropertyAddress {
	definitionID := identity.New()
	contractID := identity.UUID{}
	if nullable {
		contractID = identity.New()
		f.draft.AddPropertyContract(schema.NewPropertyContract(contractID, "nullable", typ, unit.Dimension{}, false, schema.TemporalScalar, true))
	}
	f.definitionIDs = append(f.definitionIDs, definitionID)
	f.draft.AddPropertyDefinition(schema.NewPropertyDefinition(definitionID, schema.EntityTypeOwner(f.typeID), contractID, nullable, "property", typ, unit.Unit{}, false, schema.TemporalScalar, computation))
	return address.NewPropertyAddress(address.EntityRef(f.entityID), definitionID)
}

func (f *ruleFixture) commit(t *testing.T) model.ModelVersion {
	t.Helper()
	f.draft.AddEntityType(schema.NewEntityType(f.typeID, "item", nil, f.definitionIDs))
	f.draft.AddEntity(semantic.NewEntity(f.entityID, f.typeID, "", false))
	version, err := model.Commit(model.NewModel(identity.New(), "rules"), f.draft, model.NewCommitOptions(identity.New(), "rules", precision.New(6, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func ruleAt(t *testing.T, instant string, overrides ...runtime.Override) runtime.CalculationContext {
	t.Helper()
	parsed, err := datetime.Parse(instant)
	if err != nil {
		t.Fatal(err)
	}
	return runtime.NewCalculationContext(domaintime.At(parsed), scope.Values{}, overrides)
}

func ruleBool(flag bool) ast.Expression { return ast.NewConstant(value.NewTyped(value.NewBool(flag))) }
func ruleInt(number int64) ast.Expression {
	return ast.NewConstant(value.NewTyped(value.NewInt64(number)))
}
func ruleRef(target address.PropertyAddress) ast.Expression {
	return ast.NewAbsolutePropertyReference(target)
}
func ruleBoolBinding(flag bool) semantic.BindingValue {
	return semantic.DefinedBindingValue(value.NewTyped(value.NewBool(flag)))
}
func ruleIntBinding(number int64) semantic.BindingValue {
	return semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(number)))
}

func TestRuleStatesUndefinedCausesAndLazyExpressions(t *testing.T) {
	f := newRuleFixture()
	nullable := f.property(value.Bool, schema.InputComputation(), true)
	missing := f.property(value.Bool, schema.InputComputation(), false)
	derivedUndefined := f.property(value.Bool, schema.DerivedComputation(ast.And(ruleRef(missing), ruleBool(true))), false)
	dangerous := ast.Greater(ast.Divide(ruleInt(1), ruleInt(0)), ruleInt(5))
	ids := map[string]identity.UUID{}
	add := func(name string, condition ast.Expression) {
		ids[name] = identity.New()
		f.draft.AddRule(domainrule.New(ids[name], name, condition))
	}
	add("active", ruleBool(true))
	add("inactive", ruleBool(false))
	add("missing", ruleRef(missing))
	add("null", ruleRef(nullable))
	add("dependency", ruleRef(derivedUndefined))
	add("division", dangerous)
	add("and", ast.And(ruleBool(false), dangerous))
	add("or", ast.Or(ruleBool(true), dangerous))
	add("if", ast.NewIf(ruleBool(true), ruleBool(false), dangerous))
	add("selected-undefined", ast.NewIf(ruleBool(true), ruleRef(missing), ruleBool(true)))
	add("coalesce", ast.NewCoalesce(ruleRef(nullable), ruleBool(true)))
	version := f.commit(t)
	context := ruleAt(t, "2026-01-01T00:00:00Z")

	for name, want := range map[string]ruleeval.State{
		"active":             ruleeval.StateActive,
		"inactive":           ruleeval.StateInactive,
		"missing":            ruleeval.StateUndefined,
		"null":               ruleeval.StateUndefined,
		"dependency":         ruleeval.StateUndefined,
		"division":           ruleeval.StateUndefined,
		"and":                ruleeval.StateInactive,
		"or":                 ruleeval.StateActive,
		"if":                 ruleeval.StateInactive,
		"selected-undefined": ruleeval.StateUndefined,
		"coalesce":           ruleeval.StateUndefined,
	} {
		result, err := ruleeval.One(version, ids[name], context)
		if err != nil || result.State() != want || !result.IsValid() {
			t.Fatalf("%s = %s, %v", name, result.State(), err)
		}
	}
	report, err := ruleeval.All(version, context)
	if err != nil || report.Len() != len(ids) || len(report.Active()) != 2 || len(report.Inactive()) != 3 || len(report.Undefined()) != 6 {
		t.Fatalf("complete report partition = %d/%d/%d/%d, %v", report.Len(), len(report.Active()), len(report.Inactive()), len(report.Undefined()), err)
	}
	undefined := report.Undefined()
	undefined[0] = ruleeval.Result{}
	if !report.IsValid() || len(report.Undefined()) != 6 {
		t.Fatal("Undefined report helper exposes mutable storage")
	}

	missingResult, err := ruleeval.One(version, ids["missing"], context)
	if err != nil {
		t.Fatal(err)
	}
	if cause, ok := missingResult.UndefinedCause(); !ok || cause.Kind() != value.UndefinedDependencyUndefined || !provenanceKind(missingResult.Condition().Provenance(), provenance.KindMissingValue) {
		t.Fatalf("missing cause = %#v, %v", cause, ok)
	}
	divisionResult, err := ruleeval.One(version, ids["division"], context)
	if err != nil {
		t.Fatal(err)
	}
	if cause, ok := divisionResult.UndefinedCause(); !ok || cause.Kind() != value.UndefinedDependencyUndefined || !provenanceUndefinedKind(divisionResult.Condition().Provenance(), value.UndefinedDivisionByZero) {
		t.Fatalf("division cause = %#v, %v", cause, ok)
	}
	dependencyResult, err := ruleeval.One(version, ids["dependency"], context)
	if err != nil {
		t.Fatal(err)
	}
	if cause, ok := dependencyResult.UndefinedCause(); !ok || cause.Kind() != value.UndefinedDependencyUndefined {
		t.Fatalf("dependency cause = %#v, %v", cause, ok)
	}

	for _, name := range []string{"and", "or", "if"} {
		result, err := ruleeval.One(version, ids[name], context)
		if err != nil {
			t.Fatal(err)
		}
		if provenanceOperation(result.Condition().Provenance(), "Divide") {
			t.Fatalf("%s evaluated a lazy non-selected Divide branch", name)
		}
	}

	nullContext := ruleAt(t, "2026-01-01T00:00:00Z", runtime.NewOverride(nullable, semantic.NullBindingValue()))
	nullResult, err := ruleeval.One(version, ids["null"], nullContext)
	if err != nil || nullResult.State() != ruleeval.StateUndefined || !nullResult.Condition().Value().IsNull() {
		t.Fatalf("Null Rule = %s, %#v, %v", nullResult.State(), nullResult.Condition().Value(), err)
	}
	if _, ok := nullResult.UndefinedCause(); ok {
		t.Fatal("Null incorrectly exposes an Undefined cause")
	}
	coalesced, err := ruleeval.One(version, ids["coalesce"], nullContext)
	if err != nil || coalesced.State() != ruleeval.StateActive {
		t.Fatalf("Null coalesce = %s, %v", coalesced.State(), err)
	}
}

func TestRuleUsesOrdinaryOverridesScopeAndEffectiveTime(t *testing.T) {
	t.Run("derived Scenario and calculation overrides", func(t *testing.T) {
		f := newRuleFixture()
		a := f.property(value.Integer, schema.InputComputation(), false)
		b := f.property(value.Integer, schema.DerivedComputation(ast.Multiply(ruleRef(a), ruleInt(2))), false)
		limit := f.property(value.Integer, schema.InputComputation(), false)
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(a), ruleIntBinding(4), nil, domaintime.EffectivePeriod{}, false))
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(limit), ruleIntBinding(10), nil, domaintime.EffectivePeriod{}, false))
		ruleID := identity.New()
		f.draft.AddRule(domainrule.New(ruleID, "limit", ast.GreaterOrEqual(ruleRef(b), ruleRef(limit))))
		version := f.commit(t)
		base := ruleAt(t, "2026-01-01T00:00:00Z")
		result, err := ruleeval.One(version, ruleID, base)
		if err != nil || result.State() != ruleeval.StateInactive {
			t.Fatalf("base = %s, %v", result.State(), err)
		}
		scenario := domainscenario.New(identity.New(), "b override", version.ID(), []domainscenario.Override{domainscenario.NewOverride(b, ruleIntBinding(12), nil, domaintime.EffectivePeriod{}, false)})
		result, err = ruleeval.One(version, ruleID, base.WithScenario(scenario))
		if err != nil || result.State() != ruleeval.StateActive || provenanceContains(result.Condition().Provenance(), a) {
			t.Fatalf("scenario = %s, %v", result.State(), err)
		}
		calculation := ruleAt(t, "2026-01-01T00:00:00Z", runtime.NewOverride(b, ruleIntBinding(5))).WithScenario(scenario)
		result, err = ruleeval.One(version, ruleID, calculation)
		if err != nil || result.State() != ruleeval.StateInactive || provenanceContains(result.Condition().Provenance(), a) {
			t.Fatalf("calculation precedence = %s, %v", result.State(), err)
		}
	})

	t.Run("Scope and half-open effective time", func(t *testing.T) {
		f := newRuleFixture()
		flag := f.property(value.Bool, schema.InputComputation(), false)
		scopeID := identity.New()
		f.draft.AddScopeDefinition(scope.NewDefinition(scopeID, "region", value.String))
		eu := scope.NewSelector(scopeID, value.NewTyped(value.NewString("EU")))
		us := scope.NewSelector(scopeID, value.NewTyped(value.NewString("US")))
		from, _ := datetime.Parse("2026-01-01T00:00:00Z")
		middle, _ := datetime.Parse("2026-02-01T00:00:00Z")
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(flag), ruleBoolBinding(true), []scope.Selector{eu}, domaintime.NewEffectivePeriod(from, true, middle, true), true))
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(flag), ruleBoolBinding(false), []scope.Selector{eu}, domaintime.NewEffectivePeriod(middle, true, datetime.DateTime{}, false), true))
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(flag), ruleBoolBinding(false), []scope.Selector{us}, domaintime.EffectivePeriod{}, false))
		ruleID := identity.New()
		f.draft.AddRule(domainrule.New(ruleID, "flag", ruleRef(flag)))
		version := f.commit(t)
		for _, test := range []struct {
			at    datetime.DateTime
			scope scope.Values
			want  ruleeval.State
		}{{from, scope.NewValues([]scope.Selector{eu}), ruleeval.StateActive}, {middle, scope.NewValues([]scope.Selector{eu}), ruleeval.StateInactive}, {from, scope.NewValues([]scope.Selector{us}), ruleeval.StateInactive}} {
			result, err := ruleeval.One(version, ruleID, runtime.NewCalculationContext(domaintime.At(test.at), test.scope, nil))
			if err != nil || result.State() != test.want {
				t.Fatalf("scope/time = %s, %v", result.State(), err)
			}
		}
	})
}

func TestRuleReportOwnershipDeterminismEmptyAndRequestErrors(t *testing.T) {
	f := newRuleFixture()
	ids := make([]identity.UUID, 160)
	for index := range ids {
		ids[index] = identity.New()
	}
	for index := len(ids) - 1; index >= 0; index-- {
		f.draft.AddRule(domainrule.New(ids[index], "rule", ruleBool(index%3 == 0)))
	}
	version := f.commit(t)
	context := ruleAt(t, "2026-01-01T00:00:00Z")
	report, err := ruleeval.All(version, context)
	if err != nil || report.Len() != len(ids) || !report.IsValid() {
		t.Fatalf("report = %d, %v", report.Len(), err)
	}
	if len(report.Active()) == 0 || len(report.Inactive()) == 0 || len(report.Undefined()) != 0 {
		t.Fatal("report state helpers did not retain complete state partition")
	}
	results := report.Results()
	results[0] = ruleeval.Result{}
	active := report.Active()
	active[0] = ruleeval.Result{}
	if !report.IsValid() || report.Len() != len(ids) || len(report.Active()) == 0 {
		t.Fatal("report exposes mutable slice storage")
	}
	for index := 0; index < 40; index++ {
		again, err := ruleeval.All(version, context)
		if err != nil {
			t.Fatal(err)
		}
		for resultIndex, expected := range report.Results() {
			if actual := again.Results()[resultIndex]; actual.Rule().ID() != expected.Rule().ID() || actual.State() != expected.State() {
				t.Fatal("nondeterministic Rule report")
			}
		}
	}
	if _, err := ruleeval.One(version, identity.New(), context); err == nil {
		t.Fatal("unknown Rule ID was accepted")
	} else {
		var ruleError ruleeval.Error
		if !errors.As(err, &ruleError) || ruleError.Kind() != ruleeval.ErrorRuleNotFound {
			t.Fatalf("unknown Rule error = %v", err)
		}
	}
	from, _ := datetime.Parse("2026-01-01T00:00:00Z")
	to, _ := datetime.Parse("2026-02-01T00:00:00Z")
	period := runtime.NewCalculationContext(domaintime.Period(from, to), scope.Values{}, nil)
	for _, err := range []error{func() error { _, err := ruleeval.One(version, ids[0], period); return err }(), func() error { _, err := ruleeval.All(version, period); return err }()} {
		var ruleError ruleeval.Error
		if !errors.As(err, &ruleError) || ruleError.Kind() != ruleeval.ErrorUnsupportedTimeContext {
			t.Fatalf("Period error = %v", err)
		}
	}
	empty, err := model.Commit(model.NewModel(identity.New(), "empty"), model.NewDraft(), model.NewCommitOptions(identity.New(), "empty", precision.New(6, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	emptyReport, err := ruleeval.All(empty, context)
	if err != nil || emptyReport.Len() != 0 || !emptyReport.IsValid() {
		t.Fatalf("empty report = %d, %v", emptyReport.Len(), err)
	}
}

func TestRuleEvaluationSharesImmutableStateConcurrently(t *testing.T) {
	f := newRuleFixture()
	input := f.property(value.Bool, schema.InputComputation(), false)
	derived := f.property(value.Bool, schema.DerivedComputation(ast.And(ruleRef(input), ruleBool(true))), false)
	ruleID := identity.New()
	f.draft.AddRule(domainrule.New(ruleID, "derived", ruleRef(derived)))
	f.draft.AddConstraint(constraint.New(identity.New(), "constraint", nil, false, ruleRef(derived), constraint.SeverityHard, "", false))
	version := f.commit(t)
	context := ruleAt(t, "2026-01-01T00:00:00Z", runtime.NewOverride(input, ruleBoolBinding(true)))
	var group sync.WaitGroup
	for index := 0; index < 24; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := ruleeval.One(version, ruleID, context); err != nil {
				t.Error(err)
			}
			if _, err := ruleeval.All(version, context); err != nil {
				t.Error(err)
			}
			if _, err := evaluator.Explain(version, derived, context); err != nil {
				t.Error(err)
			}
			if _, err := impact.AnalyzeObserved(version, input, context); err != nil {
				t.Error(err)
			}
			report, err := check.All(version, context)
			if err != nil {
				t.Error(err)
			} else if feasibility.Assess(report).State() != feasibility.StateFeasible {
				t.Error("unexpected feasibility")
			}
		}()
	}
	group.Wait()
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

func provenanceOperation(node provenance.Node, operation string) bool {
	if actual, ok := node.Operation(); ok && actual == operation {
		return true
	}
	for _, child := range node.Children() {
		if provenanceOperation(child, operation) {
			return true
		}
	}
	return false
}

func provenanceKind(node provenance.Node, kind provenance.Kind) bool {
	if node.Kind() == kind {
		return true
	}
	for _, child := range node.Children() {
		if provenanceKind(child, kind) {
			return true
		}
	}
	return false
}

func provenanceUndefinedKind(node provenance.Node, kind value.UndefinedKind) bool {
	if resolved, ok := node.Resolved(); ok && resolved.IsUndefined() && resolved.UndefinedInfo().Kind() == kind {
		return true
	}
	for _, child := range node.Children() {
		if provenanceUndefinedKind(child, kind) {
			return true
		}
	}
	return false
}
