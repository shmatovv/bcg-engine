package impact_test

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/impact"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
	"github.com/shmatovv/bcg-engine/internal/types/series"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

type impactFixture struct {
	draft            *model.Draft
	typeID, entityID identity.UUID
	definitions      []identity.UUID
}

func newImpactFixture() *impactFixture {
	return &impactFixture{draft: model.NewDraft(), typeID: identity.New(), entityID: identity.New()}
}

func (f *impactFixture) scalar(typ value.Type, computation schema.Computation) address.PropertyAddress {
	return f.property(typ, schema.TemporalScalar, computation)
}

func (f *impactFixture) series(typ value.Type, computation schema.Computation) address.PropertyAddress {
	return f.property(typ, schema.TemporalSeries, computation)
}

func (f *impactFixture) nullableScalar(typ value.Type, computation schema.Computation) address.PropertyAddress {
	id := identity.New()
	contractID := identity.New()
	f.definitions = append(f.definitions, id)
	f.draft.AddPropertyContract(schema.NewPropertyContract(contractID, "nullable", typ, unit.Dimension{}, false, schema.TemporalScalar, true))
	f.draft.AddPropertyDefinition(schema.NewPropertyDefinition(id, schema.EntityTypeOwner(f.typeID), contractID, true, "nullable", typ, unit.Unit{}, false, schema.TemporalScalar, computation))
	return address.NewPropertyAddress(address.EntityRef(f.entityID), id)
}

func (f *impactFixture) property(typ value.Type, temporalMode schema.TemporalMode, computation schema.Computation) address.PropertyAddress {
	id := identity.New()
	f.definitions = append(f.definitions, id)
	f.draft.AddPropertyDefinition(schema.NewPropertyDefinition(id, schema.EntityTypeOwner(f.typeID), identity.UUID{}, false, "p", typ, unit.Unit{}, false, temporalMode, computation))
	return address.NewPropertyAddress(address.EntityRef(f.entityID), id)
}

func (f *impactFixture) commit(t *testing.T) model.ModelVersion {
	t.Helper()
	f.draft.AddEntityType(schema.NewEntityType(f.typeID, "item", nil, f.definitions))
	f.draft.AddEntity(semantic.NewEntity(f.entityID, f.typeID, "", false))
	version, err := model.Commit(model.NewModel(identity.New(), "impact"), f.draft, model.NewCommitOptions(identity.New(), "impact", precision.New(6, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func impactContext(t *testing.T) runtime.CalculationContext {
	return impactContextAt(t, "2026-01-01T00:00:00Z")
}

func impactContextAt(t *testing.T, instant string) runtime.CalculationContext {
	t.Helper()
	at, err := datetime.Parse(instant)
	if err != nil {
		t.Fatal(err)
	}
	return runtime.NewCalculationContext(domaintime.At(at), scope.Values{}, nil)
}

func ref(target address.PropertyAddress) ast.Expression {
	return ast.NewAbsolutePropertyReference(target)
}
func integer(number int64) ast.Constant {
	return ast.NewConstant(value.NewTyped(value.NewInt64(number)))
}
func boolean(truth bool) ast.Constant { return ast.NewConstant(value.NewTyped(value.NewBool(truth))) }
func binding(number int64) semantic.BindingValue {
	return semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(number)))
}
func boolBinding(truth bool) semantic.BindingValue {
	return semantic.DefinedBindingValue(value.NewTyped(value.NewBool(truth)))
}

func bind(f *impactFixture, target address.PropertyAddress, number int64) {
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(target), binding(number), nil, domaintime.EffectivePeriod{}, false))
}

func bindBool(f *impactFixture, target address.PropertyAddress, truth bool) {
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(target), boolBinding(truth), nil, domaintime.EffectivePeriod{}, false))
}

func targets(result impact.Result) []address.PropertyAddress {
	affected := result.Affected()
	values := make([]address.PropertyAddress, len(affected))
	for index, item := range affected {
		values[index] = item.Target()
	}
	return values
}

func requireTargets(t *testing.T, result impact.Result, expected ...address.PropertyAddress) {
	t.Helper()
	actual := targets(result)
	if len(actual) != len(expected) {
		t.Fatalf("affected length = %d, want %d: %#v", len(actual), len(expected), actual)
	}
	set := make(map[address.PropertyAddress]struct{}, len(actual))
	for _, target := range actual {
		set[target] = struct{}{}
	}
	for _, target := range expected {
		if _, ok := set[target]; !ok {
			t.Fatalf("affected = %#v, missing %v", actual, target)
		}
	}
	if !result.IsValid() {
		t.Fatal("Impact result is invalid")
	}
}

