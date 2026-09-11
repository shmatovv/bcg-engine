package e2e_test

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/date"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestE2E_SchemaCollectionsRejectZeroAndDuplicateIDs(t *testing.T) {
	id := identity.New()
	contractID := identity.New()

	cases := []struct {
		name string
		fn   func()
	}{
		{
			"entity duplicate trait",
			func() {
				schema.NewEntityType(
					identity.New(),
					"X",
					[]identity.UUID{id, id},
					nil,
				)
			},
		},
		{
			"entity zero trait",
			func() {
				schema.NewEntityType(
					identity.New(),
					"X",
					[]identity.UUID{{}},
					nil,
				)
			},
		},
		{
			"entity duplicate property",
			func() {
				schema.NewEntityType(
					identity.New(),
					"X",
					nil,
					[]identity.UUID{id, id},
				)
			},
		},
		{
			"entity zero property",
			func() {
				schema.NewEntityType(
					identity.New(),
					"X",
					nil,
					[]identity.UUID{{}},
				)
			},
		},
		{
			"trait duplicate contract",
			func() {
				schema.NewTrait(
					identity.New(),
					"T",
					[]identity.UUID{
						contractID,
						contractID,
					},
				)
			},
		},
		{
			"trait zero contract",
			func() {
				schema.NewTrait(
					identity.New(),
					"T",
					[]identity.UUID{{}},
				)
			},
		},
		{
			"relation duplicate property",
			func() {
				schema.NewRelationType(
					identity.New(),
					"R",
					schema.TypeConstraint{},
					false,
					schema.TypeConstraint{},
					false,
					[]identity.UUID{id, id},
				)
			},
		},
		{
			"relation zero property",
			func() {
				schema.NewRelationType(
					identity.New(),
					"R",
					schema.TypeConstraint{},
					false,
					schema.TypeConstraint{},
					false,
					[]identity.UUID{{}},
				)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustPanic(t, tc.fn)
		})
	}
}

func TestE2E_SchemaConstructorsRejectBrokenIdentity(t *testing.T) {
	u := testUnits()

	cases := []struct {
		name string
		fn   func()
	}{
		{
			"entity type zero id",
			func() {
				schema.NewEntityType(
					identity.UUID{},
					"X",
					nil,
					nil,
				)
			},
		},
		{
			"entity type empty name",
			func() {
				schema.NewEntityType(
					identity.New(),
					"",
					nil,
					nil,
				)
			},
		},
		{
			"trait zero id",
			func() {
				schema.NewTrait(
					identity.UUID{},
					"T",
					nil,
				)
			},
		},
		{
			"trait empty name",
			func() {
				schema.NewTrait(
					identity.New(),
					"",
					nil,
				)
			},
		},
		{
			"contract zero id",
			func() {
				schema.NewPropertyContract(
					identity.UUID{},
					"P",
					value.Decimal,
					unit.Dimension{},
					false,
					schema.TemporalScalar,
					false,
				)
			},
		},
		{
			"contract empty name",
			func() {
				schema.NewPropertyContract(
					identity.New(),
					"",
					value.Decimal,
					unit.Dimension{},
					false,
					schema.TemporalScalar,
					false,
				)
			},
		},
		{
			"entity owner zero",
			func() {
				schema.EntityTypeOwner(
					identity.UUID{},
				)
			},
		},
		{
			"relation owner zero",
			func() {
				schema.RelationTypeOwner(
					identity.UUID{},
				)
			},
		},
		{
			"entity constraint zero",
			func() {
				schema.EntityTypeConstraint(
					identity.UUID{},
				)
			},
		},
		{
			"trait constraint zero",
			func() {
				schema.TraitConstraint(
					identity.UUID{},
				)
			},
		},
		{
			"property zero id",
			func() {
				schema.NewPropertyDefinition(
					identity.UUID{},
					schema.EntityTypeOwner(
						identity.New(),
					),
					identity.UUID{},
					false,
					"P",
					value.Decimal,
					u.rub,
					true,
					schema.TemporalScalar,
					schema.InputComputation(),
				)
			},
		},
		{
			"derived nil expression",
			func() {
				schema.DerivedComputation(nil)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustPanic(t, tc.fn)
		})
	}
}

