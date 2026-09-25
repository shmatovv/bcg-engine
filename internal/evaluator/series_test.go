package evaluator_test

import (
	"errors"
	"math/big"
	"sync"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	resolvervalue "github.com/shmatovv/bcg-engine/v2/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	scenariovalidation "github.com/shmatovv/bcg-engine/v2/internal/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/v2/internal/types/rational"
	"github.com/shmatovv/bcg-engine/v2/internal/types/series"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func (f *pipelineFixture) seriesProperty(typ value.Type, computation schema.Computation, nullable bool, u unit.Unit, hasUnit bool) address.PropertyAddress {
	id := identity.New()
	contractID := identity.UUID{}
	if nullable {
		contractID = identity.New()
		f.draft.AddPropertyContract(schema.NewPropertyContract(contractID, "nullable-series", typ, unit.Dimension{}, false, schema.TemporalSeries, true))
	}
	f.ids = append(f.ids, id)
	f.draft.AddPropertyDefinition(schema.NewPropertyDefinition(id, schema.EntityTypeOwner(f.typeID), contractID, nullable, "series", typ, u, hasUnit, schema.TemporalSeries, computation))
	return address.NewPropertyAddress(address.EntityRef(f.entityID), id)
}

func seriesContext(t *testing.T, at string) runtime.CalculationContext {
	t.Helper()
	return runtime.NewCalculationContext(domaintime.At(periodTime(t, at)), scope.Values{}, nil)
}

func pointSeriesInts(t *testing.T, dates []string, numbers []int64) series.Series {
	t.Helper()
	if len(dates) != len(numbers) {
		t.Fatal("point Series helper lengths differ")
	}
	points := make([]series.Point, len(dates))
	for index := range dates {
		points[index] = series.NewPoint(periodTime(t, dates[index]), series.Defined(value.NewInt64(numbers[index])))
	}
	return series.NewPointSeries(value.Integer, unit.Unit{}, false, points)
}

func pointSeriesBools(t *testing.T, dates []string, booleans []bool) series.Series {
	t.Helper()
	points := make([]series.Point, len(dates))
	for index := range dates {
		points[index] = series.NewPoint(periodTime(t, dates[index]), series.Defined(value.NewBool(booleans[index])))
	}
	return series.NewPointSeries(value.Bool, unit.Unit{}, false, points)
}

func pointSeriesDecimals(t *testing.T, dates, text []string, u unit.Unit) series.Series {
	t.Helper()
	points := make([]series.Point, len(dates))
	for index := range dates {
		parsed, err := decimal.Parse(text[index])
		if err != nil {
			t.Fatal(err)
		}
		points[index] = series.NewPoint(periodTime(t, dates[index]), series.Defined(value.NewDecimal(parsed)))
	}
	return series.NewPointSeries(value.Decimal, u, u.IsValid(), points)
}

func pointSeriesSamples(t *testing.T, dates []string, samples []series.Sample, elementType value.Type, u unit.Unit, hasUnit bool) series.Series {
	t.Helper()
	points := make([]series.Point, len(dates))
	for index := range dates {
		points[index] = series.NewPoint(periodTime(t, dates[index]), samples[index])
	}
	return series.NewPointSeries(elementType, u, hasUnit, points)
}

func intervalSeriesInts(t *testing.T, from, to []string, values []int64) series.Series {
	t.Helper()
	intervals := make([]series.Interval, len(values))
	for index := range values {
		intervals[index] = series.NewInterval(periodTime(t, from[index]), periodTime(t, to[index]), series.Defined(value.NewInt64(values[index])))
	}
	return series.NewIntervalSeries(value.Integer, unit.Unit{}, false, intervals)
}

func intervalSeriesBools(t *testing.T, from, to []string, values []bool) series.Series {
	t.Helper()
	intervals := make([]series.Interval, len(values))
	for index := range values {
		intervals[index] = series.NewInterval(periodTime(t, from[index]), periodTime(t, to[index]), series.Defined(value.NewBool(values[index])))
	}
	return series.NewIntervalSeries(value.Bool, unit.Unit{}, false, intervals)
}

func seriesValues(t *testing.T, result evaluator.SeriesResult) []value.Resolved {
	t.Helper()
	if !result.IsDefined() {
		t.Fatalf("whole Series undefined: %v", result)
	}
	if result.AxisKind() != series.AxisPoint {
		t.Fatalf("expected Point Series, got %s", result.AxisKind())
	}
	points := result.Points()
	values := make([]value.Resolved, len(points))
	for index := range points {
		values[index] = points[index].Result().Value()
	}
	return values
}

func requireSeriesIntegers(t *testing.T, result evaluator.SeriesResult, want ...int64) {
	t.Helper()
	got := seriesValues(t, result)
	if len(got) != len(want) {
		t.Fatalf("got %d samples, want %d", len(got), len(want))
	}
	for index, expected := range want {
		if !got[index].IsDefined() || got[index].Value().Type() != value.Integer || got[index].Value().Raw().Integer().Int64() != expected {
			t.Fatalf("sample %d = %v, want Integer(%d)", index, got[index], expected)
		}
	}
}

func addSeriesBinding(t *testing.T, draft *model.Draft, target semantic.ValueTarget, stored series.Series, from string, hasFrom bool, to string, hasTo bool, selectors []scope.Selector) {
	t.Helper()
	draft.AddValueBinding(semantic.NewValueBinding(target, semantic.SeriesBindingValue(stored), selectors, effectivePeriod(t, from, hasFrom, to, hasTo), hasFrom || hasTo))
}

func TestEvaluateSeriesInputResolutionAndOwnership(t *testing.T) {
	f := newPipelineFixture()
	input := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	first := pointSeriesInts(t, []string{"2026-01-01", "2026-01-02"}, []int64{10, 20})
	second := pointSeriesInts(t, []string{"2026-01-01", "2026-01-02"}, []int64{30, 40})
	addSeriesBinding(t, f.draft, semantic.TypePropertyTarget(schema.EntityTypeOwner(f.typeID), input.DefinitionID()), second, "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(input), first, "", false, "2026-02-01", true, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(input), second, "2026-02-01", true, "", false, nil)
	v := f.commit(t)

	result, err := evaluator.EvaluateSeries(v, input, seriesContext(t, "2026-01-15"))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 10, 20)
	if !provenanceHas(result.Points()[0].Result().Provenance(), provenance.KindBaseObjectBinding) {
		t.Fatal("direct input Series lost base-binding provenance")
	}
	result, err = evaluator.EvaluateSeries(v, input, seriesContext(t, "2026-02-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 30, 40)

	points := result.Points()
	points[0] = evaluator.SeriesPoint{}
	if !result.IsValid() || len(result.Points()) != 2 {
		t.Fatal("Series result exposed mutable samples")
	}
	if _, err := evaluator.Evaluate(v, input, seriesContext(t, "2026-01-15")); err == nil {
		t.Fatal("scalar evaluator accepted a Series target")
	}
}

func TestEvaluateSeriesDerivedOperationsAxesAndOutcomes(t *testing.T) {
	f := newPipelineFixture()
	a := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	b := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	sum := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Add(ref(a), ref(b))), false, unit.Unit{}, false)
	difference := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Subtract(ref(a), ref(b))), false, unit.Unit{}, false)
	product := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Multiply(ref(a), ref(b))), false, unit.Unit{}, false)
	quotient := f.seriesProperty(value.Decimal, schema.DerivedComputation(ast.Divide(ref(a), ref(b))), false, unit.Unit{}, false)
	dates := []string{"2026-01-01", "2026-01-02"}
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(a), pointSeriesInts(t, dates, []int64{10, 20}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(b), pointSeriesInts(t, dates, []int64{2, 0}), "", false, "", false, nil)
	v := f.commit(t)

	result, err := evaluator.EvaluateSeries(v, sum, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 12, 20)
	result, err = evaluator.EvaluateSeries(v, difference, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 8, 20)
	result, err = evaluator.EvaluateSeries(v, product, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 20, 0)
	if !provenanceHas(result.Points()[0].Result().Provenance(), provenance.KindReference) {
		t.Fatal("derived Series reference provenance missing")
	}
	division, err := evaluator.EvaluateSeries(v, quotient, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	values := seriesValues(t, division)
	if !values[0].IsDefined() || values[1].UndefinedInfo().Kind() != value.UndefinedDivisionByZero {
		t.Fatalf("division outcomes = %v", values)
	}

	mismatch := pointSeriesInts(t, []string{"2026-01-01", "2026-01-03"}, []int64{2, 3})
	context := runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-01-01")), scope.Values{}, []runtime.Override{runtime.NewOverride(b, semantic.SeriesBindingValue(mismatch))})
	_, err = evaluator.EvaluateSeries(v, sum, context)
	var evaluationError evaluator.Error
	if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorIncompatibleSeriesAxis {
		t.Fatalf("wrong incompatible axis error: %v", err)
	}
	interval := intervalSeriesInts(t, []string{"2026-01-01"}, []string{"2026-01-02"}, []int64{2})
	context = runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-01-01")), scope.Values{}, []runtime.Override{runtime.NewOverride(b, semantic.SeriesBindingValue(interval))})
	_, err = evaluator.EvaluateSeries(v, sum, context)
	if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorIncompatibleSeriesAxis {
		t.Fatalf("wrong point/interval axis error: %v", err)
	}
}

