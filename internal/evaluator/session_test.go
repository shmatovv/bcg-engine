package evaluator_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/v2/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/impact"
	ruleeval "github.com/shmatovv/bcg-engine/v2/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func sessionValue(t *testing.T, result evaluator.Result) int64 {
	t.Helper()
	if !result.Value().IsDefined() || !result.Value().Value().Raw().Integer().IsInt64() {
		t.Fatalf("result is not a defined int64: %#v", result.Value())
	}
	return result.Value().Value().Raw().Integer().Int64()
}

func sessionContext(overrides ...runtime.Override) runtime.CalculationContext {
	return runtime.NewCalculationContext(reviewContext().TimeContext(), reviewContext().ScopeValues(), overrides)
}

func TestSessionColdWarmExplainAndBatchEquivalence(t *testing.T) {
	f := newPipelineFixture()
	a := f.property(value.Integer, schema.InputComputation(), false)
	b := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))), false)
	c := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(b), ref(b))), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(a), bindingInteger(3), nil, domaintime.EffectivePeriod{}, false))
	version := f.commit(t)

	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}

	cold, err := evaluator.Explain(version, c, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	warm, err := session.Explain(c, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if !cold.Result().Value().Equal(warm.Result().Value()) || !reflect.DeepEqual(cold.Root(), warm.Root()) {
		t.Fatal("cold and session Explain differ semantically")
	}

	before := session.Stats()
	again, err := session.Explain(c, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(warm.Root(), again.Root()) {
		t.Fatal("warm Explain changed semantic provenance")
	}
	after := session.Stats()
	if after.Hits() <= before.Hits() || after.Entries() != before.Entries() {
		t.Fatalf("warm evaluation did not reuse a stable cache: before=%#v after=%#v", before, after)
	}

	results, err := session.EvaluateMany([]address.PropertyAddress{c, b, c}, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || sessionValue(t, results[0]) != 8 || sessionValue(t, results[1]) != 4 || sessionValue(t, results[2]) != 8 {
		t.Fatalf("ordered duplicate batch results = %#v", results)
	}

	ordered := sessionContext(runtime.NewOverride(a, bindingInteger(10)), runtime.NewOverride(b, bindingInteger(20)))
	reversed := sessionContext(runtime.NewOverride(b, bindingInteger(20)), runtime.NewOverride(a, bindingInteger(10)))
	if !ordered.Equal(reversed) || len(ordered.ChangedOverrideTargets(reversed)) != 0 {
		t.Fatal("equivalent calculation override sets lost canonical identity")
	}
	if _, err := session.Evaluate(c, ordered); err != nil {
		t.Fatal(err)
	}
	beforeEquivalent := session.Stats()
	if _, err := session.Evaluate(c, reversed); err != nil {
		t.Fatal(err)
	}
	if session.Stats().Hits() <= beforeEquivalent.Hits() {
		t.Fatal("equivalent override insertion order did not reuse current environment")
	}

	session.Reset()
	if session.Stats().Entries() != 0 {
		t.Fatal("Reset retained runtime entries")
	}
	resetResult, err := session.Evaluate(c, reviewContext())
	if err != nil || sessionValue(t, resetResult) != 8 {
		t.Fatalf("Reset evaluation = %#v, %v", resetResult, err)
	}
}

func TestSessionIncrementalOverrideClosureAndEffectiveCut(t *testing.T) {
	f := newPipelineFixture()
	a := f.property(value.Integer, schema.InputComputation(), false)
	b := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))), false)
	c := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(b), integer(1))), false)
	x := f.property(value.Integer, schema.InputComputation(), false)
	y := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(x), integer(1))), false)
	z := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(y), integer(1))), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(a), bindingInteger(1), nil, domaintime.EffectivePeriod{}, false))
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(x), bindingInteger(10), nil, domaintime.EffectivePeriod{}, false))
	version := f.commit(t)

	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}
	base := sessionContext()
	if result, err := session.Evaluate(c, base); err != nil || sessionValue(t, result) != 3 {
		t.Fatalf("base C = %#v, %v", result, err)
	}
	if result, err := session.Evaluate(z, base); err != nil || sessionValue(t, result) != 12 {
		t.Fatalf("base Z = %#v, %v", result, err)
	}

	changedA := sessionContext(runtime.NewOverride(a, bindingInteger(5)))
	result, err := session.Evaluate(c, changedA)
	if err != nil || sessionValue(t, result) != 7 {
		t.Fatalf("override A -> C = %#v, %v", result, err)
	}
	statsBeforeZ := session.Stats()
	result, err = session.Evaluate(z, changedA)
	if err != nil || sessionValue(t, result) != 12 {
		t.Fatalf("unrelated Z after A override = %#v, %v", result, err)
	}
	if session.Stats().Hits() <= statsBeforeZ.Hits() {
		t.Fatal("unaffected branch was not reused after incremental transition")
	}

	overrideB := sessionContext(runtime.NewOverride(b, bindingInteger(100)))
	result, err = session.Evaluate(c, overrideB)
	if err != nil || sessionValue(t, result) != 101 {
		t.Fatalf("derived override B -> C = %#v, %v", result, err)
	}
	keepBOverride := sessionContext(runtime.NewOverride(a, bindingInteger(50)), runtime.NewOverride(b, bindingInteger(100)))
	statsBeforeCut := session.Stats()
	result, err = session.Evaluate(c, keepBOverride)
	if err != nil || sessionValue(t, result) != 101 {
		t.Fatalf("A change behind B override -> C = %#v, %v", result, err)
	}
	statsAfterCut := session.Stats()
	if statsAfterCut.Hits() <= statsBeforeCut.Hits() || statsAfterCut.Misses() != statsBeforeCut.Misses() {
		t.Fatalf("effective override cut did not preserve C cache: before=%#v after=%#v", statsBeforeCut, statsAfterCut)
	}

	removedB := sessionContext(runtime.NewOverride(a, bindingInteger(50)))
	result, err = session.Evaluate(c, removedB)
	if err != nil || sessionValue(t, result) != 52 {
		t.Fatalf("B override removal did not reconnect dependency: %#v, %v", result, err)
	}

	cold, err := evaluator.Evaluate(version, c, removedB)
	if err != nil || !cold.Value().Equal(result.Value()) || !reflect.DeepEqual(cold.Provenance(), result.Provenance()) {
		t.Fatalf("incremental result differs from cold evaluator: %#v %#v %v", cold, result, err)
	}
}

