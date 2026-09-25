package bcg_test

import (
	"bytes"
	"errors"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

func TestPublicExpressionBuilders(t *testing.T) {
	decimal := mustPublicExpression(t)(bcg.Constant(bcg.DecimalValue(bcg.MustParseDecimal("12.5"))))
	boolean := mustPublicExpression(t)(bcg.Constant(bcg.Bool(true)))
	definitionID := bcg.NewID()
	contractID := bcg.NewID()
	self := mustPublicExpression(t)(bcg.SelfProperty(definitionID))
	address, err := bcg.EntityProperty(bcg.NewID(), definitionID)
	if err != nil {
		t.Fatal(err)
	}
	absolute := mustPublicExpression(t)(bcg.AbsoluteProperty(address))
	boundProperty := mustPublicExpression(t)(bcg.BoundProperty(bcg.BoundEntity, definitionID))
	boundContract := mustPublicExpression(t)(bcg.BoundContractProperty(bcg.BoundRelation, contractID))

	for _, expression := range []bcg.Expression{
		decimal,
		boolean,
		self,
		absolute,
		boundProperty,
		boundContract,
		mustPublicExpression(t)(bcg.Add(decimal, self)),
		mustPublicExpression(t)(bcg.Subtract(decimal, self)),
		mustPublicExpression(t)(bcg.Multiply(decimal, self)),
		mustPublicExpression(t)(bcg.Divide(decimal, self)),
		mustPublicExpression(t)(bcg.Negate(decimal)),
		mustPublicExpression(t)(bcg.Equal(decimal, self)),
		mustPublicExpression(t)(bcg.NotEqual(decimal, self)),
		mustPublicExpression(t)(bcg.Less(decimal, self)),
		mustPublicExpression(t)(bcg.LessOrEqual(decimal, self)),
		mustPublicExpression(t)(bcg.Greater(decimal, self)),
		mustPublicExpression(t)(bcg.GreaterOrEqual(decimal, self)),
		mustPublicExpression(t)(bcg.And(boolean, boolean)),
		mustPublicExpression(t)(bcg.Or(boolean, boolean)),
		mustPublicExpression(t)(bcg.Not(boolean)),
		mustPublicExpression(t)(bcg.If(boolean, decimal, self)),
		mustPublicExpression(t)(bcg.IsNull(decimal)),
		mustPublicExpression(t)(bcg.Coalesce(decimal, self)),
	} {
		if !expression.IsValid() {
			t.Fatalf("public expression is invalid: %#v", expression)
		}
	}
	if _, ok := (bcg.Expression{}).Depth(); ok {
		t.Fatal("zero Expression reports a depth")
	}
	if _, err := bcg.Constant(bcg.Value{}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("invalid constant = %T %[1]v", err)
	}
	if _, err := bcg.SelfProperty(bcg.ID{}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("zero self reference = %T %[1]v", err)
	}
	if _, err := bcg.Add(decimal, bcg.Expression{}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("zero binary operand = %T %[1]v", err)
	}
	if _, err := bcg.BoundProperty(bcg.AggregateBinding(0), definitionID); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("invalid aggregate binding = %T %[1]v", err)
	}

	relationTypeID := bcg.NewID()
	selfSource := mustPublicTraversal(t)(bcg.SelfTraversal(bcg.TraverseOutgoing, relationTypeID))
	absoluteSource := mustPublicTraversal(t)(bcg.AbsoluteEntityTraversal(bcg.NewID(), bcg.TraverseIncoming, relationTypeID))
	boundSource := mustPublicTraversal(t)(bcg.BoundEntityTraversal(bcg.TraverseBoth, relationTypeID))
	if selfSource.Origin() != bcg.TraversalSelf || absoluteSource.Origin() != bcg.TraversalAbsoluteEntity || boundSource.Origin() != bcg.TraversalBoundEntity {
		t.Fatalf("traversal origins = %s/%s/%s", selfSource.Origin(), absoluteSource.Origin(), boundSource.Origin())
	}
	if origin, ok := absoluteSource.AbsoluteEntityID(); !ok || origin.IsZero() {
		t.Fatal("absolute traversal origin was lost")
	}
	for _, expression := range []bcg.Expression{
		mustPublicExpression(t)(bcg.Count(selfSource)),
		mustPublicExpression(t)(bcg.FilteredCount(selfSource, boolean)),
		mustPublicExpression(t)(bcg.Aggregate(selfSource, bcg.AggregateSum, boundProperty)),
		mustPublicExpression(t)(bcg.Aggregate(selfSource, bcg.AggregateAverage, boundProperty)),
		mustPublicExpression(t)(bcg.Aggregate(selfSource, bcg.AggregateMin, boundProperty)),
		mustPublicExpression(t)(bcg.Aggregate(selfSource, bcg.AggregateMax, boundProperty)),
		mustPublicExpression(t)(bcg.FilteredAggregate(selfSource, boolean, bcg.AggregateSum, boundProperty)),
	} {
		if !expression.IsValid() {
			t.Fatalf("aggregate expression is invalid: %#v", expression)
		}
	}
	if _, err := bcg.Aggregate(selfSource, bcg.AggregateCount, boundProperty); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("mapped Count accepted: %T %[1]v", err)
	}
}