func TestEvaluateSeriesOverridesAndSeriesIfLaziness(t *testing.T) {
	f := newPipelineFixture()
	condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	a := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	b := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	chosen := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.NewIf(ref(condition), ref(a), ref(b))), false, unit.Unit{}, false)
	dates := []string{"2026-01-01", "2026-01-02"}
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(a), pointSeriesInts(t, dates, []int64{1, 2}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(b), pointSeriesInts(t, dates, []int64{3, 4}), "", false, "", false, nil)
	v := f.commit(t)

	override := domainscenario.NewOverride(chosen, semantic.SeriesBindingValue(pointSeriesInts(t, dates, []int64{9, 9})), nil, effectivePeriod(t, "2026-02-01", true, "2026-03-01", true), true)
	scenario := domainscenario.New(identity.New(), "series", v.ID(), []domainscenario.Override{override})
	result, err := evaluator.EvaluateSeries(v, chosen, seriesContext(t, "2026-01-01").WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 1, 4)
	if provenanceHas(result.Points()[0].Result().Provenance(), b) {
		t.Fatal("unselected Series IF branch appeared in sample provenance")
	}
	result, err = evaluator.EvaluateSeries(v, chosen, seriesContext(t, "2026-02-01").WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 9, 9)
	if provenanceHas(result.Points()[0].Result().Provenance(), a) || !provenanceHas(result.Points()[0].Result().Provenance(), provenance.KindScenarioOverride) {
		t.Fatal("derived Series Scenario override did not prune dependencies")
	}
	result, err = evaluator.EvaluateSeries(v, chosen, seriesContext(t, "2026-03-01").WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 1, 4)

	context := runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-02-01")), scope.Values{}, []runtime.Override{runtime.NewOverride(chosen, semantic.SeriesBindingValue(pointSeriesInts(t, dates, []int64{7, 7})))}).WithScenario(scenario)
	result, err = evaluator.EvaluateSeries(v, chosen, context)
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 7, 7)
}

