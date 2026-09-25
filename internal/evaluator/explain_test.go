package evaluator_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/series"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func traceContainsKind(node provenance.Node, kind provenance.Kind) bool {
	if node.Kind() == kind {
		return true
	}
	for _, child := range node.Children() {
		if traceContainsKind(child, kind) {
			return true
		}
	}
	return false
}

func traceFindOperation(node provenance.Node, operation string) (provenance.Node, bool) {
	if actual, ok := node.Operation(); ok && actual == operation {
		return node, true
	}
	for _, child := range node.Children() {
		if found, ok := traceFindOperation(child, operation); ok {
			return found, true
		}
	}
	return provenance.Node{}, false
}

func traceReferenceCount(node provenance.Node, target semantic.PropertyAddress) int {
	property, hasProperty := node.Property()
	count := 0
	if hasProperty && node.Kind() == provenance.KindReference && property == target {
		count++
	}
	for _, child := range node.Children() {
		count += traceReferenceCount(child, target)
	}
	return count
}

func requireCompleteTrace(t *testing.T, node provenance.Node) {
	t.Helper()
	if !node.IsValid() {
		t.Fatal("invalid provenance node")
	}
	if resolved, ok := node.Resolved(); !ok || !resolved.IsValid() {
		operation, _ := node.Operation()
		t.Fatalf("provenance %s/%s has no actual result", node.Kind(), operation)
	}
	for _, child := range node.Children() {
		requireCompleteTrace(t, child)
	}
}

func requireInputSource(t *testing.T, explanation evaluator.Explanation, kind provenance.Kind) provenance.Node {
	t.Helper()
	root := explanation.Root()
	if root.Kind() != provenance.KindInputProperty {
		t.Fatalf("root kind = %s, want InputProperty", root.Kind())
	}
	children := root.Children()
	if len(children) != 1 || children[0].Kind() != kind {
		t.Fatalf("input source = %#v, want %s", children, kind)
	}
	return children[0]
}