func TestPublicDerivedExpressionLifecycle(t *testing.T) {
	modelID := bcg.MustParseID("00000000-0000-4000-8000-000000004001")
	version1ID := bcg.MustParseID("00000000-0000-4000-8000-000000004002")
	version2ID := bcg.MustParseID("00000000-0000-4000-8000-000000004003")
	version3ID := bcg.MustParseID("00000000-0000-4000-8000-000000004004")
	entityTypeID := bcg.MustParseID("00000000-0000-4000-8000-000000004005")
	quantityID := bcg.MustParseID("00000000-0000-4000-8000-000000004006")
	priceID := bcg.MustParseID("00000000-0000-4000-8000-000000004007")
	revenueID := bcg.MustParseID("00000000-0000-4000-8000-000000004008")
	policy := mustPublicPolicy(t)

	quantityRef := mustPublicExpression(t)(bcg.SelfProperty(quantityID))
	priceRef := mustPublicExpression(t)(bcg.SelfProperty(priceID))
	revenueExpression := mustPublicExpression(t)(bcg.Multiply(quantityRef, priceRef))
	draft := newPublicRevenueDraft(t, modelID, entityTypeID, quantityID, priceID, revenueID, revenueExpression)
	if report, err := draft.Validate(); err != nil || !report.OK() {
		t.Fatalf("revenue draft Validate = %#v, %v", report.Issues(), err)
	}
	v1 := commitPublicSchema(t, draft, version1ID, "v1", policy)

	revenueInfo, ok := v1.PropertyDefinition(revenueID)
	if !ok || revenueInfo.Schema().Computation().Kind() != bcg.ComputationDerived {
		t.Fatalf("derived revenue schema = %#v, %v", revenueInfo, ok)
	}
	storedExpression, ok := revenueInfo.Schema().Computation().Expression()
	if !ok || !storedExpression.IsValid() {
		t.Fatalf("derived expression introspection = %#v, %v", storedExpression, ok)
	}
	if depth, ok := storedExpression.Depth(); !ok || depth != 2 {
		t.Fatalf("derived expression depth = %d, %v", depth, ok)
	}

	artifact, err := bcg.NewArtifact(v1)
	if err != nil {
		t.Fatal(err)
	}
	encodedV1, err := bcg.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encodedV1)
	if err != nil {
		t.Fatal(err)
	}
	decodedRevenue, ok := decoded.Model().PropertyDefinition(revenueID)
	if !ok {
		t.Fatal("round-tripped revenue definition is missing")
	}
	if expression, ok := decodedRevenue.Schema().Computation().Expression(); !ok || !expression.IsValid() {
		t.Fatalf("round-tripped derived expression = %#v, %v", expression, ok)
	}

	preserve, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	v2 := commitPublicSchema(t, preserve, version2ID, "v2", policy)
	if expression, ok := mustPublicPropertyInfo(t, v2, revenueID).Schema().Computation().Expression(); !ok || !expression.IsValid() {
		t.Fatalf("Model.Edit did not preserve expression: %#v, %v", expression, ok)
	}

	replacement, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	replacementExpression := mustPublicExpression(t)(bcg.Add(quantityRef, priceRef))
	owner := mustPublicPropertyOwner(t)(bcg.EntityTypeOwner(entityTypeID))
	replacementDefinition := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           revenueID,
		Owner:        owner,
		Name:         "revenue",
		ValueType:    bcg.DecimalType,
		TemporalMode: bcg.TemporalScalar,
		Computation:  mustPublicDerived(t)(bcg.DerivedComputation(replacementExpression)),
	}))
	if err := replacement.ReplacePropertyDefinition(revenueID, replacementDefinition); err != nil {
		t.Fatal(err)
	}
	v3 := commitPublicSchema(t, replacement, version3ID, "v3", policy)
	if parent, ok := v3.ParentVersionID(); !ok || !parent.Equal(version1ID) {
		t.Fatalf("replacement version parent = %s, %v", parent, ok)
	}
	if after, err := bcg.Marshal(mustPublicArtifact(t, v1)); err != nil || !bytes.Equal(after, encodedV1) {
		t.Fatalf("replacing v3 mutated v1: %v", err)
	}
	if v3.VersionID().Equal(v1.VersionID()) || v2.VersionID().Equal(v1.VersionID()) {
		t.Fatal("expression edits reused an immutable version ID")
	}
}

