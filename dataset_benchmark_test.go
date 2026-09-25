package bcg_test

import (
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

func BenchmarkDatasetRepeatedScalarEvaluate(b *testing.B) {
	model, typeID, propertyID, _ := datasetInputModel(b, false)
	entityID := bcg.NewID()
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		b.Fatal(err)
	}
	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: typeID})
	if err != nil {
		b.Fatal(err)
	}
	if err := builder.AddEntity(entity); err != nil {
		b.Fatal(err)
	}
	address, err := bcg.EntityProperty(entityID, propertyID)
	if err != nil {
		b.Fatal(err)
	}
	target, err := bcg.ObjectPropertyBindingTarget(address)
	if err != nil {
		b.Fatal(err)
	}
	value, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal("5")))
	if err != nil {
		b.Fatal(err)
	}
	binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: value})
	if err != nil {
		b.Fatal(err)
	}
	if err := builder.AddValueBinding(binding); err != nil {
		b.Fatal(err)
	}
	dataset, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		b.Fatal(err)
	}
	at, err := bcg.ParseDateTime("2026-01-15T00:00:00Z")
	if err != nil {
		b.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := snapshot.Evaluate(address, context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDatasetCombinedGraphBuild(b *testing.B) {
	model, dataset, _, _ := benchmarkDatasetTraversalFixture(b, 1_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := model.WithDataset(dataset); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDatasetLargeRelationTraversal(b *testing.B) {
	model, dataset, target, context := benchmarkDatasetTraversalFixture(b, 1_000)
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := snapshot.Evaluate(target, context); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkDatasetTraversalFixture(b testing.TB, relationCount int) (bcg.Model, bcg.Dataset, bcg.PropertyAddress, bcg.CalculationContext) {
	b.Helper()
	typeID, relationTypeID, countID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		b.Fatal(err)
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "Node", PropertyDefinitions: []bcg.ID{countID}})
	if err != nil {
		b.Fatal(err)
	}
	relationType, err := bcg.NewRelationType(bcg.RelationTypeSpec{ID: relationTypeID, Name: "edge"})
	if err != nil {
		b.Fatal(err)
	}
	source, err := bcg.SelfTraversal(bcg.TraverseOutgoing, relationTypeID)
	if err != nil {
		b.Fatal(err)
	}
	expression, err := bcg.Count(source)
	if err != nil {
		b.Fatal(err)
	}
	definition, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: countID, Owner: owner, Name: "count", ValueType: bcg.IntegerType, TemporalMode: bcg.TemporalScalar, Computation: mustDatasetDerived(b, expression)})
	if err != nil {
		b.Fatal(err)
	}
	logical, err := bcg.NewModel(bcg.NewID(), "dataset-benchmark")
	if err != nil {
		b.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		b.Fatal(err)
	}
	if err := draft.AddEntityType(entityType); err != nil {
		b.Fatal(err)
	}
	if err := draft.AddRelationType(relationType); err != nil {
		b.Fatal(err)
	}
	if err := draft.AddPropertyDefinition(definition); err != nil {
		b.Fatal(err)
	}
	policy, err := bcg.NewPrecisionPolicy(2, bcg.RoundHalfEven)
	if err != nil {
		b.Fatal(err)
	}
	options, err := bcg.NewCommitOptions(bcg.NewID(), "v1", policy)
	if err != nil {
		b.Fatal(err)
	}
	model, err := draft.Commit(options)
	if err != nil {
		b.Fatal(err)
	}
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		b.Fatal(err)
	}
	parent := bcg.NewID()
	parentEntity, err := bcg.NewEntity(bcg.EntitySpec{ID: parent, TypeID: typeID})
	if err != nil {
		b.Fatal(err)
	}
	if err := builder.AddEntity(parentEntity); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < relationCount; i++ {
		child := bcg.NewID()
		entity, err := bcg.NewEntity(bcg.EntitySpec{ID: child, TypeID: typeID})
		if err != nil {
			b.Fatal(err)
		}
		if err := builder.AddEntity(entity); err != nil {
			b.Fatal(err)
		}
		relation, err := bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: relationTypeID, From: parent, To: child})
		if err != nil {
			b.Fatal(err)
		}
		if err := builder.AddRelation(relation); err != nil {
			b.Fatal(err)
		}
	}
	dataset, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}
	target, err := bcg.EntityProperty(parent, countID)
	if err != nil {
		b.Fatal(err)
	}
	at, err := bcg.ParseDateTime("2026-01-15T00:00:00Z")
	if err != nil {
		b.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		b.Fatal(err)
	}
	return model, dataset, target, context
}
