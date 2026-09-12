package evaluator_test

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestAggregateResolvesPolymorphicBoundContractProperties(t *testing.T) {
	draft := model.NewDraft()
	priceContractID, pricedTraitID := identity.New(), identity.New()
	rootTypeID, firstTypeID, secondTypeID := identity.New(), identity.New(), identity.New()
	relationTypeID := identity.New()
	firstPriceID, secondPriceID, totalID := identity.New(), identity.New(), identity.New()
	rootID, firstID, secondID := identity.New(), identity.New(), identity.New()

	draft.AddPropertyContract(schema.NewPropertyContract(
		priceContractID,
		"price",
		value.Integer,
		unit.Dimension{},
		false,
		schema.TemporalScalar,
		false,
	))
	draft.AddTrait(schema.NewTrait(pricedTraitID, "priced", []identity.UUID{priceContractID}))
	firstPrice := schema.NewPropertyDefinition(firstPriceID, schema.EntityTypeOwner(firstTypeID), priceContractID, true, "first price", value.Integer, unit.Unit{}, false, schema.TemporalScalar, schema.InputComputation())
	secondPrice := schema.NewPropertyDefinition(secondPriceID, schema.EntityTypeOwner(secondTypeID), priceContractID, true, "second price", value.Integer, unit.Unit{}, false, schema.TemporalScalar, schema.InputComputation())
	total := schema.NewPropertyDefinition(
		totalID,
		schema.EntityTypeOwner(rootTypeID),
		identity.UUID{},
		false,
		"total",
		value.Integer,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.DerivedComputation(ast.NewMappedAggregate(
			ast.NewTraversalSource(ast.TraversalDirectionOutgoing, relationTypeID),
			ast.NewBoundContractPropertyReference(ast.BoundTargetEntity, priceContractID),
			ast.AggregateOperationSum,
		)),
	)
	draft.AddPropertyDefinition(firstPrice)
	draft.AddPropertyDefinition(secondPrice)
	draft.AddPropertyDefinition(total)
	draft.AddEntityType(schema.NewEntityType(rootTypeID, "root", nil, []identity.UUID{totalID}))
	draft.AddEntityType(schema.NewEntityType(firstTypeID, "first", []identity.UUID{pricedTraitID}, []identity.UUID{firstPriceID}))
	draft.AddEntityType(schema.NewEntityType(secondTypeID, "second", []identity.UUID{pricedTraitID}, []identity.UUID{secondPriceID}))
	draft.AddRelationType(schema.NewRelationType(
		relationTypeID,
		"priced child",
		schema.EntityTypeConstraint(rootTypeID),
		true,
		schema.TraitConstraint(pricedTraitID),
		true,
		nil,
	))
	draft.AddEntity(semantic.NewEntity(rootID, rootTypeID, "", false))
	draft.AddEntity(semantic.NewEntity(firstID, firstTypeID, "", false))
	draft.AddEntity(semantic.NewEntity(secondID, secondTypeID, "", false))
	draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, rootID, firstID, nil, false))
	draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, rootID, secondID, nil, false))
	draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(address.NewPropertyAddress(address.EntityRef(firstID), firstPriceID)),
		bindingInteger(2),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))
	draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(address.NewPropertyAddress(address.EntityRef(secondID), secondPriceID)),
		bindingInteger(3),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))

	result, err := evaluator.Evaluate(
		reviewCommit(t, draft),
		address.NewPropertyAddress(address.EntityRef(rootID), totalID),
		reviewContext(),
	)
	if err != nil || !result.Value().IsDefined() || result.Value().Value().Raw().Integer().Int64() != 5 {
		t.Fatalf("polymorphic contract aggregate = %#v, %v", result.Value(), err)
	}
}

