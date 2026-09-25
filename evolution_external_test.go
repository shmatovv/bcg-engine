package bcg_test

import (
	"bytes"
	"errors"
	"sync"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// This is intentionally a public-package workflow: a client can evolve a
// model, inspect the typed diff, classify it, and use its digest without any
// internal semantic representation.
func TestExternalModelDiffEvolutionAndDigest(t *testing.T) {
	a, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		t.Fatal(err)
	}
	v1 := a.Model()
	edit, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	propertyID := bcg.MustParseID("00000000-0000-4000-8000-000000000004")
	owner, _ := bcg.EntityTypeOwner(bcg.MustParseID("00000000-0000-4000-8000-000000000003"))
	required, _ := bcg.InputComputationWithPresence(bcg.PresenceRequired)
	changed, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: propertyID, Owner: owner, Name: "renamed-amount", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: required})
	if err != nil {
		t.Fatal(err)
	}
	if err := edit.ReplacePropertyDefinition(propertyID, changed); err != nil {
		t.Fatal(err)
	}
	v2, err := edit.Commit(mustCommitOptions(t, bcg.NewID(), "v2", mustPolicy(t)))
	if err != nil {
		t.Fatal(err)
	}

	diff, err := bcg.CompareModels(v1, v2)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Empty() {
		t.Fatal("expected property modification")
	}
	var propertyChange bcg.ModelChange
	for _, c := range diff.Changes() {
		if c.Kind() == bcg.ChangePropertyDefinition {
			propertyChange = c
		}
	}
	if id, ok := propertyChange.ID(); !ok || id != propertyID || propertyChange.Operation() != bcg.ChangeModified {
		t.Fatalf("stable property identity lost: %#v", propertyChange)
	}
	if len(propertyChange.ChangedFields()) == 0 {
		t.Fatal("modified declaration has no fields")
	}

	report, err := bcg.AnalyzeEvolution(v1, v2)
	if err != nil {
		t.Fatal(err)
	}
	if !report.RequiresDataMigration() || !report.Has(bcg.RequiresIntegrityRecheck) {
		t.Fatalf("required-input change effects = %#v", report.Effects().Values())
	}
	if v1.SemanticDigest() == v2.SemanticDigest() {
		t.Fatal("semantic property change did not alter digest")
	}

	// Lifecycle-only edits have equal semantic identity, including after wire IO.
	lifecycle, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	v1again, err := lifecycle.Commit(mustCommitOptions(t, bcg.NewID(), "different label", mustPolicy(t)))
	if err != nil {
		t.Fatal(err)
	}
	if v1.SemanticDigest() != v1again.SemanticDigest() {
		t.Fatal("version lifecycle changed semantic digest")
	}
	artifact, err := bcg.NewArtifact(v1)
	if err != nil {
		t.Fatal(err)
	}
	data, err := bcg.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	round, err := bcg.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if v1.SemanticDigest() != round.Model().SemanticDigest() {
		t.Fatal("round trip changed semantic digest")
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v1.SemanticDigest() != v1.SemanticDigest() {
				t.Error("non-deterministic digest")
			}
		}()
	}
	wg.Wait()

	// Comparison has an explicit logical-model precondition.
	otherArtifact, err := bcg.Unmarshal(bytes.Replace([]byte(publicArtifact), []byte("00000000-0000-4000-8000-000000000001"), []byte("00000000-0000-4000-8000-000000000099"), -1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bcg.CompareModels(v1, otherArtifact.Model()); !errors.Is(err, bcg.ErrDifferentLogicalModel) {
		t.Fatalf("logical-model mismatch = %v", err)
	}
}
