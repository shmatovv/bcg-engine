package bcg_test

import (
	"errors"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// These regressions lock down Phase 12 semantic bugs at the public boundary.
// They intentionally import only the public v2 package.
func TestEnumBusinessKeysRetainOptionIdentity(t *testing.T) {
	model, enumID, openID, closedID, entityTypeID, propertyID := phase12EnumKeyModel(t)
	first := phase12Entity(t, entityTypeID)
	second := phase12Entity(t, entityTypeID)
	builder := mustData[*bcg.DatasetBuilder](t)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	for _, entity := range []bcg.Entity{first, second} {
		if err := builder.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
	}
	phase12AddBinding(t, builder, first.ID(), propertyID, mustData[bcg.Value](t)(bcg.EnumValue(enumID, openID)))
	phase12AddBinding(t, builder, second.ID(), propertyID, mustData[bcg.Value](t)(bcg.EnumValue(enumID, closedID)))
	dataset := mustData[bcg.Dataset](t)(builder.Build())
	snapshot := mustData[bcg.EvaluationSnapshot](t)(model.WithDataset(dataset))
	for _, issue := range snapshot.CheckStructuralIntegrity().Issues() {
		if issue.Kind() == bcg.EntityKeyDuplicate {
			t.Fatalf("different enum options collapsed into one business key: %#v", issue)
		}
	}
}

func TestCompositeBusinessKeyEncodingIsInjective(t *testing.T) {
	logical := mustData[bcg.Model](t)(bcg.NewModel(bcg.NewID(), "composite-key"))
	draft := mustData[*bcg.Draft](t)(logical.Draft())
	typeID, firstID, secondID, keyID := bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID()
	owner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(typeID))
	required := mustData[bcg.PropertyComputation](t)(bcg.InputComputationWithPresence(bcg.PresenceRequired))
	for _, definition := range []bcg.PropertyDefinition{
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: firstID, Owner: owner, Name: "first", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: required})),
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: secondID, Owner: owner, Name: "second", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: required})),
	} {
		if err := draft.AddPropertyDefinition(definition); err != nil {
			t.Fatal(err)
		}
	}
	entityType := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "record", PropertyDefinitions: []bcg.ID{firstID, secondID}}))
	key := mustData[bcg.EntityKeyDefinition](t)(bcg.NewEntityKeyDefinition(bcg.EntityKeyDefinitionSpec{ID: keyID, Name: "natural", EntityTypeID: typeID, PropertyDefinitionIDs: []bcg.ID{firstID, secondID}}))
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityKeyDefinition(key); err != nil {
		t.Fatal(err)
	}
	model := mustData[bcg.Model](t)(draft.Commit(mustCommitOptions(t, bcg.NewID(), "v1", mustPolicy(t))))
	left, right := phase12Entity(t, typeID), phase12Entity(t, typeID)
	builder := mustData[*bcg.DatasetBuilder](t)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	for _, entity := range []bcg.Entity{left, right} {
		if err := builder.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
	}
	// These pairs collided when components were joined with a raw NUL byte.
	phase12AddBinding(t, builder, left.ID(), firstID, bcg.String("a\x00String:b"))
	phase12AddBinding(t, builder, left.ID(), secondID, bcg.String("c"))
	phase12AddBinding(t, builder, right.ID(), firstID, bcg.String("a"))
	phase12AddBinding(t, builder, right.ID(), secondID, bcg.String("b\x00String:c"))
	dataset := mustData[bcg.Dataset](t)(builder.Build())
	snapshot := mustData[bcg.EvaluationSnapshot](t)(model.WithDataset(dataset))
	for _, issue := range snapshot.CheckStructuralIntegrity().Issues() {
		if issue.Kind() == bcg.EntityKeyDuplicate {
			t.Fatalf("distinct composite keys collided: %#v", issue)
		}
	}
}

func TestDatasetApplyAndRebaseRequireExactSemanticModel(t *testing.T) {
	modelID, versionID, typeID, propertyID := bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID()
	decimalModel := phase12InputModel(t, modelID, versionID, typeID, propertyID, bcg.DecimalType)
	stringModel := phase12InputModel(t, modelID, versionID, typeID, propertyID, bcg.StringType)
	entity := phase12Entity(t, typeID)
	builder := mustData[*bcg.DatasetBuilder](t)(bcg.NewDatasetBuilder(decimalModel, bcg.NewID()))
	if err := builder.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	phase12AddBinding(t, builder, entity.ID(), propertyID, bcg.DecimalValue(bcg.MustParseDecimal("1")))
	dataset := mustData[bcg.Dataset](t)(builder.Build())
	patchBuilder := mustData[*bcg.DatasetPatchBuilder](t)(bcg.NewDatasetPatchBuilder(dataset.ID()))
	patch := mustData[bcg.DatasetPatch](t)(patchBuilder.Build())
	if _, _, err := dataset.Apply(stringModel, bcg.NewID(), patch); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("Apply accepted same VersionID with different semantics: %v", err)
	}
	if _, _, err := dataset.Rebase(stringModel, decimalModel, bcg.NewID()); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("Rebase accepted an old Model with mismatched semantics: %v", err)
	}

	invalid := mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: bcg.NewID()}))
	invalidBuilder := mustData[*bcg.DatasetBuilder](t)(bcg.NewDatasetBuilder(decimalModel, bcg.NewID()))
	if err := invalidBuilder.AddEntity(invalid); err != nil {
		t.Fatal(err)
	}
	if _, err := invalidBuilder.Build(); !errors.Is(err, bcg.ErrDatasetValidation) {
		t.Fatalf("Dataset validation category = %v, want ErrDatasetValidation", err)
	}
}