func TestExplainScalarResolutionAndOverrides(t *testing.T) {
	t.Run("base object binding", func(t *testing.T) {
		f := newPipelineFixture()
		input := f.property(value.Integer, schema.InputComputation(), false)
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(10), nil, domaintime.EffectivePeriod{}, false))
		version := f.commit(t)

		explanation, err := evaluator.Explain(version, input, reviewContext())
		if err != nil || !explanation.IsValid() {
			t.Fatalf("Explain: %#v, %v", explanation, err)
		}
		source := requireInputSource(t, explanation, provenance.KindBaseObjectBinding)
		if binding, ok := source.Binding(); !ok || binding.Target() != semantic.ObjectPropertyTarget(input) {
			t.Fatal("explanation lost the selected object binding")
		}
		if resolved, ok := source.Resolved(); !ok || !resolved.Equal(explanation.Result().Value()) {
			t.Fatal("source binding does not retain its actual resolved value")
		}
		requireCompleteTrace(t, explanation.Root())
	})

	t.Run("base type binding", func(t *testing.T) {
		f := newPipelineFixture()
		input := f.property(value.Integer, schema.InputComputation(), false)
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.TypePropertyTarget(schema.EntityTypeOwner(f.typeID), input.DefinitionID()), bindingInteger(11), nil, domaintime.EffectivePeriod{}, false))
		version := f.commit(t)
		explanation, err := evaluator.Explain(version, input, reviewContext())
		if err != nil {
			t.Fatal(err)
		}
		requireInputSource(t, explanation, provenance.KindBaseTypeBinding)
	})

	t.Run("scenario and calculation precedence", func(t *testing.T) {
		f := newPipelineFixture()
		input := f.property(value.Integer, schema.InputComputation(), false)
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(1), nil, domaintime.EffectivePeriod{}, false))
		version := f.commit(t)
		scenarioOverride := domainscenario.NewOverride(input, bindingInteger(2), nil, domaintime.EffectivePeriod{}, false)
		scenario := domainscenario.New(identity.New(), "explain", version.ID(), []domainscenario.Override{scenarioOverride})

		explanation, err := evaluator.Explain(version, input, reviewContext().WithScenario(scenario))
		if err != nil {
			t.Fatal(err)
		}
		source := requireInputSource(t, explanation, provenance.KindScenarioOverride)
		if selectedScenario, selectedOverride, ok := source.ScenarioOverride(); !ok || selectedScenario.ID() != scenario.ID() || selectedOverride.Target() != input {
			t.Fatal("Scenario explanation lost the selected immutable Scenario override")
		}

		context := runtime.NewCalculationContext(reviewContext().TimeContext(), reviewContext().ScopeValues(), []runtime.Override{runtime.NewOverride(input, bindingInteger(3))}).WithScenario(scenario)
		explanation, err = evaluator.Explain(version, input, context)
		if err != nil {
			t.Fatal(err)
		}
		source = requireInputSource(t, explanation, provenance.KindCalculationOverride)
		if override, ok := source.Override(); !ok || override.Target() != input {
			t.Fatal("Calculation explanation lost the selected override")
		}
		if got := explanation.Result().Value().Value().Raw().Integer().Int64(); got != 3 {
			t.Fatalf("calculation override result = %d, want 3", got)
		}
	})

	t.Run("effective period boundary selects the actual binding", func(t *testing.T) {
		f := newPipelineFixture()
		input := f.property(value.Integer, schema.InputComputation(), false)
		first := semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(1), nil, effectivePeriod(t, "", false, "2026-02-01", true), true)
		second := semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(2), nil, effectivePeriod(t, "2026-02-01", true, "", false), true)
		f.draft.AddValueBinding(first)
		f.draft.AddValueBinding(second)
		version := f.commit(t)
		explanation, err := evaluator.Explain(version, input, seriesContext(t, "2026-02-01"))
		if err != nil {
			t.Fatal(err)
		}
		source := requireInputSource(t, explanation, provenance.KindBaseObjectBinding)
		binding, ok := source.Binding()
		selected, hasSelected := binding.Value().Value()
		if !ok || !hasSelected || !selected.Raw().Integer().IsInt64() || selected.Raw().Integer().Int64() != 2 {
			t.Fatal("explanation did not retain the binding selected at the [from,to) boundary")
		}
	})
}