func TestE2E_TypeConstraintsRepresentAllEndpointPolicies(
	t *testing.T,
) {
	entityTypeID := identity.New()
	traitID := identity.New()

	concrete :=
		schema.EntityTypeConstraint(entityTypeID)

	trait :=
		schema.TraitConstraint(traitID)

	any :=
		schema.AnyEntityConstraint()

	if !concrete.IsValid() ||
		concrete.Kind() != schema.TypeConstraintEntityType {
		t.Fatal("concrete constraint invalid")
	}

	if got, ok := concrete.ID(); !ok ||
		got != entityTypeID {
		t.Fatal("concrete id mismatch")
	}

	if !trait.IsValid() ||
		trait.Kind() != schema.TypeConstraintTrait {
		t.Fatal("trait constraint invalid")
	}

	if got, ok := trait.ID(); !ok ||
		got != traitID {
		t.Fatal("trait id mismatch")
	}

	if !any.IsValid() ||
		any.Kind() != schema.TypeConstraintAnyEntity {
		t.Fatal("any constraint invalid")
	}

	if _, ok := any.ID(); ok {
		t.Fatal("AnyEntity must not expose id")
	}
}

func TestE2E_SchemaSlicesAreImmutableCopies(t *testing.T) {
	traitID := identity.New()
	propID := identity.New()
	contractID := identity.New()

	entityType := schema.NewEntityType(
		identity.New(),
		"E",
		[]identity.UUID{traitID},
		[]identity.UUID{propID},
	)

	traits := entityType.Traits()
	properties := entityType.PropertyDefinitions()

	traits[0] = identity.New()
	properties[0] = identity.New()

	if entityType.Traits()[0] != traitID ||
		entityType.PropertyDefinitions()[0] != propID {
		t.Fatal("entity type leaked internal slice")
	}

	trait := schema.NewTrait(
		identity.New(),
		"T",
		[]identity.UUID{contractID},
	)

	contracts := trait.RequiredPropertyContracts()
	contracts[0] = identity.New()

	if trait.RequiredPropertyContracts()[0] != contractID {
		t.Fatal("trait leaked internal slice")
	}

	relationType := schema.NewRelationType(
		identity.New(),
		"R",
		schema.TypeConstraint{},
		false,
		schema.TypeConstraint{},
		false,
		[]identity.UUID{propID},
	)

	definitions := relationType.PropertyDefinitions()
	definitions[0] = identity.New()

	if relationType.PropertyDefinitions()[0] != propID {
		t.Fatal("relation type leaked internal slice")
	}
}

