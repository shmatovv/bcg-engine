package bcg_test

import (
	"errors"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

type schemaFixture struct {
	modelID, version1ID, version2ID bcg.ID
	entityTypeID                    bcg.ID
	relationTypeID                  bcg.ID
	contractID                      bcg.ID
	traitID                         bcg.ID
	propertyID                      bcg.ID
	scopeID                         bcg.ID
	unit                            bcg.Unit
	policy                          bcg.PrecisionPolicy
}

func TestPublicSchemaAuthoringLifecycle(t *testing.T) {
	fixture, draft := newPublicSchemaDraft(t)

	report, err := draft.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK() {
		t.Fatalf("valid public schema report = %#v", report.Issues())
	}

	v1 := commitPublicSchema(t, draft, fixture.version1ID, "v1", fixture.policy)
	if !v1.IsValid() || !v1.ModelID().Equal(fixture.modelID) || !v1.VersionID().Equal(fixture.version1ID) {
		t.Fatalf("committed public schema model = %#v", v1)
	}
	if _, ok := v1.ParentVersionID(); ok {
		t.Fatal("first public schema version unexpectedly has a parent")
	}
	assertPublicSchemaReadback(t, v1, fixture, "Product")

	artifact, err := bcg.NewArtifact(v1)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := bcg.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	assertPublicSchemaReadback(t, decoded.Model(), fixture, "Product")

	edit, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	if parent, ok := edit.ParentVersionID(); !ok || !parent.Equal(fixture.version1ID) {
		t.Fatalf("schema edit parent = %s, %v", parent, ok)
	}

	replacementEntity, err := bcg.NewEntityType(bcg.EntityTypeSpec{
		ID:                  fixture.entityTypeID,
		Name:                "Product v2",
		Traits:              []bcg.ID{fixture.traitID},
		PropertyDefinitions: []bcg.ID{fixture.propertyID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := edit.ReplaceEntityType(fixture.entityTypeID, replacementEntity); err != nil {
		t.Fatal(err)
	}
	// Replacing every remaining schema kind verifies that updates remain typed
	// and preserve identities. The values are reconstructed from the public API.
	contract, ok := v1.PropertyContract(fixture.contractID)
	if !ok {
		t.Fatal("missing authored property contract")
	}
	if err := edit.ReplacePropertyContract(fixture.contractID, contract); err != nil {
		t.Fatal(err)
	}
	trait, ok := v1.Trait(fixture.traitID)
	if !ok {
		t.Fatal("missing authored trait")
	}
	if err := edit.ReplaceTrait(fixture.traitID, trait); err != nil {
		t.Fatal(err)
	}
	property, ok := v1.PropertyDefinition(fixture.propertyID)
	if !ok {
		t.Fatal("missing authored property definition")
	}
	if err := edit.ReplacePropertyDefinition(fixture.propertyID, property.Schema()); err != nil {
		t.Fatal(err)
	}
	relation, ok := v1.RelationType(fixture.relationTypeID)
	if !ok {
		t.Fatal("missing authored relation type")
	}
	if err := edit.ReplaceRelationType(fixture.relationTypeID, relation); err != nil {
		t.Fatal(err)
	}
	scope, ok := v1.ScopeDefinition(fixture.scopeID)
	if !ok {
		t.Fatal("missing authored scope definition")
	}
	if err := edit.ReplaceScopeDefinition(fixture.scopeID, scope); err != nil {
		t.Fatal(err)
	}

	if report, err := edit.Validate(); err != nil || !report.OK() {
		t.Fatalf("replaced schema Validate = %#v, %v", report.Issues(), err)
	}
	v2 := commitPublicSchema(t, edit, fixture.version2ID, "v2", fixture.policy)
	if parent, ok := v2.ParentVersionID(); !ok || !parent.Equal(fixture.version1ID) {
		t.Fatalf("schema v2 parent = %s, %v", parent, ok)
	}
	assertPublicSchemaReadback(t, v2, fixture, "Product v2")
	assertPublicSchemaReadback(t, v1, fixture, "Product")

	wrongIDEntity, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: bcg.NewID(), Name: "wrong"})
	if err != nil {
		t.Fatal(err)
	}
	wrongIDDraft, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	if err := wrongIDDraft.ReplaceEntityType(fixture.entityTypeID, wrongIDEntity); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("replacement with a new ID = %T %[1]v", err)
	}
}

func TestPublicSchemaRemovalAndValidation(t *testing.T) {
	fixture, source := newPublicSchemaDraft(t)
	v1 := commitPublicSchema(t, source, fixture.version1ID, "v1", fixture.policy)

	removals, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	// Every schema kind has an explicit non-cascading removal operation.
	for _, remove := range []func() error{
		func() error { return removals.RemoveEntityType(fixture.entityTypeID) },
		func() error { return removals.RemoveRelationType(fixture.relationTypeID) },
		func() error { return removals.RemovePropertyDefinition(fixture.propertyID) },
		func() error { return removals.RemovePropertyContract(fixture.contractID) },
		func() error { return removals.RemoveTrait(fixture.traitID) },
		func() error { return removals.RemoveScopeDefinition(fixture.scopeID) },
	} {
		if err := remove(); err != nil {
			t.Fatal(err)
		}
	}

	invalid, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	// Removing a required contract intentionally leaves Trait and property
	// references dangling. Existing validation, rather than this API, owns the
	// precise diagnostics and Commit refusal.
	if err := invalid.RemovePropertyContract(fixture.contractID); err != nil {
		t.Fatal(err)
	}
	report, err := invalid.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if report.OK() || report.Len() == 0 {
		t.Fatal("dangling schema removals unexpectedly validated")
	}
	issuesBeforeFailure := report.Len()
	if _, err := invalid.Commit(mustCommitOptions(t, bcg.NewID(), "invalid", fixture.policy)); !errors.Is(err, bcg.ErrModelValidation) {
		t.Fatalf("invalid schema Commit = %T %[1]v", err)
	}
	if report, err := invalid.Validate(); err != nil || report.OK() || report.Len() != issuesBeforeFailure {
		t.Fatalf("failed schema Commit mutated its Draft: %#v, %v", report.Issues(), err)
	}
	if got, ok := v1.EntityType(fixture.entityTypeID); !ok || got.Name() != "Product" {
		t.Fatal("failed schema commit mutated the source model")
	}

	incompatible, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := bcg.EntityTypeOwner(fixture.entityTypeID)
	if err != nil {
		t.Fatal(err)
	}
	badProperty, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           fixture.propertyID,
		Owner:        owner,
		ContractID:   fixture.contractID,
		Name:         "price",
		ValueType:    bcg.StringType,
		Unit:         fixture.unit,
		TemporalMode: bcg.TemporalScalar,
		Computation:  bcg.InputComputation(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := incompatible.ReplacePropertyDefinition(fixture.propertyID, badProperty); err != nil {
		t.Fatal(err)
	}
	if report, err := incompatible.Validate(); err != nil || report.OK() {
		t.Fatalf("incompatible public property = %#v, %v", report.Issues(), err)
	}
	if _, err := incompatible.Commit(mustCommitOptions(t, bcg.NewID(), "invalid-unit-type", fixture.policy)); !errors.Is(err, bcg.ErrModelValidation) {
		t.Fatalf("incompatible public property Commit = %T %[1]v", err)
	}

	if _, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: bcg.ID{}, Name: "invalid"}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("invalid entity spec = %T %[1]v", err)
	}
	if _, err := bcg.DerivedComputation(bcg.Expression{}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("zero derived expression = %T %[1]v", err)
	}
}

