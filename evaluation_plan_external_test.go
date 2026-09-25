package bcg_test

import (
	"reflect"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

func TestPublicPlanEvaluationDirectAndTraversal(t *testing.T) {
	artifact, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		t.Fatal(err)
	}
	model := artifact.Model()
	definition := model.PropertyDefinitionDeclarations()[0]
	target, err := bcg.PropertyDefinitionTarget(definition)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := model.PlanEvaluation([]bcg.PropertyTarget{target, target})
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.RequestedPropertyDefinitions(); len(got) != 1 || !got[0].ID().Equal(definition.ID()) {
		t.Fatalf("targets = %#v", got)
	}
	if !reflect.DeepEqual(plan, mustPlan(t, model, target)) {
		t.Fatal("repeated plans differ")
	}
	if _, err := model.PlanEvaluation([]bcg.PropertyTarget{{}}); err == nil {
		t.Fatal("invalid target accepted")
	}
	at, _ := bcg.ParseDateTime("2026-01-02T03:04:05Z")
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.PlanEvaluationWithContext([]bcg.PropertyTarget{target}, context); err != nil {
		t.Fatalf("context plan: %v", err)
	}

	aggregate, err := bcg.Unmarshal([]byte(publicAggregateArtifact))
	if err != nil {
		t.Fatal(err)
	}
	am := aggregate.Model()
	var total bcg.PropertyDefinition
	for _, d := range am.PropertyDefinitionDeclarations() {
		if d.Name() == "total" {
			total = d
		}
	}
	aggregateTarget, err := bcg.PropertyDefinitionTarget(total)
	if err != nil {
		t.Fatal(err)
	}
	aggregatePlan, err := am.PlanEvaluation([]bcg.PropertyTarget{aggregateTarget})
	if err != nil {
		t.Fatal(err)
	}
	if len(aggregatePlan.RequiredRelationTypes()) != 1 || len(aggregatePlan.TraversalRequirements()) != 1 {
		t.Fatalf("aggregate topology = %#v %#v", aggregatePlan.RequiredRelationTypes(), aggregatePlan.TraversalRequirements())
	}
	req := aggregatePlan.TraversalRequirements()[0]
	if req.Direction() != bcg.TraverseOutgoing || len(req.FilterDependencies()) == 0 || len(req.MapDependencies()) == 0 {
		t.Fatalf("traversal requirement omitted filter/map dependency: %#v", req)
	}
}

func TestPublicPlanEvaluationFunctionRequirement(t *testing.T) {
	model, _, _, _ := phase34BenchmarkFunction(t)
	var derived bcg.PropertyDefinition
	for _, candidate := range model.PropertyDefinitionDeclarations() {
		if candidate.Computation().Kind() == bcg.ComputationDerived {
			derived = candidate
			break
		}
	}
	target, err := bcg.PropertyDefinitionTarget(derived)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := model.PlanEvaluation([]bcg.PropertyTarget{target})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.FunctionRequirements()) != 1 {
		t.Fatalf("function requirements = %#v", plan.FunctionRequirements())
	}
}

func mustPlan(t testing.TB, m bcg.Model, target bcg.PropertyTarget) bcg.DependencyPlan {
	t.Helper()
	p, err := m.PlanEvaluation([]bcg.PropertyTarget{target})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