func TestPublicExpressionValidationBoundaries(t *testing.T) {
	policy := mustPublicPolicy(t)
	modelID := bcg.NewID()
	entityTypeID := bcg.NewID()
	quantityID := bcg.NewID()
	priceID := bcg.NewID()
	revenueID := bcg.NewID()
	quantityRef := mustPublicExpression(t)(bcg.SelfProperty(quantityID))

	wrongType := mustPublicExpression(t)(bcg.Add(quantityRef, mustPublicExpression(t)(bcg.Constant(bcg.Bool(true)))))
	invalidType := newPublicRevenueDraft(t, modelID, entityTypeID, quantityID, priceID, revenueID, wrongType)
	assertInvalidExpressionDraft(t, invalidType, policy)

	dimension, err := bcg.NewBaseDimension("count")
	if err != nil {
		t.Fatal(err)
	}
	countUnit, err := bcg.NewUnit("item", dimension, "count", bcg.OneRational())
	if err != nil {
		t.Fatal(err)
	}
	unitValue, err := bcg.DecimalValue(bcg.MustParseDecimal("1")).WithUnit(countUnit)
	if err != nil {
		t.Fatal(err)
	}
	wrongUnit := mustPublicExpression(t)(bcg.Constant(unitValue))
	invalidUnit := newPublicRevenueDraft(t, bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID(), wrongUnit)
	assertInvalidExpressionDraft(t, invalidUnit, policy)

	bound := mustPublicExpression(t)(bcg.BoundProperty(bcg.BoundEntity, quantityID))
	invalidBinding := newPublicRevenueDraft(t, bcg.NewID(), bcg.NewID(), quantityID, bcg.NewID(), bcg.NewID(), bound)
	assertInvalidExpressionDraft(t, invalidBinding, policy)
	boundSource := mustPublicTraversal(t)(bcg.BoundEntityTraversal(bcg.TraverseOutgoing, bcg.NewID()))
	invalidBoundSource := newPublicRevenueDraft(t, bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID(), mustPublicExpression(t)(bcg.Count(boundSource)))
	assertInvalidExpressionDraft(t, invalidBoundSource, policy)

	invalidSelfTraversal := newRelationSelfTraversalDraft(t)
	assertInvalidExpressionDraft(t, invalidSelfTraversal, policy)

	deepModelID := bcg.NewID()
	deepEntityTypeID := bcg.NewID()
	deepQuantityID := bcg.NewID()
	deepPriceID := bcg.NewID()
	deepRevenueID := bcg.NewID()
	deep := mustPublicExpression(t)(bcg.SelfProperty(deepQuantityID))
	for index := 0; index < bcg.MaxExpressionDepth; index++ {
		deep = mustPublicExpression(t)(bcg.Negate(deep))
	}
	if depth, ok := deep.Depth(); !ok || depth != bcg.MaxExpressionDepth+1 {
		t.Fatalf("deep expression depth = %d, %v", depth, ok)
	}
	deepDraft := newPublicRevenueDraft(t, deepModelID, deepEntityTypeID, deepQuantityID, deepPriceID, deepRevenueID, deep)
	assertInvalidExpressionDraft(t, deepDraft, policy)
}

