package bcg_test

import (
	"errors"
	"strconv"
	"sync"
	"testing"

	bcg "github.com/shmatovv/bcg-engine"
)

func TestDatasetSnapshotEvaluatesDerivedPropertyAndSession(t *testing.T) {
	modelID, typeID, quantityID, priceID, totalID := bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID()
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		t.Fatal(err)
	}
	quantityRef, err := bcg.SelfProperty(quantityID)
	if err != nil {
		t.Fatal(err)
	}
	priceRef, err := bcg.SelfProperty(priceID)
	if err != nil {
		t.Fatal(err)
	}
	totalExpression, err := bcg.Multiply(quantityRef, priceRef)
	if err != nil {
		t.Fatal(err)
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "Order", PropertyDefinitions: []bcg.ID{quantityID, priceID, totalID}})
	if err != nil {
		t.Fatal(err)
	}
	definitions := []bcg.PropertyDefinition{}
	for _, spec := range []bcg.PropertyDefinitionSpec{
		{ID: quantityID, Owner: owner, Name: "quantity", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()},
		{ID: priceID, Owner: owner, Name: "price", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()},
		{ID: totalID, Owner: owner, Name: "total", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: mustDatasetDerived(t, totalExpression)},
	} {
		value, err := bcg.NewPropertyDefinition(spec)
		if err != nil {
			t.Fatal(err)
		}
		definitions = append(definitions, value)
	}
	logical, err := bcg.NewModel(modelID, "dataset-order")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	for _, definition := range definitions {
		if err := draft.AddPropertyDefinition(definition); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := bcg.NewPrecisionPolicy(2, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	options, err := bcg.NewCommitOptions(bcg.NewID(), "v1", policy)
	if err != nil {
		t.Fatal(err)
	}
	model, err := draft.Commit(options)
	if err != nil {
		t.Fatal(err)
	}

	datasetID, entityID := bcg.NewID(), bcg.NewID()
	builder, err := bcg.NewDatasetBuilder(model, datasetID)
	if err != nil {
		t.Fatal(err)
	}
	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		definition bcg.ID
		value      string
	}{{quantityID, "10"}, {priceID, "500"}} {
		address, err := bcg.EntityProperty(entityID, item.definition)
		if err != nil {
			t.Fatal(err)
		}
		target, err := bcg.ObjectPropertyBindingTarget(address)
		if err != nil {
			t.Fatal(err)
		}
		bindingValue, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal(item.value)))
		if err != nil {
			t.Fatal(err)
		}
		binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: bindingValue})
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.AddValueBinding(binding); err != nil {
			t.Fatal(err)
		}
	}
	if report, err := builder.Validate(); err != nil || !report.OK() {
		t.Fatalf("Dataset.Validate = %#v, %v", report.Issues(), err)
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	entities := dataset.Entities()
	entities[0] = bcg.Entity{}
	if stored, ok := dataset.Entity(entityID); !ok || !stored.ID().Equal(entityID) {
		t.Fatal("Dataset entity accessor exposed mutable collection storage")
	}
	bindings := dataset.ValueBindings()
	bindings[0] = bcg.ValueBinding{}
	if stored := dataset.ValueBindings()[0]; !stored.IsValid() {
		t.Fatal("Dataset binding accessor exposed mutable collection storage")
	}
	if _, err := builder.Build(); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("second Dataset.Build error = %v", err)
	}
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	contextAt, err := bcg.ParseDateTime("2026-01-15T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(contextAt)
	if err != nil {
		t.Fatal(err)
	}
	totalAddress, err := bcg.EntityProperty(entityID, totalID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Evaluate(totalAddress, context)
	if err != nil {
		t.Fatal(err)
	}
	assertDatasetDecimal(t, result, "5000")
	derivedBinding, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	if err := derivedBinding.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	derivedTarget, err := bcg.ObjectPropertyBindingTarget(totalAddress)
	if err != nil {
		t.Fatal(err)
	}
	derivedValue, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal("1")))
	if err != nil {
		t.Fatal(err)
	}
	illegalBinding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: derivedTarget, Value: derivedValue})
	if err != nil {
		t.Fatal(err)
	}
	if err := derivedBinding.AddValueBinding(illegalBinding); err != nil {
		t.Fatal(err)
	}
	if _, err := derivedBinding.Build(); err == nil {
		t.Fatal("Dataset base binding for Derived property unexpectedly succeeded")
	}
	many, err := snapshot.EvaluateMany([]bcg.PropertyAddress{totalAddress, totalAddress}, context)
	if err != nil || len(many) != 2 {
		t.Fatalf("Dataset.EvaluateMany = %#v, %v", many, err)
	}
	assertDatasetDecimal(t, many[0], "5000")
	session, err := snapshot.NewSession(context)
	if err != nil {
		t.Fatal(err)
	}
	fromSession, err := session.Evaluate(totalAddress)
	if err != nil {
		t.Fatal(err)
	}
	assertDatasetDecimal(t, fromSession, "5000")
}