func TestPublicSchemaOptionalFormsAndOrdering(t *testing.T) {
	anyRelation, err := bcg.NewRelationType(bcg.RelationTypeSpec{
		ID:   bcg.NewID(),
		Name: "any",
		From: bcg.AnyEntityEndpoint(),
	})
	if err != nil {
		t.Fatal(err)
	}
	from, hasFrom := anyRelation.From()
	if !hasFrom || from.Kind() != bcg.EndpointAnyEntity || !from.IsValid() {
		t.Fatalf("AnyEntity endpoint = %#v, %v", from, hasFrom)
	}
	if _, hasTo := anyRelation.To(); hasTo {
		t.Fatal("omitted relation endpoint became present")
	}
	omittedRelation, err := bcg.NewRelationType(bcg.RelationTypeSpec{ID: bcg.NewID(), Name: "omitted"})
	if err != nil {
		t.Fatal(err)
	}
	if _, hasFrom := omittedRelation.From(); hasFrom {
		t.Fatal("zero endpoint constraint is not an omission")
	}

	relationOwnerID := bcg.NewID()
	owner, err := bcg.RelationTypeOwner(relationOwnerID)
	if err != nil {
		t.Fatal(err)
	}
	relationProperty, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           bcg.NewID(),
		Owner:        owner,
		Name:         "rank",
		ValueType:    bcg.IntegerType,
		TemporalMode: bcg.TemporalScalar,
		Computation:  bcg.InputComputation(),
	})
	if err != nil || relationProperty.Owner().Kind() != bcg.PropertyOwnerRelationType {
		t.Fatalf("relation-owned property = %#v, %v", relationProperty, err)
	}
	dimensionless := bcg.DimensionlessDimension()
	dimensionlessContract, err := bcg.NewPropertyContract(bcg.PropertyContractSpec{
		ID:           bcg.NewID(),
		Name:         "count",
		ValueType:    bcg.IntegerType,
		Dimension:    &dimensionless,
		TemporalMode: bcg.TemporalScalar,
	})
	if err != nil {
		t.Fatal(err)
	}
	gotDimension, hasDimension := dimensionlessContract.Dimension()
	if !hasDimension || !gotDimension.IsDimensionless() {
		t.Fatalf("explicit dimensionless contract = %#v, %v", gotDimension, hasDimension)
	}

	policy, err := bcg.NewPrecisionPolicy(0, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	logical, err := bcg.NewModel(bcg.NewID(), "schema-order")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	firstID := bcg.MustParseID("00000000-0000-4000-8000-000000003111")
	secondID := bcg.MustParseID("00000000-0000-4000-8000-000000003110")
	first, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: firstID, Name: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: secondID, Name: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityType(first); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddEntityType(second); err != nil {
		t.Fatal(err)
	}
	model := commitPublicSchema(t, draft, bcg.NewID(), "v1", policy)
	entityTypes := model.EntityTypes()
	if len(entityTypes) != 2 || !entityTypes[0].ID().Equal(secondID) || !entityTypes[1].ID().Equal(firstID) {
		t.Fatalf("EntityTypes are not canonically ordered: %#v", entityTypes)
	}
}