func TestSessionUndefinedNullAndSharedConstraintRuleReuse(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), true)
	derived := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(input), integer(1))), true)
	constraintID, ruleID := identity.New(), identity.New()
	f.draft.AddConstraint(constraint.New(constraintID, "derived positive", nil, false, ast.Greater(ref(derived), integer(0)), constraint.SeverityHard, "", false))
	f.draft.AddRule(domainrule.New(ruleID, "derived positive", ast.Greater(ref(derived), integer(0))))
	version := f.commit(t)

	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}

	missing := sessionContext()
	undefined, err := session.Evaluate(derived, missing)
	if err != nil || !undefined.Value().IsUndefined() {
		t.Fatalf("missing input result = %#v, %v", undefined, err)
	}
	defined := sessionContext(runtime.NewOverride(input, bindingInteger(2)))
	result, err := session.Evaluate(derived, defined)
	if err != nil || sessionValue(t, result) != 3 {
		t.Fatalf("Undefined -> Defined transition = %#v, %v", result, err)
	}
	nullContext := sessionContext(runtime.NewOverride(input, semantic.NullBindingValue()))
	result, err = session.Evaluate(derived, nullContext)
	if err != nil || !result.Value().IsNull() {
		t.Fatalf("Defined -> Null transition = %#v, %v", result, err)
	}

	shared := sessionContext(runtime.NewOverride(input, bindingInteger(2)))
	constraintReport, err := check.AllWithSession(session, shared)
	if err != nil || constraintReport.Results()[0].State() != check.StateSatisfied {
		t.Fatalf("session constraints = %#v, %v", constraintReport, err)
	}
	statsBeforeRules := session.Stats()
	ruleReport, err := ruleeval.AllWithSession(session, shared)
	if err != nil || ruleReport.Results()[0].State() != ruleeval.StateActive {
		t.Fatalf("session rules = %#v, %v", ruleReport, err)
	}
	if session.Stats().Hits() <= statsBeforeRules.Hits() {
		t.Fatal("Rule did not reuse shared Constraint property evaluation")
	}
	feasible, err := feasibility.EvaluateWithSession(session, shared)
	if err != nil || feasible.State() != feasibility.StateFeasible {
		t.Fatalf("session feasibility = %#v, %v", feasible, err)
	}
}

