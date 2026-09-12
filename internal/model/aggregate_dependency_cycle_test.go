package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestAggregateConcreteDependenciesParticipateInCycleValidation(t *testing.T) {
	typeID, relationTypeID, definitionID := identity.New(), identity.New(), identity.New()
	entityA, entityB := identity.New(), identity.New()
	draft := NewDraft()
	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(typeID),
		identity.UUID{},
		false,
		"total",
		value.Integer,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.DerivedComputation(ast.NewMappedAggregate(
			ast.NewTraversalSource(ast.TraversalDirectionOutgoing, relationTypeID),
			ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, definitionID),
			ast.AggregateOperationSum,
		)),
	)
	draft.AddEntityType(schema.NewEntityType(typeID, "node", nil, []identity.UUID{definitionID}))
	draft.AddPropertyDefinition(definition)
	endpoint := schema.EntityTypeConstraint(typeID)
	draft.AddRelationType(schema.NewRelationType(relationTypeID, "edge", endpoint, true, endpoint, true, nil))
	draft.AddEntity(semantic.NewEntity(entityA, typeID, "", false))
	draft.AddEntity(semantic.NewEntity(entityB, typeID, "", false))
	draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, entityA, entityB, nil, false))
	draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, entityB, entityA, nil, false))

	first := draft.Validate()
	second := draft.Validate()
	if !first.Has(ValidationIssueCycle) || !second.Has(ValidationIssueCycle) {
		t.Fatalf("aggregate cycle was not rejected: %v", first.Issues())
	}
	if first.Error() != second.Error() {
		t.Fatalf("cycle diagnostic is not deterministic:\n%s\n%s", first.Error(), second.Error())
	}
}