func TestEvaluateSeriesAmbiguityPeriodAndConcurrency(t *testing.T) {
	f := newPipelineFixture()
	input := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	stored := pointSeriesInts(t, []string{"2026-01-01"}, []int64{1})
	v := f.commit(t)
	override := domainscenario.NewOverride(input, semantic.SeriesBindingValue(stored), nil, domaintime.EffectivePeriod{}, false)
	scenario := domainscenario.New(identity.New(), "ambiguous", v.ID(), []domainscenario.Override{override, override})
	result, err := evaluator.EvaluateSeries(v, input, seriesContext(t, "2026-01-01").WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsUndefined() {
		t.Fatal("ambiguous Series Scenario was not whole-Series Undefined")
	}
	if info, ok := result.UndefinedInfo(); !ok || info.Kind() != value.UndefinedAmbiguousValue {
		t.Fatal("wrong ambiguous Series Undefined reason")
	}
	periodContext := runtime.NewCalculationContext(domaintime.Period(periodTime(t, "2026-01-01"), periodTime(t, "2026-02-01")), scope.Values{}, nil)
	_, err = evaluator.EvaluateSeries(v, input, periodContext)
	var evaluationError evaluator.Error
	if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorUnsupportedTimeContext {
		t.Fatalf("wrong Period x Series error: %v", err)
	}

	base := runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-01-01")), scope.Values{}, []runtime.Override{runtime.NewOverride(input, semantic.SeriesBindingValue(stored))})
	var wg sync.WaitGroup
	for index := 0; index < 12; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := evaluator.EvaluateSeries(v, input, base)
			if err != nil || !got.IsDefined() {
				t.Errorf("concurrent Series evaluation: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestEvaluateSeriesIntervalScopeAndMissing(t *testing.T) {
	f := newPipelineFixture()
	input := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	scopeID := identity.New()
	f.draft.AddScopeDefinition(scope.NewDefinition(scopeID, "region", value.String))
	fallback := intervalSeriesInts(t, []string{"2026-01-01"}, []string{"2026-02-01"}, []int64{1})
	eu := intervalSeriesInts(t, []string{"2026-01-01"}, []string{"2026-02-01"}, []int64{2})
	selector := scope.NewSelector(scopeID, value.NewTyped(value.NewString("EU")))
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(input), fallback, "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(input), eu, "", false, "", false, []scope.Selector{selector})
	v := f.commit(t)
	context := runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-01-01")), scope.NewValues([]scope.Selector{selector}), nil)
	result, err := evaluator.EvaluateSeries(v, input, context)
	if err != nil {
		t.Fatal(err)
	}
	intervals := result.Intervals()
	if len(intervals) != 1 || !intervals[0].Result().Value().IsDefined() || intervals[0].Result().Value().Value().Raw().Integer().Int64() != 2 {
		t.Fatalf("scope-selected Interval Series = %#v", result)
	}

	// Use a separate committed model for the MissingValue case rather than
	// mutating the model snapshot used above.
	missingFixture := newPipelineFixture()
	missingInput := missingFixture.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	missingVersion := missingFixture.commit(t)
	missingResult, err := evaluator.EvaluateSeries(missingVersion, missingInput, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	if !missingResult.IsUndefined() {
		t.Fatal("missing effective Series did not produce whole-Series Undefined")
	}
	if info, ok := missingResult.UndefinedInfo(); !ok || info.Kind() != value.UndefinedMissingValue {
		t.Fatal("wrong missing Series reason")
	}
}

func TestEvaluateSeriesElementOperationsAndLaziness(t *testing.T) {
	f := newPipelineFixture()
	dates := []string{"2026-01-01", "2026-01-02"}
	ints := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	decimals := f.seriesProperty(value.Decimal, schema.InputComputation(), false, unit.Unit{}, false)
	nullable := f.seriesProperty(value.Integer, schema.InputComputation(), true, unit.Unit{}, false)
	falseValues := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	trueValues := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	missingBools := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	add := f.seriesProperty(value.Decimal, schema.DerivedComputation(ast.Add(ref(ints), ref(decimals))), false, unit.Unit{}, false)
	comparison := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.Greater(ref(ints), ref(decimals))), false, unit.Unit{}, false)
	strict := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Add(ref(nullable), ref(ints))), true, unit.Unit{}, false)
	negated := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.NewNegate(ref(ints))), false, unit.Unit{}, false)
	not := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewNot(ref(falseValues))), false, unit.Unit{}, false)
	isNull := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIsNull(ref(nullable))), false, unit.Unit{}, false)
	and := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(falseValues), ref(missingBools))), false, unit.Unit{}, false)
	or := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.Or(ref(trueValues), ref(missingBools))), false, unit.Unit{}, false)
	coalesce := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.NewCoalesce(ref(nullable), ref(ints))), true, unit.Unit{}, false)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(ints), pointSeriesInts(t, dates, []int64{1, 2}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(decimals), pointSeriesDecimals(t, dates, []string{"0.5", "1.5"}, unit.Unit{}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(nullable), pointSeriesSamples(t, dates, []series.Sample{series.Defined(value.NewInt64(8)), series.Null()}, value.Integer, unit.Unit{}, false), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(falseValues), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(trueValues), pointSeriesBools(t, dates, []bool{true, true}), "", false, "", false, nil)
	v := f.commit(t)

	result, err := evaluator.EvaluateSeries(v, add, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	values := seriesValues(t, result)
	if values[0].Value().Type() != value.Decimal || values[0].Value().Raw().Decimal().String() != "1.5" || values[1].Value().Raw().Decimal().String() != "3.5" {
		t.Fatalf("integer/decimal Series promotion = %v", values)
	}
	result, err = evaluator.EvaluateSeries(v, comparison, seriesContext(t, "2026-01-01"))
	if err != nil || !seriesValues(t, result)[0].Value().Raw().Bool() || !seriesValues(t, result)[1].Value().Raw().Bool() {
		t.Fatalf("Series comparison: %v", err)
	}
	result, err = evaluator.EvaluateSeries(v, strict, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	strictValues := seriesValues(t, result)
	if !strictValues[0].IsDefined() || strictValues[0].Value().Raw().Integer().Int64() != 9 || !strictValues[1].IsNull() {
		t.Fatalf("Series strict Null propagation = %v", strictValues)
	}
	result, err = evaluator.EvaluateSeries(v, negated, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, -1, -2)
	result, err = evaluator.EvaluateSeries(v, not, seriesContext(t, "2026-01-01"))
	if err != nil || !seriesValues(t, result)[0].Value().Raw().Bool() || !seriesValues(t, result)[1].Value().Raw().Bool() {
		t.Fatalf("Series NOT: %v", err)
	}
	result, err = evaluator.EvaluateSeries(v, isNull, seriesContext(t, "2026-01-01"))
	if err != nil || seriesValues(t, result)[0].Value().Raw().Bool() || !seriesValues(t, result)[1].Value().Raw().Bool() {
		t.Fatalf("Series IsNull: %v", err)
	}
	result, err = evaluator.EvaluateSeries(v, and, seriesContext(t, "2026-01-01"))
	if err != nil || seriesValues(t, result)[0].Value().Raw().Bool() || provenanceHas(result.Points()[0].Result().Provenance(), missingBools) {
		t.Fatalf("Series AND did not short-circuit: %v", err)
	}
	result, err = evaluator.EvaluateSeries(v, or, seriesContext(t, "2026-01-01"))
	if err != nil || !seriesValues(t, result)[0].Value().Raw().Bool() || provenanceHas(result.Points()[0].Result().Provenance(), missingBools) {
		t.Fatalf("Series OR did not short-circuit: %v", err)
	}
	result, err = evaluator.EvaluateSeries(v, coalesce, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 8, 2)
	if provenanceHas(result.Points()[0].Result().Provenance(), ints) || !provenanceHas(result.Points()[1].Result().Provenance(), ints) {
		t.Fatal("Series Coalesce did not preserve sample-level laziness")
	}
}

func TestEvaluateSeriesIntervalAndEmptyAxisCompatibility(t *testing.T) {
	f := newPipelineFixture()
	a := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	b := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	sum := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Add(ref(a), ref(b))), false, unit.Unit{}, false)
	from := []string{"2026-01-01", "2026-02-01"}
	to := []string{"2026-02-01", "2026-03-01"}
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(a), intervalSeriesInts(t, from, to, []int64{1, 2}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(b), intervalSeriesInts(t, from, to, []int64{3, 4}), "", false, "", false, nil)
	v := f.commit(t)
	result, err := evaluator.EvaluateSeries(v, sum, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	intervals := result.Intervals()
	if len(intervals) != 2 || intervals[0].Result().Value().Value().Raw().Integer().Int64() != 4 || intervals[1].Result().Value().Value().Raw().Integer().Int64() != 6 {
		t.Fatalf("matching Interval Series result = %#v", result)
	}
	mismatchedIntervals := intervalSeriesInts(t, []string{"2026-01-01", "2026-02-02"}, []string{"2026-02-02", "2026-03-01"}, []int64{3, 4})
	context := runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-01-01")), scope.Values{}, []runtime.Override{runtime.NewOverride(b, semantic.SeriesBindingValue(mismatchedIntervals))})
	_, err = evaluator.EvaluateSeries(v, sum, context)
	var evaluationError evaluator.Error
	if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorIncompatibleSeriesAxis {
		t.Fatalf("different Interval axes = %v", err)
	}

	emptyFixture := newPipelineFixture()
	emptyA := emptyFixture.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	emptyB := emptyFixture.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	emptySum := emptyFixture.seriesProperty(value.Integer, schema.DerivedComputation(ast.Add(ref(emptyA), ref(emptyB))), false, unit.Unit{}, false)
	empty := series.NewPointSeries(value.Integer, unit.Unit{}, false, nil)
	addSeriesBinding(t, emptyFixture.draft, semantic.ObjectPropertyTarget(emptyA), empty, "", false, "", false, nil)
	addSeriesBinding(t, emptyFixture.draft, semantic.ObjectPropertyTarget(emptyB), empty, "", false, "", false, nil)
	emptyVersion := emptyFixture.commit(t)
	emptyResult, err := evaluator.EvaluateSeries(emptyVersion, emptySum, seriesContext(t, "2026-01-01"))
	if err != nil || !emptyResult.IsDefined() || len(emptyResult.Points()) != 0 {
		t.Fatalf("matching empty Series result = %#v, %v", emptyResult, err)
	}
	nonEmpty := pointSeriesInts(t, []string{"2026-01-01"}, []int64{1})
	context = runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-01-01")), scope.Values{}, []runtime.Override{runtime.NewOverride(emptyB, semantic.SeriesBindingValue(nonEmpty))})
	_, err = evaluator.EvaluateSeries(emptyVersion, emptySum, context)
	if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorIncompatibleSeriesAxis {
		t.Fatalf("empty/non-empty axis mismatch = %v", err)
	}
}

func TestEvaluateSeriesCompatibleUnitNormalizationAndWholeIf(t *testing.T) {
	meter := unit.NewUnit("meter", unit.Base("length"), "length", rational.One())
	centimeter := unit.NewUnit("centimeter", unit.Base("length"), "length", rational.New(big.NewInt(1), big.NewInt(100)))
	f := newPipelineFixture()
	a := f.seriesProperty(value.Decimal, schema.InputComputation(), false, meter, true)
	b := f.seriesProperty(value.Decimal, schema.InputComputation(), false, centimeter, true)
	output := f.seriesProperty(value.Decimal, schema.DerivedComputation(ast.Add(ref(a), ref(b))), false, meter, true)
	scalarCondition := f.property(value.Bool, schema.InputComputation(), false)
	selected := f.seriesProperty(value.Decimal, schema.DerivedComputation(ast.NewIf(ref(scalarCondition), ref(a), ref(b))), false, meter, true)
	unbound := f.seriesProperty(value.Decimal, schema.InputComputation(), false, meter, true)
	lazySelected := f.seriesProperty(value.Decimal, schema.DerivedComputation(ast.NewIf(ref(scalarCondition), ref(a), ref(unbound))), false, meter, true)
	dates := []string{"2026-01-01"}
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(a), pointSeriesDecimals(t, dates, []string{"1"}, meter), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(b), pointSeriesDecimals(t, dates, []string{"50"}, centimeter), "", false, "", false, nil)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(scalarCondition), semantic.DefinedBindingValue(value.NewTyped(value.NewBool(true))), nil, domaintime.EffectivePeriod{}, false))
	v := f.commit(t)
	result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	if got := seriesValues(t, result)[0].Value(); got.Raw().Decimal().String() != "1.5" || !got.Unit().Equal(meter) {
		t.Fatalf("Series unit normalization = %v", got)
	}
	result, err = evaluator.EvaluateSeries(v, selected, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	if got := seriesValues(t, result)[0].Value(); got.Raw().Decimal().String() != "1" || !got.Unit().Equal(meter) {
		t.Fatalf("whole-Series IF did not select/normalize branch: %v", got)
	}
	result, err = evaluator.EvaluateSeries(v, lazySelected, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	if got := seriesValues(t, result)[0].Value(); got.Raw().Decimal().String() != "1" || provenanceHas(result.Points()[0].Result().Provenance(), unbound) {
		t.Fatal("unselected whole-Series IF branch was evaluated")
	}
}

func TestEvaluateSeriesOverrideValidationAndOwnership(t *testing.T) {
	f := newPipelineFixture()
	input := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	dates := []string{"2026-01-01"}
	base := pointSeriesInts(t, dates, []int64{1})
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(input), base, "", false, "", false, nil)
	v := f.commit(t)

	badScenario := domainscenario.New(identity.New(), "bad-series", v.ID(), []domainscenario.Override{
		domainscenario.NewOverride(input, semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(3))), nil, domaintime.EffectivePeriod{}, false),
	})
	if report := scenariovalidation.Validate(v, badScenario); !report.Has(scenariovalidation.ValidationIssueTemporalValueMismatch) {
		t.Fatalf("scalar Scenario override on Series property was accepted: %v", report)
	}
	badContext := runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-01-01")), scope.Values{}, []runtime.Override{
		runtime.NewOverride(input, semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(3)))),
	})
	if err := resolvervalue.ValidateContext(v, badContext); err == nil {
		t.Fatal("scalar calculation override on Series property was accepted")
	}

	scenarioOverrides := []domainscenario.Override{
		domainscenario.NewOverride(input, semantic.SeriesBindingValue(pointSeriesInts(t, dates, []int64{9})), nil, domaintime.EffectivePeriod{}, false),
	}
	scenario := domainscenario.New(identity.New(), "owned-series", v.ID(), scenarioOverrides)
	scenarioOverrides[0] = domainscenario.Override{}
	returnedScenarioOverrides := scenario.Overrides()
	returnedScenarioOverrides[0] = domainscenario.Override{}
	result, err := evaluator.EvaluateSeries(v, input, seriesContext(t, "2026-01-01").WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 9)

	runtimeOverrides := []runtime.Override{
		runtime.NewOverride(input, semantic.SeriesBindingValue(pointSeriesInts(t, dates, []int64{7}))),
	}
	context := runtime.NewCalculationContext(domaintime.At(periodTime(t, "2026-01-01")), scope.Values{}, runtimeOverrides)
	runtimeOverrides[0] = runtime.Override{}
	returnedRuntimeOverrides := context.Overrides()
	returnedRuntimeOverrides[0] = runtime.Override{}
	result, err = evaluator.EvaluateSeries(v, input, context.WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 7)
}

