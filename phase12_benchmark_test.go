package bcg_test

import (
	"fmt"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

type phase12BenchmarkFixture struct {
	model    bcg.Model
	dataset  bcg.Dataset
	snapshot bcg.EvaluationSnapshot
	runtime  bcg.Runtime
	context  bcg.CalculationContext
	label    bcg.PropertyAddress
	target   bcg.PropertyTarget
	workflow canonicalWorkflow
}

func newPhase12BenchmarkFixture(b testing.TB) phase12BenchmarkFixture {
	b.Helper()
	workflow := newCanonicalWorkflow(b)
	model := mustData[bcg.Model](b)(workflow.draft.Commit(mustCommitOptions(b, bcg.NewID(), "benchmark", mustPolicy(b))))
	customer, order := phase12Entity(b, workflow.customerTypeID), phase12Entity(b, workflow.orderTypeID)
	relation := mustData[bcg.Relation](b)(bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: workflow.relationTypeID, From: order.ID(), To: customer.ID()}))
	builder := mustData[*bcg.DatasetBuilder](b)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	for _, entity := range []bcg.Entity{customer, order} {
		if err := builder.AddEntity(entity); err != nil {
			b.Fatal(err)
		}
	}
	if err := builder.AddRelation(relation); err != nil {
		b.Fatal(err)
	}
	for _, binding := range []bcg.ValueBinding{
		workflowBinding(b, order.ID(), workflow.numberID, bcg.String("benchmark")),
		workflowBinding(b, order.ID(), workflow.statusID, mustData[bcg.Value](b)(bcg.EnumValue(workflow.enumID, workflow.openID))),
	} {
		if err := builder.AddValueBinding(binding); err != nil {
			b.Fatal(err)
		}
	}
	dataset := mustData[bcg.Dataset](b)(builder.Build())
	runtime := mustData[bcg.Runtime](b)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](b)(bcg.NewFunctionRegistry(workflowFunction{workflow.functionRef}))))
	snapshot := mustData[bcg.EvaluationSnapshot](b)(runtime.WithDataset(model, dataset))
	context := mustData[bcg.CalculationContext](b)(bcg.NewCalculationContextAt(mustData[bcg.DateTime](b)(bcg.ParseDateTime("2026-06-01T00:00:00Z"))))
	label := mustData[bcg.PropertyAddress](b)(bcg.EntityProperty(order.ID(), workflow.labelID))
	var definition bcg.PropertyDefinition
	for _, candidate := range model.PropertyDefinitionDeclarations() {
		if candidate.ID() == workflow.labelID {
			definition = candidate
			break
		}
	}
	return phase12BenchmarkFixture{model: model, dataset: dataset, snapshot: snapshot, runtime: runtime, context: context, label: label, target: mustData[bcg.PropertyTarget](b)(bcg.PropertyDefinitionTarget(definition)), workflow: workflow}
}