func TestExplainExpressionPathsOutcomesAndDecisions(t *testing.T) {
	f := newPipelineFixture()
	integerInput := f.property(value.Integer, schema.InputComputation(), false)
	boolInput := f.property(value.Bool, schema.InputComputation(), false)
	nullable := f.property(value.Integer, schema.InputComputation(), true)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(integerInput), bindingInteger(5), nil, domaintime.EffectivePeriod{}, false))
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(nullable), semantic.NullBindingValue(), nil, domaintime.EffectivePeriod{}, false))

	sum := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(integerInput), ref(integerInput))), false)
	andShort := f.property(value.Bool, schema.DerivedComputation(ast.And(boolean(false), ref(boolInput))), false)
	orShort := f.property(value.Bool, schema.DerivedComputation(ast.Or(boolean(true), ref(boolInput))), false)
	ifThen := f.property(value.Integer, schema.DerivedComputation(ast.NewIf(boolean(true), integer(7), ref(integerInput))), false)
	ifElse := f.property(value.Integer, schema.DerivedComputation(ast.NewIf(boolean(false), ref(integerInput), integer(8))), false)
	coalescePrimary := f.property(value.Integer, schema.DerivedComputation(ast.NewCoalesce(integer(9), ref(integerInput))), false)
	coalesceFallback := f.property(value.Integer, schema.DerivedComputation(ast.NewCoalesce(ref(nullable), integer(10))), false)
	isNull := f.property(value.Bool, schema.DerivedComputation(ast.NewIsNull(ref(nullable))), false)
	missingResult := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(integerInput), ref(f.property(value.Integer, schema.InputComputation(), false)))), false)
	division := f.property(value.Decimal, schema.DerivedComputation(ast.Divide(integer(1), integer(0))), false)
	comparison := f.property(value.Bool, schema.DerivedComputation(ast.Greater(integer(2), integer(1))), false)
	notResult := f.property(value.Bool, schema.DerivedComputation(ast.NewNot(boolean(false))), false)
	negated := f.property(value.Integer, schema.DerivedComputation(ast.NewNegate(integer(2))), false)
	strictAnd := f.property(value.Bool, schema.DerivedComputation(ast.And(boolean(true), boolean(false))), false)
	version := f.commit(t)

	assertDecision := func(target semantic.PropertyAddress, operation string, want provenance.Decision, absent semantic.PropertyAddress) {
		t.Helper()
		explanation, err := evaluator.Explain(version, target, reviewContext())
		if err != nil {
			t.Fatal(err)
		}
		op, ok := traceFindOperation(explanation.Root(), operation)
		if !ok || op.Decision() != want {
			t.Fatalf("%s decision = %v, want %v", operation, op.Decision(), want)
		}
		if absent.IsValid() && provenanceContains(explanation.Root(), absent) {
			t.Fatalf("unselected %s dependency appears in explanation", operation)
		}
		requireCompleteTrace(t, explanation.Root())
	}

	explanation, err := evaluator.Explain(version, sum, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if got := traceReferenceCount(explanation.Root(), integerInput); got != 2 {
		t.Fatalf("x+x reference occurrences = %d, want 2", got)
	}
	assertDecision(andShort, "And", provenance.DecisionShortCircuited, boolInput)
	assertDecision(orShort, "Or", provenance.DecisionShortCircuited, boolInput)
	assertDecision(ifThen, "If", provenance.DecisionThen, integerInput)
	assertDecision(ifElse, "If", provenance.DecisionElse, integerInput)
	assertDecision(coalescePrimary, "Coalesce", provenance.DecisionPrimary, integerInput)
	assertDecision(coalesceFallback, "Coalesce", provenance.DecisionFallback, semantic.PropertyAddress{})
	for _, expected := range []struct {
		target    semantic.PropertyAddress
		operation string
	}{
		{comparison, "Greater"},
		{notResult, "Not"},
		{negated, "Negate"},
		{strictAnd, "And"},
	} {
		explanation, err := evaluator.Explain(version, expected.target, reviewContext())
		if err != nil {
			t.Fatal(err)
		}
		node, ok := traceFindOperation(explanation.Root(), expected.operation)
		if !ok || node.Decision() != provenance.DecisionNone {
			t.Fatalf("strict %s operation is absent or has an unexpected decision", expected.operation)
		}
		requireCompleteTrace(t, explanation.Root())
	}

	explanation, err = evaluator.Explain(version, isNull, reviewContext())
	if err != nil || !explanation.Result().Value().Value().Raw().Bool() {
		t.Fatalf("IsNull explanation: %#v, %v", explanation, err)
	}
	if !traceContainsKind(explanation.Root(), provenance.KindBaseObjectBinding) {
		t.Fatal("Null source is absent from IsNull explanation")
	}

	explanation, err = evaluator.Explain(version, missingResult, reviewContext())
	if err != nil || !explanation.Result().Value().IsUndefined() || explanation.Result().Value().UndefinedInfo().Kind() != value.UndefinedDependencyUndefined {
		t.Fatalf("dependency Undefined explanation: %#v, %v", explanation, err)
	}
	if !traceContainsKind(explanation.Root(), provenance.KindMissingValue) {
		t.Fatal("dependency Undefined lost MissingValue root cause")
	}

	explanation, err = evaluator.Explain(version, division, reviewContext())
	if err != nil || !explanation.Result().Value().IsUndefined() || explanation.Result().Value().UndefinedInfo().Kind() != value.UndefinedDivisionByZero {
		t.Fatalf("division explanation: %#v, %v", explanation, err)
	}
	if op, ok := traceFindOperation(explanation.Root(), "Divide"); !ok {
		t.Fatal("division operation is absent")
	} else if resolved, ok := op.Resolved(); !ok || !resolved.IsUndefined() || resolved.UndefinedInfo().Kind() != value.UndefinedDivisionByZero {
		t.Fatal("division operation does not carry its actual Undefined outcome")
	}
}