func TestDatasetRejectsTypeBindingAndEmbeddedTarget(t *testing.T) {
	model, typeID, propertyID, embeddedID := datasetInputModel(t, true)
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		t.Fatal(err)
	}
	typeTarget, err := bcg.TypePropertyBindingTarget(owner, propertyID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal("1")))
	if err != nil {
		t.Fatal(err)
	}
	typeBinding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: typeTarget, Value: value})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddValueBinding(typeBinding); err != nil {
		t.Fatal(err)
	}
	embeddedAddress, err := bcg.EntityProperty(embeddedID, propertyID)
	if err != nil {
		t.Fatal(err)
	}
	objectTarget, err := bcg.ObjectPropertyBindingTarget(embeddedAddress)
	if err != nil {
		t.Fatal(err)
	}
	objectBinding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: objectTarget, Value: value})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddValueBinding(objectBinding); err != nil {
		t.Fatal(err)
	}
	report, err := builder.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if report.OK() {
		t.Fatal("invalid Dataset validation unexpectedly succeeded")
	}
	if _, err := builder.Build(); err == nil {
		t.Fatal("invalid Dataset Build unexpectedly succeeded")
	}
}

func TestDatasetValidationAppliesCombinedOwnershipAndValueRules(t *testing.T) {
	model, typeID, propertyID, embeddedID := datasetInputModel(t, true)
	rejected := func(builder *bcg.DatasetBuilder) {
		t.Helper()
		if report, err := builder.Validate(); err != nil || report.OK() {
			t.Fatalf("invalid Dataset validation = %#v, %v", report.Issues(), err)
		}
		if _, err := builder.Build(); err == nil {
			t.Fatal("invalid Dataset Build unexpectedly succeeded")
		}
	}

	collision, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	collidingEntity, err := bcg.NewEntity(bcg.EntitySpec{ID: embeddedID, TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	if err := collision.AddEntity(collidingEntity); err != nil {
		t.Fatal(err)
	}
	rejected(collision)

	unknownType, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	danglingEntity, err := bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: bcg.NewID()})
	if err != nil {
		t.Fatal(err)
	}
	if err := unknownType.AddEntity(danglingEntity); err != nil {
		t.Fatal(err)
	}
	rejected(unknownType)

	wrongValue, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	datasetEntity, err := bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	if err := wrongValue.AddEntity(datasetEntity); err != nil {
		t.Fatal(err)
	}
	address, err := bcg.EntityProperty(datasetEntity.ID(), propertyID)
	if err != nil {
		t.Fatal(err)
	}
	target, err := bcg.ObjectPropertyBindingTarget(address)
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := bcg.DefinedBindingValue(bcg.Bool(true))
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: boolean})
	if err != nil {
		t.Fatal(err)
	}
	if err := wrongValue.AddValueBinding(binding); err != nil {
		t.Fatal(err)
	}
	rejected(wrongValue)

	valid, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	if err := valid.AddEntity(datasetEntity); err != nil {
		t.Fatal(err)
	}
	dataset, err := valid.Build()
	if err != nil {
		t.Fatal(err)
	}
	otherModel, _, _, _ := datasetInputModel(t, false)
	if _, err := otherModel.WithDataset(dataset); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("Dataset accepted by a different ModelVersion: %v", err)
	}
}