func TestPublicScalarExpressionRoundTripParity(t *testing.T) {
	modelID := bcg.NewID()
	entityTypeID := bcg.NewID()
	decimalID := bcg.NewID()
	boolID := bcg.NewID()
	decimalRef := mustPublicExpression(t)(bcg.SelfProperty(decimalID))
	boolRef := mustPublicExpression(t)(bcg.SelfProperty(boolID))
	decimalConstant := mustPublicExpression(t)(bcg.Constant(bcg.DecimalValue(bcg.MustParseDecimal("2"))))
	boolConstant := mustPublicExpression(t)(bcg.Constant(bcg.Bool(true)))

	type expressionDefinition struct {
		id         bcg.ID
		name       string
		valueType  bcg.ValueType
		expression bcg.Expression
	}
	definitions := []expressionDefinition{
		{bcg.NewID(), "constant-decimal", bcg.DecimalType, decimalConstant},
		{bcg.NewID(), "constant-bool", bcg.BoolType, boolConstant},
		{bcg.NewID(), "add", bcg.DecimalType, mustPublicExpression(t)(bcg.Add(decimalRef, decimalConstant))},
		{bcg.NewID(), "subtract", bcg.DecimalType, mustPublicExpression(t)(bcg.Subtract(decimalRef, decimalConstant))},
		{bcg.NewID(), "multiply", bcg.DecimalType, mustPublicExpression(t)(bcg.Multiply(decimalRef, decimalConstant))},
		{bcg.NewID(), "divide", bcg.DecimalType, mustPublicExpression(t)(bcg.Divide(decimalRef, decimalConstant))},
		{bcg.NewID(), "negate", bcg.DecimalType, mustPublicExpression(t)(bcg.Negate(decimalRef))},
		{bcg.NewID(), "equal", bcg.BoolType, mustPublicExpression(t)(bcg.Equal(decimalRef, decimalConstant))},
		{bcg.NewID(), "not-equal", bcg.BoolType, mustPublicExpression(t)(bcg.NotEqual(decimalRef, decimalConstant))},
		{bcg.NewID(), "less", bcg.BoolType, mustPublicExpression(t)(bcg.Less(decimalRef, decimalConstant))},
		{bcg.NewID(), "less-or-equal", bcg.BoolType, mustPublicExpression(t)(bcg.LessOrEqual(decimalRef, decimalConstant))},
		{bcg.NewID(), "greater", bcg.BoolType, mustPublicExpression(t)(bcg.Greater(decimalRef, decimalConstant))},
		{bcg.NewID(), "greater-or-equal", bcg.BoolType, mustPublicExpression(t)(bcg.GreaterOrEqual(decimalRef, decimalConstant))},
		{bcg.NewID(), "and", bcg.BoolType, mustPublicExpression(t)(bcg.And(boolRef, boolConstant))},
		{bcg.NewID(), "or", bcg.BoolType, mustPublicExpression(t)(bcg.Or(boolRef, boolConstant))},
		{bcg.NewID(), "not", bcg.BoolType, mustPublicExpression(t)(bcg.Not(boolRef))},
		{bcg.NewID(), "if", bcg.DecimalType, mustPublicExpression(t)(bcg.If(boolRef, decimalRef, decimalConstant))},
		{bcg.NewID(), "is-null", bcg.BoolType, mustPublicExpression(t)(bcg.IsNull(decimalRef))},
		{bcg.NewID(), "coalesce", bcg.DecimalType, mustPublicExpression(t)(bcg.Coalesce(decimalRef, decimalConstant))},
	}
	propertyIDs := []bcg.ID{decimalID, boolID}
	for _, definition := range definitions {
		propertyIDs = append(propertyIDs, definition.id)
	}
	entity := mustPublicEntityType(t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: entityTypeID, Name: "Scalar", PropertyDefinitions: propertyIDs}))
	owner := mustPublicPropertyOwner(t)(bcg.EntityTypeOwner(entityTypeID))
	decimalInput := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: decimalID, Owner: owner, Name: "decimal", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()}))
	boolInput := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: boolID, Owner: owner, Name: "bool", ValueType: bcg.BoolType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()}))
	logical, err := bcg.NewModel(modelID, "scalar-expression-parity")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	for _, add := range []func() error{
		func() error { return draft.AddEntityType(entity) },
		func() error { return draft.AddPropertyDefinition(decimalInput) },
		func() error { return draft.AddPropertyDefinition(boolInput) },
	} {
		if err := add(); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range definitions {
		definition := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
			ID:           item.id,
			Owner:        owner,
			Name:         item.name,
			ValueType:    item.valueType,
			TemporalMode: bcg.TemporalScalar,
			Computation:  mustPublicDerived(t)(bcg.DerivedComputation(item.expression)),
		}))
		if err := draft.AddPropertyDefinition(definition); err != nil {
			t.Fatal(err)
		}
	}
	model := commitPublicSchema(t, draft, bcg.NewID(), "v1", mustPublicPolicy(t))
	encoded, err := bcg.Marshal(mustPublicArtifact(t, model))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range definitions {
		expression, ok := mustPublicPropertyInfo(t, decoded.Model(), item.id).Schema().Computation().Expression()
		if !ok || !expression.IsValid() {
			t.Fatalf("round-tripped %s expression = %#v, %v", item.name, expression, ok)
		}
		// The public v2 introspection contract must be sufficient to rebuild
		// every decoded v1 node without internal AST access.
		assertInspectableExpression(t, expression)
	}
}

