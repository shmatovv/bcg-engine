package bcg_test

import (
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

func phase34BenchmarkFunction(b testing.TB) (bcg.Model, bcg.PropertyAddress, bcg.CalculationContext, bcg.Runtime) {
	b.Helper()
	ref := mustData[bcg.FunctionRef](b)(bcg.NewFunctionRef("benchmark", "identity", bcg.NewSemanticVersion(1, 0, 0)))
	signature := mustData[bcg.FunctionSignature](b)(bcg.NewFunctionSignature([]bcg.ValueType{bcg.IntegerType}, bcg.IntegerType, false))
	requirement := mustData[bcg.FunctionRequirement](b)(bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: ref, Signature: signature}))
	call := mustData[bcg.Expression](b)(bcg.CallFunction(ref, phase34Constant(b, bcg.Int64(7))))
	model, target, context := phase34DerivedModel(b, "benchmark-function", call, bcg.IntegerType, requirement)
	runtime := mustData[bcg.Runtime](b)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](b)(bcg.NewFunctionRegistry(phase34Implementation{ref: ref, call: func(args []bcg.Value) (bcg.Value, error) {
		return args[0], nil
	}}))))
	return model, target, context, runtime
}

func BenchmarkPhase34FunctionCall(b *testing.B) {
	model, target, context, runtime := phase34BenchmarkFunction(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := runtime.Evaluate(model, target, context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase34FunctionCallSession(b *testing.B) {
	model, target, context, runtime := phase34BenchmarkFunction(b)
	builder := mustData[*bcg.DatasetBuilder](b)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	dataset := mustData[bcg.Dataset](b)(builder.Build())
	snapshot := mustData[bcg.EvaluationSnapshot](b)(runtime.WithDataset(model, dataset))
	session := mustData[*bcg.Session](b)(runtime.NewSession(snapshot, context))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := session.Evaluate(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase34FunctionCallBatch(b *testing.B) {
	model, target, context, runtime := phase34BenchmarkFunction(b)
	builder := mustData[*bcg.DatasetBuilder](b)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	dataset := mustData[bcg.Dataset](b)(builder.Build())
	snapshot := mustData[bcg.EvaluationSnapshot](b)(runtime.WithDataset(model, dataset))
	targets := []bcg.PropertyAddress{target, target, target, target}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := snapshot.EvaluateBatch(targets, context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase34FunctionCallRepeatedEvaluate(b *testing.B) {
	model, target, context, runtime := phase34BenchmarkFunction(b)
	builder := mustData[*bcg.DatasetBuilder](b)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	dataset := mustData[bcg.Dataset](b)(builder.Build())
	snapshot := mustData[bcg.EvaluationSnapshot](b)(runtime.WithDataset(model, dataset))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for n := 0; n < 4; n++ {
			if _, err := snapshot.Evaluate(target, context); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkPhase34FunctionCallPlanEvaluation(b *testing.B) {
	model, _, _, _ := phase34BenchmarkFunction(b)
	var definition bcg.PropertyDefinition
	for _, candidate := range model.PropertyDefinitionDeclarations() {
		if candidate.Computation().Kind() == bcg.ComputationDerived {
			definition = candidate
			break
		}
	}
	target := mustData[bcg.PropertyTarget](b)(bcg.PropertyDefinitionTarget(definition))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := model.PlanEvaluation([]bcg.PropertyTarget{target}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase34QuantifiedAggregate(b *testing.B) {
	relationType := bcg.MustParseID("00000000-0000-4000-8000-000000001005")
	quantity := bcg.MustParseID("00000000-0000-4000-8000-000000001009")
	source := mustData[bcg.TraversalSource](b)(bcg.SelfTraversal(bcg.TraverseOutgoing, relationType))
	property := mustData[bcg.Expression](b)(bcg.BoundProperty(bcg.BoundRelation, quantity))
	truth := mustData[bcg.Expression](b)(bcg.Equal(property, property))
	expression := mustData[bcg.Expression](b)(bcg.Any(source, truth))
	model, target, context := phase34AggregateModel(b, expression, bcg.BoolType)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := model.Evaluate(target, context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhase34DistinctCount(b *testing.B) {
	relationType := bcg.MustParseID("00000000-0000-4000-8000-000000001005")
	source := mustData[bcg.TraversalSource](b)(bcg.SelfTraversal(bcg.TraverseOutgoing, relationType))
	expression := mustData[bcg.Expression](b)(bcg.DistinctCount(source, phase34Constant(b, bcg.Int64(7))))
	model, target, context := phase34AggregateModel(b, expression, bcg.IntegerType)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := model.Evaluate(target, context); err != nil {
			b.Fatal(err)
		}
	}
}
