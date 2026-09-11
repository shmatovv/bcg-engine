// internal/expression/inference/traversal_optional_endpoints_test.go

package inference

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
)

func TestBindingFromTraversal_OptionalEndpointsAreAnyEntity(t *testing.T) {
	selfID := identity.New()
	otherID := identity.New()

	self := schema.NewEntityType(
		selfID,
		"Self",
		nil,
		nil,
	)

	other := schema.NewEntityType(
		otherID,
		"Other",
		nil,
		nil,
	)

	tests := []struct {
		name               string
		from               schema.TypeConstraint
		hasFrom            bool
		to                 schema.TypeConstraint
		hasTo              bool
		direction          ast.TraversalDirection
		wantConstraintKind schema.TypeConstraintKind
		wantConstraintID   identity.UUID
		wantConstraintIDOK bool
	}{
		{
			name:               "outgoing absent from accepts self",
			from:               schema.TypeConstraint{},
			hasFrom:            false,
			to:                 schema.EntityTypeConstraint(otherID),
			hasTo:              true,
			direction:          ast.TraversalDirectionOutgoing,
			wantConstraintKind: schema.TypeConstraintEntityType,
			wantConstraintID:   otherID,
			wantConstraintIDOK: true,
		},
		{
			name:               "outgoing absent to binds any entity",
			from:               schema.EntityTypeConstraint(selfID),
			hasFrom:            true,
			to:                 schema.TypeConstraint{},
			hasTo:              false,
			direction:          ast.TraversalDirectionOutgoing,
			wantConstraintKind: schema.TypeConstraintAnyEntity,
			wantConstraintIDOK: false,
		},
		{
			name:               "incoming absent to accepts self",
			from:               schema.EntityTypeConstraint(otherID),
			hasFrom:            true,
			to:                 schema.TypeConstraint{},
			hasTo:              false,
			direction:          ast.TraversalDirectionIncoming,
			wantConstraintKind: schema.TypeConstraintEntityType,
			wantConstraintID:   otherID,
			wantConstraintIDOK: true,
		},
		{
			name:               "incoming absent from binds any entity",
			from:               schema.TypeConstraint{},
			hasFrom:            false,
			to:                 schema.EntityTypeConstraint(selfID),
			hasTo:              true,
			direction:          ast.TraversalDirectionIncoming,
			wantConstraintKind: schema.TypeConstraintAnyEntity,
			wantConstraintIDOK: false,
		},
		{
			name:               "outgoing both absent binds any entity",
			from:               schema.TypeConstraint{},
			hasFrom:            false,
			to:                 schema.TypeConstraint{},
			hasTo:              false,
			direction:          ast.TraversalDirectionOutgoing,
			wantConstraintKind: schema.TypeConstraintAnyEntity,
			wantConstraintIDOK: false,
		},
		{
			name:               "incoming both absent binds any entity",
			from:               schema.TypeConstraint{},
			hasFrom:            false,
			to:                 schema.TypeConstraint{},
			hasTo:              false,
			direction:          ast.TraversalDirectionIncoming,
			wantConstraintKind: schema.TypeConstraintAnyEntity,
			wantConstraintIDOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			relationID := identity.New()

			relation := schema.NewRelationType(
				relationID,
				"Relation",
				tc.from,
				tc.hasFrom,
				tc.to,
				tc.hasTo,
				nil,
			)

			resolver := traversalTestResolver{
				entityTypes: map[identity.UUID]schema.EntityType{
					selfID:  self,
					otherID: other,
				},
				relationTypes: map[identity.UUID]schema.RelationType{
					relationID: relation,
				},
			}

			binding, err := BindingFromTraversal(
				ast.NewTraversalSource(
					tc.direction,
					relationID,
				),
				NewContext(
					schema.EntityTypeOwner(selfID),
				),
				resolver,
			)
			if err != nil {
				t.Fatalf(
					"BindingFromTraversal() error = %v",
					err,
				)
			}

			constraints := binding.Entity().Constraints()
			if len(constraints) != 1 {
				t.Fatalf(
					"got %d entity constraints, want 1",
					len(constraints),
				)
			}

			constraint := constraints[0]

			if constraint.Kind() != tc.wantConstraintKind {
				t.Fatalf(
					"constraint kind = %s, want %s",
					constraint.Kind(),
					tc.wantConstraintKind,
				)
			}

			gotID, gotIDOK := constraint.ID()

			if gotIDOK != tc.wantConstraintIDOK {
				t.Fatalf(
					"constraint ID presence = %v, want %v",
					gotIDOK,
					tc.wantConstraintIDOK,
				)
			}

			if tc.wantConstraintIDOK &&
				gotID != tc.wantConstraintID {
				t.Fatalf(
					"constraint ID = %s, want %s",
					gotID,
					tc.wantConstraintID,
				)
			}
		})
	}
}

