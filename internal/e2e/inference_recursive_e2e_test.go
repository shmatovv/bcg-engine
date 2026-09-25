package e2e_test

import (
	"errors"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

type recursiveInferenceResolver struct {
	definitions   map[identity.UUID]schema.PropertyDefinition
	contracts     map[identity.UUID]schema.PropertyContract
	entityTypes   map[identity.UUID]schema.EntityType
	relationTypes map[identity.UUID]schema.RelationType
	contractTypes map[identity.UUID]typing.Type
}

func (r *recursiveInferenceResolver) PropertyDefinition(
	id identity.UUID,
) (schema.PropertyDefinition, bool) {
	v, ok := r.definitions[id]
	return v, ok
}

func (r *recursiveInferenceResolver) PropertyContract(
	id identity.UUID,
) (schema.PropertyContract, bool) {
	v, ok := r.contracts[id]
	return v, ok
}

func (r *recursiveInferenceResolver) EntityType(
	id identity.UUID,
) (schema.EntityType, bool) {
	v, ok := r.entityTypes[id]
	return v, ok
}

func (r *recursiveInferenceResolver) RelationType(
	id identity.UUID,
) (schema.RelationType, bool) {
	v, ok := r.relationTypes[id]
	return v, ok
}

func (r *recursiveInferenceResolver) ObjectType(
	address.ObjectRef,
) (schema.PropertyOwner, bool) {
	return schema.PropertyOwner{}, false
}

func (r *recursiveInferenceResolver) ContractImplementation(
	schema.PropertyOwner,
	identity.UUID,
) (schema.PropertyDefinition, bool) {
	return schema.PropertyDefinition{}, false
}

func (r *recursiveInferenceResolver) ContractType(
	_ schema.TypeConstraint,
	contractID identity.UUID,
) (typing.Type, bool) {
	v, ok := r.contractTypes[contractID]
	return v, ok
}

func TestE2E_RecursiveInference_FullChain(t *testing.T) {
	palletID := identity.New()
	boardID := identity.New()
	relationID := identity.New()

	baseID := identity.New()
	costContractID := identity.New()
	boardCostID := identity.New()

	base := schema.NewPropertyDefinition(
		baseID,
		schema.EntityTypeOwner(palletID),
		identity.UUID{},
		false,
		"Base",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	costContract := schema.NewPropertyContract(
		costContractID,
		"Cost",
		value.Decimal,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		true,
	)

	boardCost := schema.NewPropertyDefinition(
		boardCostID,
		schema.EntityTypeOwner(boardID),
		costContractID,
		true,
		"Cost",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	pallet := schema.NewEntityType(
		palletID,
		"Pallet",
		nil,
		[]identity.UUID{
			baseID,
		},
	)

	board := schema.NewEntityType(
		boardID,
		"Board",
		nil,
		[]identity.UUID{
			boardCostID,
		},
	)

	contains := schema.NewRelationType(
		relationID,
		"Contains",
		schema.EntityTypeConstraint(palletID),
		true,
		schema.EntityTypeConstraint(boardID),
		true,
		nil,
	)

	resolver := &recursiveInferenceResolver{
		definitions: map[identity.UUID]schema.PropertyDefinition{
			baseID:      base,
			boardCostID: boardCost,
		},
		contracts: map[identity.UUID]schema.PropertyContract{
			costContractID: costContract,
		},
		entityTypes: map[identity.UUID]schema.EntityType{
			palletID: pallet,
			boardID:  board,
		},
		relationTypes: map[identity.UUID]schema.RelationType{
			relationID: contains,
		},
		contractTypes: map[identity.UUID]typing.Type{},
	}

	context := inference.NewContext(
		schema.EntityTypeOwner(palletID),
	)

	source := ast.NewTraversalSource(
		ast.TraversalDirectionOutgoing,
		relationID,
	)

	t.Run(
		"self plus aggregate bound definition propagates nullable",
		func(t *testing.T) {
			aggregate := ast.NewMappedAggregate(
				source,
				ast.NewBoundPropertyDefinitionReference(
					ast.BoundTargetEntity,
					boardCostID,
				),
				ast.AggregateOperationSum,
			)

			expression := ast.Add(
				ast.NewSelfPropertyReference(baseID),
				aggregate,
			)

			got, err := inference.Infer(
				expression,
				context,
				resolver,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireRecursiveType(
				t,
				got,
				value.Decimal,
				typing.TemporalScalar,
				true,
			)
		},
	)

	t.Run(
		"average through traversal preserves nullable",
		func(t *testing.T) {
			aggregate := ast.NewMappedAggregate(
				source,
				ast.NewBoundPropertyDefinitionReference(
					ast.BoundTargetEntity,
					boardCostID,
				),
				ast.AggregateOperationAverage,
			)

			got, err := inference.Infer(
				aggregate,
				context,
				resolver,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireRecursiveType(
				t,
				got,
				value.Decimal,
				typing.TemporalScalar,
				true,
			)
		},
	)

	t.Run(
		"count through traversal is scalar nonnullable integer",
		func(t *testing.T) {
			got, err := inference.Infer(
				ast.NewCountAggregate(source),
				context,
				resolver,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireRecursiveType(
				t,
				got,
				value.Integer,
				typing.TemporalScalar,
				false,
			)
		},
	)

	t.Run(
		"wrong traversal direction fails before map inference",
		func(t *testing.T) {
			incoming := ast.NewTraversalSource(
				ast.TraversalDirectionIncoming,
				relationID,
			)

			expression := ast.NewMappedAggregate(
				incoming,
				ast.NewBoundPropertyDefinitionReference(
					ast.BoundTargetEntity,
					boardCostID,
				),
				ast.AggregateOperationSum,
			)

			_, err := inference.Infer(
				expression,
				context,
				resolver,
			)

			var aggregateErr inference.AggregateInferenceError
			var traversalErr inference.TraversalError

			if !errors.As(err, &aggregateErr) ||
				aggregateErr.Kind() !=
					inference.AggregateInferenceErrorTraversal {
				t.Fatalf(
					"got %v, want aggregate traversal error",
					err,
				)
			}

			if !errors.As(err, &traversalErr) ||
				traversalErr.Kind() !=
					inference.TraversalErrorEndpointNotApplicable {
				t.Fatalf(
					"got %v, want EndpointNotApplicable",
					err,
				)
			}
		},
	)

	t.Run(
		"bound reference cannot escape aggregate binding",
		func(t *testing.T) {
			_, err := inference.Infer(
				ast.NewBoundPropertyDefinitionReference(
					ast.BoundTargetEntity,
					boardCostID,
				),
				context,
				resolver,
			)

			var refErr inference.ReferenceError

			if !errors.As(err, &refErr) ||
				refErr.Kind() !=
					inference.ReferenceErrorMissingBinding {
				t.Fatalf(
					"got %v, want MissingBinding",
					err,
				)
			}
		},
	)

	t.Run(
		"aggregate filter is inferred inside binding",
		func(t *testing.T) {
			filter := ast.NewIsNull(
				ast.NewBoundPropertyDefinitionReference(
					ast.BoundTargetEntity,
					boardCostID,
				),
			)

			expression := ast.NewFilteredCountAggregate(
				source,
				filter,
			)

			got, err := inference.Infer(
				expression,
				context,
				resolver,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireRecursiveType(
				t,
				got,
				value.Integer,
				typing.TemporalScalar,
				false,
			)
		},
	)
}

func TestE2E_RecursiveInference_TraitContractBinding(
	t *testing.T,
) {
	palletID := identity.New()
	boardID := identity.New()
	materialTraitID := identity.New()
	relationID := identity.New()
	priceContractID := identity.New()

	pallet := schema.NewEntityType(
		palletID,
		"Pallet",
		nil,
		nil,
	)

	board := schema.NewEntityType(
		boardID,
		"Board",
		[]identity.UUID{
			materialTraitID,
		},
		nil,
	)

	relation := schema.NewRelationType(
		relationID,
		"ContainsMaterial",
		schema.EntityTypeConstraint(palletID),
		true,
		schema.TraitConstraint(materialTraitID),
		true,
		nil,
	)

	contract := schema.NewPropertyContract(
		priceContractID,
		"Price",
		value.Decimal,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		true,
	)

	resolver := &recursiveInferenceResolver{
		definitions: map[identity.UUID]schema.PropertyDefinition{},
		contracts: map[identity.UUID]schema.PropertyContract{
			priceContractID: contract,
		},
		entityTypes: map[identity.UUID]schema.EntityType{
			palletID: pallet,
			boardID:  board,
		},
		relationTypes: map[identity.UUID]schema.RelationType{
			relationID: relation,
		},
		contractTypes: map[identity.UUID]typing.Type{
			priceContractID: typing.New(
				value.Decimal,
			).WithNullable(true),
		},
	}

	expression := ast.NewMappedAggregate(
		ast.NewTraversalSource(
			ast.TraversalDirectionOutgoing,
			relationID,
		),
		ast.NewBoundContractPropertyReference(
			ast.BoundTargetEntity,
			priceContractID,
		),
		ast.AggregateOperationSum,
	)

	got, err := inference.Infer(
		expression,
		inference.NewContext(
			schema.EntityTypeOwner(palletID),
		),
		resolver,
	)
	if err != nil {
		t.Fatal(err)
	}

	requireRecursiveType(
		t,
		got,
		value.Decimal,
		typing.TemporalScalar,
		true,
	)
}

func requireRecursiveType(
	t *testing.T,
	got typing.Type,
	wantValue value.Type,
	wantTemporal typing.TemporalMode,
	wantNullable bool,
) {
	t.Helper()

	if got.ValueType() != wantValue ||
		got.TemporalMode() != wantTemporal ||
		got.Nullable() != wantNullable {
		t.Fatalf(
			"got value=%s temporal=%s nullable=%v; want value=%s temporal=%s nullable=%v",
			got.ValueType(),
			got.TemporalMode(),
			got.Nullable(),
			wantValue,
			wantTemporal,
			wantNullable,
		)
	}
}