func TestExplainDerivedOverridesDeterminismOwnershipAndErrors(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	derived := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(input), integer(1))), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(4), nil, domaintime.EffectivePeriod{}, false))
	version := f.commit(t)
	scenarioOverride := domainscenario.NewOverride(derived, bindingInteger(100), nil, domaintime.EffectivePeriod{}, false)
	scenario := domainscenario.New(identity.New(), "derived", version.ID(), []domainscenario.Override{scenarioOverride})

	explanation, err := evaluator.Explain(version, derived, reviewContext().WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if !traceContainsKind(explanation.Root(), provenance.KindScenarioOverride) || provenanceContains(explanation.Root(), input) {
		t.Fatal("derived scenario override did not explain the effective pruned plan")
	}
	calculation := runtime.NewCalculationContext(reviewContext().TimeContext(), reviewContext().ScopeValues(), []runtime.Override{runtime.NewOverride(derived, bindingInteger(101))}).WithScenario(scenario)
	explanation, err = evaluator.Explain(version, derived, calculation)
	if err != nil || !traceContainsKind(explanation.Root(), provenance.KindCalculationOverride) || provenanceContains(explanation.Root(), input) {
		t.Fatalf("derived calculation override explanation: %#v, %v", explanation, err)
	}

	first, err := evaluator.Explain(version, derived, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	second, err := evaluator.Explain(version, derived, reviewContext())
	if err != nil || !reflect.DeepEqual(first.Root(), second.Root()) {
		t.Fatalf("Explain is not deterministic: %v", err)
	}
	children := first.Root().Children()
	children[0] = provenance.Node{}
	if !first.IsValid() || !second.IsValid() || !first.Root().IsValid() {
		t.Fatal("caller mutation of explanation children escaped ownership")
	}

	var wait sync.WaitGroup
	for index := 0; index < 16; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if result, err := evaluator.Explain(version, derived, reviewContext()); err != nil || !result.IsValid() {
				t.Errorf("concurrent Explain: %#v, %v", result, err)
			}
			if _, err := evaluator.Evaluate(version, derived, reviewContext()); err != nil {
				t.Errorf("concurrent Evaluate: %v", err)
			}
		}()
	}
	wait.Wait()

}