func BenchmarkPhase12ModelValidateCommit(b *testing.B) {
	workflow := newCanonicalWorkflow(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if report, err := workflow.draft.Validate(); err != nil || !report.OK() {
			b.Fatal(err, report.Issues())
		}
		if _, err := workflow.draft.Commit(mustCommitOptions(b, bcg.NewID(), "benchmark", mustPolicy(b))); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12BCGV1DecodeIntoV2(b *testing.B) {
	data := []byte(publicArtifact)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := bcg.Unmarshal(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12BCGV2EncodeDecode(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	artifact := mustData[bcg.Artifact](b)(bcg.NewArtifact(fixture.model))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded := mustData[[]byte](b)(bcg.Marshal(artifact))
		if _, err := bcg.Unmarshal(encoded); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12SemanticDigest(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fixture.model.SemanticDigest()
	}
}

func BenchmarkPhase12ModelDiffAndEvolution(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	edit := mustData[*bcg.Draft](b)(fixture.model.Edit())
	owner := mustData[bcg.PropertyOwner](b)(bcg.EntityTypeOwner(fixture.workflow.orderTypeID))
	changed := mustData[bcg.PropertyDefinition](b)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: fixture.workflow.noteID, Owner: owner, Name: "comment", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()}))
	if err := edit.ReplacePropertyDefinition(fixture.workflow.noteID, changed); err != nil {
		b.Fatal(err)
	}
	next := mustData[bcg.Model](b)(edit.Commit(mustCommitOptions(b, bcg.NewID(), "next", mustPolicy(b))))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := bcg.CompareModels(fixture.model, next); err != nil {
			b.Fatal(err)
		}
		if _, err := bcg.AnalyzeEvolution(fixture.model, next); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12ModelDiffLargeSchema(b *testing.B) {
	oldModel, newModel := newWideSchemaBenchmarkModels(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := bcg.CompareModels(oldModel, newModel); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12AnalyzeEvolutionLargeSchema(b *testing.B) {
	oldModel, newModel := newWideSchemaBenchmarkModels(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := bcg.AnalyzeEvolution(oldModel, newModel); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12DatasetBuild(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	entity := phase12Entity(b, fixture.workflow.customerTypeID)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := mustData[*bcg.DatasetBuilder](b)(bcg.NewDatasetBuilder(fixture.model, bcg.NewID()))
		if err := builder.AddEntity(entity); err != nil {
			b.Fatal(err)
		}
		if _, err := builder.Build(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12DatasetPatchOverLargeDataset(b *testing.B) {
	model, typeID, _, _ := datasetInputModel(b, false)
	builder := mustData[*bcg.DatasetBuilder](b)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	for i := 0; i < 512; i++ {
		if err := builder.AddEntity(phase12Entity(b, typeID)); err != nil {
			b.Fatal(err)
		}
	}
	base := mustData[bcg.Dataset](b)(builder.Build())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		patchBuilder := mustData[*bcg.DatasetPatchBuilder](b)(bcg.NewDatasetPatchBuilder(base.ID()))
		if err := patchBuilder.AddEntity(phase12Entity(b, typeID)); err != nil {
			b.Fatal(err)
		}
		if _, report, err := base.Apply(model, bcg.NewID(), mustData[bcg.DatasetPatch](b)(patchBuilder.Build())); err != nil || !report.OK() {
			b.Fatal(err, report.Issues())
		}
	}
}

func BenchmarkPhase12DatasetRebase(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	edit := mustData[*bcg.Draft](b)(fixture.model.Edit())
	owner := mustData[bcg.PropertyOwner](b)(bcg.EntityTypeOwner(fixture.workflow.orderTypeID))
	changed := mustData[bcg.PropertyDefinition](b)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: fixture.workflow.noteID, Owner: owner, Name: "comment", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()}))
	if err := edit.ReplacePropertyDefinition(fixture.workflow.noteID, changed); err != nil {
		b.Fatal(err)
	}
	next := mustData[bcg.Model](b)(edit.Commit(mustCommitOptions(b, bcg.NewID(), "next", mustPolicy(b))))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, report, err := fixture.dataset.Rebase(fixture.model, next, bcg.NewID()); err != nil || !report.OK() {
			b.Fatal(err, report.Issues())
		}
	}
}

func BenchmarkPhase12BCGDEncodeDecode(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	artifact := mustData[bcg.DatasetArtifact](b)(bcg.NewDatasetArtifact(fixture.dataset))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded := mustData[[]byte](b)(bcg.MarshalDataset(artifact))
		if _, err := bcg.UnmarshalDataset(fixture.model, encoded); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12EvaluationSnapshotPlanEvaluateBatch(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fixture.runtime.WithDataset(fixture.model, fixture.dataset); err != nil {
			b.Fatal(err)
		}
		if _, err := fixture.model.PlanEvaluation([]bcg.PropertyTarget{fixture.target}); err != nil {
			b.Fatal(err)
		}
		if _, err := fixture.snapshot.Evaluate(fixture.label, fixture.context); err != nil {
			b.Fatal(err)
		}
		if _, err := fixture.snapshot.EvaluateBatch([]bcg.PropertyAddress{fixture.label, fixture.label}, fixture.context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12EvaluationSnapshotBuild(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fixture.runtime.WithDataset(fixture.model, fixture.dataset); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12PlanEvaluation(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fixture.model.PlanEvaluation([]bcg.PropertyTarget{fixture.target}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12RepeatedEvaluate(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fixture.snapshot.Evaluate(fixture.label, fixture.context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12EvaluateBatch(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fixture.snapshot.EvaluateBatch([]bcg.PropertyAddress{fixture.label, fixture.label}, fixture.context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12FunctionCallOverhead(b *testing.B) {
	fixture := newPhase12BenchmarkFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := fixture.snapshot.Evaluate(fixture.label, fixture.context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase12IntegrityLargeKeysAndRelations(b *testing.B) {
	fixture := newLargeIntegrityBenchmarkFixture(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fixture.snapshot.CheckStructuralIntegrity()
	}
}

func newWideSchemaBenchmarkModels(b testing.TB) (bcg.Model, bcg.Model) {
	b.Helper()
	logical := mustData[bcg.Model](b)(bcg.NewModel(bcg.NewID(), "wide-schema"))
	draft := mustData[*bcg.Draft](b)(logical.Draft())
	typeID := bcg.NewID()
	owner := mustData[bcg.PropertyOwner](b)(bcg.EntityTypeOwner(typeID))
	ids := make([]bcg.ID, 256)
	for index := range ids {
		ids[index] = bcg.NewID()
		definition := mustData[bcg.PropertyDefinition](b)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
			ID: ids[index], Owner: owner, Name: fmt.Sprintf("value_%03d", index), ValueType: bcg.StringType,
			TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation(),
		}))
		if err := draft.AddPropertyDefinition(definition); err != nil {
			b.Fatal(err)
		}
	}
	typeDefinition := mustData[bcg.EntityType](b)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "wide", PropertyDefinitions: ids}))
	if err := draft.AddEntityType(typeDefinition); err != nil {
		b.Fatal(err)
	}
	oldModel := mustData[bcg.Model](b)(draft.Commit(mustCommitOptions(b, bcg.NewID(), "old", mustPolicy(b))))
	edit := mustData[*bcg.Draft](b)(oldModel.Edit())
	changed := mustData[bcg.PropertyDefinition](b)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID: ids[len(ids)-1], Owner: owner, Name: "renamed", ValueType: bcg.StringType,
		TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation(),
	}))
	if err := edit.ReplacePropertyDefinition(changed.ID(), changed); err != nil {
		b.Fatal(err)
	}
	return oldModel, mustData[bcg.Model](b)(edit.Commit(mustCommitOptions(b, bcg.NewID(), "new", mustPolicy(b))))
}

func newLargeIntegrityBenchmarkFixture(b testing.TB) phase12BenchmarkFixture {
	b.Helper()
	fixture := newPhase12BenchmarkFixture(b)
	builder := mustData[*bcg.DatasetBuilder](b)(bcg.NewDatasetBuilder(fixture.model, bcg.NewID()))
	customer := phase12Entity(b, fixture.workflow.customerTypeID)
	if err := builder.AddEntity(customer); err != nil {
		b.Fatal(err)
	}
	for index := 0; index < 256; index++ {
		order := phase12Entity(b, fixture.workflow.orderTypeID)
		if err := builder.AddEntity(order); err != nil {
			b.Fatal(err)
		}
		relation := mustData[bcg.Relation](b)(bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: fixture.workflow.relationTypeID, From: order.ID(), To: customer.ID()}))
		if err := builder.AddRelation(relation); err != nil {
			b.Fatal(err)
		}
		for _, binding := range []bcg.ValueBinding{
			workflowBinding(b, order.ID(), fixture.workflow.numberID, bcg.String(fmt.Sprintf("order-%03d", index))),
			workflowBinding(b, order.ID(), fixture.workflow.statusID, mustData[bcg.Value](b)(bcg.EnumValue(fixture.workflow.enumID, fixture.workflow.openID))),
		} {
			if err := builder.AddValueBinding(binding); err != nil {
				b.Fatal(err)
			}
		}
	}
	dataset := mustData[bcg.Dataset](b)(builder.Build())
	fixture.dataset = dataset
	fixture.snapshot = mustData[bcg.EvaluationSnapshot](b)(fixture.runtime.WithDataset(fixture.model, dataset))
	return fixture
}