func TestAggregateUsesNormalRelationPropertyCalculationOverrides(t *testing.T) {
	f := newPipelineFixture()
	relationTypeID, quantityID, relationID := identity.New(), identity.New(), identity.New()
	quantity := schema.NewPropertyDefinition(
		quantityID,
		schema.RelationTypeOwner(relationTypeID),
		identity.UUID{},
		false,
		"quantity",
		value.Integer,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)
	f.draft.AddPropertyDefinition(quantity)
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(relationTypeID, "links", endpoint, true, endpoint, true, []identity.UUID{quantityID}))
	f.draft.AddRelation(semantic.NewRelation(relationID, relationTypeID, f.entityID, f.entityID, nil, false))
	total := f.property(value.Integer, schema.DerivedComputation(ast.NewMappedAggregate(
		ast.NewTraversalSource(ast.TraversalDirectionBoth, relationTypeID),
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetRelation, quantityID),
		ast.AggregateOperationSum,
	)), false)
	quantityAddress := address.NewPropertyAddress(address.RelationRef(relationID), quantityID)
	f.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(quantityAddress),
		bindingInteger(2),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))
	version := f.commit(t)
	base, err := evaluator.Evaluate(version, total, reviewContext())
	if err != nil || !base.Value().IsDefined() || base.Value().Value().Raw().Integer().Int64() != 2 {
		t.Fatalf("base relation aggregate = %#v, %v", base.Value(), err)
	}
	overridden := runtime.NewCalculationContext(
		reviewContext().TimeContext(),
		scope.Values{},
		[]runtime.Override{runtime.NewOverride(quantityAddress, bindingInteger(4))},
	)
	result, err := evaluator.Evaluate(version, total, overridden)
	if err != nil || !result.Value().IsDefined() || result.Value().Value().Raw().Integer().Int64() != 4 {
		t.Fatalf("relation override aggregate = %#v, %v", result.Value(), err)
	}
}

func TestAggregateLargeSemanticGraphRemainsDeterministic(t *testing.T) {
	const itemCount = 128

	draft := model.NewDraft()
	typeID, relationTypeID := identity.New(), identity.New()
	inputID, totalID := identity.New(), identity.New()
	rootID := identity.New()
	input := schema.NewPropertyDefinition(inputID, schema.EntityTypeOwner(typeID), identity.UUID{}, false, "input", value.Integer, unit.Unit{}, false, schema.TemporalScalar, schema.InputComputation())
	total := schema.NewPropertyDefinition(
		totalID,
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
			ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, inputID),
			ast.AggregateOperationSum,
		)),
	)
	draft.AddPropertyDefinition(input)
	draft.AddPropertyDefinition(total)
	draft.AddEntityType(schema.NewEntityType(typeID, "node", nil, []identity.UUID{inputID, totalID}))
	endpoint := schema.EntityTypeConstraint(typeID)
	draft.AddRelationType(schema.NewRelationType(relationTypeID, "edge", endpoint, true, endpoint, true, nil))
	draft.AddEntity(semantic.NewEntity(rootID, typeID, "", false))
	leafIDs := make([]identity.UUID, itemCount)
	for index := range leafIDs {
		leafIDs[index] = identity.New()
		draft.AddEntity(semantic.NewEntity(leafIDs[index], typeID, "", false))
		draft.AddValueBinding(semantic.NewValueBinding(
			semantic.ObjectPropertyTarget(address.NewPropertyAddress(address.EntityRef(leafIDs[index]), inputID)),
			bindingInteger(1),
			nil,
			domaintime.EffectivePeriod{},
			false,
		))
	}
	for index := len(leafIDs) - 1; index >= 0; index-- {
		draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, rootID, leafIDs[index], nil, false))
	}

	version := reviewCommit(t, draft)
	totalAddress := address.NewPropertyAddress(address.EntityRef(rootID), totalID)
	for run := 0; run < 8; run++ {
		result, err := evaluator.Evaluate(version, totalAddress, reviewContext())
		if err != nil || !result.Value().IsDefined() || result.Value().Value().Raw().Integer().Int64() != itemCount {
			t.Fatalf("large aggregate run %d = %#v, %v", run, result.Value(), err)
		}
	}
}
