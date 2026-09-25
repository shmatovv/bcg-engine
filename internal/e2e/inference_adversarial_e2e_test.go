package e2e_test

import (
	"errors"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

type adversarialResolver struct {
	definitions   map[identity.UUID]schema.PropertyDefinition
	contracts     map[identity.UUID]schema.PropertyContract
	entityTypes   map[identity.UUID]schema.EntityType
	relationTypes map[identity.UUID]schema.RelationType
	contractType  func(
		schema.TypeConstraint,
		identity.UUID,
	) (typing.Type, bool)
}

func (r *adversarialResolver) PropertyDefinition(
	id identity.UUID,
) (schema.PropertyDefinition, bool) {
	v, ok := r.definitions[id]
	return v, ok
}

func (r *adversarialResolver) PropertyContract(
	id identity.UUID,
) (schema.PropertyContract, bool) {
	v, ok := r.contracts[id]
	return v, ok
}

func (r *adversarialResolver) EntityType(
	id identity.UUID,
) (schema.EntityType, bool) {
	v, ok := r.entityTypes[id]
	return v, ok
}

func (r *adversarialResolver) RelationType(
	id identity.UUID,
) (schema.RelationType, bool) {
	v, ok := r.relationTypes[id]
	return v, ok
}

func (*adversarialResolver) ObjectType(
	address.ObjectRef,
) (schema.PropertyOwner, bool) {
	return schema.PropertyOwner{}, false
}

func (*adversarialResolver) ContractImplementation(
	schema.PropertyOwner,
	identity.UUID,
) (schema.PropertyDefinition, bool) {
	return schema.PropertyDefinition{}, false
}

func (r *adversarialResolver) ContractType(
	constraint schema.TypeConstraint,
	contractID identity.UUID,
) (typing.Type, bool) {
	if r.contractType == nil {
		return typing.Type{}, false
	}

	return r.contractType(
		constraint,
		contractID,
	)
}

func TestE2E_InferenceAdversarial_ReferencesAndContracts(
	t *testing.T,
) {
	ownerA := identity.New()
	ownerB := identity.New()
	definitionID := identity.New()

	definition := schema.NewPropertyDefinition(
		definitionID,
		schema.EntityTypeOwner(ownerA),
		identity.UUID{},
		false,
		"Amount",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	resolver := &adversarialResolver{
		definitions: map[identity.UUID]schema.PropertyDefinition{
			definitionID: definition,
		},
		contracts:     map[identity.UUID]schema.PropertyContract{},
		entityTypes:   map[identity.UUID]schema.EntityType{},
		relationTypes: map[identity.UUID]schema.RelationType{},
	}

	t.Run(
		"self property wrong owner rejected",
		func(t *testing.T) {
			_, err := inference.Infer(
				ast.NewSelfPropertyReference(
					definitionID,
				),
				inference.NewContext(
					schema.EntityTypeOwner(ownerB),
				),
				resolver,
			)

			requireReferenceErrorKind(
				t,
				err,
				inference.ReferenceErrorOwnerMismatch,
			)
		},
	)

	t.Run(
		"missing definition rejected",
		func(t *testing.T) {
			_, err := inference.Infer(
				ast.NewSelfPropertyReference(
					identity.New(),
				),
				inference.NewContext(
					schema.EntityTypeOwner(ownerA),
				),
				resolver,
			)

			requireReferenceErrorKind(
				t,
				err,
				inference.ReferenceErrorDefinitionNotFound,
			)
		},
	)

	t.Run(
		"missing contract rejected",
		func(t *testing.T) {
			contractID := identity.New()

			definition :=
				schema.NewPropertyDefinition(
					identity.New(),
					schema.EntityTypeOwner(ownerA),
					contractID,
					true,
					"Contracted",
					value.Decimal,
					unit.Unit{},
					false,
					schema.TemporalScalar,
					schema.InputComputation(),
				)

			_, err :=
				inference.TypeFromPropertyDefinition(
					definition,
					resolver,
				)

			requireReferenceErrorKind(
				t,
				err,
				inference.ReferenceErrorContractNotFound,
			)
		},
	)

	t.Run(
		"contract value type mismatch rejected",
		func(t *testing.T) {
			contractID := identity.New()

			contract :=
				schema.NewPropertyContract(
					contractID,
					"Metric",
					value.Integer,
					unit.Dimension{},
					false,
					schema.TemporalScalar,
					false,
				)

			resolver.contracts[contractID] =
				contract

			definition :=
				schema.NewPropertyDefinition(
					identity.New(),
					schema.EntityTypeOwner(ownerA),
					contractID,
					true,
					"Metric",
					value.Decimal,
					unit.Unit{},
					false,
					schema.TemporalScalar,
					schema.InputComputation(),
				)

			_, err :=
				inference.TypeFromPropertyDefinition(
					definition,
					resolver,
				)

			requireReferenceErrorKind(
				t,
				err,
				inference.ReferenceErrorContractMismatch,
			)
		},
	)
}

func TestE2E_InferenceAdversarial_TemporalRules(
	t *testing.T,
) {
	entityID := identity.New()
	relationID := identity.New()
	scalarID := identity.New()
	seriesID := identity.New()

	scalar := schema.NewPropertyDefinition(
		scalarID,
		schema.EntityTypeOwner(entityID),
		identity.UUID{},
		false,
		"Scalar",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	series := schema.NewPropertyDefinition(
		seriesID,
		schema.EntityTypeOwner(entityID),
		identity.UUID{},
		false,
		"Series",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalSeries,
		schema.InputComputation(),
	)

	entity := schema.NewEntityType(
		entityID,
		"Node",
		nil,
		[]identity.UUID{
			scalarID,
			seriesID,
		},
	)

	relation := schema.NewRelationType(
		relationID,
		"Link",
		schema.EntityTypeConstraint(entityID),
		true,
		schema.EntityTypeConstraint(entityID),
		true,
		nil,
	)

	resolver := &adversarialResolver{
		definitions: map[identity.UUID]schema.PropertyDefinition{
			scalarID: scalar,
			seriesID: series,
		},
		contracts: map[identity.UUID]schema.PropertyContract{},
		entityTypes: map[identity.UUID]schema.EntityType{
			entityID: entity,
		},
		relationTypes: map[identity.UUID]schema.RelationType{
			relationID: relation,
		},
	}

	context := inference.NewContext(
		schema.EntityTypeOwner(entityID),
	)

	t.Run(
		"recursive scalar plus series rejected",
		func(t *testing.T) {
			expression := ast.Add(
				ast.NewSelfPropertyReference(scalarID),
				ast.NewSelfPropertyReference(seriesID),
			)

			_, err := inference.Infer(
				expression,
				context,
				resolver,
			)

			if err == nil {
				t.Fatal(
					"expected scalar/series arithmetic rejection",
				)
			}

			var arithmeticErr typecheck.ArithmeticError

			if !errors.As(
				err,
				&arithmeticErr,
			) ||
				arithmeticErr.Kind() !=
					typecheck.ArithmeticErrorIncompatibleTemporalModes {
				t.Fatalf(
					"got %v, want arithmetic temporal mismatch",
					err,
				)
			}
		},
	)

	t.Run(
		"series aggregate filter rejected",
		func(t *testing.T) {
			filter := ast.Equal(
				ast.NewBoundPropertyDefinitionReference(
					ast.BoundTargetEntity,
					seriesID,
				),
				ast.NewBoundPropertyDefinitionReference(
					ast.BoundTargetEntity,
					seriesID,
				),
			)

			expression :=
				ast.NewFilteredCountAggregate(
					ast.NewTraversalSource(
						ast.TraversalDirectionOutgoing,
						relationID,
					),
					filter,
				)

			_, err := inference.Infer(
				expression,
				context,
				resolver,
			)

			var aggregateErr typecheck.AggregateError

			if !errors.As(
				err,
				&aggregateErr,
			) ||
				aggregateErr.Kind() !=
					typecheck.AggregateErrorNonScalarFilter {
				t.Fatalf(
					"got %v, want NonScalarFilter",
					err,
				)
			}
		},
	)

	t.Run(
		"series if condition with scalar branches rejected",
		func(t *testing.T) {
			condition := ast.Equal(
				ast.NewSelfPropertyReference(seriesID),
				ast.NewSelfPropertyReference(seriesID),
			)

			expression := ast.NewIf(
				condition,
				ast.NewSelfPropertyReference(scalarID),
				ast.NewSelfPropertyReference(scalarID),
			)

			_, err := inference.Infer(
				expression,
				context,
				resolver,
			)

			var conditionalErr typecheck.ConditionalError

			if !errors.As(
				err,
				&conditionalErr,
			) ||
				conditionalErr.Kind() !=
					typecheck.ConditionalErrorIncompatibleTemporalModes {
				t.Fatalf(
					"got %v, want IncompatibleTemporalModes",
					err,
				)
			}
		},
	)
}

func TestE2E_InferenceAdversarial_HeterogeneousBoth(
	t *testing.T,
) {
	selfID := identity.New()
	traitID := identity.New()
	relationID := identity.New()
	concreteDefinitionID := identity.New()
	contractID := identity.New()

	self := schema.NewEntityType(
		selfID,
		"Self",
		[]identity.UUID{
			traitID,
		},
		nil,
	)

	relation := schema.NewRelationType(
		relationID,
		"Mixed",
		schema.TraitConstraint(traitID),
		true,
		schema.AnyEntityConstraint(),
		true,
		nil,
	)

	concrete := schema.NewPropertyDefinition(
		concreteDefinitionID,
		schema.EntityTypeOwner(selfID),
		identity.UUID{},
		false,
		"Concrete",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	contract := schema.NewPropertyContract(
		contractID,
		"Common",
		value.Decimal,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		false,
	)

	resolver := &adversarialResolver{
		definitions: map[identity.UUID]schema.PropertyDefinition{
			concreteDefinitionID: concrete,
		},
		contracts: map[identity.UUID]schema.PropertyContract{
			contractID: contract,
		},
		entityTypes: map[identity.UUID]schema.EntityType{
			selfID: self,
		},
		relationTypes: map[identity.UUID]schema.RelationType{
			relationID: relation,
		},
	}

	context := inference.NewContext(
		schema.EntityTypeOwner(selfID),
	)

	source := ast.NewTraversalSource(
		ast.TraversalDirectionBoth,
		relationID,
	)

	t.Run(
		"heterogeneous both rejects concrete bound definition",
		func(t *testing.T) {
			expression := ast.NewMappedAggregate(
				source,
				ast.NewBoundPropertyDefinitionReference(
					ast.BoundTargetEntity,
					concreteDefinitionID,
				),
				ast.AggregateOperationSum,
			)

			_, err := inference.Infer(
				expression,
				context,
				resolver,
			)

			requireReferenceErrorKind(
				t,
				err,
				inference.ReferenceErrorTargetNotConcrete,
			)
		},
	)

	t.Run(
		"heterogeneous both accepts common compatible contract",
		func(t *testing.T) {
			resolver.contractType = func(
				_ schema.TypeConstraint,
				id identity.UUID,
			) (typing.Type, bool) {
				if id != contractID {
					return typing.Type{}, false
				}

				return typing.New(
					value.Decimal,
				), true
			}

			expression := ast.NewMappedAggregate(
				source,
				ast.NewBoundContractPropertyReference(
					ast.BoundTargetEntity,
					contractID,
				),
				ast.AggregateOperationSum,
			)

			got, err := inference.Infer(
				expression,
				context,
				resolver,
			)
			if err != nil {
				t.Fatal(err)
			}

			if got.ValueType() != value.Decimal ||
				got.TemporalMode() !=
					typing.TemporalScalar ||
				got.Nullable() {
				t.Fatalf(
					"unexpected result: value=%s temporal=%s nullable=%v",
					got.ValueType(),
					got.TemporalMode(),
					got.Nullable(),
				)
			}
		},
	)

	t.Run(
		"heterogeneous both rejects incompatible contract types",
		func(t *testing.T) {
			resolver.contractType = func(
				constraint schema.TypeConstraint,
				id identity.UUID,
			) (typing.Type, bool) {
				if id != contractID {
					return typing.Type{}, false
				}

				switch constraint.Kind() {
				case schema.TypeConstraintAnyEntity:
					return typing.New(
						value.Decimal,
					), true

				case schema.TypeConstraintTrait:
					return typing.New(
						value.String,
					), true

				default:
					return typing.Type{}, false
				}
			}

			expression := ast.NewMappedAggregate(
				source,
				ast.NewBoundContractPropertyReference(
					ast.BoundTargetEntity,
					contractID,
				),
				ast.AggregateOperationSum,
			)

			_, err := inference.Infer(
				expression,
				context,
				resolver,
			)

			requireReferenceErrorKind(
				t,
				err,
				inference.ReferenceErrorIncompatibleContractTypes,
			)
		},
	)

	t.Run(
		"heterogeneous both rejects contract not guaranteed on one side",
		func(t *testing.T) {
			resolver.contractType = func(
				constraint schema.TypeConstraint,
				id identity.UUID,
			) (typing.Type, bool) {
				if id != contractID ||
					constraint.Kind() ==
						schema.TypeConstraintAnyEntity {
					return typing.Type{}, false
				}

				return typing.New(
					value.Decimal,
				), true
			}

			expression := ast.NewMappedAggregate(
				source,
				ast.NewBoundContractPropertyReference(
					ast.BoundTargetEntity,
					contractID,
				),
				ast.AggregateOperationSum,
			)

			_, err := inference.Infer(
				expression,
				context,
				resolver,
			)

			requireReferenceErrorKind(
				t,
				err,
				inference.ReferenceErrorContractTypeNotGuaranteed,
			)
		},
	)
}

func TestE2E_InferenceAdversarial_NestedAggregateBindingShadowing(
	t *testing.T,
) {
	rootID := identity.New()
	outerID := identity.New()
	innerID := identity.New()

	outerRelationID := identity.New()
	innerRelationID := identity.New()

	outerValueID := identity.New()
	innerValueID := identity.New()

	root := schema.NewEntityType(
		rootID,
		"Root",
		nil,
		nil,
	)

	outer := schema.NewEntityType(
		outerID,
		"Outer",
		nil,
		[]identity.UUID{
			outerValueID,
		},
	)

	inner := schema.NewEntityType(
		innerID,
		"Inner",
		nil,
		[]identity.UUID{
			innerValueID,
		},
	)

	outerValue := schema.NewPropertyDefinition(
		outerValueID,
		schema.EntityTypeOwner(outerID),
		identity.UUID{},
		false,
		"OuterValue",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	innerValue := schema.NewPropertyDefinition(
		innerValueID,
		schema.EntityTypeOwner(innerID),
		identity.UUID{},
		false,
		"InnerValue",
		value.Decimal,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	outerRelation := schema.NewRelationType(
		outerRelationID,
		"OuterLink",
		schema.EntityTypeConstraint(rootID),
		true,
		schema.EntityTypeConstraint(outerID),
		true,
		nil,
	)

	innerRelation := schema.NewRelationType(
		innerRelationID,
		"InnerLink",
		schema.EntityTypeConstraint(rootID),
		true,
		schema.EntityTypeConstraint(innerID),
		true,
		nil,
	)

	resolver := &adversarialResolver{
		definitions: map[identity.UUID]schema.PropertyDefinition{
			outerValueID: outerValue,
			innerValueID: innerValue,
		},
		contracts: map[identity.UUID]schema.PropertyContract{},
		entityTypes: map[identity.UUID]schema.EntityType{
			rootID:  root,
			outerID: outer,
			innerID: inner,
		},
		relationTypes: map[identity.UUID]schema.RelationType{
			outerRelationID: outerRelation,
			innerRelationID: innerRelation,
		},
	}

	context := inference.NewContext(
		schema.EntityTypeOwner(rootID),
	)

	innerAggregate := ast.NewMappedAggregate(
		ast.NewTraversalSource(
			ast.TraversalDirectionOutgoing,
			innerRelationID,
		),
		ast.NewBoundPropertyDefinitionReference(
			ast.BoundTargetEntity,
			innerValueID,
		),
		ast.AggregateOperationSum,
	)

	outerAggregate := ast.NewMappedAggregate(
		ast.NewTraversalSource(
			ast.TraversalDirectionOutgoing,
			outerRelationID,
		),
		innerAggregate,
		ast.AggregateOperationSum,
	)

	got, err := inference.Infer(
		outerAggregate,
		context,
		resolver,
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.ValueType() != value.Decimal ||
		got.TemporalMode() != typing.TemporalScalar ||
		got.Nullable() {
		t.Fatalf(
			"unexpected result: value=%s temporal=%s nullable=%v",
			got.ValueType(),
			got.TemporalMode(),
			got.Nullable(),
		)
	}
}

func requireReferenceErrorKind(
	t *testing.T,
	err error,
	want inference.ReferenceErrorKind,
) {
	t.Helper()

	var referenceErr inference.ReferenceError

	if !errors.As(
		err,
		&referenceErr,
	) ||
		referenceErr.Kind() != want {
		t.Fatalf(
			"got %v, want reference error %s",
			err,
			want,
		)
	}
}