func TestE2E_PropertyDefinitionPreservesOwnerContractUnitAndComputation(
	t *testing.T,
) {
	u := testUnits()

	entityTypeID := identity.New()
	contractID := identity.New()

	inputID := identity.New()
	derivedID := identity.New()

	input := schema.NewPropertyDefinition(
		inputID,
		schema.EntityTypeOwner(entityTypeID),
		contractID,
		true,
		"Price",
		value.Decimal,
		u.rubPerItem,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	expr :=
		ast.NewSelfPropertyReference(inputID)

	derived := schema.NewPropertyDefinition(
		derivedID,
		schema.EntityTypeOwner(entityTypeID),
		identity.UUID{},
		false,
		"Copy",
		value.Decimal,
		u.rubPerItem,
		true,
		schema.TemporalScalar,
		schema.DerivedComputation(expr),
	)

	if !input.IsValid() || !derived.IsValid() {
		t.Fatal("definitions invalid")
	}

	if input.Owner().Kind() !=
		schema.PropertyOwnerEntityType ||
		input.Owner().ID() != entityTypeID {
		t.Fatal("owner mismatch")
	}

	if got, ok := input.ContractID(); !ok ||
		got != contractID {
		t.Fatal("contract mismatch")
	}

	if got, ok := input.Unit(); !ok ||
		!got.Equal(u.rubPerItem) {
		t.Fatal("unit mismatch")
	}

	if _, ok := input.Computation().Expression(); ok {
		t.Fatal("input must not have expression")
	}

	if got, ok :=
		derived.Computation().Expression(); !ok ||
		got == nil {
		t.Fatal("derived expression missing")
	}
}

func TestE2E_SemanticObjectsRejectBrokenIdentity(
	t *testing.T,
) {
	typeID := identity.New()
	entityID := identity.New()
	relationTypeID := identity.New()

	cases := []struct {
		name string
		fn   func()
	}{
		{
			"entity zero id",
			func() {
				semantic.NewEntity(
					identity.UUID{},
					typeID,
					"",
					false,
				)
			},
		},
		{
			"entity zero type",
			func() {
				semantic.NewEntity(
					identity.New(),
					identity.UUID{},
					"",
					false,
				)
			},
		},
		{
			"relation zero id",
			func() {
				semantic.NewRelation(
					identity.UUID{},
					relationTypeID,
					entityID,
					identity.New(),
					nil,
					false,
				)
			},
		},
		{
			"relation zero type",
			func() {
				semantic.NewRelation(
					identity.New(),
					identity.UUID{},
					entityID,
					identity.New(),
					nil,
					false,
				)
			},
		},
		{
			"relation zero from",
			func() {
				semantic.NewRelation(
					identity.New(),
					relationTypeID,
					identity.UUID{},
					identity.New(),
					nil,
					false,
				)
			},
		},
		{
			"relation zero to",
			func() {
				semantic.NewRelation(
					identity.New(),
					relationTypeID,
					entityID,
					identity.UUID{},
					nil,
					false,
				)
			},
		},
		{
			"relation missing applicability",
			func() {
				semantic.NewRelation(
					identity.New(),
					relationTypeID,
					entityID,
					identity.New(),
					nil,
					true,
				)
			},
		},
		{
			"entity ref zero",
			func() {
				semantic.EntityRef(
					identity.UUID{},
				)
			},
		},
		{
			"relation ref zero",
			func() {
				semantic.RelationRef(
					identity.UUID{},
				)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustPanic(t, tc.fn)
		})
	}
}

func TestE2E_MultigraphPreservesParallelRelations(
	t *testing.T,
) {
	pallet := semantic.NewEntity(
		identity.New(),
		identity.New(),
		"Pallet",
		true,
	)

	board := semantic.NewEntity(
		identity.New(),
		identity.New(),
		"Board",
		true,
	)

	containsTypeID := identity.New()

	r1 := semantic.NewRelation(
		identity.New(),
		containsTypeID,
		pallet.ID(),
		board.ID(),
		nil,
		false,
	)

	r2 := semantic.NewRelation(
		identity.New(),
		containsTypeID,
		pallet.ID(),
		board.ID(),
		nil,
		false,
	)

	if !r1.IsValid() || !r2.IsValid() {
		t.Fatal("relations invalid")
	}

	if r1.ID() == r2.ID() {
		t.Fatal(
			"parallel relations must keep independent identity",
		)
	}

	if r1.From() != r2.From() ||
		r1.To() != r2.To() ||
		r1.TypeID() != r2.TypeID() {
		t.Fatal(
			"parallel edge endpoints/type changed",
		)
	}
}

func TestE2E_RelationApplicabilityIsOptionalAndPreserved(
	t *testing.T,
) {
	expr := ast.NewConstant(
		value.NewTyped(
			value.NewBool(true),
		),
	)

	relation := semantic.NewRelation(
		identity.New(),
		identity.New(),
		identity.New(),
		identity.New(),
		expr,
		true,
	)

	got, ok := relation.Applicability()

	if !ok || got == nil {
		t.Fatal("applicability missing")
	}

	// If hasApplicability=false, supplied expression is deliberately
	// normalized away.
	plain := semantic.NewRelation(
		identity.New(),
		identity.New(),
		identity.New(),
		identity.New(),
		expr,
		false,
	)

	if got, ok := plain.Applicability(); ok ||
		got != nil {
		t.Fatal(
			"absent applicability must be normalized away",
		)
	}
}

func TestE2E_PropertyAddressesDistinguishEntityAndRelation(
	t *testing.T,
) {
	definitionID := identity.New()
	entityID := identity.New()
	relationID := identity.New()

	entityAddress :=
		semantic.NewPropertyAddress(
			semantic.EntityRef(entityID),
			definitionID,
		)

	relationAddress :=
		semantic.NewPropertyAddress(
			semantic.RelationRef(relationID),
			definitionID,
		)

	if !entityAddress.IsValid() ||
		!relationAddress.IsValid() {
		t.Fatal("addresses invalid")
	}

	if entityAddress.Object().Kind() !=
		semantic.ObjectRefEntity {
		t.Fatal("entity object kind lost")
	}

	if relationAddress.Object().Kind() !=
		semantic.ObjectRefRelation {
		t.Fatal("relation object kind lost")
	}

	if entityAddress.Object().ID() ==
		relationAddress.Object().ID() {
		t.Fatal("objects unexpectedly identical")
	}
}

func TestE2E_ValueTargetsSupportObjectAndTypeDefaults(
	t *testing.T,
) {
	definitionID := identity.New()

	address :=
		semantic.NewPropertyAddress(
			semantic.EntityRef(
				identity.New(),
			),
			definitionID,
		)

	objectTarget :=
		semantic.ObjectPropertyTarget(address)

	if !objectTarget.IsValid() ||
		objectTarget.Kind() !=
			semantic.ValueTargetObjectProperty {
		t.Fatal("object target invalid")
	}

	if got, ok :=
		objectTarget.ObjectProperty(); !ok ||
		!got.IsValid() {
		t.Fatal("object target payload missing")
	}

	owner :=
		schema.EntityTypeOwner(identity.New())

	typeTarget :=
		semantic.TypePropertyTarget(
			owner,
			definitionID,
		)

	if !typeTarget.IsValid() ||
		typeTarget.Kind() !=
			semantic.ValueTargetTypeProperty {
		t.Fatal("type target invalid")
	}

	gotOwner, gotDefinition, ok :=
		typeTarget.TypeProperty()

	if !ok ||
		gotOwner.ID() != owner.ID() ||
		gotDefinition != definitionID {
		t.Fatal("type target payload mismatch")
	}
}

func TestE2E_ValueBindingPreservesDefinedNullScopeAndTime(
	t *testing.T,
) {
	u := testUnits()

	definitionID := identity.New()
	scopeDefinitionID := identity.New()

	target :=
		semantic.ObjectPropertyTarget(
			semantic.NewPropertyAddress(
				semantic.EntityRef(
					identity.New(),
				),
				definitionID,
			),
		)

	selector := scope.NewSelector(
		scopeDefinitionID,
		value.NewTyped(
			value.NewString("DE"),
		),
	)

	from := datetime.New(
		date.New(2026, 1, 1),
		0,
		0,
		0,
		0,
	)

	to := datetime.New(
		date.New(2027, 1, 1),
		0,
		0,
		0,
		0,
	)

	period :=
		domaintime.NewEffectivePeriod(
			from,
			true,
			to,
			true,
		)

	defined := semantic.NewValueBinding(
		target,
		semantic.DefinedBindingValue(
			value.NewTypedWithUnit(
				value.NewDecimal(
					dec("12.50"),
				),
				u.rub,
			),
		),
		[]scope.Selector{
			selector,
		},
		period,
		true,
	)

	if !defined.IsValid() {
		t.Fatal("defined binding invalid")
	}

	selectors := defined.ScopeSelectors()

	if len(selectors) != 1 ||
		selectors[0].DefinitionID() !=
			scopeDefinitionID {
		t.Fatal("selector missing")
	}

	// Getter must return a copy.
	selectors[0] = scope.NewSelector(
		identity.New(),
		value.NewTyped(
			value.NewString("US"),
		),
	)

	if defined.ScopeSelectors()[0].DefinitionID() !=
		scopeDefinitionID {
		t.Fatal("binding leaked selector slice")
	}

	gotPeriod, ok := defined.EffectivePeriod()

	if !ok || !gotPeriod.IsValid() {
		t.Fatal("effective period missing")
	}

	nullBinding := semantic.NewValueBinding(
		target,
		semantic.NullBindingValue(),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	if !nullBinding.IsValid() ||
		nullBinding.Value().Kind() !=
			semantic.BindingValueNull {
		t.Fatal("null binding invalid")
	}

	noPeriod, ok :=
		nullBinding.EffectivePeriod()

	if ok {
		t.Fatal(
			"absent period must report hasEffectivePeriod=false",
		)
	}

	if noPeriod != (domaintime.EffectivePeriod{}) {
		t.Fatal(
			"absent period must be normalized to zero value",
		)
	}
}

func TestE2E_EffectivePeriodSupportsOpenAndClosedBounds(
	t *testing.T,
) {
	from := datetime.New(
		date.New(2026, 1, 1),
		0,
		0,
		0,
		0,
	)

	to := datetime.New(
		date.New(2027, 1, 1),
		0,
		0,
		0,
		0,
	)

	periods := []domaintime.EffectivePeriod{
		// (-∞, +∞)
		domaintime.NewEffectivePeriod(
			datetime.DateTime{},
			false,
			datetime.DateTime{},
			false,
		),

		// [from, +∞)
		domaintime.NewEffectivePeriod(
			from,
			true,
			datetime.DateTime{},
			false,
		),

		// (-∞, to)
		domaintime.NewEffectivePeriod(
			datetime.DateTime{},
			false,
			to,
			true,
		),

		// [from, to)
		domaintime.NewEffectivePeriod(
			from,
			true,
			to,
			true,
		),
	}

	for i, period := range periods {
		if !period.IsValid() {
			t.Fatalf(
				"period %d invalid",
				i,
			)
		}
	}
}

func TestE2E_CurrentLayerIntentionallyDoesNotPerformReferentialValidation(
	t *testing.T,
) {
	// These IDs intentionally do not prove that referenced schema
	// objects actually exist.
	//
	// Constructors only protect structural invariants.
	// Referential validity belongs to the future schema/commit
	// validation layer.

	ghostTypeID := identity.New()
	ghostTraitID := identity.New()
	ghostPropertyID := identity.New()

	entityType := schema.NewEntityType(
		identity.New(),
		"StructurallyValid",
		[]identity.UUID{
			ghostTraitID,
		},
		[]identity.UUID{
			ghostPropertyID,
		},
	)

	entity := semantic.NewEntity(
		identity.New(),
		ghostTypeID,
		"Ghost",
		true,
	)

	relationType := schema.NewRelationType(
		identity.New(),
		"R",
		schema.EntityTypeConstraint(
			ghostTypeID,
		),
		true,
		schema.TraitConstraint(
			ghostTraitID,
		),
		true,
		nil,
	)

	relation := semantic.NewRelation(
		identity.New(),
		relationType.ID(),
		entity.ID(),
		identity.New(),
		nil,
		false,
	)

	if !entityType.IsValid() ||
		!entity.IsValid() ||
		!relationType.IsValid() ||
		!relation.IsValid() {
		t.Fatal(
			"structural layer rejected unresolved references",
		)
	}
}

func TestE2E_RealPalletSchemaSemanticChain(
	t *testing.T,
) {
	u := testUnits()

	priceContractID := identity.New()
	pricedTraitID := identity.New()

	palletTypeID := identity.New()
	boardTypeID := identity.New()
	nailTypeID := identity.New()

	containsTypeID := identity.New()

	boardPriceID := identity.New()
	nailPriceID := identity.New()
	quantityID := identity.New()

	priceContract := schema.NewPropertyContract(
		priceContractID,
		"Price",
		value.Decimal,
		unit.Base("currency").
			Div(unit.Base("count")),
		true,
		schema.TemporalScalar,
		false,
	)

	pricedTrait := schema.NewTrait(
		pricedTraitID,
		"Priced",
		[]identity.UUID{
			priceContractID,
		},
	)

	boardPrice := schema.NewPropertyDefinition(
		boardPriceID,
		schema.EntityTypeOwner(boardTypeID),
		priceContractID,
		true,
		"Price",
		value.Decimal,
		u.rubPerItem,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	nailPrice := schema.NewPropertyDefinition(
		nailPriceID,
		schema.EntityTypeOwner(nailTypeID),
		priceContractID,
		true,
		"Price",
		value.Decimal,
		u.rubPerItem,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	// Quantity is a property of the EDGE, not the component.
	quantity := schema.NewPropertyDefinition(
		quantityID,
		schema.RelationTypeOwner(
			containsTypeID,
		),
		identity.UUID{},
		false,
		"Quantity",
		value.Integer,
		u.item,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	palletType := schema.NewEntityType(
		palletTypeID,
		"Pallet",
		nil,
		nil,
	)

	boardType := schema.NewEntityType(
		boardTypeID,
		"Board",
		[]identity.UUID{
			pricedTraitID,
		},
		[]identity.UUID{
			boardPriceID,
		},
	)

	nailType := schema.NewEntityType(
		nailTypeID,
		"Nail",
		[]identity.UUID{
			pricedTraitID,
		},
		[]identity.UUID{
			nailPriceID,
		},
	)

	// Pallet -> anything implementing Priced.
	contains := schema.NewRelationType(
		containsTypeID,
		"Contains",
		schema.EntityTypeConstraint(
			palletTypeID,
		),
		true,
		schema.TraitConstraint(
			pricedTraitID,
		),
		true,
		[]identity.UUID{
			quantityID,
		},
	)

	pallet := semantic.NewEntity(
		identity.New(),
		palletTypeID,
		"Pallet A",
		true,
	)

	board := semantic.NewEntity(
		identity.New(),
		boardTypeID,
		"Board A",
		true,
	)

	nail := semantic.NewEntity(
		identity.New(),
		nailTypeID,
		"Nail A",
		true,
	)

	boardRelation := semantic.NewRelation(
		identity.New(),
		containsTypeID,
		pallet.ID(),
		board.ID(),
		nil,
		false,
	)

	nailRelation := semantic.NewRelation(
		identity.New(),
		containsTypeID,
		pallet.ID(),
		nail.ID(),
		nil,
		false,
	)

	checks := []bool{
		priceContract.IsValid(),
		pricedTrait.IsValid(),

		boardPrice.IsValid(),
		nailPrice.IsValid(),
		quantity.IsValid(),

		palletType.IsValid(),
		boardType.IsValid(),
		nailType.IsValid(),

		contains.IsValid(),

		pallet.IsValid(),
		board.IsValid(),
		nail.IsValid(),

		boardRelation.IsValid(),
		nailRelation.IsValid(),
	}

	for i, ok := range checks {
		if !ok {
			t.Fatalf(
				"chain element %d invalid",
				i,
			)
		}
	}

	if quantity.Owner().Kind() !=
		schema.PropertyOwnerRelationType {
		t.Fatal(
			"quantity must belong to relation type",
		)
	}

	to, ok := contains.To()

	if !ok ||
		to.Kind() != schema.TypeConstraintTrait {
		t.Fatal(
			"contains target must be trait constrained",
		)
	}
}
