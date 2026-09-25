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
	contextPlan, err := model.PlanEvaluationWithContext([]bcg.PropertyTarget{target}, context)
	if err != nil {
		t.Fatalf("context plan: %v", err)
	}
	if !reflect.DeepEqual(contextPlan, mustPlan(t, model, target)) {
		t.Fatal("context plan widened or changed the conservative model plan")
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
	// This is deliberately an external-consumer proof: the model-level plan,
	// Dataset snapshot evaluation, and dynamically recorded provenance agree
	// without relying on internal graph types or display strings.
	builder, err := bcg.NewDatasetBuilder(am, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := am.WithDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot.Evaluate(mustPlanAddress(t, am, total), context); err != nil {
		t.Fatalf("snapshot aggregate evaluation: %v", err)
	}
	explained, err := snapshot.Explain(mustPlanAddress(t, am, total), context)
	if err != nil {
		t.Fatal(err)
	}
	assertPlanCoversExplanation(t, aggregatePlan, explained.Root())
}

func TestPublicPlanEvaluationFunctionRequirement(t *testing.T) {
	model, targetAddress, context, runtime := phase34BenchmarkFunction(t)
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
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.WithDataset(model, dataset)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot.Evaluate(targetAddress, context); err != nil {
		t.Fatalf("snapshot function evaluation: %v", err)
	}
}

func mustPlanAddress(t testing.TB, model bcg.Model, definition bcg.PropertyDefinition) bcg.PropertyAddress {
	t.Helper()
	for _, entity := range model.Entities() {
		if entity.TypeID().Equal(definition.Owner().ID()) {
			address, err := bcg.EntityProperty(entity.ID(), definition.ID())
			if err != nil {
				t.Fatal(err)
			}
			return address
		}
	}
	t.Fatal("no embedded entity owns planned definition")
	return bcg.PropertyAddress{}
}

func assertPlanCoversExplanation(t testing.TB, plan bcg.DependencyPlan, node bcg.ExplanationNode) {
	t.Helper()
	allowed := make(map[bcg.ID]struct{})
	for _, definition := range append(plan.Targets(), plan.DependentPropertyDefinitions()...) {
		allowed[definition.ID()] = struct{}{}
	}
	var walk func(bcg.ExplanationNode)
	walk = func(current bcg.ExplanationNode) {
		if property, ok := current.Property(); ok {
			if _, exists := allowed[property.DefinitionID()]; !exists {
				t.Fatalf("observed property definition %s is absent from dependency plan", property.DefinitionID())
			}
		}
		for _, child := range current.Children() {
			walk(child)
		}
	}
	walk(node)
}

func mustPlan(t testing.TB, m bcg.Model, target bcg.PropertyTarget) bcg.DependencyPlan {
	t.Helper()
	p, err := m.PlanEvaluation([]bcg.PropertyTarget{target})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