func TestDatasetRejectsRelationCollisionAndMissingEndpoint(t *testing.T) {
	fixture, model := publicAggregateModel(t)
	childID := bcg.NewID()
	newChild := func(t *testing.T) bcg.Entity {
		t.Helper()
		entity, err := bcg.NewEntity(bcg.EntitySpec{ID: childID, TypeID: fixture.childType})
		if err != nil {
			t.Fatal(err)
		}
		return entity
	}

	collision, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	if err := collision.AddEntity(newChild(t)); err != nil {
		t.Fatal(err)
	}
	collidingRelation, err := bcg.NewRelation(bcg.RelationSpec{ID: fixture.relationAWeightAddress.ObjectID(), TypeID: fixture.relationType, From: fixture.parent, To: childID})
	if err != nil {
		t.Fatal(err)
	}
	if err := collision.AddRelation(collidingRelation); err != nil {
		t.Fatal(err)
	}
	if _, err := collision.Build(); err == nil {
		t.Fatal("Dataset relation ID collision unexpectedly succeeded")
	}

	missingEndpoint, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	if err := missingEndpoint.AddEntity(newChild(t)); err != nil {
		t.Fatal(err)
	}
	danglingRelation, err := bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: fixture.relationType, From: childID, To: bcg.NewID()})
	if err != nil {
		t.Fatal(err)
	}
	if err := missingEndpoint.AddRelation(danglingRelation); err != nil {
		t.Fatal(err)
	}
	if _, err := missingEndpoint.Build(); err == nil {
		t.Fatal("Dataset relation with missing endpoint unexpectedly succeeded")
	}

	unknownType, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	if err := unknownType.AddEntity(newChild(t)); err != nil {
		t.Fatal(err)
	}
	unknownRelation, err := bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: bcg.NewID(), From: childID, To: fixture.parent})
	if err != nil {
		t.Fatal(err)
	}
	if err := unknownType.AddRelation(unknownRelation); err != nil {
		t.Fatal(err)
	}
	if _, err := unknownType.Build(); err == nil {
		t.Fatal("Dataset relation with unknown RelationType unexpectedly succeeded")
	}

	embeddedOnly, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	modelRelation, err := bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: fixture.relationType, From: fixture.parent, To: fixture.childA})
	if err != nil {
		t.Fatal(err)
	}
	if err := embeddedOnly.AddRelation(modelRelation); err != nil {
		t.Fatal(err)
	}
	if _, err := embeddedOnly.Build(); err == nil {
		t.Fatal("Dataset relation between two embedded entities unexpectedly succeeded")
	}
}

func TestDatasetObjectBindingsRespectEffectivePeriods(t *testing.T) {
	model, typeID, propertyID, _ := datasetInputModel(t, false)
	entityID := bcg.NewID()
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	targetAddress, err := bcg.EntityProperty(entityID, propertyID)
	if err != nil {
		t.Fatal(err)
	}
	target, err := bcg.ObjectPropertyBindingTarget(targetAddress)
	if err != nil {
		t.Fatal(err)
	}
	start, err := bcg.ParseDateTime("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	switchAt, err := bcg.ParseDateTime("2026-02-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	firstPeriod, err := bcg.NewEffectivePeriod(&start, &switchAt)
	if err != nil {
		t.Fatal(err)
	}
	secondPeriod, err := bcg.NewEffectivePeriod(&switchAt, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []struct {
		value  string
		period bcg.EffectivePeriod
	}{{value: "10", period: firstPeriod}, {value: "20", period: secondPeriod}} {
		value, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal(candidate.value)))
		if err != nil {
			t.Fatal(err)
		}
		binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: value, EffectivePeriod: &candidate.period})
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.AddValueBinding(binding); err != nil {
			t.Fatal(err)
		}
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []struct {
		at   string
		want string
	}{{at: "2026-01-15T00:00:00Z", want: "10"}, {at: "2026-02-01T00:00:00Z", want: "20"}} {
		at, err := bcg.ParseDateTime(candidate.at)
		if err != nil {
			t.Fatal(err)
		}
		context, err := bcg.NewCalculationContextAt(at)
		if err != nil {
			t.Fatal(err)
		}
		result, err := snapshot.Evaluate(targetAddress, context)
		if err != nil {
			t.Fatal(err)
		}
		assertDatasetDecimal(t, result, candidate.want)
	}
}

