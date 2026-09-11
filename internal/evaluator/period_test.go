package evaluator_test

import (
	"errors"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/provenance"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func periodTime(t *testing.T, value string) datetime.DateTime {
	t.Helper()
	result, err := datetime.Parse(value + "T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func periodContext(t *testing.T, from, to string) runtime.CalculationContext {
	t.Helper()
	return runtime.NewCalculationContext(
		domaintime.Period(periodTime(t, from), periodTime(t, to)),
		scope.Values{},
		nil,
	)
}

func effectivePeriod(
	t *testing.T,
	from string,
	hasFrom bool,
	to string,
	hasTo bool,
) domaintime.EffectivePeriod {
	t.Helper()
	var fromTime, toTime datetime.DateTime
	if hasFrom {
		fromTime = periodTime(t, from)
	}
	if hasTo {
		toTime = periodTime(t, to)
	}
	return domaintime.NewEffectivePeriod(fromTime, hasFrom, toTime, hasTo)
}

func addPeriodBinding(
	t *testing.T,
	draft *model.Draft,
	target semantic.ValueTarget,
	value int64,
	from string,
	hasFrom bool,
	to string,
	hasTo bool,
	selectors []scope.Selector,
) {
	t.Helper()
	draft.AddValueBinding(semantic.NewValueBinding(
		target,
		bindingInteger(value),
		selectors,
		effectivePeriod(t, from, hasFrom, to, hasTo),
		hasFrom || hasTo,
	))
}

func periodIntegers(t *testing.T, result evaluator.PeriodResult) []int64 {
	t.Helper()
	segments := result.Segments()
	values := make([]int64, len(segments))
	for index, segment := range segments {
		resolved := segment.Result().Value()
		if !resolved.IsDefined() || resolved.Value().Type() != value.Integer {
			t.Fatalf("segment %d is not a defined Integer: %v", index, resolved)
		}
		values[index] = resolved.Value().Raw().Integer().Int64()
	}
	return values
}

func requireBoundaries(t *testing.T, result evaluator.PeriodResult, dates ...string) {
	t.Helper()
	segments := result.Segments()
	if len(segments) != len(dates)-1 {
		t.Fatalf("got %d segments, want %d", len(segments), len(dates)-1)
	}
	for index, segment := range segments {
		if !segment.From().Equal(periodTime(t, dates[index])) ||
			!segment.To().Equal(periodTime(t, dates[index+1])) {
			t.Fatalf("segment %d has [%s, %s)", index, segment.From(), segment.To())
		}
	}
}

func TestEvaluatePeriod_BaseBindingsAndHalfOpenBoundaries(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(input), 100,
		"", false, "2026-02-01", true, nil)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(input), 200,
		"2026-02-01", true, "", false, nil)
	v := f.commit(t)

	result, err := evaluator.EvaluatePeriod(v, input, periodContext(t, "2026-01-01", "2026-04-01"))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsValid() {
		t.Fatal("invalid PeriodResult")
	}
	requireBoundaries(t, result, "2026-01-01", "2026-02-01", "2026-04-01")
	got := periodIntegers(t, result)
	if got[0] != 100 || got[1] != 200 {
		t.Fatalf("unexpected values: %v", got)
	}
	if !provenanceHas(result.Segments()[0].Result().Provenance(), provenance.KindBaseObjectBinding) {
		t.Fatal("base binding provenance is absent")
	}

	// The segment starting at the prior binding's exclusive end must select the
	// following binding. No zero-length segment is introduced at the request edge.
	result, err = evaluator.EvaluatePeriod(v, input, periodContext(t, "2026-02-01", "2026-04-01"))
	if err != nil || len(result.Segments()) != 1 || periodIntegers(t, result)[0] != 200 {
		t.Fatalf("exact boundary result: %#v, %v", result, err)
	}
}