func TestEvaluateSeriesSampleDemandLaziness(t *testing.T) {
	t.Run("AND skips nested incompatible path and preserves provenance", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		innerCondition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		incompatibleThen := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		coalescePrimary := f.seriesProperty(value.Bool, schema.InputComputation(), true, unit.Unit{}, false)
		coalesceFallback := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(innerCondition), ref(incompatibleThen), ast.NewCoalesce(ref(coalescePrimary), ref(coalesceFallback)))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(left), ref(right))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), pointSeriesBools(t, dates, []bool{false, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(innerCondition), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(incompatibleThen), pointSeriesBools(t, []string{"2026-01-01", "2026-01-03"}, []bool{true, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(coalescePrimary), pointSeriesSamples(t, dates, []series.Sample{series.Defined(value.NewBool(true)), series.Null()}, value.Bool, unit.Unit{}, false), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(coalesceFallback), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		values := seriesValues(t, result)
		for index, want := range []bool{false, false} {
			if !values[index].IsDefined() || values[index].Value().Raw().Bool() != want {
				t.Fatalf("sample %d = %v, want Bool(%v)", index, values[index], want)
			}
		}
		if provenanceHas(result.Points()[0].Result().Provenance(), right) || provenanceHas(result.Points()[0].Result().Provenance(), incompatibleThen) || provenanceHas(result.Points()[1].Result().Provenance(), incompatibleThen) {
			t.Fatal("AND result contains an unselected Series dependency in provenance")
		}
	})

	t.Run("OR skips nested incompatible path", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		innerCondition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		incompatibleThen := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		compatibleElse := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(innerCondition), ref(incompatibleThen), ref(compatibleElse))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.Or(ref(left), ref(right))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(innerCondition), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(incompatibleThen), pointSeriesBools(t, []string{"2026-01-01", "2026-01-03"}, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(compatibleElse), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		values := seriesValues(t, result)
		for index, want := range []bool{true, false} {
			if !values[index].IsDefined() || values[index].Value().Raw().Bool() != want {
				t.Fatalf("sample %d = %v, want Bool(%v)", index, values[index], want)
			}
		}
		if provenanceHas(result.Points()[0].Result().Provenance(), incompatibleThen) {
			t.Fatal("OR result contains an unselected Series dependency in provenance")
		}
	})

	t.Run("IF evaluates only each sample selected branch", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		innerCondition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		incompatibleThen := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		compatibleElse := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		outerElse := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		thenBranch := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(innerCondition), ref(incompatibleThen), ref(compatibleElse))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(condition), ref(thenBranch), ref(outerElse))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), pointSeriesBools(t, dates, []bool{false, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(innerCondition), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(incompatibleThen), pointSeriesBools(t, []string{"2026-01-01", "2026-01-03"}, []bool{true, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(compatibleElse), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(outerElse), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		for index, resolved := range seriesValues(t, result) {
			if !resolved.IsDefined() || resolved.Value().Raw().Bool() {
				t.Fatalf("sample %d = %v, want Bool(false)", index, resolved)
			}
		}
		if provenanceHas(result.Points()[0].Result().Provenance(), thenBranch) || provenanceHas(result.Points()[1].Result().Provenance(), incompatibleThen) {
			t.Fatal("IF result contains an unselected Series dependency in provenance")
		}
	})

	t.Run("Coalesce evaluates fallback only for Null samples", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		primary := f.seriesProperty(value.Bool, schema.InputComputation(), true, unit.Unit{}, false)
		innerCondition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		incompatibleThen := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		compatibleElse := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		fallback := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(innerCondition), ref(incompatibleThen), ref(compatibleElse))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewCoalesce(ref(primary), ref(fallback))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(primary), pointSeriesSamples(t, dates, []series.Sample{series.Defined(value.NewBool(true)), series.Null()}, value.Bool, unit.Unit{}, false), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(innerCondition), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(incompatibleThen), pointSeriesBools(t, []string{"2026-01-01", "2026-01-03"}, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(compatibleElse), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		values := seriesValues(t, result)
		if !values[0].IsDefined() || !values[0].Value().Raw().Bool() || !values[1].IsDefined() || values[1].Value().Raw().Bool() {
			t.Fatalf("Coalesce values = %v", values)
		}
		if provenanceHas(result.Points()[0].Result().Provenance(), fallback) || provenanceHas(result.Points()[1].Result().Provenance(), incompatibleThen) {
			t.Fatal("Coalesce result contains an unselected Series dependency in provenance")
		}
	})

	t.Run("required incompatible path still fails", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		innerCondition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		incompatibleThen := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		compatibleElse := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(innerCondition), ref(incompatibleThen), ref(compatibleElse))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(left), ref(right))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(innerCondition), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(incompatibleThen), pointSeriesBools(t, []string{"2025-12-31", "2026-01-03"}, []bool{true, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(compatibleElse), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		_, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		var evaluationError evaluator.Error
		if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorIncompatibleSeriesAxis {
			t.Fatalf("required incompatible path error = %v", err)
		}
	})
}