func newPublicSchemaDraft(t *testing.T) (schemaFixture, *bcg.Draft) {
	t.Helper()
	fixture := schemaFixture{
		modelID:        bcg.MustParseID("00000000-0000-4000-8000-000000003001"),
		version1ID:     bcg.MustParseID("00000000-0000-4000-8000-000000003002"),
		version2ID:     bcg.MustParseID("00000000-0000-4000-8000-000000003003"),
		entityTypeID:   bcg.MustParseID("00000000-0000-4000-8000-000000003004"),
		relationTypeID: bcg.MustParseID("00000000-0000-4000-8000-000000003005"),
		contractID:     bcg.MustParseID("00000000-0000-4000-8000-000000003006"),
		traitID:        bcg.MustParseID("00000000-0000-4000-8000-000000003007"),
		propertyID:     bcg.MustParseID("00000000-0000-4000-8000-000000003008"),
		scopeID:        bcg.MustParseID("00000000-0000-4000-8000-000000003009"),
	}
	var err error
	fixture.policy, err = bcg.NewPrecisionPolicy(8, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}

	dimension, err := bcg.NewBaseDimension("currency")
	if err != nil {
		t.Fatal(err)
	}
	fixture.unit, err = bcg.NewUnit("credit", dimension, "currency", bcg.OneRational())
	if err != nil {
		t.Fatal(err)
	}
	contract, err := bcg.NewPropertyContract(bcg.PropertyContractSpec{
		ID:           fixture.contractID,
		Name:         "price",
		ValueType:    bcg.DecimalType,
		Dimension:    &dimension,
		TemporalMode: bcg.TemporalScalar,
		Nullable:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	trait, err := bcg.NewTrait(bcg.TraitSpec{
		ID:                        fixture.traitID,
		Name:                      "Priced",
		RequiredPropertyContracts: []bcg.ID{fixture.contractID},
	})
	if err != nil {
		t.Fatal(err)
	}
	entity, err := bcg.NewEntityType(bcg.EntityTypeSpec{
		ID:                  fixture.entityTypeID,
		Name:                "Product",
		Traits:              []bcg.ID{fixture.traitID},
		PropertyDefinitions: []bcg.ID{fixture.propertyID},
	})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := bcg.EntityTypeOwner(fixture.entityTypeID)
	if err != nil {
		t.Fatal(err)
	}
	property, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID:           fixture.propertyID,
		Owner:        owner,
		ContractID:   fixture.contractID,
		Name:         "price",
		ValueType:    bcg.DecimalType,
		Unit:         fixture.unit,
		TemporalMode: bcg.TemporalScalar,
		Computation:  bcg.InputComputation(),
	})
	if err != nil {
		t.Fatal(err)
	}
	entityEndpoint, err := bcg.EntityTypeEndpoint(fixture.entityTypeID)
	if err != nil {
		t.Fatal(err)
	}
	traitEndpoint, err := bcg.TraitEndpoint(fixture.traitID)
	if err != nil {
		t.Fatal(err)
	}
	relation, err := bcg.NewRelationType(bcg.RelationTypeSpec{
		ID:   fixture.relationTypeID,
		Name: "offers",
		From: entityEndpoint,
		To:   traitEndpoint,
	})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := bcg.NewScopeDefinition(bcg.ScopeDefinitionSpec{
		ID:        fixture.scopeID,
		Name:      "region",
		ValueType: bcg.StringType,
	})
	if err != nil {
		t.Fatal(err)
	}

	logical, err := bcg.NewModel(fixture.modelID, "public-schema")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	for _, add := range []func() error{
		func() error { return draft.AddPropertyContract(contract) },
		func() error { return draft.AddTrait(trait) },
		func() error { return draft.AddEntityType(entity) },
		func() error { return draft.AddPropertyDefinition(property) },
		func() error { return draft.AddRelationType(relation) },
		func() error { return draft.AddScopeDefinition(scope) },
	} {
		if err := add(); err != nil {
			t.Fatal(err)
		}
	}
	return fixture, draft
}