func TestExplainSeriesPerSampleAndWholeOutcomes(t *testing.T) {
	f := newPipelineFixture()
	condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	thenBranch := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	elseBranch := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	derived := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.NewIf(ref(condition), ref(thenBranch), ref(elseBranch))), false, unit.Unit{}, false)
	axis := []string{"2026-01-01", "2026-01-02"}
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), pointSeriesBools(t, axis, []bool{true, false}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(thenBranch), pointSeriesInts(t, axis, []int64{10, 20}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(elseBranch), pointSeriesInts(t, axis, []int64{30, 40}), "", false, "", false, nil)
	version := f.commit(t)

	explanation, err := evaluator.ExplainSeries(version, derived, seriesContext(t, "2026-01-15"))
	if err != nil || !explanation.IsValid() {
		t.Fatalf("ExplainSeries: %#v, %v", explanation, err)
	}
	points := explanation.Points()
	if len(points) != 2 || points[0].At().Equal(points[1].At()) {
		t.Fatal("Point explanations lost temporal coordinates")
	}
	first, ok := traceFindOperation(points[0].Root(), "If")
	if !ok || first.Decision() != provenance.DecisionThen || provenanceContains(points[0].Root(), elseBranch) {
		t.Fatal("first Point explanation leaked an unselected else branch")
	}
	second, ok := traceFindOperation(points[1].Root(), "If")
	if !ok || second.Decision() != provenance.DecisionElse || provenanceContains(points[1].Root(), thenBranch) {
		t.Fatal("second Point explanation leaked an unselected then branch")
	}
	for _, point := range points {
		requireCompleteTrace(t, point.Root())
	}
	points[0] = evaluator.ExplainedSeriesPoint{}
	if !explanation.IsValid() || len(explanation.Points()) != 2 {
		t.Fatal("Series explanation leaked its sample backing slice")
	}

	intervalFixture := newPipelineFixture()
	intervalInput := intervalFixture.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	addSeriesBinding(t, intervalFixture.draft, semantic.ObjectPropertyTarget(intervalInput), intervalSeriesInts(t, []string{"2026-01-01"}, []string{"2026-02-01"}, []int64{5}), "", false, "", false, nil)
	intervalVersion := intervalFixture.commit(t)
	intervalExplanation, err := evaluator.ExplainSeries(intervalVersion, intervalInput, seriesContext(t, "2026-01-15"))
	if err != nil || len(intervalExplanation.Intervals()) != 1 {
		t.Fatalf("interval ExplainSeries: %#v, %v", intervalExplanation, err)
	}
	interval := intervalExplanation.Intervals()[0]
	if !interval.From().Equal(periodTime(t, "2026-01-01")) || !interval.To().Equal(periodTime(t, "2026-02-01")) {
		t.Fatal("Interval explanation lost half-open coordinate")
	}
	requireCompleteTrace(t, interval.Root())

	missingFixture := newPipelineFixture()
	missing := missingFixture.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	missingVersion := missingFixture.commit(t)
	missingExplanation, err := evaluator.ExplainSeries(missingVersion, missing, seriesContext(t, "2026-01-15"))
	if err != nil {
		t.Fatal(err)
	}
	info, root, ok := missingExplanation.Undefined()
	if !ok || info.Kind() != value.UndefinedMissingValue || !traceContainsKind(root, provenance.KindMissingValue) {
		t.Fatal("whole-Series MissingValue explanation is incomplete")
	}
	requireCompleteTrace(t, root)

	emptyFixture := newPipelineFixture()
	empty := emptyFixture.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	addSeriesBinding(t, emptyFixture.draft, semantic.ObjectPropertyTarget(empty), series.NewPointSeries(value.Integer, unit.Unit{}, false, nil), "", false, "", false, nil)
	emptyVersion := emptyFixture.commit(t)
	emptyExplanation, err := evaluator.ExplainSeries(emptyVersion, empty, seriesContext(t, "2026-01-15"))
	if err != nil || !emptyExplanation.IsValid() || len(emptyExplanation.Points()) != 0 {
		t.Fatalf("empty Series explanation: %#v, %v", emptyExplanation, err)
	}

	if _, err := evaluator.Explain(version, derived, seriesContext(t, "2026-01-15")); err == nil {
		t.Fatal("scalar Explain accepted a Series target instead of retaining the evaluator engine error")
	}
	if _, err := evaluator.ExplainSeries(version, derived, periodContext(t, "2026-01-01", "2026-02-01")); err == nil {
		t.Fatal("ExplainSeries accepted Period x Series instead of retaining the evaluator engine error")
	}
	scalarFixture := newPipelineFixture()
	scalar := scalarFixture.property(value.Integer, schema.InputComputation(), false)
	scalarFixture.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(scalar), bindingInteger(1), nil, domaintime.EffectivePeriod{}, false))
	scalarVersion := scalarFixture.commit(t)
	if _, err := evaluator.ExplainSeries(scalarVersion, scalar, reviewContext()); err == nil {
		t.Fatal("ExplainSeries accepted a Scalar target instead of retaining the evaluator engine error")
	}
}

