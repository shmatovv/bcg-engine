package evaluator_test

import (
	"sync"
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
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/plan"
	"github.com/shmatovv/bcg-engine/internal/provenance"
	resolver "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

type pipelineFixture struct {
	draft            *model.Draft
	typeID, entityID identity.UUID
	ids              []identity.UUID
}

func newPipelineFixture() *pipelineFixture {
	return &pipelineFixture{draft: model.NewDraft(), typeID: identity.New(), entityID: identity.New()}
}
func (f *pipelineFixture) property(typ value.Type, computation schema.Computation, nullable bool) address.PropertyAddress {
	id := identity.New()
	contractID := identity.UUID{}
	if nullable {
		contractID = identity.New()
		f.draft.AddPropertyContract(schema.NewPropertyContract(contractID, "nullable", typ, unit.Dimension{}, false, schema.TemporalScalar, true))
	}
	f.ids = append(f.ids, id)
	f.draft.AddPropertyDefinition(schema.NewPropertyDefinition(id, schema.EntityTypeOwner(f.typeID), contractID, nullable, "property", typ, unit.Unit{}, false, schema.TemporalScalar, computation))
	return address.NewPropertyAddress(address.EntityRef(f.entityID), id)
}
func (f *pipelineFixture) commit(t *testing.T) model.ModelVersion {
	f.draft.AddEntityType(schema.NewEntityType(f.typeID, "item", nil, f.ids))
	f.draft.AddEntity(semantic.NewEntity(f.entityID, f.typeID, "", false))
	return reviewCommit(t, f.draft)
}
func bindingInteger(n int64) semantic.BindingValue {
	return semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(n)))
}
func ref(a address.PropertyAddress) ast.Expression { return ast.NewAbsolutePropertyReference(a) }

