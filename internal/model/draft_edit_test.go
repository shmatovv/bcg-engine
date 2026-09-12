package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
)

func TestNewDraftFromVersionCopiesContentForReplacementAndRemoval(t *testing.T) {
	modelID := identity.MustParse("00000000-0000-4000-8000-000000000201")
	versionID := identity.MustParse("00000000-0000-4000-8000-000000000202")
	typeID := identity.MustParse("00000000-0000-4000-8000-000000000203")

	source := NewDraft()
	source.AddEntityType(schema.NewEntityType(typeID, "original", nil, nil))
	version, err := Commit(
		NewModel(modelID, "draft-copy"),
		source,
		NewCommitOptions(versionID, "v1", precision.New(8, precision.HalfEven)),
	)
	if err != nil {
		t.Fatal(err)
	}

	editable := NewDraftFromVersion(version)
	if !editable.ReplaceEntityType(typeID, schema.NewEntityType(typeID, "replacement", nil, nil)) {
		t.Fatal("ReplaceEntityType did not find copied content")
	}
	if got := editable.EntityTypes(); len(got) != 1 || got[0].Name() != "replacement" {
		t.Fatalf("edited Draft content = %#v", got)
	}
	if got := version.EntityTypes(); len(got) != 1 || got[0].Name() != "original" {
		t.Fatalf("committed version mutated through Draft: %#v", got)
	}
	if !editable.RemoveEntityType(typeID) || !editable.IsEmpty() {
		t.Fatal("RemoveEntityType did not mutate only the editable Draft")
	}
	if got := version.EntityTypes(); len(got) != 1 || got[0].Name() != "original" {
		t.Fatal("committed version mutated through Draft removal")
	}
}