func TestEvaluatePeriod_StaticAndNullSegmentValues(t *testing.T) {
	staticFixture := newPipelineFixture()
	staticInput := staticFixture.property(value.Integer, schema.InputComputation(), false)
	staticFixture.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(staticInput), bindingInteger(7), nil,
		domaintime.EffectivePeriod{}, false,
	))
	staticVersion := staticFixture.commit(t)
	staticResult, err := evaluator.EvaluatePeriod(staticVersion, staticInput,
		periodContext(t, "2026-01-01", "2026-04-01"))
	if err != nil || len(staticResult.Segments()) != 1 || periodIntegers(t, staticResult)[0] != 7 {
		t.Fatalf("static Period value: %#v, %v", staticResult, err)
	}

	nullableFixture := newPipelineFixture()
	nullableInput := nullableFixture.property(value.Integer, schema.InputComputation(), true)
	nullableFixture.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(nullableInput), semantic.NullBindingValue(), nil,
		effectivePeriod(t, "", false, "2026-02-01", true), true,
	))
	nullableFixture.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(nullableInput), bindingInteger(4), nil,
		effectivePeriod(t, "2026-02-01", true, "", false), true,
	))
	nullableVersion := nullableFixture.commit(t)
	nullableResult, err := evaluator.EvaluatePeriod(nullableVersion, nullableInput,
		periodContext(t, "2026-01-01", "2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireBoundaries(t, nullableResult, "2026-01-01", "2026-02-01", "2026-03-01")
	if !nullableResult.Segments()[0].Result().Value().IsNull() {
		// The second assertion validates the Defined segment without changing
		// Null into a Period-level error.
		t.Fatal("Null Period segment was not preserved")
	}
	resolved := nullableResult.Segments()[1].Result().Value()
	if !resolved.IsDefined() || resolved.Value().Raw().Integer().Int64() != 4 {
		t.Fatal("Defined Period segment was not preserved")
	}
}

func TestEvaluatePeriod_DerivedDependenciesAndScenarioOverrides(t *testing.T) {
	f := newPipelineFixture()
	a := f.property(value.Integer, schema.InputComputation(), false)
	b := f.property(value.Integer, schema.InputComputation(), false)
	c := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(a), ref(b))), false)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(a), 10,
		"", false, "2026-02-01", true, nil)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(a), 20,
		"2026-02-01", true, "", false, nil)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(b), 1,
		"", false, "2026-03-01", true, nil)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(b), 2,
		"2026-03-01", true, "", false, nil)
	v := f.commit(t)

	override := domainscenario.NewOverride(c, bindingInteger(100), nil,
		effectivePeriod(t, "2026-03-01", true, "2026-03-15", true), true)
	scenario := domainscenario.New(identity.New(), "period", v.ID(), []domainscenario.Override{override})
	context := periodContext(t, "2026-01-01", "2026-04-01").WithScenario(scenario)
	result, err := evaluator.EvaluatePeriod(v, c, context)
	if err != nil {
		t.Fatal(err)
	}
	requireBoundaries(t, result,
		"2026-01-01", "2026-02-01", "2026-03-01", "2026-03-15", "2026-04-01")
	got := periodIntegers(t, result)
	want := []int64{11, 21, 100, 22}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("segment %d: got %v, want %v", index, got, want)
		}
	}
	if !provenanceHas(result.Segments()[2].Result().Provenance(), provenance.KindScenarioOverride) {
		t.Fatal("derived Scenario override provenance is absent")
	}
	if provenanceHas(result.Segments()[2].Result().Provenance(), a) ||
		provenanceHas(result.Segments()[2].Result().Provenance(), b) {
		t.Fatal("derived Scenario override did not prune its dependencies")
	}
}

func TestEvaluatePeriod_CalculationOverrideAndScenarioAmbiguity(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(input), 1,
		"", false, "", false, nil)
	v := f.commit(t)

	period := effectivePeriod(t, "2026-02-01", true, "2026-03-01", true)
	overrideOne := domainscenario.NewOverride(input, bindingInteger(2), nil, period, true)
	overrideTwo := domainscenario.NewOverride(input, bindingInteger(3), nil, period, true)
	ambiguous := domainscenario.New(identity.New(), "ambiguous", v.ID(),
		[]domainscenario.Override{overrideOne, overrideTwo})
	context := periodContext(t, "2026-01-01", "2026-04-01").WithScenario(ambiguous)

	result, err := evaluator.EvaluatePeriod(v, input, context)
	if err != nil {
		t.Fatal(err)
	}
	requireBoundaries(t, result, "2026-01-01", "2026-02-01", "2026-03-01", "2026-04-01")
	for index, segment := range result.Segments() {
		if index == 1 {
			if !segment.Result().Value().IsUndefined() ||
				segment.Result().Value().UndefinedInfo().Kind() != value.UndefinedAmbiguousValue {
				t.Fatal("ambiguous Scenario outcome was not preserved")
			}
			continue
		}
		resolved := segment.Result().Value()
		if !resolved.IsDefined() || resolved.Value().Raw().Integer().Int64() != 1 {
			t.Fatal("base value changed outside ambiguous interval")
		}
	}

	calculationContext := runtime.NewCalculationContext(context.TimeContext(), scope.Values{},
		[]runtime.Override{runtime.NewOverride(input, bindingInteger(9))}).WithScenario(ambiguous)
	result, err = evaluator.EvaluatePeriod(v, input, calculationContext)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range periodIntegers(t, result) {
		if got != 9 {
			t.Fatal("calculation override did not win over Scenario override")
		}
	}
}

