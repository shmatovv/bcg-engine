package evaluator_test

import (
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	resolver "github.com/shmatovv/bcg-engine/v2/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func TestBaseBindingIndexPreservesLayerAndScopePrecedence(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	scopeID := identity.New()
	f.draft.AddScopeDefinition(scope.NewDefinition(scopeID, "region", value.String))
	selector := scope.NewSelector(scopeID, value.NewTyped(value.NewString("north")))
	objectTarget := semantic.ObjectPropertyTarget(input)
	typeTarget := semantic.TypePropertyTarget(schema.EntityTypeOwner(f.typeID), input.DefinitionID())
	f.draft.AddValueBinding(semantic.NewValueBinding(typeTarget, bindingInteger(1), nil, domaintime.EffectivePeriod{}, false))
	selectors := []scope.Selector{selector}
	f.draft.AddValueBinding(semantic.NewValueBinding(objectTarget, bindingInteger(2), selectors, domaintime.EffectivePeriod{}, false))
	selectors[0] = scope.Selector{}
	v := f.commit(t)
	// Returned indexed bindings must remain independent of the committed index.
	bindings := v.ValueBindingsForTarget(objectTarget)
	bindings[0].ScopeSelectors()[0] = scope.Selector{}
	bindings[0] = semantic.ValueBinding{}
	for _, north := range []bool{false, true} {
		values := scope.Values{}
		want := int64(1)
		if north {
			values = scope.NewValues([]scope.Selector{selector})
			want = 2
		}
		context := runtime.NewCalculationContext(reviewContext().TimeContext(), values, nil)
		result, err := evaluator.Evaluate(v, input, context)
		if err != nil || result.Value().Value().Raw().Integer().Int64() != want {
			t.Fatalf("base precedence: %v", err)
		}
	}
	from, _ := datetime.Parse("2026-01-01T00:00:00Z")
	middle, _ := datetime.Parse("2026-02-01T00:00:00Z")
	to, _ := datetime.Parse("2026-03-01T00:00:00Z")
	override := domainscenario.NewOverride(input, bindingInteger(3), []scope.Selector{selector}, domaintime.NewEffectivePeriod(middle, true, datetime.DateTime{}, false), true)
	scenario := domainscenario.New(identity.New(), "dated", v.ID(), []domainscenario.Override{override})
	context := runtime.NewCalculationContext(domaintime.Period(from, to), scope.NewValues([]scope.Selector{selector}), nil).WithScenario(scenario)
	resolution, err := resolver.Resolve(v, input, context)
	if err != nil {
		t.Fatal(err)
	}
	segments := resolution.Segments()
	if len(segments) != 2 {
		t.Fatal("scenario boundary omitted")
	}
	for i, segment := range segments {
		if segment.Outcome().Value().Value().Raw().Integer().Int64() != int64(i+2) {
			t.Fatal("scenario period precedence")
		}
	}
}

func TestCoalesceFallbackAndUndefined(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), true)
	root := f.property(value.Decimal, schema.DerivedComputation(ast.NewCoalesce(ref(input), ast.NewConstant(value.NewTyped(value.NewDecimal(decimal.MustParse("1.5")))))), false)
	v := f.commit(t)
	for _, binding := range []semantic.BindingValue{semantic.NullBindingValue(), bindingInteger(2)} {
		context := runtime.NewCalculationContext(reviewContext().TimeContext(), scope.Values{}, []runtime.Override{runtime.NewOverride(input, binding)})
		result, err := evaluator.Evaluate(v, root, context)
		if err != nil {
			t.Fatal(err)
		}
		want := "1.5"
		if binding.Kind() == semantic.BindingValueDefined {
			want = "2"
		}
		if result.Value().Value().Raw().Decimal().String() != want {
			t.Fatal("wrong coalesce branch")
		}
	}
	result, err := evaluator.Evaluate(v, root, reviewContext())
	if err != nil || !result.Value().IsUndefined() {
		t.Fatalf("Coalesce must preserve Undefined: %v", err)
	}
}
