package bcg_test

import (
	"errors"
	bcg "github.com/shmatovv/bcg-engine/v2"
	"testing"
)

// This external-only test locks down public Schema v2 authoring and
// inspection without relying on internal AST or model packages.
func TestSchemaV2PublicAuthoringAndV1WireBoundary(t *testing.T) {
	policy := mustPolicy(t)
	model, err := bcg.NewModel(bcg.NewID(), "v2 schema")
	if err != nil {
		t.Fatal(err)
	}
	d, err := model.Draft()
	if err != nil {
		t.Fatal(err)
	}
	enumID, openID, closedID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	status, err := bcg.NewEnumDefinition(bcg.EnumDefinitionSpec{ID: enumID, Name: "Status", Options: []bcg.EnumOptionSpec{{ID: openID, Name: "Open"}, {ID: closedID, Name: "Closed"}}})
	if err != nil {
		t.Fatal(err)
	}
	entityTypeID, propertyID, keyID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	entityID, invariantID := bcg.NewID(), bcg.NewID()
	owner, err := bcg.EntityTypeOwner(entityTypeID)
	if err != nil {
		t.Fatal(err)
	}
	presence, err := bcg.InputComputationWithPresence(bcg.PresenceRequired)
	if err != nil {
		t.Fatal(err)
	}
	property, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: propertyID, Owner: owner, Name: "status", ValueType: bcg.EnumType, EnumDefinitionID: enumID, TemporalMode: bcg.TemporalScalar, Computation: presence})
	if err != nil {
		t.Fatal(err)
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: entityTypeID, Name: "Order", PropertyDefinitions: []bcg.ID{propertyID}})
	if err != nil {
		t.Fatal(err)
	}
	key, err := bcg.NewEntityKeyDefinition(bcg.EntityKeyDefinitionSpec{ID: keyID, Name: "status-key", EntityTypeID: entityTypeID, PropertyDefinitionIDs: []bcg.ID{propertyID}})
	if err != nil {
		t.Fatal(err)
	}
	subject, err := bcg.EntityInvariantSubject(entityTypeID)
	if err != nil {
		t.Fatal(err)
	}
	assertion, err := bcg.Constant(bcg.Bool(true))
	if err != nil {
		t.Fatal(err)
	}
	invariant, err := bcg.NewInvariant(bcg.InvariantSpec{ID: invariantID, Name: "always", Subject: subject, Assertion: assertion})
	if err != nil {
		t.Fatal(err)
	}
	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: entityTypeID})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []any{status, entityType, property, key} {
		switch x := v.(type) {
		case bcg.EnumDefinition:
			err = d.AddEnumDefinition(x)
		case bcg.EntityType:
			err = d.AddEntityType(x)
		case bcg.PropertyDefinition:
			err = d.AddPropertyDefinition(x)
		case bcg.EntityKeyDefinition:
			err = d.AddEntityKeyDefinition(x)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := d.AddInvariant(invariant); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	committed, err := d.Commit(mustCommitOptions(t, bcg.NewID(), "v2", policy))
	if err != nil {
		t.Fatal(err)
	}
	enums := committed.EnumDefinitions()
	if len(enums) != 1 || len(enums[0].Options()) != 2 || enums[0].Options()[1].ID() != closedID {
		t.Fatalf("enum inspection lost semantic identity: %#v", enums)
	}
	definitions := committed.PropertyDefinitionDeclarations()
	if got, ok := definitions[0].EnumDefinitionID(); !ok || got != enumID {
		t.Fatalf("enum property identity = %v %v", got, ok)
	}
	if p, ok := definitions[0].Computation().Presence(); !ok || p != bcg.PresenceRequired {
		t.Fatalf("presence = %v %v", p, ok)
	}
	if len(committed.EntityKeyDefinitions()) != 1 || committed.EntityKeyDefinitions()[0].PropertyDefinitionIDs()[0] != propertyID {
		t.Fatal("key inspection lost component ordering")
	}
	at, _ := bcg.ParseDateTime("2026-01-01T00:00:00Z")
	ctx, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	result, err := committed.CheckInvariant(invariantID, entity, ctx)
	if err != nil || result.State() != bcg.InvariantSatisfied {
		t.Fatalf("invariant = %#v, %v", result, err)
	}
	if _, err := bcg.NewArtifact(committed); !errors.Is(err, bcg.ErrWireSemantic) {
		t.Fatalf("v1 artifact must reject v2 schema, got %v", err)
	}
}