func TestEvaluateSeriesSampleDemandNullAndUndefined(t *testing.T) {
	f := newPipelineFixture()
	dates := []string{"2026-01-01", "2026-01-02"}
	falseLeft := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	trueLeft := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	primary := f.seriesProperty(value.Bool, schema.InputComputation(), true, unit.Unit{}, false)
	otherwise := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	missing := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	and := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(falseLeft), ref(missing))), false, unit.Unit{}, false)
	or := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.Or(ref(trueLeft), ref(missing))), false, unit.Unit{}, false)
	selected := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(condition), ref(missing), ref(otherwise))), false, unit.Unit{}, false)
	coalesced := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewCoalesce(ref(primary), ref(missing))), false, unit.Unit{}, false)

	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(falseLeft), pointSeriesBools(t, dates, []bool{false, true}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(trueLeft), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), pointSeriesBools(t, dates, []bool{false, true}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(primary), pointSeriesSamples(t, dates, []series.Sample{series.Defined(value.NewBool(true)), series.Null()}, value.Bool, unit.Unit{}, false), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(otherwise), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
	v := f.commit(t)

	for _, test := range []struct {
		name   string
		target address.PropertyAddress
		first  bool
	}{
		{name: "AND", target: and, first: false},
		{name: "OR", target: or, first: true},
		{name: "If", target: selected, first: false},
		{name: "Coalesce", target: coalesced, first: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := evaluator.EvaluateSeries(v, test.target, seriesContext(t, "2026-01-01"))
			if err != nil {
				t.Fatal(err)
			}
			values := seriesValues(t, result)
			if !values[0].IsDefined() || values[0].Value().Raw().Bool() != test.first {
				t.Fatalf("first sample = %v, want Bool(%v)", values[0], test.first)
			}
			if !values[1].IsUndefined() || values[1].UndefinedInfo().Kind() != value.UndefinedDependencyUndefined {
				t.Fatalf("second sample = %v, want Undefined(DependencyUndefined)", values[1])
			}
			if provenanceHas(result.Points()[0].Result().Provenance(), missing) {
				t.Fatal("short-circuited sample contains missing dependency provenance")
			}
		})
	}
}