func TestPlanPreservesDottedFunctionRefIdentity(t *testing.T) {
	version := bcg.NewSemanticVersion(1, 0, 0)
	left := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example.math", "round", version))
	right := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example", "math.round", version))
	if left.String() != right.String() {
		t.Fatal("test setup requires colliding display forms")
	}
	signature := mustData[bcg.FunctionSignature](t)(bcg.NewFunctionSignature([]bcg.ValueType{bcg.StringType}, bcg.StringType, false))
	leftRequirement := mustData[bcg.FunctionRequirement](t)(bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: left, Signature: signature}))
	rightRequirement := mustData[bcg.FunctionRequirement](t)(bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: right, Signature: signature}))
	leftCall := mustData[bcg.Expression](t)(bcg.CallFunction(left, phase34Constant(t, bcg.String("a"))))
	rightCall := mustData[bcg.Expression](t)(bcg.CallFunction(right, phase34Constant(t, bcg.String("b"))))
	expression := mustData[bcg.Expression](t)(bcg.Concat(leftCall, rightCall))
	model, _, _ := phase34DerivedModel(t, "dotted-function-plan", expression, bcg.StringType, leftRequirement, rightRequirement)
	target := mustData[bcg.PropertyTarget](t)(bcg.PropertyDefinitionTarget(model.PropertyDefinitionDeclarations()[0]))
	plan := mustData[bcg.DependencyPlan](t)(model.PlanEvaluation([]bcg.PropertyTarget{target}))
	got := plan.FunctionRequirements()
	if len(got) != 2 || got[0].Ref().String() == "" || got[0].Ref() == got[1].Ref() {
		t.Fatalf("plan lost exact FunctionRef identity: %#v", got)
	}
}

func TestPublicHostSafetyBounds(t *testing.T) {
	options := make([]bcg.EnumOptionSpec, bcg.MaxEnumOptions+1)
	for index := range options {
		options[index] = bcg.EnumOptionSpec{ID: bcg.NewID(), Name: "option"}
	}
	if _, err := bcg.NewEnumDefinition(bcg.EnumDefinitionSpec{ID: bcg.NewID(), Name: "large", Options: options}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("enum bound error = %v", err)
	}
	components := make([]bcg.ID, bcg.MaxEntityKeyComponents+1)
	for index := range components {
		components[index] = bcg.NewID()
	}
	if _, err := bcg.NewEntityKeyDefinition(bcg.EntityKeyDefinitionSpec{ID: bcg.NewID(), Name: "large", EntityTypeID: bcg.NewID(), PropertyDefinitionIDs: components}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("key bound error = %v", err)
	}
	parameters := make([]bcg.ValueType, bcg.MaxFunctionArguments+1)
	for index := range parameters {
		parameters[index] = bcg.StringType
	}
	if _, err := bcg.NewFunctionSignature(parameters, bcg.StringType, false); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("signature bound error = %v", err)
	}
	argument := phase34Constant(t, bcg.String("x"))
	arguments := make([]bcg.Expression, bcg.MaxFunctionArguments+1)
	for index := range arguments {
		arguments[index] = argument
	}
	ref := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example", "large", bcg.NewSemanticVersion(1, 0, 0)))
	if _, err := bcg.CallFunction(ref, arguments...); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("call bound error = %v", err)
	}
}