func TestPublicAbsolutePropertyExpressionRoundTrip(t *testing.T) {
	artifact, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		t.Fatal(err)
	}
	model := artifact.Model()
	entityTypeID := bcg.MustParseID("00000000-0000-4000-8000-000000000003")
	entityID := bcg.MustParseID("00000000-0000-4000-8000-000000000005")
	amountID := bcg.MustParseID("00000000-0000-4000-8000-000000000004")
	derivedID := bcg.NewID()
	address, err := bcg.EntityProperty(entityID, amountID)
	if err != nil {
		t.Fatal(err)
	}
	expression := mustPublicExpression(t)(bcg.AbsoluteProperty(address))
	entity := mustPublicEntityType(t)(bcg.NewEntityType(bcg.EntityTypeSpec{
		ID:                  entityTypeID,
		Name:                "node",
		PropertyDefinitions: []bcg.ID{amountID, derivedID},
	}))
	definition := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           derivedID,
		Owner:        mustPublicPropertyOwner(t)(bcg.EntityTypeOwner(entityTypeID)),
		Name:         "copied-amount",
		ValueType:    bcg.DecimalType,
		TemporalMode: bcg.TemporalScalar,
		Computation:  mustPublicDerived(t)(bcg.DerivedComputation(expression)),
	}))
	edit, err := model.Edit()
	if err != nil {
		t.Fatal(err)
	}
	if err := edit.ReplaceEntityType(entityTypeID, entity); err != nil {
		t.Fatal(err)
	}
	if err := edit.AddPropertyDefinition(definition); err != nil {
		t.Fatal(err)
	}
	policy, ok := model.PrecisionPolicy()
	if !ok {
		t.Fatal("fixture has no precision policy")
	}
	v2 := commitPublicSchema(t, edit, bcg.NewID(), "absolute-property", policy)
	encoded, err := bcg.Marshal(mustPublicArtifact(t, v2))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil {
		t.Fatalf("absolute property expression round trip: %v", err)
	}
	stored, ok := mustPublicPropertyInfo(t, decoded.Model(), derivedID).Schema().Computation().Expression()
	if !ok {
		t.Fatal("absolute property expression was not decoded")
	}
	assertInspectableExpression(t, stored)
}