func TestExplainDirectNullAndMissing(t *testing.T) {
	f := newPipelineFixture()
	nullable := f.property(value.Integer, schema.InputComputation(), true)
	missing := f.property(value.Integer, schema.InputComputation(), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(nullable), semantic.NullBindingValue(), nil, domaintime.EffectivePeriod{}, false))
	version := f.commit(t)

	nullExplanation, err := evaluator.Explain(version, nullable, reviewContext())
	if err != nil || !nullExplanation.Result().Value().IsNull() {
		t.Fatalf("direct Null explanation: %#v, %v", nullExplanation, err)
	}
	source := requireInputSource(t, nullExplanation, provenance.KindBaseObjectBinding)
	if resolved, ok := source.Resolved(); !ok || !resolved.IsNull() {
		t.Fatal("explicit Null source was not retained in explanation")
	}

	missingExplanation, err := evaluator.Explain(version, missing, reviewContext())
	if err != nil || !missingExplanation.Result().Value().IsUndefined() || missingExplanation.Result().Value().UndefinedInfo().Kind() != value.UndefinedMissingValue {
		t.Fatalf("direct missing explanation: %#v, %v", missingExplanation, err)
	}
	if !traceContainsKind(missingExplanation.Root(), provenance.KindMissingValue) {
		t.Fatal("MissingValue explanation has no MissingValue provenance")
	}
	requireCompleteTrace(t, missingExplanation.Root())
}

