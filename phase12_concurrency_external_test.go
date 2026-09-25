package bcg_test

import (
	"sync"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// TestPhase12ConcurrentImmutablePublicReads gives -race real concurrent work
// across every newly introduced immutable v2 boundary, including a custom
// FunctionCall and a warm Session.
func TestPhase12ConcurrentImmutablePublicReads(t *testing.T) {
	workflow := newCanonicalWorkflow(t)
	model := mustData[bcg.Model](t)(workflow.draft.Commit(mustCommitOptions(t, bcg.NewID(), "concurrent", mustPolicy(t))))
	customer, order := phase12Entity(t, workflow.customerTypeID), phase12Entity(t, workflow.orderTypeID)
	relation := mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: workflow.relationTypeID, From: order.ID(), To: customer.ID()}))
	builder := mustData[*bcg.DatasetBuilder](t)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	for _, entity := range []bcg.Entity{customer, order} {
		if err := builder.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
	}
	if err := builder.AddRelation(relation); err != nil {
		t.Fatal(err)
	}
	for _, binding := range []bcg.ValueBinding{
		workflowBinding(t, order.ID(), workflow.numberID, bcg.String("concurrent")),
		workflowBinding(t, order.ID(), workflow.statusID, mustData[bcg.Value](t)(bcg.EnumValue(workflow.enumID, workflow.openID))),
	} {
		if err := builder.AddValueBinding(binding); err != nil {
			t.Fatal(err)
		}
	}
	dataset := mustData[bcg.Dataset](t)(builder.Build())
	runtime := mustData[bcg.Runtime](t)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(workflowFunction{workflow.functionRef}))))
	snapshot := mustData[bcg.EvaluationSnapshot](t)(runtime.WithDataset(model, dataset))
	context := mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-06-01T00:00:00Z"))))
	label := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(order.ID(), workflow.labelID))
	var labelDefinition bcg.PropertyDefinition
	for _, definition := range model.PropertyDefinitionDeclarations() {
		if definition.ID() == workflow.labelID {
			labelDefinition = definition
			break
		}
	}
	if !labelDefinition.IsValid() {
		t.Fatal("label definition is absent")
	}
	definitionTarget := mustData[bcg.PropertyTarget](t)(bcg.PropertyDefinitionTarget(labelDefinition))
	session := mustData[*bcg.Session](t)(runtime.NewSession(snapshot, context))
	patchBuilder := mustData[*bcg.DatasetPatchBuilder](t)(bcg.NewDatasetPatchBuilder(dataset.ID()))
	if err := patchBuilder.SetValueBinding(workflowBinding(t, order.ID(), workflow.numberID, bcg.String("patched"))); err != nil {
		t.Fatal(err)
	}
	patched, report, err := dataset.Apply(model, bcg.NewID(), mustData[bcg.DatasetPatch](t)(patchBuilder.Build()))
	if err != nil || !report.OK() {
		t.Fatalf("Apply patch: report=%#v err=%v", report, err)
	}

	var group sync.WaitGroup
	for index := 0; index < 64; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			switch index % 8 {
			case 0:
				_ = model.SemanticDigest()
				_, _ = bcg.CompareModels(model, model)
				_, _ = bcg.AnalyzeEvolution(model, model)
			case 1:
				_ = dataset.Entities()
				_ = dataset.Relations()
				_ = dataset.ValueBindings()
				_ = dataset.ContentDigest()
			case 2:
				if _, err := model.PlanEvaluation([]bcg.PropertyTarget{definitionTarget}); err != nil {
					t.Error(err)
				}
			case 3:
				if result, err := snapshot.Evaluate(label, context); err != nil {
					t.Error(err)
				} else {
					assertWorkflowString(t, result, "concurrent")
				}
			case 4:
				if batch, err := snapshot.EvaluateBatch([]bcg.PropertyAddress{label, label}, context); err != nil || batch.Len() != 2 {
					t.Errorf("EvaluateBatch = %#v, %v", batch, err)
				}
			case 5:
				if result, err := session.Evaluate(label); err != nil {
					t.Error(err)
				} else {
					assertWorkflowString(t, result, "concurrent")
				}
			case 6:
				if _, ok := runtime.Registry().Lookup(workflow.functionRef); !ok {
					t.Error("runtime registry lost function")
				}
				if err := runtime.ValidateModel(model); err != nil {
					t.Error(err)
				}
			case 7:
				_ = patched.Entities()
				_ = patched.ValueBindings()
				_ = patched.ContentDigest()
				_ = snapshot.Model()
				_ = snapshot.Dataset()
			}
		}(index)
	}
	group.Wait()
}
