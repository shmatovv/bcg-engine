package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
)

func TestSemanticTraversalIndexesAreCanonicalAndDefensive(t *testing.T) {
	typeID := identity.MustParse("00000000-0000-4000-8000-000000000001")
	relationTypeID := identity.MustParse("00000000-0000-4000-8000-000000000002")
	entityA := identity.MustParse("00000000-0000-4000-8000-000000000003")
	entityB := identity.MustParse("00000000-0000-4000-8000-000000000004")
	selfRelationID := identity.MustParse("00000000-0000-4000-8000-000000000010")
	outgoingRelationID := identity.MustParse("00000000-0000-4000-8000-000000000020")
	incomingRelationID := identity.MustParse("00000000-0000-4000-8000-000000000030")

	draft := NewDraft()
	draft.AddEntityType(schema.NewEntityType(typeID, "node", nil, nil))
	endpoint := schema.EntityTypeConstraint(typeID)
	draft.AddRelationType(schema.NewRelationType(
		relationTypeID,
		"edge",
		endpoint,
		true,
		endpoint,
		true,
		nil,
	))
	draft.AddEntity(semantic.NewEntity(entityA, typeID, "", false))
	draft.AddEntity(semantic.NewEntity(entityB, typeID, "", false))
	// Deliberately shuffled insertion order. Two differently identified
	// relations between the same endpoints would remain two occurrences too.
	draft.AddRelation(semantic.NewRelation(incomingRelationID, relationTypeID, entityB, entityA, nil, false))
	draft.AddRelation(semantic.NewRelation(outgoingRelationID, relationTypeID, entityA, entityB, nil, false))
	draft.AddRelation(semantic.NewRelation(selfRelationID, relationTypeID, entityA, entityA, nil, false))

	version, err := Commit(
		NewModel(identity.MustParse("00000000-0000-4000-8000-000000000040"), "graph"),
		draft,
		NewCommitOptions(identity.MustParse("00000000-0000-4000-8000-000000000041"), "graph", precision.New(6, precision.HalfEven)),
	)
	if err != nil {
		t.Fatal(err)
	}

	outgoing := version.OutgoingRelations(entityA, relationTypeID)
	if len(outgoing) != 2 || outgoing[0].ID() != selfRelationID || outgoing[1].ID() != outgoingRelationID {
		t.Fatalf("outgoing traversal index = %#v", outgoing)
	}
	incoming := version.IncomingRelations(entityA, relationTypeID)
	if len(incoming) != 2 || incoming[0].ID() != selfRelationID || incoming[1].ID() != incomingRelationID {
		t.Fatalf("incoming traversal index = %#v", incoming)
	}

	outgoing[0] = semantic.Relation{}
	again := version.OutgoingRelations(entityA, relationTypeID)
	if len(again) != 2 || again[0].ID() != selfRelationID {
		t.Fatal("traversal index getter exposes mutable storage")
	}
}