func TestSessionConcurrentEvaluationsAndTransitionsMatchCold(t *testing.T) {
	f := newPipelineFixture()
	a := f.property(value.Integer, schema.InputComputation(), false)
	b := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(a), bindingInteger(1), nil, domaintime.EffectivePeriod{}, false))
	version := f.commit(t)
	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}

	contexts := []runtime.CalculationContext{
		sessionContext(),
		sessionContext(runtime.NewOverride(a, bindingInteger(2))),
		sessionContext(runtime.NewOverride(a, bindingInteger(3))),
	}
	var wait sync.WaitGroup
	for index := 0; index < 36; index++ {
		context := contexts[index%len(contexts)]
		wait.Add(1)
		go func() {
			defer wait.Done()
			got, err := session.Evaluate(b, context)
			if err != nil {
				t.Error(err)
				return
			}
			cold, err := evaluator.Evaluate(version, b, context)
			if err != nil || !cold.Value().Equal(got.Value()) || !reflect.DeepEqual(cold.Provenance(), got.Provenance()) {
				t.Errorf("concurrent session result differs from cold: %#v %#v %v", cold, got, err)
			}
		}()
	}
	wait.Wait()
}

func TestSessionAggregateAndObservedImpactMatchCold(t *testing.T) {
	f := newPipelineFixture()
	price := f.property(value.Integer, schema.InputComputation(), false)
	typeID := identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(typeID, "links", endpoint, true, endpoint, true, nil))
	f.draft.AddRelation(semantic.NewRelation(identity.New(), typeID, f.entityID, f.entityID, boolean(true), true))
	source := ast.NewTraversalSource(ast.TraversalDirectionOutgoing, typeID)
	total := f.property(value.Integer, schema.DerivedComputation(ast.NewMappedAggregate(
		source,
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, price.DefinitionID()),
		ast.AggregateOperationSum,
	)), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(price), bindingInteger(5), nil, domaintime.EffectivePeriod{}, false))
	version := f.commit(t)

	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}
	base := sessionContext()
	result, err := session.Evaluate(total, base)
	if err != nil || sessionValue(t, result) != 5 {
		t.Fatalf("session aggregate = %#v, %v", result, err)
	}
	before := session.Stats()
	result, err = session.Evaluate(total, base)
	if err != nil || sessionValue(t, result) != 5 || session.Stats().Hits() <= before.Hits() {
		t.Fatalf("warm aggregate = %#v, %v, stats=%#v", result, err, session.Stats())
	}

	changed := sessionContext(runtime.NewOverride(price, bindingInteger(9)))
	result, err = session.Evaluate(total, changed)
	if err != nil || sessionValue(t, result) != 9 {
		t.Fatalf("aggregate override = %#v, %v", result, err)
	}
	coldImpact, err := impact.AnalyzeObserved(version, price, changed)
	if err != nil {
		t.Fatal(err)
	}
	warmImpact, err := impact.AnalyzeObservedWithSession(session, price, changed)
	if err != nil || !reflect.DeepEqual(coldImpact, warmImpact) {
		t.Fatalf("cold/warm observed impact differ: %#v %#v %v", coldImpact, warmImpact, err)
	}
}