func affectedFor(result impact.Result, target address.PropertyAddress) (impact.Affected, bool) {
	for _, item := range result.Affected() {
		if item.Target() == target {
			return item, true
		}
	}
	return impact.Affected{}, false
}

func TestAnalyzeStructuralGraphClosureAndDeterminism(t *testing.T) {
	f := newImpactFixture()
	a := f.scalar(value.Integer, schema.InputComputation())
	b := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))))
	c := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(2))))
	d := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(b), ref(c))))
	isolated := f.scalar(value.Integer, schema.InputComputation())
	version := f.commit(t)

	result, err := impact.AnalyzeStructural(version, a)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, result, b, c, d)
	bAffected, _ := affectedFor(result, b)
	cAffected, _ := affectedFor(result, c)
	dAffected, _ := affectedFor(result, d)
	if !bAffected.IsDirect() || !cAffected.IsDirect() || dAffected.IsDirect() || dAffected.MinimumDepth() != 2 {
		t.Fatalf("direct/depth metadata = %#v", result.Affected())
	}
	if len(result.Edges()) != 4 {
		t.Fatalf("edge count = %d, want 4", len(result.Edges()))
	}
	if result.Source() != a || result.Mode() != impact.ModeStructural {
		t.Fatal("result metadata is wrong")
	}
	leaf, err := impact.AnalyzeStructural(version, isolated)
	if err != nil || leaf.Len() != 0 || len(leaf.Edges()) != 0 {
		t.Fatalf("leaf impact = %#v, %v", leaf, err)
	}
	for iteration := 0; iteration < 100; iteration++ {
		repeated, err := impact.AnalyzeStructural(version, a)
		if err != nil || !reflect.DeepEqual(result, repeated) {
			t.Fatalf("nondeterministic structural result at %d: %v", iteration, err)
		}
	}
}

func TestAnalyzeEffectivePrunesOnlyOverrideIncomingEdges(t *testing.T) {
	f := newImpactFixture()
	a := f.scalar(value.Integer, schema.InputComputation())
	b := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))))
	c := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(b), integer(1))))
	version := f.commit(t)
	base, err := impact.AnalyzeEffective(version, a, impactContext(t))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, base, b, c)

	scenario := domainscenario.New(identity.New(), "override", version.ID(), []domainscenario.Override{domainscenario.NewOverride(b, binding(100), nil, domaintime.EffectivePeriod{}, false)})
	context := impactContext(t).WithScenario(scenario)
	pruned, err := impact.AnalyzeEffective(version, a, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, pruned)
	fromOverride, err := impact.AnalyzeEffective(version, b, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, fromOverride, c)
	cAffected, found := affectedFor(fromOverride, c)
	if !found || !cAffected.IsDirect() {
		t.Fatal("downstream edge from overridden value was removed")
	}
	observedFromA, err := impact.AnalyzeObserved(version, a, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, observedFromA)
	observedFromB, err := impact.AnalyzeObserved(version, b, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, observedFromB, c)

	calculation := runtime.NewCalculationContext(context.TimeContext(), context.ScopeValues(), []runtime.Override{runtime.NewOverride(b, binding(200))}).WithScenario(scenario)
	calculationPruned, err := impact.AnalyzeEffective(version, a, calculation)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, calculationPruned)
	calculationFromB, err := impact.AnalyzeEffective(version, b, calculation)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, calculationFromB, c)
}

func TestAnalyzeObservedUsesActualLazyProvenance(t *testing.T) {
	f := newImpactFixture()
	flag := f.scalar(value.Bool, schema.InputComputation())
	a := f.scalar(value.Integer, schema.InputComputation())
	b := f.scalar(value.Integer, schema.InputComputation())
	selected := f.scalar(value.Integer, schema.DerivedComputation(ast.NewIf(ref(flag), ref(a), ref(b))))
	andValue := f.scalar(value.Bool, schema.DerivedComputation(ast.And(boolean(false), ast.Greater(ref(b), integer(0)))))
	bindBool(f, flag, true)
	bind(f, a, 10)
	bind(f, b, 20)
	version := f.commit(t)
	context := impactContext(t)

	structuralB, err := impact.AnalyzeStructural(version, b)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, structuralB, andValue, selected)
	observedB, err := impact.AnalyzeObserved(version, b, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, observedB)
	observedA, err := impact.AnalyzeObserved(version, a, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, observedA, selected)
	observedFlag, err := impact.AnalyzeObserved(version, flag, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, observedFlag, selected)
	if observedA.Mode() != impact.ModeObserved {
		t.Fatal("observed mode not recorded")
	}

	observedCopy := observedA.Affected()
	observedCopy[0] = impact.Affected{}
	selectedAffected, found := affectedFor(observedA, selected)
	if !observedA.IsValid() || !found || selectedAffected.Target() != selected {
		t.Fatal("Impact getter leaked backing storage")
	}
	var group sync.WaitGroup
	for index := 0; index < 16; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := impact.AnalyzeObserved(version, a, context); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
}