func TestDatasetRelationToEmbeddedEntityParticipatesInAggregate(t *testing.T) {
	fixture, model := publicAggregateModel(t)
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	childID, relationID := bcg.NewID(), bcg.NewID()
	child, err := bcg.NewEntity(bcg.EntitySpec{ID: childID, TypeID: fixture.childType})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddEntity(child); err != nil {
		t.Fatal(err)
	}
	relation, err := bcg.NewRelation(bcg.RelationSpec{ID: relationID, TypeID: fixture.relationType, From: fixture.parent, To: childID})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddRelation(relation); err != nil {
		t.Fatal(err)
	}
	address, err := bcg.EntityProperty(childID, fixture.childAValueAddress.DefinitionID())
	if err != nil {
		t.Fatal(err)
	}
	target, err := bcg.ObjectPropertyBindingTarget(address)
	if err != nil {
		t.Fatal(err)
	}
	value, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal("40")))
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: value})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddValueBinding(binding); err != nil {
		t.Fatal(err)
	}
	relationAddress, err := bcg.RelationProperty(relationID, fixture.relationAWeightAddress.DefinitionID())
	if err != nil {
		t.Fatal(err)
	}
	relationTarget, err := bcg.ObjectPropertyBindingTarget(relationAddress)
	if err != nil {
		t.Fatal(err)
	}
	relationBinding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: relationTarget, Value: value})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddValueBinding(relationBinding); err != nil {
		t.Fatal(err)
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	relations := dataset.Relations()
	relations[0] = bcg.Relation{}
	if stored, ok := dataset.Relation(relationID); !ok || !stored.ID().Equal(relationID) {
		t.Fatal("Dataset relation accessor exposed mutable collection storage")
	}
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	at, err := bcg.ParseDateTime("2026-01-15T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Evaluate(fixture.totalAddress, context)
	if err != nil {
		t.Fatal(err)
	}
	// The embedded model contributes 10+20; the Dataset relation contributes 40.
	assertDatasetDecimal(t, result, "70")
	relationResult, err := snapshot.Evaluate(relationAddress, context)
	if err != nil {
		t.Fatal(err)
	}
	assertDatasetDecimal(t, relationResult, "40")
	explained, err := snapshot.Explain(fixture.totalAddress, context)
	if err != nil {
		t.Fatal(err)
	}
	if !explanationContainsProperty(explained.Root(), address) {
		t.Fatal("Dataset aggregate dependency is absent from Explain")
	}
	for _, mode := range []bcg.ImpactMode{bcg.StructuralImpact, bcg.EffectiveImpact, bcg.ObservedImpact} {
		impacted, err := snapshot.AnalyzeImpact(mode, address, context)
		if err != nil {
			t.Fatal(err)
		}
		if impacted.Len() == 0 {
			t.Fatalf("Dataset aggregate dependency is absent from %s Impact", mode)
		}
		if datasetID, ok := impacted.DatasetID(); !ok || !datasetID.Equal(dataset.ID()) {
			t.Fatal("Dataset-backed Impact lost its effective source identity")
		}
	}
}

func TestDatasetRelationTraversalDirections(t *testing.T) {
	typeID, relationTypeID := bcg.NewID(), bcg.NewID()
	outID, inID, bothID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		t.Fatal(err)
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "Node", PropertyDefinitions: []bcg.ID{outID, inID, bothID}})
	if err != nil {
		t.Fatal(err)
	}
	relationType, err := bcg.NewRelationType(bcg.RelationTypeSpec{ID: relationTypeID, Name: "link"})
	if err != nil {
		t.Fatal(err)
	}
	makeCount := func(direction bcg.TraversalDirection) bcg.PropertyComputation {
		source, err := bcg.SelfTraversal(direction, relationTypeID)
		if err != nil {
			t.Fatal(err)
		}
		expression, err := bcg.Count(source)
		if err != nil {
			t.Fatal(err)
		}
		return mustDatasetDerived(t, expression)
	}
	logical, err := bcg.NewModel(bcg.NewID(), "dataset-traversal")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddRelationType(relationType); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []bcg.PropertyDefinitionSpec{
		{ID: outID, Owner: owner, Name: "out", ValueType: bcg.IntegerType, TemporalMode: bcg.TemporalScalar, Computation: makeCount(bcg.TraverseOutgoing)},
		{ID: inID, Owner: owner, Name: "in", ValueType: bcg.IntegerType, TemporalMode: bcg.TemporalScalar, Computation: makeCount(bcg.TraverseIncoming)},
		{ID: bothID, Owner: owner, Name: "both", ValueType: bcg.IntegerType, TemporalMode: bcg.TemporalScalar, Computation: makeCount(bcg.TraverseBoth)},
	} {
		definition, err := bcg.NewPropertyDefinition(spec)
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.AddPropertyDefinition(definition); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := bcg.NewPrecisionPolicy(2, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	options, err := bcg.NewCommitOptions(bcg.NewID(), "v1", policy)
	if err != nil {
		t.Fatal(err)
	}
	model, err := draft.Commit(options)
	if err != nil {
		t.Fatal(err)
	}
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	a, b := bcg.NewID(), bcg.NewID()
	for _, id := range []bcg.ID{a, b} {
		entity, err := bcg.NewEntity(bcg.EntitySpec{ID: id, TypeID: typeID})
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
	}
	relation, err := bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: relationTypeID, From: a, To: b})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddRelation(relation); err != nil {
		t.Fatal(err)
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	at, err := bcg.ParseDateTime("2026-01-15T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	assertDatasetIntegerAt(t, snapshot, a, outID, context, 1)
	assertDatasetIntegerAt(t, snapshot, b, inID, context, 1)
	assertDatasetIntegerAt(t, snapshot, a, bothID, context, 1)
}

func TestDatasetInducedDependencyCycleIsRejectedBySnapshot(t *testing.T) {
	typeID, relationTypeID, derivedID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		t.Fatal(err)
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "Node", PropertyDefinitions: []bcg.ID{derivedID}})
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := bcg.EntityTypeEndpoint(typeID)
	if err != nil {
		t.Fatal(err)
	}
	relationType, err := bcg.NewRelationType(bcg.RelationTypeSpec{ID: relationTypeID, Name: "next", From: endpoint, To: endpoint})
	if err != nil {
		t.Fatal(err)
	}
	source, err := bcg.SelfTraversal(bcg.TraverseOutgoing, relationTypeID)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := bcg.BoundProperty(bcg.BoundEntity, derivedID)
	if err != nil {
		t.Fatal(err)
	}
	expression, err := bcg.Aggregate(source, bcg.AggregateSum, bound)
	if err != nil {
		t.Fatal(err)
	}
	derived, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: derivedID, Owner: owner, Name: "sum", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: mustDatasetDerived(t, expression)})
	if err != nil {
		t.Fatal(err)
	}
	logical, err := bcg.NewModel(bcg.NewID(), "dataset-cycle")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddRelationType(relationType); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddPropertyDefinition(derived); err != nil {
		t.Fatal(err)
	}
	policy, err := bcg.NewPrecisionPolicy(2, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	options, err := bcg.NewCommitOptions(bcg.NewID(), "v1", policy)
	if err != nil {
		t.Fatal(err)
	}
	model, err := draft.Commit(options)
	if err != nil {
		t.Fatal(err)
	}
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	a, b := bcg.NewID(), bcg.NewID()
	for _, id := range []bcg.ID{a, b} {
		entity, err := bcg.NewEntity(bcg.EntitySpec{ID: id, TypeID: typeID})
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
	}
	for _, endpoints := range [][2]bcg.ID{{a, b}, {b, a}} {
		relation, err := bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: relationTypeID, From: endpoints[0], To: endpoints[1]})
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.AddRelation(relation); err != nil {
			t.Fatal(err)
		}
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.WithDataset(dataset); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("Dataset-induced cycle error = %v", err)
	}
}