func TestBindingFromTraversal_BothWithOptionalEndpoint(t *testing.T) {
	selfID := identity.New()
	otherID := identity.New()
	relationID := identity.New()

	self := schema.NewEntityType(
		selfID,
		"Self",
		nil,
		nil,
	)

	relation := schema.NewRelationType(
		relationID,
		"Relation",
		schema.EntityTypeConstraint(selfID),
		true,
		schema.TypeConstraint{},
		false,
		nil,
	)

	resolver := traversalTestResolver{
		entityTypes: map[identity.UUID]schema.EntityType{
			selfID: self,
			otherID: schema.NewEntityType(
				otherID,
				"Other",
				nil,
				nil,
			),
		},
		relationTypes: map[identity.UUID]schema.RelationType{
			relationID: relation,
		},
	}

	binding, err := BindingFromTraversal(
		ast.NewTraversalSource(
			ast.TraversalDirectionBoth,
			relationID,
		),
		NewContext(
			schema.EntityTypeOwner(selfID),
		),
		resolver,
	)
	if err != nil {
		t.Fatalf(
			"BindingFromTraversal() error = %v",
			err,
		)
	}

	constraints := binding.Entity().Constraints()

	if len(constraints) != 2 {
		t.Fatalf(
			"got %d entity constraints, want 2",
			len(constraints),
		)
	}

	if constraints[0].Kind() !=
		schema.TypeConstraintAnyEntity {
		t.Fatalf(
			"first constraint = %s, want AnyEntity",
			constraints[0].Kind(),
		)
	}

	if constraints[1].Kind() !=
		schema.TypeConstraintEntityType {
		t.Fatalf(
			"second constraint = %s, want EntityType",
			constraints[1].Kind(),
		)
	}

	gotID, ok := constraints[1].ID()

	if !ok || gotID != selfID {
		t.Fatalf(
			"second constraint ID = %s, %v; want %s, true",
			gotID,
			ok,
			selfID,
		)
	}
}

type traversalTestResolver struct {
	entityTypes   map[identity.UUID]schema.EntityType
	relationTypes map[identity.UUID]schema.RelationType
}

func (r traversalTestResolver) PropertyDefinition(
	identity.UUID,
) (schema.PropertyDefinition, bool) {
	return schema.PropertyDefinition{}, false
}

func (r traversalTestResolver) PropertyContract(
	identity.UUID,
) (schema.PropertyContract, bool) {
	return schema.PropertyContract{}, false
}

func (r traversalTestResolver) EntityType(
	id identity.UUID,
) (schema.EntityType, bool) {
	entityType, ok := r.entityTypes[id]

	return entityType, ok
}

func (r traversalTestResolver) RelationType(
	id identity.UUID,
) (schema.RelationType, bool) {
	relationType, ok := r.relationTypes[id]

	return relationType, ok
}

func (r traversalTestResolver) ObjectType(
	address.ObjectRef,
) (schema.PropertyOwner, bool) {
	return schema.PropertyOwner{}, false
}

func (r traversalTestResolver) ContractImplementation(
	schema.PropertyOwner,
	identity.UUID,
) (schema.PropertyDefinition, bool) {
	return schema.PropertyDefinition{}, false
}

func (r traversalTestResolver) ContractType(
	schema.TypeConstraint,
	identity.UUID,
) (typing.Type, bool) {
	return typing.Type{}, false
}

var _ Resolver = traversalTestResolver{}