func pointSeries(t *testing.T, at string, number int64) series.Series {
	t.Helper()
	instant, err := datetime.Parse(at)
	if err != nil {
		t.Fatal(err)
	}
	return series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{series.NewPoint(instant, series.Defined(value.NewInt64(number)))})
}

func TestAnalyzeEffectiveSeriesOverridePrunesIncomingOnly(t *testing.T) {
	f := newImpactFixture()
	a := f.series(value.Integer, schema.InputComputation())
	b := f.series(value.Integer, schema.DerivedComputation(ref(a)))
	c := f.series(value.Integer, schema.DerivedComputation(ref(b)))
	version := f.commit(t)
	base, err := impact.AnalyzeStructural(version, a)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, base, b, c)
	override := domainscenario.NewOverride(b, semantic.SeriesBindingValue(pointSeries(t, "2026-01-01T00:00:00Z", 100)), nil, domaintime.EffectivePeriod{}, false)
	context := impactContext(t).WithScenario(domainscenario.New(identity.New(), "series", version.ID(), []domainscenario.Override{override}))
	fromA, err := impact.AnalyzeEffective(version, a, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, fromA)
	fromB, err := impact.AnalyzeEffective(version, b, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, fromB, c)
}

func TestAnalyzeObservedPreservesNestedAndUndefinedDependencies(t *testing.T) {
	f := newImpactFixture()
	a := f.scalar(value.Integer, schema.InputComputation())
	b := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))))
	c := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(b), integer(1))))
	version := f.commit(t)

	// A has no binding, so the ordinary evaluation is Undefined(MissingValue).
	// The actual provenance still records both reads and must remain usable for
	// observed downstream analysis.
	result, err := impact.AnalyzeObserved(version, a, impactContext(t))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, result, b, c)
	bAffected, _ := affectedFor(result, b)
	cAffected, _ := affectedFor(result, c)
	if bAffected.MinimumDepth() != 1 || cAffected.MinimumDepth() != 2 {
		t.Fatalf("nested observed depth = %#v", result.Affected())
	}
	if len(result.Edges()) != 2 {
		t.Fatalf("nested observed edges = %#v", result.Edges())
	}
}

func TestAnalyzeObservedSeriesUsesRecordedSamplesWithoutIndexPairing(t *testing.T) {
	f := newImpactFixture()
	flag := f.series(value.Bool, schema.InputComputation())
	a := f.series(value.Integer, schema.InputComputation())
	b := f.series(value.Integer, schema.InputComputation())
	result := f.series(value.Integer, schema.DerivedComputation(ast.NewIf(ref(flag), ref(a), ref(b))))
	at1, err := datetime.Parse("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	at2, err := datetime.Parse("2026-01-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	flags := series.NewPointSeries(value.Bool, unit.Unit{}, false, []series.Point{
		series.NewPoint(at1, series.Defined(value.NewBool(true))),
		series.NewPoint(at2, series.Defined(value.NewBool(false))),
	})
	valuesA := series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{
		series.NewPoint(at1, series.Defined(value.NewInt64(1))),
		series.NewPoint(at2, series.Defined(value.NewInt64(2))),
	})
	valuesB := series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{
		series.NewPoint(at1, series.Defined(value.NewInt64(3))),
		series.NewPoint(at2, series.Defined(value.NewInt64(4))),
	})
	for target, stored := range map[address.PropertyAddress]series.Series{flag: flags, a: valuesA, b: valuesB} {
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(target), semantic.SeriesBindingValue(stored), nil, domaintime.EffectivePeriod{}, false))
	}
	version := f.commit(t)
	fromA, err := impact.AnalyzeObserved(version, a, impactContext(t))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, fromA, result)
	fromB, err := impact.AnalyzeObserved(version, b, impactContext(t))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, fromB, result)
	fromFlag, err := impact.AnalyzeObserved(version, flag, impactContext(t))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, fromFlag, result)
}