func TestResolveSeriesObjectFallbackEffectiveBoundaryAndNestedDerived(t *testing.T) {
	f := newPipelineFixture()
	input := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
	typeValue := pointSeriesInts(t, []string{"2026-01-01"}, []int64{7})
	objectValue := pointSeriesInts(t, []string{"2026-01-01"}, []int64{3})
	addSeriesBinding(t, f.draft, semantic.TypePropertyTarget(schema.EntityTypeOwner(f.typeID), input.DefinitionID()), typeValue, "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(input), objectValue, "2026-01-01", true, "2026-02-01", true, nil)
	middle := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Add(ref(input), ref(input))), false, unit.Unit{}, false)
	output := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Subtract(ref(middle), ref(input))), false, unit.Unit{}, false)
	v := f.commit(t)

	atStart := seriesContext(t, "2026-01-01")
	resolved, err := resolvervalue.ResolveSeries(v, input, atStart)
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := resolved.Series()
	if !ok {
		t.Fatal("object Series binding was not selected at inclusive start")
	}
	sample, ok := stored.Points()[0].Sample().Value()
	if !ok || sample.Integer().Int64() != 3 {
		t.Fatalf("object binding value = %v, want 3", sample)
	}
	result, err := evaluator.EvaluateSeries(v, output, atStart)
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 3)

	atEnd := seriesContext(t, "2026-02-01")
	resolved, err = resolvervalue.ResolveSeries(v, input, atEnd)
	if err != nil {
		t.Fatal(err)
	}
	stored, ok = resolved.Series()
	if !ok {
		t.Fatal("type Series binding was not selected at exclusive object-binding end")
	}
	sample, ok = stored.Points()[0].Sample().Value()
	if !ok || sample.Integer().Int64() != 7 {
		t.Fatalf("type fallback value = %v, want 7", sample)
	}
	result, err = evaluator.EvaluateSeries(v, output, atEnd)
	if err != nil {
		t.Fatal(err)
	}
	requireSeriesIntegers(t, result, 7)
}