func TestPublicAggregateExpressionRoundTrip(t *testing.T) {
	artifact, err := bcg.Unmarshal([]byte(publicAggregateArtifact))
	if err != nil {
		t.Fatal(err)
	}
	model := artifact.Model()
	rootTypeID := bcg.MustParseID("00000000-0000-4000-8000-000000001003")
	relationTypeID := bcg.MustParseID("00000000-0000-4000-8000-000000001005")
	contractID := bcg.MustParseID("00000000-0000-4000-8000-000000001007")
	quantityID := bcg.MustParseID("00000000-0000-4000-8000-000000001009")
	totalID := bcg.MustParseID("00000000-0000-4000-8000-000000001010")

	source := mustPublicTraversal(t)(bcg.SelfTraversal(bcg.TraverseOutgoing, relationTypeID))
	quantity := mustPublicExpression(t)(bcg.BoundProperty(bcg.BoundRelation, quantityID))
	quantityInfo, ok := model.PropertyDefinition(quantityID)
	if !ok {
		t.Fatal("aggregate fixture quantity definition is missing")
	}
	quantityUnit, ok := quantityInfo.Schema().Unit()
	if !ok {
		t.Fatal("aggregate fixture quantity has no unit")
	}
	zero, err := bcg.Int64(0).WithUnit(quantityUnit)
	if err != nil {
		t.Fatal(err)
	}
	filter := mustPublicExpression(t)(bcg.Greater(quantity, mustPublicExpression(t)(bcg.Constant(zero))))
	price := mustPublicExpression(t)(bcg.BoundContractProperty(bcg.BoundEntity, contractID))
	mapped := mustPublicExpression(t)(bcg.Multiply(price, quantity))
	expression := mustPublicExpression(t)(bcg.FilteredAggregate(source, filter, bcg.AggregateSum, mapped))
	countExpression := mustPublicExpression(t)(bcg.FilteredCount(source, filter))
	averageExpression := mustPublicExpression(t)(bcg.Aggregate(source, bcg.AggregateAverage, mapped))
	minExpression := mustPublicExpression(t)(bcg.Aggregate(source, bcg.AggregateMin, mapped))
	maxExpression := mustPublicExpression(t)(bcg.Aggregate(source, bcg.AggregateMax, mapped))

	edit, err := model.Edit()
	if err != nil {
		t.Fatal(err)
	}
	owner := mustPublicPropertyOwner(t)(bcg.EntityTypeOwner(rootTypeID))
	totalInfo, ok := model.PropertyDefinition(totalID)
	if !ok {
		t.Fatal("aggregate fixture total definition is missing")
	}
	totalUnit, ok := totalInfo.Schema().Unit()
	if !ok {
		t.Fatal("aggregate fixture total has no unit")
	}
	countID := bcg.NewID()
	averageID := bcg.NewID()
	minID := bcg.NewID()
	maxID := bcg.NewID()
	root, ok := model.EntityType(rootTypeID)
	if !ok {
		t.Fatal("aggregate fixture root type is missing")
	}
	rootDefinitions := append(root.PropertyDefinitions(), countID, averageID, minID, maxID)
	rootReplacement := mustPublicEntityType(t)(bcg.NewEntityType(bcg.EntityTypeSpec{
		ID:                  rootTypeID,
		Name:                root.Name(),
		Traits:              root.Traits(),
		PropertyDefinitions: rootDefinitions,
	}))
	if err := edit.ReplaceEntityType(rootTypeID, rootReplacement); err != nil {
		t.Fatal(err)
	}
	replacement := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           totalID,
		Owner:        owner,
		Name:         "total",
		ValueType:    bcg.DecimalType,
		Unit:         totalUnit,
		TemporalMode: bcg.TemporalScalar,
		Computation:  mustPublicDerived(t)(bcg.DerivedComputation(expression)),
	}))
	if err := edit.ReplacePropertyDefinition(totalID, replacement); err != nil {
		t.Fatal(err)
	}
	for _, definition := range []bcg.PropertyDefinition{
		mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
			ID:           countID,
			Owner:        owner,
			Name:         "count",
			ValueType:    bcg.IntegerType,
			TemporalMode: bcg.TemporalScalar,
			Computation:  mustPublicDerived(t)(bcg.DerivedComputation(countExpression)),
		})),
		mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
			ID:           averageID,
			Owner:        owner,
			Name:         "average",
			ValueType:    bcg.DecimalType,
			Unit:         totalUnit,
			TemporalMode: bcg.TemporalScalar,
			Computation:  mustPublicDerived(t)(bcg.DerivedComputation(averageExpression)),
		})),
		mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
			ID:           minID,
			Owner:        owner,
			Name:         "min",
			ValueType:    bcg.DecimalType,
			Unit:         totalUnit,
			TemporalMode: bcg.TemporalScalar,
			Computation:  mustPublicDerived(t)(bcg.DerivedComputation(minExpression)),
		})),
		mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
			ID:           maxID,
			Owner:        owner,
			Name:         "max",
			ValueType:    bcg.DecimalType,
			Unit:         totalUnit,
			TemporalMode: bcg.TemporalScalar,
			Computation:  mustPublicDerived(t)(bcg.DerivedComputation(maxExpression)),
		})),
	} {
		if err := edit.AddPropertyDefinition(definition); err != nil {
			t.Fatal(err)
		}
	}
	policy, ok := model.PrecisionPolicy()
	if !ok {
		t.Fatal("aggregate fixture has no precision policy")
	}
	v2 := commitPublicSchema(t, edit, bcg.NewID(), "public-aggregate", policy)
	encoded, err := bcg.Marshal(mustPublicArtifact(t, v2))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, definitionID := range []bcg.ID{totalID, countID, averageID, minID, maxID} {
		if expression, ok := mustPublicPropertyInfo(t, decoded.Model(), definitionID).Schema().Computation().Expression(); !ok || !expression.IsValid() {
			t.Fatalf("aggregate expression %s round trip = %#v, %v", definitionID, expression, ok)
		}
	}
}