func TestDatasetScenarioAndCalculationOverrideAreSourceBound(t *testing.T) {
	model, typeID, propertyID, embeddedID := datasetInputModel(t, true)
	entityID := bcg.NewID()
	build := func(datasetID bcg.ID) bcg.Dataset {
		builder, err := bcg.NewDatasetBuilder(model, datasetID)
		if err != nil {
			t.Fatal(err)
		}
		entity, err := bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: typeID})
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
		dataset, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		return dataset
	}
	first, second := build(bcg.NewID()), build(bcg.NewID())
	snapshot, err := model.WithDataset(first)
	if err != nil {
		t.Fatal(err)
	}
	target, err := bcg.EntityProperty(entityID, propertyID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal("77")))
	if err != nil {
		t.Fatal(err)
	}
	override, err := bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{Target: target, Value: value})
	if err != nil {
		t.Fatal(err)
	}
	embeddedTarget, err := bcg.EntityProperty(embeddedID, propertyID)
	if err != nil {
		t.Fatal(err)
	}
	embeddedOverride, err := bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{Target: embeddedTarget, Value: value})
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := snapshot.NewScenario(bcg.ScenarioSpec{ID: bcg.NewID(), Name: "runtime", Overrides: []bcg.ScenarioOverride{embeddedOverride, override}})
	if err != nil {
		t.Fatal(err)
	}
	if id, bound := scenario.BaseDatasetID(); !bound || !id.Equal(first.ID()) {
		t.Fatalf("Scenario Dataset binding = %s, %v", id, bound)
	}
	at, err := bcg.ParseDateTime("2026-01-15T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	withScenario, err := context.WithScenario(scenario)
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Evaluate(target, withScenario)
	if err != nil {
		t.Fatal(err)
	}
	assertDatasetDecimal(t, result, "77")
	calculationOverride, err := bcg.NewOverride(target, bcg.DecimalValue(bcg.MustParseDecimal("88")))
	if err != nil {
		t.Fatal(err)
	}
	withCalculation, err := withScenario.WithCalculationOverrides(calculationOverride)
	if err != nil {
		t.Fatal(err)
	}
	result, err = snapshot.Evaluate(target, withCalculation)
	if err != nil {
		t.Fatal(err)
	}
	assertDatasetDecimal(t, result, "88")
	other, err := model.WithDataset(second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Evaluate(target, withScenario); err == nil {
		t.Fatal("Dataset-bound Scenario unexpectedly applied to a different Dataset")
	}
	if _, err := bcg.NewArtifact(model, scenario); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("Dataset-bound Artifact error = %v", err)
	}
}