func TestEvaluateSeriesIntervalSampleDemandLaziness(t *testing.T) {
	f := newPipelineFixture()
	from := []string{"2026-01-01", "2026-02-01"}
	to := []string{"2026-02-01", "2026-03-01"}
	left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	innerCondition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	incompatibleThen := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	compatibleElse := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
	right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(innerCondition), ref(incompatibleThen), ref(compatibleElse))), false, unit.Unit{}, false)
	output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(left), ref(right))), false, unit.Unit{}, false)

	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), intervalSeriesBools(t, from, to, []bool{false, true}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(innerCondition), intervalSeriesBools(t, from, to, []bool{true, false}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(incompatibleThen), intervalSeriesBools(t, []string{"2026-01-01", "2026-03-01"}, []string{"2026-02-01", "2026-04-01"}, []bool{true, true}), "", false, "", false, nil)
	addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(compatibleElse), intervalSeriesBools(t, from, to, []bool{false, false}), "", false, "", false, nil)
	v := f.commit(t)

	result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	intervals := result.Intervals()
	if len(intervals) != 2 {
		t.Fatalf("interval count = %d, want 2", len(intervals))
	}
	for index, interval := range intervals {
		resolved := interval.Result().Value()
		if !resolved.IsDefined() || resolved.Value().Raw().Bool() {
			t.Fatalf("interval %d = %v, want Bool(false)", index, resolved)
		}
	}
}

