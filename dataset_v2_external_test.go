package bcg_test

import (
	"sync"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

func TestDatasetContentDigestIsIdentityIndependentAndDeterministic(t *testing.T) {
	m, typ, _, _ := datasetInputModel(t, false)
	e, err := bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: typ})
	if err != nil {
		t.Fatal(err)
	}
	build := func() bcg.Dataset {
		b, err := bcg.NewDatasetBuilder(m, bcg.NewID())
		if err != nil {
			t.Fatal(err)
		}
		if err = b.AddEntity(e); err != nil {
			t.Fatal(err)
		}
		d, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	a, b := build(), build()
	if a.ContentDigest() != b.ContentDigest() {
		t.Fatal("DatasetID affected content digest")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if a.ContentDigest() != a.ContentDigest() {
				t.Error("non-deterministic digest")
			}
		}()
	}
	wg.Wait()
}

func TestDatasetPatchAndRebasePublicWorkflow(t *testing.T) {
	model, typeID, propertyID, _ := datasetInputModel(t, false)
	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	b, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	d1, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	d1Digest := d1.ContentDigest()
	validator := model.NewRuntimeValidator()
	if report := validator.ValidateEntity(entity); !report.OK() {
		t.Fatalf("local entity validation: %#v", report.Issues())
	}

	second, err := bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	pb, err := bcg.NewDatasetPatchBuilder(d1.ID())
	if err != nil {
		t.Fatal(err)
	}
	if err = pb.AddEntity(second); err != nil {
		t.Fatal(err)
	}
	patch, err := pb.Build()
	if err != nil {
		t.Fatal(err)
	}
	d2, report, err := d1.Apply(model, bcg.NewID(), patch)
	if err != nil || !report.OK() {
		t.Fatalf("Apply: %#v %v", report.Issues(), err)
	}
	if len(d1.Entities()) != 1 || len(d2.Entities()) != 2 || d1.ID() == d2.ID() {
		t.Fatal("patch did not create an independent Dataset")
	}
	if d1Digest == d2.ContentDigest() {
		t.Fatal("content-changing patch retained DatasetContentDigest")
	}
	snapshot, err := model.WithDataset(d2)
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.CheckStructuralIntegrity(); !got.OK() {
		t.Fatalf("unexpected integrity: %#v", got.Issues())
	}

	edit, err := model.Edit()
	if err != nil {
		t.Fatal(err)
	}
	optional, err := bcg.InputComputationWithPresence(bcg.PresenceOptional)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		t.Fatal(err)
	}
	addition, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: bcg.NewID(), Owner: owner, Name: "optional", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: optional})
	if err != nil {
		t.Fatal(err)
	}
	if err = edit.AddPropertyDefinition(addition); err != nil {
		t.Fatal(err)
	}
	// The EntityType must expose the new property for a valid authored model.
	et, _ := model.EntityType(typeID)
	replacement, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: et.Name(), Traits: et.Traits(), PropertyDefinitions: append(et.PropertyDefinitions(), addition.ID())})
	if err != nil {
		t.Fatal(err)
	}
	if err = edit.ReplaceEntityType(typeID, replacement); err != nil {
		t.Fatal(err)
	}
	v2, err := edit.Commit(mustDatasetCommitOptions(t, mustPolicy(t)))
	if err != nil {
		t.Fatal(err)
	}
	if evolution, err := bcg.AnalyzeEvolution(model, v2); err != nil || len(evolution.Changes()) == 0 {
		t.Fatalf("evolution = %#v %v", evolution, err)
	}
	d3, structural, err := d2.Rebase(model, v2, bcg.NewID())
	if err != nil || !structural.OK() {
		t.Fatalf("Rebase: %#v %v", structural.Issues(), err)
	}
	if d3.ModelVersionID() != v2.VersionID() || len(d3.Entities()) != 2 {
		t.Fatal("rebase did not preserve data under target model")
	}
	if d3.ContentDigest() != d2.ContentDigest() || d3.ModelSemanticDigest() == d2.ModelSemanticDigest() {
		t.Fatal("rebase did not preserve content identity while changing model identity")
	}
	_ = propertyID
}