func TestEvaluatePeriod_AggregateScopeProvenanceAndOwnership(t *testing.T) {
	f := newPipelineFixture()
	mapped := f.property(value.Integer, schema.InputComputation(), false)
	relationTypeID := identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(relationTypeID, "links", endpoint, true, endpoint, true, nil))
	f.draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, f.entityID, f.entityID, nil, false))
	total := f.property(value.Integer, schema.DerivedComputation(ast.NewMappedAggregate(
		ast.NewTraversalSource(ast.TraversalDirectionBoth, relationTypeID),
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, mapped.DefinitionID()),
		ast.AggregateOperationSum,
	)), false)
	scopeID := identity.New()
	f.draft.AddScopeDefinition(scope.NewDefinition(scopeID, "region", value.String))
	us := scope.NewSelector(scopeID, value.NewTyped(value.NewString("US")))
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(mapped), 5,
		"", false, "2026-02-01", true, nil)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(mapped), 8,
		"2026-02-01", true, "", false, nil)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(mapped), 50,
		"2026-03-01", true, "", false, []scope.Selector{us})
	v := f.commit(t)

	result, err := evaluator.EvaluatePeriod(v, total, periodContext(t, "2026-01-01", "2026-04-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireBoundaries(t, result, "2026-01-01", "2026-02-01", "2026-04-01")
	got := periodIntegers(t, result)
	if got[0] != 5 || got[1] != 8 {
		t.Fatal("aggregate did not recompute for its mapped-value boundary")
	}

	// The result owns the segment slice and preserves the normal At provenance.
	segments := result.Segments()
	segments[0] = evaluator.PeriodSegment{}
	if !result.IsValid() || len(result.Segments()) != 2 ||
		!provenanceHas(result.Segments()[0].Result().Provenance(), provenance.KindBaseObjectBinding) {
		t.Fatal("PeriodResult exposed mutable storage, lost provenance, or added the irrelevant Scope boundary")
	}
}

func TestEvaluatePeriod_AggregateRelationApplicabilityChange(t *testing.T) {
	f := newPipelineFixture()
	mapped := f.property(value.Integer, schema.InputComputation(), false)
	relationTypeID, applicabilityID, relationID := identity.New(), identity.New(), identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddPropertyDefinition(schema.NewPropertyDefinition(
		applicabilityID,
		schema.RelationTypeOwner(relationTypeID),
		identity.UUID{},
		false,
		"active",
		value.Bool,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	))
	f.draft.AddRelationType(schema.NewRelationType(
		relationTypeID,
		"links",
		endpoint,
		true,
		endpoint,
		true,
		[]identity.UUID{applicabilityID},
	))
	f.draft.AddRelation(semantic.NewRelation(
		relationID,
		relationTypeID,
		f.entityID,
		f.entityID,
		ast.NewSelfPropertyReference(applicabilityID),
		true,
	))
	total := f.property(value.Integer, schema.DerivedComputation(ast.NewMappedAggregate(
		ast.NewTraversalSource(ast.TraversalDirectionBoth, relationTypeID),
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, mapped.DefinitionID()),
		ast.AggregateOperationSum,
	)), false)
	addPeriodBinding(t, f.draft, semantic.ObjectPropertyTarget(mapped), 5,
		"", false, "", false, nil)
	activeTarget := semantic.ObjectPropertyTarget(address.NewPropertyAddress(
		address.RelationRef(relationID),
		applicabilityID,
	))
	f.draft.AddValueBinding(semantic.NewValueBinding(
		activeTarget,
		semantic.DefinedBindingValue(value.NewTyped(value.NewBool(true))),
		nil,
		effectivePeriod(t, "", false, "2026-02-01", true),
		true,
	))
	f.draft.AddValueBinding(semantic.NewValueBinding(
		activeTarget,
		semantic.DefinedBindingValue(value.NewTyped(value.NewBool(false))),
		nil,
		effectivePeriod(t, "2026-02-01", true, "", false),
		true,
	))
	v := f.commit(t)

	result, err := evaluator.EvaluatePeriod(v, total,
		periodContext(t, "2026-01-01", "2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	requireBoundaries(t, result, "2026-01-01", "2026-02-01", "2026-03-01")
	got := periodIntegers(t, result)
	if got[0] != 5 || got[1] != 0 {
		t.Fatalf("aggregate did not recompute relation applicability: %v", got)
	}
}

func provenanceHas(node provenance.Node, want any) bool {
	if kind, ok := want.(provenance.Kind); ok && node.Kind() == kind {
		return true
	}
	if target, ok := want.(address.PropertyAddress); ok {
		if property, exists := node.Property(); exists && property == target {
			return true
		}
	}
	for _, child := range node.Children() {
		if provenanceHas(child, want) {
			return true
		}
	}
	return false
}

func TestEvaluatePeriod_ValidationAndContextCopy(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	v := f.commit(t)
	atContext := reviewContext()
	if _, err := evaluator.EvaluatePeriod(v, input, atContext); err == nil {
		t.Fatal("At context accepted by EvaluatePeriod")
	} else {
		var evaluationError evaluator.Error
		if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorUnsupportedTimeContext {
			t.Fatalf("wrong At error: %v", err)
		}
	}

	period := periodContext(t, "2026-01-01", "2026-02-01")
	scopeID := identity.New()
	selector := scope.NewSelector(scopeID, value.NewTyped(value.NewString("EU")))
	scopeValues := scope.NewValues([]scope.Selector{selector})
	override := runtime.NewOverride(input, bindingInteger(7))
	scenario := domainscenario.New(identity.New(), "context", identity.New(), nil)
	period = runtime.NewCalculationContext(period.TimeContext(), scopeValues, []runtime.Override{override}).WithScenario(scenario)
	derived := period.WithTimeContext(domaintime.At(periodTime(t, "2026-01-15")))
	if period.TimeContext().Kind() != domaintime.ContextPeriod ||
		derived.TimeContext().Kind() != domaintime.ContextAt {
		t.Fatal("WithTimeContext mutated the original context")
	}
	if actual, ok := derived.ScopeValues().Get(scopeID); !ok || !actual.Equal(selector.Value()) {
		t.Fatal("WithTimeContext lost Scope values")
	}
	if selected, ok := derived.Scenario(); !ok || selected.ID() != scenario.ID() {
		t.Fatal("WithTimeContext lost the Scenario")
	}
	if retained, ok := derived.Override(input); !ok || retained.Value().Kind() != semantic.BindingValueDefined {
		t.Fatal("WithTimeContext lost calculation overrides")
	}

	seriesDraft := model.NewDraft()
	typeID, definitionID, entityID := identity.New(), identity.New(), identity.New()
	seriesDraft.AddEntityType(schema.NewEntityType(typeID, "series", nil, []identity.UUID{definitionID}))
	seriesDraft.AddPropertyDefinition(schema.NewPropertyDefinition(definitionID,
		schema.EntityTypeOwner(typeID), identity.UUID{}, false, "series", value.Integer,
		unit.Unit{}, false, schema.TemporalSeries, schema.InputComputation()))
	seriesDraft.AddEntity(semantic.NewEntity(entityID, typeID, "", false))
	seriesVersion := reviewCommit(t, seriesDraft)
	_, err := evaluator.EvaluatePeriod(seriesVersion,
		address.NewPropertyAddress(address.EntityRef(entityID), definitionID),
		periodContext(t, "2026-01-01", "2026-02-01"))
	if err == nil {
		t.Fatal("Series target accepted by EvaluatePeriod")
	}
	var evaluationError evaluator.Error
	if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorUnsupportedSeries {
		t.Fatalf("wrong Series error: %v", err)
	}
}