func TestLazyPlanAndProvenance(t *testing.T) {
	f := newPipelineFixture()
	missing := f.property(value.Integer, schema.InputComputation(), false)
	badBool := ast.Greater(ref(missing), integer(0))
	roots := []address.PropertyAddress{
		f.property(value.Bool, schema.DerivedComputation(ast.And(boolean(false), badBool)), false),
		f.property(value.Bool, schema.DerivedComputation(ast.Or(boolean(true), badBool)), false),
		f.property(value.Integer, schema.DerivedComputation(ast.NewIf(boolean(true), integer(7), ref(missing))), false),
		f.property(value.Integer, schema.DerivedComputation(ast.NewCoalesce(integer(7), ref(missing))), false),
	}
	v := f.commit(t)
	p, err := plan.Build(v, roots, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := p.Node(missing); !exists {
		t.Fatal("expected static dependency")
	}
	result, err := evaluator.EvaluatePlan(v, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := result.Result(missing); exists {
		t.Fatal("short-circuited dependency was evaluated")
	}
	for _, entry := range result.RootResults() {
		if !entry.Result().Value().IsDefined() {
			t.Fatal("unselected branch propagated Undefined")
		}
		if provenanceContains(entry.Result().Provenance(), missing) {
			t.Fatal("unselected branch in provenance")
		}
	}
	// Public collection getters and the mutable Draft cannot alter a plan or snapshot.
	p.Roots()[0] = missing
	p.Order()[0] = missing
	result.Entries()[0] = evaluator.Entry{}
	v.PropertyDefinitions()[0] = schema.PropertyDefinition{}
	f.draft.AddEntity(semantic.NewEntity(identity.New(), f.typeID, "", false))
	if !p.IsValid() || !result.IsValid() || len(v.Entities()) != 1 {
		t.Fatal("snapshot alias")
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := evaluator.EvaluatePlan(v, p); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestEvaluatePlanRejectsPlanForAnotherDataset(t *testing.T) {
	f := newPipelineFixture()
	prototype := f.property(value.Integer, schema.InputComputation(), false)
	v := f.commit(t)
	runtimeEntityID := identity.New()
	target := address.NewPropertyAddress(address.EntityRef(runtimeEntityID), prototype.DefinitionID())

	buildSource := func() model.EvaluationSource {
		builder, err := model.NewDatasetBuilder(v, identity.New())
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.AddEntity(semantic.NewEntity(runtimeEntityID, f.typeID, "", false)); err != nil {
			t.Fatal(err)
		}
		dataset, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		source, err := model.NewEvaluationSource(v, dataset)
		if err != nil {
			t.Fatal(err)
		}
		return source
	}

	first, second := buildSource(), buildSource()
	p, err := plan.Build(first, []address.PropertyAddress{target}, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if result, err := evaluator.EvaluatePlan(first, p); err != nil || !result.SourceIdentity().Equal(first.SourceIdentity()) {
		t.Fatalf("Dataset A PlanResult = %#v, %v", result, err)
	}
	if _, err := evaluator.EvaluatePlan(second, p); err == nil {
		t.Fatal("Plan for Dataset A unexpectedly evaluated against Dataset B")
	}
}

func provenanceContains(n provenance.Node, target address.PropertyAddress) bool {
	if p, ok := n.Property(); ok && p == target {
		return true
	}
	for _, child := range n.Children() {
		if provenanceContains(child, target) {
			return true
		}
	}
	return false
}

func TestScenarioOverridePruningAndAmbiguity(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	cost := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(input), integer(1))), false)
	total := f.property(value.Integer, schema.DerivedComputation(ast.Multiply(ref(cost), integer(2))), false)
	v := f.commit(t)
	base, err := dependency.Build(v)
	if err != nil {
		t.Fatal(err)
	}
	override := domainscenario.NewOverride(cost, bindingInteger(100), nil, domaintime.EffectivePeriod{}, false)
	overrides := []domainscenario.Override{override}
	scenario := domainscenario.New(identity.New(), "scenario", v.ID(), overrides)
	overrides[0] = domainscenario.Override{}
	scenario.Overrides()[0] = domainscenario.Override{}
	context := reviewContext().WithScenario(scenario)
	p, err := plan.Build(v, []address.PropertyAddress{total}, context)
	if err != nil {
		t.Fatal(err)
	}
	if p.Len() != 2 || len(base.DependenciesOf(cost)) != 1 {
		t.Fatal("override failed to prune, or mutated base graph")
	}
	result, err := evaluator.EvaluatePlan(v, p)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := result.Result(total)
	if got.Value().Value().Raw().Integer().Int64() != 200 {
		t.Fatal("derived override ignored")
	}
	if _, ok := result.Result(input); ok {
		t.Fatal("pruned input evaluated")
	}
	context = runtime.NewCalculationContext(context.TimeContext(), scope.Values{}, []runtime.Override{runtime.NewOverride(cost, bindingInteger(3))}).WithScenario(scenario)
	got, err = evaluator.Evaluate(v, total, context)
	if err != nil || got.Value().Value().Raw().Integer().Int64() != 6 {
		t.Fatalf("calculation precedence: %v", err)
	}
	ambiguous := domainscenario.New(identity.New(), "ambiguous", v.ID(), []domainscenario.Override{override, override})
	got, err = evaluator.Evaluate(v, cost, reviewContext().WithScenario(ambiguous))
	if err != nil || !got.Value().IsUndefined() || got.Value().UndefinedInfo().Kind() != value.UndefinedAmbiguousValue {
		t.Fatalf("ambiguous scenario fell back: %v", err)
	}
}

func TestResolverPeriodBoundaries(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	from, _ := datetime.Parse("2026-01-01T00:00:00Z")
	middle, _ := datetime.Parse("2026-02-01T00:00:00Z")
	to, _ := datetime.Parse("2026-03-01T00:00:00Z")
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(1), nil, domaintime.NewEffectivePeriod(datetime.DateTime{}, false, middle, true), true))
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(input), bindingInteger(2), nil, domaintime.NewEffectivePeriod(middle, true, datetime.DateTime{}, false), true))
	v := f.commit(t)
	resolution, err := resolver.Resolve(v, input, runtime.NewCalculationContext(domaintime.Period(from, to), scope.Values{}, nil))
	if err != nil {
		t.Fatal(err)
	}
	segments := resolution.Segments()
	if len(segments) != 2 {
		t.Fatalf("got %d segments", len(segments))
	}
	if !segments[0].From().Equal(from) || !segments[0].To().Equal(middle) || !segments[1].From().Equal(middle) || !segments[1].To().Equal(to) {
		t.Fatal("wrong boundaries")
	}
	for i, segment := range segments {
		if segment.Outcome().Value().Value().Raw().Integer().Int64() != int64(i+1) {
			t.Fatal("wrong segment value")
		}
	}
	at, err := resolver.Resolve(v, input, runtime.NewCalculationContext(domaintime.At(middle), scope.Values{}, nil))
	if err != nil {
		t.Fatal(err)
	}
	outcome, _ := at.At()
	if outcome.Value().Value().Raw().Integer().Int64() != 2 {
		t.Fatal("end bound is inclusive")
	}
	if _, err := evaluator.Evaluate(v, input, runtime.NewCalculationContext(domaintime.Period(from, to), scope.Values{}, nil)); err == nil {
		t.Fatal("Period evaluator must remain explicitly unsupported")
	}
}