func TestDatasetSeriesEvaluationUsesSnapshotAndSession(t *testing.T) {
	typeID, propertyID := bcg.NewID(), bcg.NewID()
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		t.Fatal(err)
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "SeriesInput", PropertyDefinitions: []bcg.ID{propertyID}})
	if err != nil {
		t.Fatal(err)
	}
	definition, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: propertyID, Owner: owner, Name: "samples", ValueType: bcg.IntegerType, TemporalMode: bcg.TemporalSeries, Computation: bcg.InputComputation()})
	if err != nil {
		t.Fatal(err)
	}
	logical, err := bcg.NewModel(bcg.NewID(), "dataset-series")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddPropertyDefinition(definition); err != nil {
		t.Fatal(err)
	}
	policy, err := bcg.NewPrecisionPolicy(2, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	model, err := draft.Commit(mustDatasetCommitOptions(t, policy))
	if err != nil {
		t.Fatal(err)
	}

	entityID := bcg.NewID()
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	points := make([]bcg.SourcePoint, 2)
	for i, text := range []string{"2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z"} {
		at, err := bcg.ParseDateTime(text)
		if err != nil {
			t.Fatal(err)
		}
		sample, err := bcg.DefinedSourceSample(bcg.Int64(int64((i + 1) * 10)))
		if err != nil {
			t.Fatal(err)
		}
		points[i], err = bcg.NewSourcePoint(at, sample)
		if err != nil {
			t.Fatal(err)
		}
	}
	storedSeries, err := bcg.NewPointSourceSeries(bcg.IntegerType, nil, points)
	if err != nil {
		t.Fatal(err)
	}
	bindingValue, err := bcg.SeriesBindingValue(storedSeries)
	if err != nil {
		t.Fatal(err)
	}
	target, err := bcg.EntityProperty(entityID, propertyID)
	if err != nil {
		t.Fatal(err)
	}
	bindingTarget, err := bcg.ObjectPropertyBindingTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: bindingTarget, Value: bindingValue})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddValueBinding(binding); err != nil {
		t.Fatal(err)
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	contextAt, err := bcg.ParseDateTime("2026-01-15T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(contextAt)
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.EvaluateSeries(target, context)
	if err != nil {
		t.Fatal(err)
	}
	assertDatasetSeriesIntegers(t, result, 10, 20)
	session, err := snapshot.NewSession(context)
	if err != nil {
		t.Fatal(err)
	}
	result, err = session.EvaluateSeries(target)
	if err != nil {
		t.Fatal(err)
	}
	assertDatasetSeriesIntegers(t, result, 10, 20)
}

func TestDatasetSnapshotConcurrentReads(t *testing.T) {
	model, typeID, propertyID, _ := datasetInputModel(t, false)
	entityID := bcg.NewID()
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	address, err := bcg.EntityProperty(entityID, propertyID)
	if err != nil {
		t.Fatal(err)
	}
	target, err := bcg.ObjectPropertyBindingTarget(address)
	if err != nil {
		t.Fatal(err)
	}
	value, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal("5")))
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: value})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.AddValueBinding(binding); err != nil {
		t.Fatal(err)
	}
	dataset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	at, err := bcg.ParseDateTime("2026-01-15T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := snapshot.Evaluate(address, context)
			if err != nil {
				t.Errorf("concurrent Dataset Evaluate: %v", err)
				return
			}
			assertDatasetDecimal(t, result, "5")
		}()
	}
	group.Wait()
}