func newPublicRevenueDraft(t *testing.T, modelID, entityTypeID, quantityID, priceID, revenueID bcg.ID, revenueExpression bcg.Expression) *bcg.Draft {
	t.Helper()
	entity := mustPublicEntityType(t)(bcg.NewEntityType(bcg.EntityTypeSpec{
		ID:                  entityTypeID,
		Name:                "Product",
		PropertyDefinitions: []bcg.ID{quantityID, priceID, revenueID},
	}))
	owner := mustPublicPropertyOwner(t)(bcg.EntityTypeOwner(entityTypeID))
	quantity := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           quantityID,
		Owner:        owner,
		Name:         "quantity",
		ValueType:    bcg.DecimalType,
		TemporalMode: bcg.TemporalScalar,
		Computation:  bcg.InputComputation(),
	}))
	price := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           priceID,
		Owner:        owner,
		Name:         "price",
		ValueType:    bcg.DecimalType,
		TemporalMode: bcg.TemporalScalar,
		Computation:  bcg.InputComputation(),
	}))
	revenue := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           revenueID,
		Owner:        owner,
		Name:         "revenue",
		ValueType:    bcg.DecimalType,
		TemporalMode: bcg.TemporalScalar,
		Computation:  mustPublicDerived(t)(bcg.DerivedComputation(revenueExpression)),
	}))
	logical, err := bcg.NewModel(modelID, "expression-product")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	for _, add := range []func() error{
		func() error { return draft.AddEntityType(entity) },
		func() error { return draft.AddPropertyDefinition(quantity) },
		func() error { return draft.AddPropertyDefinition(price) },
		func() error { return draft.AddPropertyDefinition(revenue) },
	} {
		if err := add(); err != nil {
			t.Fatal(err)
		}
	}
	return draft
}