func TestAnalyzeStructuralIncludesConcreteAggregateMapDependencies(t *testing.T) {
	f := newImpactFixture()
	input := f.scalar(value.Integer, schema.InputComputation())
	relationTypeID := identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(relationTypeID, "links", endpoint, true, endpoint, true, nil))
	f.draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, f.entityID, f.entityID, boolean(true), true))
	source := ast.NewTraversalSource(ast.TraversalDirectionBoth, relationTypeID)
	total := f.scalar(value.Integer, schema.DerivedComputation(ast.NewFilteredMappedAggregate(source, boolean(true), ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, input.DefinitionID()), ast.AggregateOperationSum)))
	bind(f, input, 5)
	version := f.commit(t)
	result, err := impact.AnalyzeStructural(version, input)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, result, total)
	observed, err := impact.AnalyzeObserved(version, input, impactContext(t))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, observed, total)
}

func TestAnalyzeObservedCoalesceIncludesOnlyActualFallback(t *testing.T) {
	withPrimary := func(t *testing.T, primary semantic.BindingValue) (model.ModelVersion, address.PropertyAddress, address.PropertyAddress) {
		t.Helper()
		f := newImpactFixture()
		first := f.nullableScalar(value.Integer, schema.InputComputation())
		fallback := f.scalar(value.Integer, schema.InputComputation())
		f.scalar(value.Integer, schema.DerivedComputation(ast.NewCoalesce(ref(first), ref(fallback))))
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(first), primary, nil, domaintime.EffectivePeriod{}, false))
		bind(f, fallback, 7)
		return f.commit(t), fallback, first
	}
	t.Run("Null primary evaluates fallback", func(t *testing.T) {
		version, fallback, _ := withPrimary(t, semantic.NullBindingValue())
		result, err := impact.AnalyzeObserved(version, fallback, impactContext(t))
		if err != nil {
			t.Fatal(err)
		}
		if result.Len() != 1 || !result.Affected()[0].IsDirect() {
			t.Fatalf("fallback impact = %#v", result)
		}
	})
	t.Run("defined primary short-circuits fallback", func(t *testing.T) {
		version, fallback, _ := withPrimary(t, binding(3))
		result, err := impact.AnalyzeObserved(version, fallback, impactContext(t))
		if err != nil {
			t.Fatal(err)
		}
		requireTargets(t, result)
	})
}

func TestAnalyzeObservedCoalesceDoesNotUseFallbackForUndefinedPrimary(t *testing.T) {
	f := newImpactFixture()
	primary := f.scalar(value.Integer, schema.InputComputation()) // deliberately MissingValue
	fallback := f.scalar(value.Integer, schema.InputComputation())
	f.scalar(value.Integer, schema.DerivedComputation(ast.NewCoalesce(ref(primary), ref(fallback))))
	bind(f, fallback, 9)
	version := f.commit(t)
	result, err := impact.AnalyzeObserved(version, fallback, impactContext(t))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, result)
}

func TestAnalyzeObservedSeriesKeepsDemandedCoordinateSemantics(t *testing.T) {
	f := newImpactFixture()
	flag := f.series(value.Bool, schema.InputComputation())
	a := f.series(value.Integer, schema.InputComputation())
	b := f.series(value.Integer, schema.InputComputation())
	derived := f.series(value.Integer, schema.DerivedComputation(ast.NewIf(ref(flag), ref(a), ref(b))))
	dec31, err := datetime.Parse("2025-12-31T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	jan1, err := datetime.Parse("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	jan2, err := datetime.Parse("2026-01-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	flags := series.NewPointSeries(value.Bool, unit.Unit{}, false, []series.Point{
		series.NewPoint(jan1, series.Defined(value.NewBool(true))),
		series.NewPoint(jan2, series.Defined(value.NewBool(false))),
	})
	selectedA := series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{
		series.NewPoint(jan1, series.Defined(value.NewInt64(1))),
		series.NewPoint(jan2, series.Defined(value.NewInt64(2))),
	})
	// Dec-31 belongs to an unselected B sample. Jan-02 is the one exact
	// coordinate B must contribute to; no index-based alignment is involved.
	partiallyRelevantB := series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{
		series.NewPoint(dec31, series.Defined(value.NewInt64(3))),
		series.NewPoint(jan2, series.Defined(value.NewInt64(4))),
	})
	for target, stored := range map[address.PropertyAddress]series.Series{flag: flags, a: selectedA, b: partiallyRelevantB} {
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(target), semantic.SeriesBindingValue(stored), nil, domaintime.EffectivePeriod{}, false))
	}
	version := f.commit(t)
	observed, err := impact.AnalyzeObserved(version, b, impactContext(t))
	if err != nil {
		t.Fatalf("undemanded coordinate leaked into Impact evaluation: %v", err)
	}
	requireTargets(t, observed, derived)
}