func TestExplainSeriesLazyCoordinatesAndOverrides(t *testing.T) {
	t.Run("coordinate-specific lazy path", func(t *testing.T) {
		f := newPipelineFixture()
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		derived := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(left), ref(right))), false, unit.Unit{}, false)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), pointSeriesBools(t, []string{"2026-01-01", "2026-01-02"}, []bool{false, true}), "", false, "", false, nil)
		// Dec-31 shares neither position nor coordinate with the undemanded
		// Jan-01 left sample. Jan-02 is the only demanded right coordinate.
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(right), pointSeriesBools(t, []string{"2025-12-31", "2026-01-02"}, []bool{true, false}), "", false, "", false, nil)
		version := f.commit(t)

		explanation, err := evaluator.ExplainSeries(version, derived, seriesContext(t, "2026-01-15"))
		if err != nil {
			t.Fatal(err)
		}
		points := explanation.Points()
		if len(points) != 2 || points[0].Result().Value().Value().Raw().Bool() || points[1].Result().Value().Value().Raw().Bool() {
			t.Fatal("coordinate-specific lazy evaluation produced wrong values")
		}
		if provenanceContains(points[0].Root(), right) {
			t.Fatal("short-circuited Point explanation includes undemanded right coordinate")
		}
		if !provenanceContains(points[1].Root(), right) {
			t.Fatal("demanded Point explanation omitted its exact right coordinate")
		}
		operation, ok := traceFindOperation(points[0].Root(), "And")
		if !ok || operation.Decision() != provenance.DecisionShortCircuited {
			t.Fatal("short-circuited Point did not retain its decision")
		}
	})

	t.Run("scenario and calculation Series overrides", func(t *testing.T) {
		f := newPipelineFixture()
		input := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
		derived := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Add(ref(input), ref(input))), false, unit.Unit{}, false)
		axis := []string{"2026-01-01"}
		base := pointSeriesInts(t, axis, []int64{1})
		scenarioValue := pointSeriesInts(t, axis, []int64{2})
		calculationValue := pointSeriesInts(t, axis, []int64{3})
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(input), base, "", false, "", false, nil)
		version := f.commit(t)
		scenarioOverride := domainscenario.NewOverride(derived, semantic.SeriesBindingValue(scenarioValue), nil, domaintime.EffectivePeriod{}, false)
		scenario := domainscenario.New(identity.New(), "series-explain", version.ID(), []domainscenario.Override{scenarioOverride})

		explanation, err := evaluator.ExplainSeries(version, derived, seriesContext(t, "2026-01-15").WithScenario(scenario))
		if err != nil {
			t.Fatal(err)
		}
		if !traceContainsKind(explanation.Points()[0].Root(), provenance.KindScenarioOverride) || provenanceContains(explanation.Points()[0].Root(), input) {
			t.Fatal("derived Series Scenario override did not explain pruning")
		}

		context := runtime.NewCalculationContext(seriesContext(t, "2026-01-15").TimeContext(), seriesContext(t, "2026-01-15").ScopeValues(), []runtime.Override{runtime.NewOverride(derived, semantic.SeriesBindingValue(calculationValue))}).WithScenario(scenario)
		explanation, err = evaluator.ExplainSeries(version, derived, context)
		if err != nil {
			t.Fatal(err)
		}
		if !traceContainsKind(explanation.Points()[0].Root(), provenance.KindCalculationOverride) || provenanceContains(explanation.Points()[0].Root(), input) {
			t.Fatal("derived Series Calculation override did not explain pruning")
		}
	})

	t.Run("coalesce per sample", func(t *testing.T) {
		f := newPipelineFixture()
		primary := f.seriesProperty(value.Integer, schema.InputComputation(), true, unit.Unit{}, false)
		fallback := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
		derived := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.NewCoalesce(ref(primary), ref(fallback))), false, unit.Unit{}, false)
		axis := []string{"2026-01-01", "2026-01-02"}
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(primary), pointSeriesSamples(t, axis, []series.Sample{series.Defined(value.NewInt64(1)), series.Null()}, value.Integer, unit.Unit{}, false), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(fallback), pointSeriesInts(t, axis, []int64{2, 3}), "", false, "", false, nil)
		version := f.commit(t)
		explanation, err := evaluator.ExplainSeries(version, derived, seriesContext(t, "2026-01-15"))
		if err != nil {
			t.Fatal(err)
		}
		points := explanation.Points()
		first, ok := traceFindOperation(points[0].Root(), "Coalesce")
		if !ok || first.Decision() != provenance.DecisionPrimary || provenanceContains(points[0].Root(), fallback) {
			t.Fatal("defined primary Point should not explain fallback")
		}
		second, ok := traceFindOperation(points[1].Root(), "Coalesce")
		if !ok || second.Decision() != provenance.DecisionFallback || !provenanceContains(points[1].Root(), fallback) {
			t.Fatal("Null primary Point should explain fallback")
		}
	})
}

func TestExplainAggregateUsesRecordedFilterMapAndApplicabilityTrace(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(5), nil, domaintime.EffectivePeriod{}, false))
	relationTypeID := identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(relationTypeID, "links", endpoint, true, endpoint, true, nil))
	f.draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, f.entityID, f.entityID, boolean(true), true))
	source := ast.NewTraversalSource(ast.TraversalDirectionBoth, relationTypeID)
	root := f.property(value.Integer, schema.DerivedComputation(ast.NewFilteredMappedAggregate(source, boolean(true), ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, input.DefinitionID()), ast.AggregateOperationSum)), false)
	version := f.commit(t)

	raw, err := evaluator.Evaluate(version, root, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	requireCompleteTrace(t, raw.Provenance())
	explanation, err := evaluator.Explain(version, root, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	requireCompleteTrace(t, explanation.Root())
	for _, operation := range []string{"AggregateFilter", "AggregateMap", "RelationApplicability", "AggregateSum"} {
		node, ok := traceFindOperation(explanation.Root(), operation)
		if !ok {
			t.Fatalf("aggregate explanation omitted %s", operation)
		}
		if resolved, ok := node.Resolved(); !ok || !resolved.IsValid() {
			t.Fatalf("aggregate %s did not retain its actual result", operation)
		}
	}
}