func assertPublicSchemaReadback(t *testing.T, model bcg.Model, fixture schemaFixture, entityName string) {
	t.Helper()
	if got := model.EntityTypes(); len(got) != 1 || !got[0].ID().Equal(fixture.entityTypeID) {
		t.Fatalf("EntityTypes = %#v", got)
	}
	if got := model.RelationTypes(); len(got) != 1 || !got[0].ID().Equal(fixture.relationTypeID) {
		t.Fatalf("RelationTypes = %#v", got)
	}
	if got := model.Traits(); len(got) != 1 || !got[0].ID().Equal(fixture.traitID) {
		t.Fatalf("Traits = %#v", got)
	}
	if got := model.PropertyContracts(); len(got) != 1 || !got[0].ID().Equal(fixture.contractID) {
		t.Fatalf("PropertyContracts = %#v", got)
	}

	entity, ok := model.EntityType(fixture.entityTypeID)
	if !ok || entity.Name() != entityName || len(entity.Traits()) != 1 || !entity.Traits()[0].Equal(fixture.traitID) ||
		len(entity.PropertyDefinitions()) != 1 || !entity.PropertyDefinitions()[0].Equal(fixture.propertyID) {
		t.Fatalf("EntityType = %#v, %v", entity, ok)
	}
	trait, ok := model.Trait(fixture.traitID)
	if !ok || len(trait.RequiredPropertyContracts()) != 1 || !trait.RequiredPropertyContracts()[0].Equal(fixture.contractID) {
		t.Fatalf("Trait = %#v, %v", trait, ok)
	}
	contract, ok := model.PropertyContract(fixture.contractID)
	dimension, hasDimension := contract.Dimension()
	if !ok || contract.ValueType() != bcg.DecimalType || contract.TemporalMode() != bcg.TemporalScalar || !contract.Nullable() ||
		!hasDimension || !dimension.Equal(fixture.unit.Dimension()) {
		t.Fatalf("PropertyContract = %#v, %v", contract, ok)
	}

	relation, ok := model.RelationType(fixture.relationTypeID)
	from, hasFrom := relation.From()
	to, hasTo := relation.To()
	fromID, hasFromID := from.ID()
	toID, hasToID := to.ID()
	if !ok || !hasFrom || !hasTo || from.Kind() != bcg.EndpointEntityType || to.Kind() != bcg.EndpointTrait ||
		!hasFromID || !hasToID || !fromID.Equal(fixture.entityTypeID) || !toID.Equal(fixture.traitID) {
		t.Fatalf("RelationType = %#v, %v", relation, ok)
	}

	propertyInfo, ok := model.PropertyDefinition(fixture.propertyID)
	property := propertyInfo.Schema()
	unit, hasUnit := property.Unit()
	if !ok || propertyInfo.ValueKind() != bcg.ValueDecimal || property.Owner().Kind() != bcg.PropertyOwnerEntityType ||
		property.ValueType() != bcg.DecimalType || property.TemporalMode() != bcg.TemporalScalar ||
		property.Computation().Kind() != bcg.ComputationInput || !hasUnit || !unit.Equal(fixture.unit) {
		t.Fatalf("PropertyDefinition = %#v, %#v, %v", propertyInfo, property, ok)
	}

	scope, ok := model.ScopeDefinition(fixture.scopeID)
	if !ok || scope.ValueType() != bcg.StringType {
		t.Fatalf("ScopeDefinition = %#v, %v", scope, ok)
	}
	if infos := model.ScopeDefinitions(); len(infos) != 1 || infos[0].Schema().ValueType() != bcg.StringType {
		t.Fatalf("ScopeDefinitions = %#v", infos)
	}
}

func commitPublicSchema(t *testing.T, draft *bcg.Draft, versionID bcg.ID, label string, policy bcg.PrecisionPolicy) bcg.Model {
	t.Helper()
	model, err := draft.Commit(mustCommitOptions(t, versionID, label, policy))
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func mustCommitOptions(t testing.TB, versionID bcg.ID, label string, policy bcg.PrecisionPolicy) bcg.CommitOptions {
	t.Helper()
	options, err := bcg.NewCommitOptions(versionID, label, policy)
	if err != nil {
		t.Fatal(err)
	}
	return options
}