func TestSessionScopeTimeAndScenarioTransitionsNeverContaminateCache(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	scopeID := identity.New()
	f.draft.AddScopeDefinition(scope.NewDefinition(scopeID, "region", value.String))
	north := scope.NewSelector(scopeID, value.NewTyped(value.NewString("north")))
	first, _ := datetime.Parse("2026-01-01T00:00:00Z")
	second, _ := datetime.Parse("2026-02-01T00:00:00Z")
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(1), nil, domaintime.NewEffectivePeriod(datetime.DateTime{}, false, second, true), true))
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(2), []scope.Selector{north}, domaintime.EffectivePeriod{}, false))
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(3), nil, domaintime.NewEffectivePeriod(second, true, datetime.DateTime{}, false), true))
	version := f.commit(t)

	scenarioA := domainscenario.New(identity.New(), "A", version.ID(), []domainscenario.Override{domainscenario.NewOverride(input, bindingInteger(4), nil, domaintime.EffectivePeriod{}, false)})
	scenarioB := domainscenario.New(identity.New(), "B", version.ID(), []domainscenario.Override{domainscenario.NewOverride(input, bindingInteger(5), nil, domaintime.EffectivePeriod{}, false)})
	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}

	contexts := []struct {
		context runtime.CalculationContext
		want    int64
	}{
		{runtime.NewCalculationContext(domaintime.At(first), scope.Values{}, nil), 1},
		{runtime.NewCalculationContext(domaintime.At(first), scope.NewValues([]scope.Selector{north}), nil), 2},
		{runtime.NewCalculationContext(domaintime.At(second), scope.Values{}, nil), 3},
		{runtime.NewCalculationContext(domaintime.At(first), scope.Values{}, nil).WithScenario(scenarioA), 4},
		{runtime.NewCalculationContext(domaintime.At(first), scope.Values{}, nil).WithScenario(scenarioB), 5},
		{runtime.NewCalculationContext(domaintime.At(first), scope.Values{}, nil), 1},
	}

	for _, test := range contexts {
		result, err := session.Evaluate(input, test.context)
		if err != nil || sessionValue(t, result) != test.want {
			t.Fatalf("context transition result = %#v, %v; want %d", result, err, test.want)
		}
	}
}

func TestSessionKeepsSeriesOutsidePersistentScalarCache(t *testing.T) {
	f := newPipelineFixture()
	seriesID := identity.New()
	f.ids = append(f.ids, seriesID)
	f.draft.AddPropertyDefinition(schema.NewPropertyDefinition(
		seriesID,
		schema.EntityTypeOwner(f.typeID),
		identity.UUID{},
		false,
		"series",
		value.Integer,
		unit.Unit{},
		false,
		schema.TemporalSeries,
		schema.InputComputation(),
	))
	version := f.commit(t)
	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}
	_, err = session.Evaluate(address.NewPropertyAddress(address.EntityRef(f.entityID), seriesID), reviewContext())
	if err == nil {
		t.Fatal("Session scalar API accepted Series target")
	}
	if session.Stats().Entries() != 0 {
		t.Fatal("unsupported Series evaluation populated scalar cache")
	}
}

func FuzzSessionOverrideTransitionsMatchCold(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{3, 255, 1, 42})
	f.Fuzz(func(t *testing.T, sequence []byte) {
		if len(sequence) == 0 {
			sequence = []byte{0}
		}
		if len(sequence) > 64 {
			sequence = sequence[:64]
		}

		fixture := newPipelineFixture()
		a := fixture.property(value.Integer, schema.InputComputation(), false)
		b := fixture.property(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))), false)
		c := fixture.property(value.Integer, schema.DerivedComputation(ast.Add(ref(b), integer(1))), false)
		fixture.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(a), bindingInteger(1), nil, domaintime.EffectivePeriod{}, false))
		version := fixture.commit(t)
		session, err := evaluator.NewSession(version)
		if err != nil {
			t.Fatal(err)
		}

		for _, item := range sequence {
			valueA := int64(item%11) - 5
			valueB := int64((item/11)%11) - 5
			var context runtime.CalculationContext
			switch item % 4 {
			case 0:
				context = sessionContext()
			case 1:
				context = sessionContext(runtime.NewOverride(a, bindingInteger(valueA)))
			case 2:
				context = sessionContext(runtime.NewOverride(b, bindingInteger(valueB)))
			default:
				context = sessionContext(runtime.NewOverride(a, bindingInteger(valueA)), runtime.NewOverride(b, bindingInteger(valueB)))
			}

			incremental, err := session.Evaluate(c, context)
			if err != nil {
				t.Fatal(err)
			}
			cold, err := evaluator.Evaluate(version, c, context)
			if err != nil || !cold.Value().Equal(incremental.Value()) || !reflect.DeepEqual(cold.Provenance(), incremental.Provenance()) {
				t.Fatalf("sequence item %d produced stale session result: cold=%#v incremental=%#v err=%v", item, cold, incremental, err)
			}
		}
	})
}