func TestEvaluateSeriesPartialTemporalDemand(t *testing.T) {
	t.Run("AND and nested If ignore an unused Point coordinate", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		thenBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		elseBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(condition), ref(thenBranch), ref(elseBranch))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(left), ref(right))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), pointSeriesBools(t, dates, []bool{false, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(thenBranch), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		// Dec-31 is not demanded: false AND right short-circuits Jan-01. The
		// selected Jan-02 sample must still be matched by its timestamp, not index.
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(elseBranch), pointSeriesBools(t, []string{"2025-12-31", "2026-01-02"}, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		values := seriesValues(t, result)
		if len(values) != 2 || !values[0].IsDefined() || values[0].Value().Raw().Bool() || !values[1].IsDefined() || values[1].Value().Raw().Bool() {
			t.Fatalf("AND output = %v, want [false false]", values)
		}
		if provenanceHas(result.Points()[0].Result().Provenance(), right) || !provenanceHas(result.Points()[1].Result().Provenance(), elseBranch) {
			t.Fatal("partial AND provenance does not reflect the selected samples")
		}
	})

	t.Run("OR and nested If ignore an unused Point coordinate", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		thenBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		elseBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(condition), ref(thenBranch), ref(elseBranch))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.Or(ref(left), ref(right))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(thenBranch), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(elseBranch), pointSeriesBools(t, []string{"2025-12-31", "2026-01-02"}, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		values := seriesValues(t, result)
		if len(values) != 2 || !values[0].IsDefined() || !values[0].Value().Raw().Bool() || !values[1].IsDefined() || values[1].Value().Raw().Bool() {
			t.Fatalf("OR output = %v, want [true false]", values)
		}
		if provenanceHas(result.Points()[0].Result().Provenance(), right) {
			t.Fatal("short-circuited OR sample contains right-side provenance")
		}
	})

	t.Run("Coalesce evaluates a partial fallback by Point timestamp", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		primary := f.seriesProperty(value.Bool, schema.InputComputation(), true, unit.Unit{}, false)
		fallback := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewCoalesce(ref(primary), ref(fallback))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(primary), pointSeriesSamples(t, dates, []series.Sample{series.Defined(value.NewBool(true)), series.Null()}, value.Bool, unit.Unit{}, false), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(fallback), pointSeriesBools(t, []string{"2025-12-31", "2026-01-02"}, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		values := seriesValues(t, result)
		if !values[0].IsDefined() || !values[0].Value().Raw().Bool() || !values[1].IsDefined() || values[1].Value().Raw().Bool() {
			t.Fatalf("Coalesce output = %v, want [true false]", values)
		}
		if provenanceHas(result.Points()[0].Result().Provenance(), fallback) || !provenanceHas(result.Points()[1].Result().Provenance(), fallback) {
			t.Fatal("Coalesce fallback provenance does not follow the Null demand")
		}
	})

	t.Run("If propagates temporal demand through arithmetic", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		thenBranch := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
		left := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Integer, schema.InputComputation(), false, unit.Unit{}, false)
		elseBranch := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.Add(ref(left), ref(right))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Integer, schema.DerivedComputation(ast.NewIf(ref(condition), ref(thenBranch), ref(elseBranch))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), pointSeriesBools(t, dates, []bool{true, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(thenBranch), pointSeriesInts(t, dates, []int64{1, 1}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), pointSeriesInts(t, []string{"2025-12-31", "2026-01-02"}, []int64{2, 2}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(right), pointSeriesInts(t, []string{"2025-12-31", "2026-01-02"}, []int64{3, 3}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		requireSeriesIntegers(t, result, 1, 5)
		if provenanceHas(result.Points()[0].Result().Provenance(), left) || !provenanceHas(result.Points()[1].Result().Provenance(), left) {
			t.Fatal("IF arithmetic provenance does not follow the selected samples")
		}
	})

	t.Run("Interval demand matches both half-open bounds", func(t *testing.T) {
		f := newPipelineFixture()
		from := []string{"2026-01-01", "2026-02-01"}
		to := []string{"2026-02-01", "2026-03-01"}
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		thenBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		elseBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(condition), ref(thenBranch), ref(elseBranch))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(left), ref(right))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), intervalSeriesBools(t, from, to, []bool{false, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), intervalSeriesBools(t, from, to, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(thenBranch), intervalSeriesBools(t, from, to, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(elseBranch), intervalSeriesBools(t, []string{"2025-12-01", "2026-02-01"}, []string{"2026-01-01", "2026-03-01"}, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		result, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		intervals := result.Intervals()
		if len(intervals) != 2 || !intervals[0].Result().Value().IsDefined() || intervals[0].Result().Value().Value().Raw().Bool() || !intervals[1].Result().Value().IsDefined() || intervals[1].Result().Value().Value().Raw().Bool() {
			t.Fatalf("Interval output = %#v, want [false false]", result)
		}
	})

	t.Run("a demanded Interval coordinate requires both bounds", func(t *testing.T) {
		f := newPipelineFixture()
		from := []string{"2026-01-01", "2026-02-01"}
		to := []string{"2026-02-01", "2026-03-01"}
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		thenBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		elseBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(condition), ref(thenBranch), ref(elseBranch))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(left), ref(right))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), intervalSeriesBools(t, from, to, []bool{false, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), intervalSeriesBools(t, from, to, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(thenBranch), intervalSeriesBools(t, from, to, []bool{false, false}), "", false, "", false, nil)
		// The demanded interval has the same From but a different To, which is
		// not an exact [from,to) identity and must not match by index or From.
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(elseBranch), intervalSeriesBools(t, []string{"2025-12-01", "2026-02-01"}, []string{"2026-01-01", "2026-04-01"}, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		_, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		var evaluationError evaluator.Error
		if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorIncompatibleSeriesAxis {
			t.Fatalf("error = %v, want ErrorIncompatibleSeriesAxis", err)
		}
	})

	t.Run("a demanded Point coordinate must not match by index", func(t *testing.T) {
		f := newPipelineFixture()
		dates := []string{"2026-01-01", "2026-01-02"}
		left := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		condition := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		thenBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		elseBranch := f.seriesProperty(value.Bool, schema.InputComputation(), false, unit.Unit{}, false)
		right := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.NewIf(ref(condition), ref(thenBranch), ref(elseBranch))), false, unit.Unit{}, false)
		output := f.seriesProperty(value.Bool, schema.DerivedComputation(ast.And(ref(left), ref(right))), false, unit.Unit{}, false)

		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(left), pointSeriesBools(t, dates, []bool{false, true}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(condition), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(thenBranch), pointSeriesBools(t, dates, []bool{false, false}), "", false, "", false, nil)
		addSeriesBinding(t, f.draft, semantic.ObjectPropertyTarget(elseBranch), pointSeriesBools(t, []string{"2025-12-31", "2026-01-03"}, []bool{false, false}), "", false, "", false, nil)
		v := f.commit(t)

		_, err := evaluator.EvaluateSeries(v, output, seriesContext(t, "2026-01-01"))
		var evaluationError evaluator.Error
		if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorIncompatibleSeriesAxis {
			t.Fatalf("error = %v, want ErrorIncompatibleSeriesAxis", err)
		}
	})
}