func newRelationSelfTraversalDraft(t *testing.T) *bcg.Draft {
	t.Helper()
	relationTypeID := bcg.NewID()
	definitionID := bcg.NewID()
	relation := mustPublicRelationType(t)(bcg.NewRelationType(bcg.RelationTypeSpec{
		ID:                  relationTypeID,
		Name:                "links",
		PropertyDefinitions: []bcg.ID{definitionID},
	}))
	source := mustPublicTraversal(t)(bcg.SelfTraversal(bcg.TraverseOutgoing, relationTypeID))
	definition := mustPublicPropertyDefinition(t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           definitionID,
		Owner:        mustPublicPropertyOwner(t)(bcg.RelationTypeOwner(relationTypeID)),
		Name:         "bad",
		ValueType:    bcg.IntegerType,
		TemporalMode: bcg.TemporalScalar,
		Computation:  mustPublicDerived(t)(bcg.DerivedComputation(mustPublicExpression(t)(bcg.Count(source)))),
	}))
	logical, err := bcg.NewModel(bcg.NewID(), "invalid-self-traversal")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.AddRelationType(relation); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddPropertyDefinition(definition); err != nil {
		t.Fatal(err)
	}
	return draft
}

func assertInvalidExpressionDraft(t *testing.T, draft *bcg.Draft, policy bcg.PrecisionPolicy) {
	t.Helper()
	report, err := draft.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if report.OK() {
		t.Fatal("invalid public expression Draft.Validate unexpectedly succeeded")
	}
	if _, err := draft.Commit(mustCommitOptions(t, bcg.NewID(), "invalid-expression", policy)); !errors.Is(err, bcg.ErrModelValidation) {
		t.Fatalf("invalid public expression Commit = %T %[1]v", err)
	}
}

func mustPublicExpression(t *testing.T) func(bcg.Expression, error) bcg.Expression {
	t.Helper()
	return func(expression bcg.Expression, err error) bcg.Expression {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return expression
	}
}

func mustPublicTraversal(t *testing.T) func(bcg.TraversalSource, error) bcg.TraversalSource {
	t.Helper()
	return func(source bcg.TraversalSource, err error) bcg.TraversalSource {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return source
	}
}

func mustPublicDerived(t *testing.T) func(bcg.PropertyComputation, error) bcg.PropertyComputation {
	t.Helper()
	return func(computation bcg.PropertyComputation, err error) bcg.PropertyComputation {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return computation
	}
}

func mustPublicPropertyOwner(t *testing.T) func(bcg.PropertyOwner, error) bcg.PropertyOwner {
	t.Helper()
	return func(owner bcg.PropertyOwner, err error) bcg.PropertyOwner {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return owner
	}
}

func mustPublicPropertyDefinition(t *testing.T) func(bcg.PropertyDefinition, error) bcg.PropertyDefinition {
	t.Helper()
	return func(definition bcg.PropertyDefinition, err error) bcg.PropertyDefinition {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return definition
	}
}

func mustPublicEntityType(t *testing.T) func(bcg.EntityType, error) bcg.EntityType {
	t.Helper()
	return func(entityType bcg.EntityType, err error) bcg.EntityType {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return entityType
	}
}

func mustPublicRelationType(t *testing.T) func(bcg.RelationType, error) bcg.RelationType {
	t.Helper()
	return func(relationType bcg.RelationType, err error) bcg.RelationType {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return relationType
	}
}

func mustPublicPolicy(t *testing.T) bcg.PrecisionPolicy {
	t.Helper()
	policy, err := bcg.NewPrecisionPolicy(8, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func mustPublicArtifact(t *testing.T, model bcg.Model) bcg.Artifact {
	t.Helper()
	artifact, err := bcg.NewArtifact(model)
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

// mustPublicPropertyInfo keeps the root-only tests readable while preserving
// the public API's ordinary (PropertyInfo, bool) lookup contract.
func mustPublicPropertyInfo(t *testing.T, model bcg.Model, id bcg.ID) bcg.PropertyInfo {
	t.Helper()
	info, ok := model.PropertyDefinition(id)
	if !ok {
		t.Fatalf("PropertyDefinition(%s) is missing", id)
	}
	return info
}