func TestRuntimeValidatorValidatesRelationOwnedBindingLocally(t *testing.T) {
	logical := mustData[bcg.Model](t)(bcg.NewModel(bcg.NewID(), "local-relation-binding"))
	draft := mustData[*bcg.Draft](t)(logical.Draft())
	leftID, rightID, relationID, propertyID := bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID()
	owner := mustData[bcg.PropertyOwner](t)(bcg.RelationTypeOwner(relationID))
	property := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID: propertyID, Owner: owner, Name: "note", ValueType: bcg.StringType,
		TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation(),
	}))
	left := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: leftID, Name: "left"}))
	right := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: rightID, Name: "right"}))
	from := mustData[bcg.EndpointConstraint](t)(bcg.EntityTypeEndpoint(leftID))
	to := mustData[bcg.EndpointConstraint](t)(bcg.EntityTypeEndpoint(rightID))
	relationType := mustData[bcg.RelationType](t)(bcg.NewRelationType(bcg.RelationTypeSpec{
		ID: relationID, Name: "connects", From: from, To: to, PropertyDefinitions: []bcg.ID{propertyID},
	}))
	for _, declaration := range []any{property, left, right, relationType} {
		switch value := declaration.(type) {
		case bcg.PropertyDefinition:
			if err := draft.AddPropertyDefinition(value); err != nil {
				t.Fatal(err)
			}
		case bcg.EntityType:
			if err := draft.AddEntityType(value); err != nil {
				t.Fatal(err)
			}
		case bcg.RelationType:
			if err := draft.AddRelationType(value); err != nil {
				t.Fatal(err)
			}
		}
	}
	model := mustData[bcg.Model](t)(draft.Commit(mustCommitOptions(t, bcg.NewID(), "v1", mustPolicy(t))))
	relation := mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: relationID, From: bcg.NewID(), To: bcg.NewID()}))
	address := mustData[bcg.PropertyAddress](t)(bcg.RelationProperty(relation.ID(), propertyID))
	target := mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(address))
	value := mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(bcg.String("valid")))
	binding := mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: value}))
	if report := model.NewRuntimeValidator().ValidateValueBinding(binding, bcg.RelationRuntimeObject(relation)); !report.OK() {
		t.Fatalf("valid local relation binding rejected: %#v", report.Issues())
	}
}

func phase12EnumKeyModel(t testing.TB) (bcg.Model, bcg.ID, bcg.ID, bcg.ID, bcg.ID, bcg.ID) {
	t.Helper()
	logical := mustData[bcg.Model](t)(bcg.NewModel(bcg.NewID(), "enum-key"))
	draft := mustData[*bcg.Draft](t)(logical.Draft())
	enumID, openID, closedID, entityTypeID, propertyID := bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID()
	enum := mustData[bcg.EnumDefinition](t)(bcg.NewEnumDefinition(bcg.EnumDefinitionSpec{ID: enumID, Name: "status", Options: []bcg.EnumOptionSpec{{ID: closedID, Name: "closed"}, {ID: openID, Name: "open"}}}))
	owner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(entityTypeID))
	required := mustData[bcg.PropertyComputation](t)(bcg.InputComputationWithPresence(bcg.PresenceRequired))
	property := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: propertyID, Owner: owner, Name: "status", ValueType: bcg.EnumType, EnumDefinitionID: enumID, TemporalMode: bcg.TemporalScalar, Computation: required}))
	entityType := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: entityTypeID, Name: "record", PropertyDefinitions: []bcg.ID{propertyID}}))
	key := mustData[bcg.EntityKeyDefinition](t)(bcg.NewEntityKeyDefinition(bcg.EntityKeyDefinitionSpec{ID: bcg.NewID(), Name: "status", EntityTypeID: entityTypeID, PropertyDefinitionIDs: []bcg.ID{propertyID}}))
	for _, declaration := range []any{enum, property, entityType, key} {
		switch value := declaration.(type) {
		case bcg.EnumDefinition:
			if err := draft.AddEnumDefinition(value); err != nil {
				t.Fatal(err)
			}
		case bcg.PropertyDefinition:
			if err := draft.AddPropertyDefinition(value); err != nil {
				t.Fatal(err)
			}
		case bcg.EntityType:
			if err := draft.AddEntityType(value); err != nil {
				t.Fatal(err)
			}
		case bcg.EntityKeyDefinition:
			if err := draft.AddEntityKeyDefinition(value); err != nil {
				t.Fatal(err)
			}
		}
	}
	return mustData[bcg.Model](t)(draft.Commit(mustCommitOptions(t, bcg.NewID(), "v1", mustPolicy(t)))), enumID, openID, closedID, entityTypeID, propertyID
}

func phase12InputModel(t testing.TB, modelID, versionID, typeID, propertyID bcg.ID, valueType bcg.ValueType) bcg.Model {
	t.Helper()
	logical := mustData[bcg.Model](t)(bcg.NewModel(modelID, "identity"))
	draft := mustData[*bcg.Draft](t)(logical.Draft())
	owner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(typeID))
	property := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: propertyID, Owner: owner, Name: "value", ValueType: valueType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()}))
	entityType := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "record", PropertyDefinitions: []bcg.ID{propertyID}}))
	if err := draft.AddPropertyDefinition(property); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	return mustData[bcg.Model](t)(draft.Commit(mustCommitOptions(t, versionID, "v1", mustPolicy(t))))
}

func phase12Entity(t testing.TB, typeID bcg.ID) bcg.Entity {
	t.Helper()
	return mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: typeID}))
}

func phase12AddBinding(t testing.TB, builder *bcg.DatasetBuilder, objectID, definitionID bcg.ID, value bcg.Value) {
	t.Helper()
	address := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(objectID, definitionID))
	target := mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(address))
	bindingValue := mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(value))
	binding := mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: bindingValue}))
	if err := builder.AddValueBinding(binding); err != nil {
		t.Fatal(err)
	}
}