func TestAnalyzeObservedPropagatesRequiredSeriesAxisFailure(t *testing.T) {
	f := newImpactFixture()
	flag := f.series(value.Bool, schema.InputComputation())
	a := f.series(value.Integer, schema.InputComputation())
	b := f.series(value.Integer, schema.InputComputation())
	f.series(value.Integer, schema.DerivedComputation(ast.NewIf(ref(flag), ref(a), ref(b))))
	jan1, err := datetime.Parse("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	dec31, err := datetime.Parse("2025-12-31T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	for target, stored := range map[address.PropertyAddress]series.Series{
		flag: series.NewPointSeries(value.Bool, unit.Unit{}, false, []series.Point{series.NewPoint(jan1, series.Defined(value.NewBool(false)))}),
		a:    series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{series.NewPoint(jan1, series.Defined(value.NewInt64(1)))}),
		b:    series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{series.NewPoint(dec31, series.Defined(value.NewInt64(2)))}),
	} {
		f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(target), semantic.SeriesBindingValue(stored), nil, domaintime.EffectivePeriod{}, false))
	}
	version := f.commit(t)
	_, err = impact.AnalyzeObserved(version, b, impactContext(t))
	var impactError impact.Error
	if !errors.As(err, &impactError) || impactError.Kind() != impact.ErrorEvaluation {
		t.Fatalf("required axis incompatibility must remain an engine error, got %v", err)
	}
}

func TestAnalyzeEffectiveUsesOverrideEffectiveTime(t *testing.T) {
	f := newImpactFixture()
	a := f.scalar(value.Integer, schema.InputComputation())
	b := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))))
	version := f.commit(t)
	from, err := datetime.Parse("2026-02-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	to, err := datetime.Parse("2026-03-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	override := domainscenario.NewOverride(b, binding(100), nil, domaintime.NewEffectivePeriod(from, true, to, true), true)
	context := impactContextAt(t, "2026-01-01T00:00:00Z").WithScenario(domainscenario.New(identity.New(), "period", version.ID(), []domainscenario.Override{override}))
	before, err := impact.AnalyzeEffective(version, a, context)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, before, b)
	scenario, ok := context.Scenario()
	if !ok {
		t.Fatal("context lost Scenario")
	}
	during, err := impact.AnalyzeEffective(version, a, impactContextAt(t, "2026-02-01T00:00:00Z").WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, during)
	after, err := impact.AnalyzeEffective(version, a, impactContextAt(t, "2026-03-01T00:00:00Z").WithScenario(scenario))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, after, b)
}

func TestAnalyzeEffectiveUsesResolverScopeSelection(t *testing.T) {
	f := newImpactFixture()
	a := f.scalar(value.Integer, schema.InputComputation())
	b := f.scalar(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))))
	scopeID := identity.New()
	f.draft.AddScopeDefinition(scope.NewDefinition(scopeID, "region", value.String))
	version := f.commit(t)
	override := domainscenario.NewOverride(b, binding(100), []scope.Selector{scope.NewSelector(scopeID, value.NewTyped(value.NewString("US")))}, domaintime.EffectivePeriod{}, false)
	scenario := domainscenario.New(identity.New(), "scope", version.ID(), []domainscenario.Override{override})
	at := impactContext(t).TimeContext()
	us := runtime.NewCalculationContext(at, scope.NewValues([]scope.Selector{scope.NewSelector(scopeID, value.NewTyped(value.NewString("US")))}), nil).WithScenario(scenario)
	eu := runtime.NewCalculationContext(at, scope.NewValues([]scope.Selector{scope.NewSelector(scopeID, value.NewTyped(value.NewString("EU")))}), nil).WithScenario(scenario)
	usResult, err := impact.AnalyzeEffective(version, a, us)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, usResult)
	euResult, err := impact.AnalyzeEffective(version, a, eu)
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, euResult, b)
}

func TestAnalyzeObservedIntervalSeriesUsesIntervalProvenance(t *testing.T) {
	f := newImpactFixture()
	a := f.series(value.Integer, schema.InputComputation())
	b := f.series(value.Integer, schema.DerivedComputation(ref(a)))
	from, err := datetime.Parse("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	to, err := datetime.Parse("2026-02-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	stored := series.NewIntervalSeries(value.Integer, unit.Unit{}, false, []series.Interval{
		series.NewInterval(from, to, series.Defined(value.NewInt64(1))),
	})
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(a), semantic.SeriesBindingValue(stored), nil, domaintime.EffectivePeriod{}, false))
	version := f.commit(t)
	result, err := impact.AnalyzeObserved(version, a, impactContext(t))
	if err != nil {
		t.Fatal(err)
	}
	requireTargets(t, result, b)
}
