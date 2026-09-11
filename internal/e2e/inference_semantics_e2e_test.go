package e2e_test

import (
	"errors"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

type inferenceResolver struct {
	definitions   map[identity.UUID]schema.PropertyDefinition
	contracts     map[identity.UUID]schema.PropertyContract
	entityTypes   map[identity.UUID]schema.EntityType
	relationTypes map[identity.UUID]schema.RelationType
}

func (r *inferenceResolver) PropertyDefinition(
	id identity.UUID,
) (schema.PropertyDefinition, bool) {
	v, ok := r.definitions[id]
	return v, ok
}

func (r *inferenceResolver) PropertyContract(
	id identity.UUID,
) (schema.PropertyContract, bool) {
	v, ok := r.contracts[id]
	return v, ok
}

func (r *inferenceResolver) EntityType(
	id identity.UUID,
) (schema.EntityType, bool) {
	v, ok := r.entityTypes[id]
	return v, ok
}

func (r *inferenceResolver) RelationType(
	id identity.UUID,
) (schema.RelationType, bool) {
	v, ok := r.relationTypes[id]
	return v, ok
}

func (r *inferenceResolver) ObjectType(
	address.ObjectRef,
) (schema.PropertyOwner, bool) {
	return schema.PropertyOwner{}, false
}

func (r *inferenceResolver) ContractImplementation(
	schema.PropertyOwner,
	identity.UUID,
) (schema.PropertyDefinition, bool) {
	return schema.PropertyDefinition{}, false
}

func (r *inferenceResolver) ContractType(
	schema.TypeConstraint,
	identity.UUID,
) (typing.Type, bool) {
	return typing.Type{}, false
}

func TestE2E_StaticQualifiers(t *testing.T) {
	scalar := typing.New(value.Decimal)
	nullableScalar := scalar.WithNullable(true)

	series := scalar.WithTemporalMode(
		typing.TemporalSeries,
	)

	nullableSeries := series.WithNullable(true)

	t.Run(
		"arithmetic propagates nullable",
		func(t *testing.T) {
			got, err := typecheck.InferArithmetic(
				ast.ArithmeticAdd,
				nullableScalar,
				scalar,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalScalar,
				true,
			)
		},
	)

	t.Run(
		"arithmetic preserves series",
		func(t *testing.T) {
			got, err := typecheck.InferArithmetic(
				ast.ArithmeticAdd,
				series,
				series,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalSeries,
				false,
			)
		},
	)

	t.Run(
		"arithmetic rejects scalar series mixing",
		func(t *testing.T) {
			_, err := typecheck.InferArithmetic(
				ast.ArithmeticAdd,
				scalar,
				series,
			)

			if err == nil {
				t.Fatal(
					"expected temporal mismatch",
				)
			}
		},
	)

	t.Run(
		"logical propagates nullable",
		func(t *testing.T) {
			b := typing.New(value.Bool)

			got, err := typecheck.InferLogical(
				b.WithNullable(true),
				b,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalScalar,
				true,
			)
		},
	)

	t.Run(
		"not preserves qualifiers",
		func(t *testing.T) {
			operand := typing.New(value.Bool).
				WithTemporalMode(
					typing.TemporalSeries,
				).
				WithNullable(true)

			got, err := typecheck.InferNot(
				operand,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalSeries,
				true,
			)
		},
	)

	t.Run(
		"is null removes nullable and preserves temporal",
		func(t *testing.T) {
			got, err := typecheck.InferIsNull(
				nullableSeries,
			)
			if err != nil {
				t.Fatal(err)
			}

			if got.ValueType() != value.Bool {
				t.Fatalf(
					"got %v, want Bool",
					got.ValueType(),
				)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalSeries,
				false,
			)
		},
	)

	t.Run(
		"coalesce nullable with nonnullable becomes nonnullable",
		func(t *testing.T) {
			got, err := typecheck.InferCoalesce(
				nullableScalar,
				scalar,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalScalar,
				false,
			)
		},
	)

	t.Run(
		"coalesce nullable with nullable remains nullable",
		func(t *testing.T) {
			got, err := typecheck.InferCoalesce(
				nullableScalar,
				nullableScalar,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalScalar,
				true,
			)
		},
	)

	t.Run(
		"if rejects series condition",
		func(t *testing.T) {
			condition := typing.New(value.Bool).
				WithTemporalMode(
					typing.TemporalSeries,
				)

			_, err := typecheck.InferIf(
				condition,
				scalar,
				scalar,
			)

			if err == nil {
				t.Fatal(
					"expected non-scalar condition error",
				)
			}
		},
	)

	t.Run(
		"unify unions nullability",
		func(t *testing.T) {
			got, err := typecheck.Unify(
				nullableScalar,
				scalar,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalScalar,
				true,
			)
		},
	)

	t.Run(
		"unify rejects different temporal modes",
		func(t *testing.T) {
			_, err := typecheck.Unify(
				scalar,
				series,
			)

			if err == nil {
				t.Fatal(
					"expected temporal mismatch",
				)
			}
		},
	)

	t.Run(
		"average preserves series nullable and promotes integer",
		func(t *testing.T) {
			input := typing.New(value.Integer).
				WithTemporalMode(
					typing.TemporalSeries,
				).
				WithNullable(true)

			got, err := typecheck.InferAggregate(
				ast.AggregateOperationAverage,
				input,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			if got.ValueType() != value.Decimal {
				t.Fatalf(
					"got %v, want Decimal",
					got.ValueType(),
				)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalSeries,
				true,
			)
		},
	)

	t.Run(
		"count is scalar nonnullable integer",
		func(t *testing.T) {
			got, err := typecheck.InferAggregate(
				ast.AggregateOperationCount,
				typing.Type{},
				false,
			)
			if err != nil {
				t.Fatal(err)
			}

			if got.ValueType() != value.Integer {
				t.Fatalf(
					"got %v, want Integer",
					got.ValueType(),
				)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalScalar,
				false,
			)
		},
	)
}

func TestE2E_PropertyDefinitionQualifiers(
	t *testing.T,
) {
	ownerID := identity.New()
	contractID := identity.New()
	definitionID := identity.New()

	contract := schema.NewPropertyContract(
		contractID,
		"Metric",
		value.Decimal,
		unit.Dimension{},
		false,
		schema.TemporalSeries,
		true,
	)

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(ownerID),
		contractID,
		true,
		"Metric",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalSeries,
		schema.InputComputation(),
	)

	resolver := &inferenceResolver{
		definitions: map[identity.UUID]schema.PropertyDefinition{
			definitionID: definition,
		},
		contracts: map[identity.UUID]schema.PropertyContract{
			contractID: contract,
		},
	}

	got, err := inference.TypeFromPropertyDefinition(
		definition,
		resolver,
	)
	if err != nil {
		t.Fatal(err)
	}

	requireTypeQualifiers(
		t,
		got,
		typing.TemporalSeries,
		true,
	)

	t.Run(
		"standalone definition is nonnullable",
		func(t *testing.T) {
			d := schema.NewPropertyDefinition(
				identity.New(),
				schema.EntityTypeOwner(ownerID),
				identity.UUID{},
				false,
				"Standalone",
				value.Decimal,
				unit.Unit{},
				false,
				schema.TemporalScalar,
				schema.InputComputation(),
			)

			got, err := inference.TypeFromPropertyDefinition(
				d,
				resolver,
			)
			if err != nil {
				t.Fatal(err)
			}

			requireTypeQualifiers(
				t,
				got,
				typing.TemporalScalar,
				false,
			)
		},
	)

	t.Run(
		"temporal contract mismatch rejected",
		func(t *testing.T) {
			d := schema.NewPropertyDefinition(
				identity.New(),
				schema.EntityTypeOwner(ownerID),
				contractID,
				true,
				"Bad",
				value.Decimal,
				unit.Unit{},
				false,
				schema.TemporalScalar,
				schema.InputComputation(),
			)

			_, err := inference.TypeFromPropertyDefinition(
				d,
				resolver,
			)

			var refErr inference.ReferenceError

			if !errors.As(err, &refErr) ||
				refErr.Kind() !=
					inference.ReferenceErrorContractMismatch {
				t.Fatalf(
					"got %v, want ContractMismatch",
					err,
				)
			}
		},
	)
}

func TestE2E_SelfRelativeTraversal(
	t *testing.T,
) {
	palletID := identity.New()
	boardID := identity.New()
	materialTraitID := identity.New()
	relationID := identity.New()

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
		"Contains",
		schema.EntityTypeConstraint(palletID),
		true,
		schema.EntityTypeConstraint(boardID),
		true,
		nil,
	)

	resolver := &inferenceResolver{
		entityTypes: map[identity.UUID]schema.EntityType{
			palletID: pallet,
			boardID:  board,
		},
		relationTypes: map[identity.UUID]schema.RelationType{
			relationID: relation,
		},
	}

	cases := []struct {
		name      string
		self      identity.UUID
		direction ast.TraversalDirection
		want      identity.UUID
		wantErr   bool
	}{
		{
			name:      "pallet outgoing board",
			self:      palletID,
			direction: ast.TraversalDirectionOutgoing,
			want:      boardID,
		},
		{
			name:      "pallet incoming rejected",
			self:      palletID,
			direction: ast.TraversalDirectionIncoming,
			wantErr:   true,
		},
		{
			name:      "pallet both board only",
			self:      palletID,
			direction: ast.TraversalDirectionBoth,
			want:      boardID,
		},
		{
			name:      "board incoming pallet",
			self:      boardID,
			direction: ast.TraversalDirectionIncoming,
			want:      palletID,
		},
		{
			name:      "board outgoing rejected",
			self:      boardID,
			direction: ast.TraversalDirectionOutgoing,
			wantErr:   true,
		},
		{
			name:      "board both pallet only",
			self:      boardID,
			direction: ast.TraversalDirectionBoth,
			want:      palletID,
		},
	}

	for _, tc := range cases {
		t.Run(
			tc.name,
			func(t *testing.T) {
				binding, err :=
					inference.BindingFromTraversal(
						ast.NewTraversalSource(
							tc.direction,
							relationID,
						),
						inference.NewContext(
							schema.EntityTypeOwner(
								tc.self,
							),
						),
						resolver,
					)

				if tc.wantErr {
					if err == nil {
						t.Fatal(
							"expected traversal error",
						)
					}

					return
				}

				if err != nil {
					t.Fatal(err)
				}

				requireSingleEntityConstraint(
					t,
					binding,
					schema.TypeConstraintEntityType,
					tc.want,
				)
			},
		)
	}

	t.Run(
		"trait endpoint accepts implementing entity",
		func(t *testing.T) {
			traitRelationID := identity.New()

			resolver.relationTypes[traitRelationID] =
				schema.NewRelationType(
					traitRelationID,
					"UsesMaterial",
					schema.EntityTypeConstraint(
						palletID,
					),
					true,
					schema.TraitConstraint(
						materialTraitID,
					),
					true,
					nil,
				)

			binding, err :=
				inference.BindingFromTraversal(
					ast.NewTraversalSource(
						ast.TraversalDirectionIncoming,
						traitRelationID,
					),
					inference.NewContext(
						schema.EntityTypeOwner(
							boardID,
						),
					),
					resolver,
				)

			if err != nil {
				t.Fatal(err)
			}

			requireSingleEntityConstraint(
				t,
				binding,
				schema.TypeConstraintEntityType,
				palletID,
			)
		},
	)

	t.Run(
		"trait endpoint rejects unrelated entity",
		func(t *testing.T) {
			unrelatedID := identity.New()

			resolver.entityTypes[unrelatedID] =
				schema.NewEntityType(
					unrelatedID,
					"Unrelated",
					nil,
					nil,
				)

			traitRelationID := identity.New()

			resolver.relationTypes[traitRelationID] =
				schema.NewRelationType(
					traitRelationID,
					"UsesMaterial",
					schema.EntityTypeConstraint(
						palletID,
					),
					true,
					schema.TraitConstraint(
						materialTraitID,
					),
					true,
					nil,
				)

			_, err :=
				inference.BindingFromTraversal(
					ast.NewTraversalSource(
						ast.TraversalDirectionIncoming,
						traitRelationID,
					),
					inference.NewContext(
						schema.EntityTypeOwner(
							unrelatedID,
						),
					),
					resolver,
				)

			var traversalErr inference.TraversalError

			if !errors.As(
				err,
				&traversalErr,
			) ||
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
		"both when self matches both exposes both opposite endpoints",
		func(t *testing.T) {
			otherID := identity.New()

			other := schema.NewEntityType(
				otherID,
				"Other",
				[]identity.UUID{
					materialTraitID,
				},
				nil,
			)

			resolver.entityTypes[otherID] = other

			bothID := identity.New()

			resolver.relationTypes[bothID] =
				schema.NewRelationType(
					bothID,
					"MaterialLink",
					schema.TraitConstraint(
						materialTraitID,
					),
					true,
					schema.AnyEntityConstraint(),
					true,
					nil,
				)

			binding, err :=
				inference.BindingFromTraversal(
					ast.NewTraversalSource(
						ast.TraversalDirectionBoth,
						bothID,
					),
					inference.NewContext(
						schema.EntityTypeOwner(
							boardID,
						),
					),
					resolver,
				)

			if err != nil {
				t.Fatal(err)
			}

			constraints :=
				binding.Entity().Constraints()

			if len(constraints) != 2 {
				t.Fatalf(
					"got %d constraints, want 2",
					len(constraints),
				)
			}

			if constraints[0].Kind() !=
				schema.TypeConstraintAnyEntity ||
				constraints[1].Kind() !=
					schema.TypeConstraintTrait {
				t.Fatalf(
					"unexpected constraints: %v, %v",
					constraints[0].Kind(),
					constraints[1].Kind(),
				)
			}
		},
	)
}

func requireTypeQualifiers(
	t *testing.T,
	got typing.Type,
	mode typing.TemporalMode,
	nullable bool,
) {
	t.Helper()

	if got.TemporalMode() != mode ||
		got.Nullable() != nullable {
		t.Fatalf(
			"got mode=%s nullable=%v, want mode=%s nullable=%v",
			got.TemporalMode(),
			got.Nullable(),
			mode,
			nullable,
		)
	}
}

func requireSingleEntityConstraint(
	t *testing.T,
	binding inference.Binding,
	kind schema.TypeConstraintKind,
	id identity.UUID,
) {
	t.Helper()

	constraints := binding.Entity().Constraints()

	if len(constraints) != 1 {
		t.Fatalf(
			"got %d constraints, want 1",
			len(constraints),
		)
	}

	if constraints[0].Kind() != kind {
		t.Fatalf(
			"got kind %s, want %s",
			constraints[0].Kind(),
			kind,
		)
	}

	gotID, ok := constraints[0].ID()

	if !ok || gotID != id {
		t.Fatalf(
			"got id=%v ok=%v, want %v",
			gotID,
			ok,
			id,
		)
	}
}