func datasetInputModel(t testing.TB, embedded bool) (bcg.Model, bcg.ID, bcg.ID, bcg.ID) {
	t.Helper()
	typeID, propertyID := bcg.NewID(), bcg.NewID()
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		t.Fatal(err)
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "Input", PropertyDefinitions: []bcg.ID{propertyID}})
	if err != nil {
		t.Fatal(err)
	}
	property, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: propertyID, Owner: owner, Name: "amount", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()})
	if err != nil {
		t.Fatal(err)
	}
	logical, err := bcg.NewModel(bcg.NewID(), "dataset-input")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddPropertyDefinition(property); err != nil {
		t.Fatal(err)
	}
	embeddedID := bcg.ID{}
	if embedded {
		embeddedID = bcg.NewID()
		entity, err := bcg.NewEntity(bcg.EntitySpec{ID: embeddedID, TypeID: typeID})
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := bcg.NewPrecisionPolicy(2, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	options, err := bcg.NewCommitOptions(bcg.NewID(), "v1", policy)
	if err != nil {
		t.Fatal(err)
	}
	model, err := draft.Commit(options)
	if err != nil {
		t.Fatal(err)
	}
	return model, typeID, propertyID, embeddedID
}

func mustDatasetDerived(t testing.TB, expression bcg.Expression) bcg.PropertyComputation {
	t.Helper()
	value, err := bcg.DerivedComputation(expression)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustDatasetCommitOptions(t testing.TB, policy bcg.PrecisionPolicy) bcg.CommitOptions {
	t.Helper()
	options, err := bcg.NewCommitOptions(bcg.NewID(), "v1", policy)
	if err != nil {
		t.Fatal(err)
	}
	return options
}
func assertDatasetDecimal(t *testing.T, result bcg.Result, want string) {
	t.Helper()
	value, ok := result.Value()
	if !ok {
		t.Fatalf("result %#v has no value", result)
	}
	decimal, ok := value.Decimal()
	if !ok || decimal.String() != want {
		t.Fatalf("decimal = %v, %v; want %s", decimal, ok, want)
	}
}
func assertDatasetIntegerAt(t *testing.T, snapshot bcg.EvaluationSnapshot, entityID, definitionID bcg.ID, context bcg.CalculationContext, want int64) {
	t.Helper()
	address, err := bcg.EntityProperty(entityID, definitionID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Evaluate(address, context)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.Value()
	if !ok {
		t.Fatalf("%s has no value", address)
	}
	got, ok := value.IntegerString()
	if !ok || got != strconv.FormatInt(want, 10) {
		t.Fatalf("%s = %s, %v; want %d", address, got, ok, want)
	}
}

func assertDatasetSeriesIntegers(t *testing.T, result bcg.SeriesResult, want ...int64) {
	t.Helper()
	points := result.Points()
	if !result.IsDefined() || len(points) != len(want) {
		t.Fatalf("Series result = %#v, points = %d; want %d points", result, len(points), len(want))
	}
	for i, expected := range want {
		value, ok := points[i].Result().Value()
		if !ok {
			t.Fatalf("Series point %d has no value", i)
		}
		got, ok := value.IntegerString()
		if !ok || got != strconv.FormatInt(expected, 10) {
			t.Fatalf("Series point %d = %s, %v; want %d", i, got, ok, expected)
		}
	}
}
