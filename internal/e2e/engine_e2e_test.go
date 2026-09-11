package e2e_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/types/date"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/duration"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

type units struct {
	meter      unit.Unit
	centimeter unit.Unit
	kilogram   unit.Unit
	gram       unit.Unit
	item       unit.Unit
	rub        unit.Unit
	rubPerItem unit.Unit
}

func testUnits() units {
	length := unit.Base("length")
	mass := unit.Base("mass")
	count := unit.Base("count")
	currency := unit.Base("currency")

	meter := unit.NewUnit(
		"meter",
		length,
		"length",
		rational.One(),
	)

	centimeter := unit.NewUnit(
		"centimeter",
		length,
		"length",
		rational.New(
			big.NewInt(1),
			big.NewInt(100),
		),
	)

	kilogram := unit.NewUnit(
		"kilogram",
		mass,
		"mass",
		rational.One(),
	)

	gram := unit.NewUnit(
		"gram",
		mass,
		"mass",
		rational.New(
			big.NewInt(1),
			big.NewInt(1000),
		),
	)

	item := unit.NewUnit(
		"item",
		count,
		"count",
		rational.One(),
	)

	rub := unit.NewUnit(
		"rub",
		currency,
		"rub",
		rational.One(),
	)

	rubPerItem, err := rub.Div(item)
	if err != nil {
		panic(err)
	}

	return units{
		meter:      meter,
		centimeter: centimeter,
		kilogram:   kilogram,
		gram:       gram,
		item:       item,
		rub:        rub,
		rubPerItem: rubPerItem,
	}
}

func dec(s string) decimal.Decimal {
	var d decimal.Decimal

	if err := d.UnmarshalText([]byte(s)); err != nil {
		panic(err)
	}

	return d
}

func assertType(
	t *testing.T,
	got typing.Type,
	wantValue value.Type,
	wantUnit unit.Unit,
	wantHasUnit bool,
) {
	t.Helper()

	if got.ValueType() != wantValue {
		t.Fatalf(
			"value type: got %s want %s",
			got.ValueType(),
			wantValue,
		)
	}

	gotUnit, gotHasUnit := got.Unit()

	if gotHasUnit != wantHasUnit {
		t.Fatalf(
			"has unit: got %v want %v",
			gotHasUnit,
			wantHasUnit,
		)
	}

	if wantHasUnit && !gotUnit.Equal(wantUnit) {
		t.Fatal("unit mismatch")
	}
}

func TestE2E_PrimitivesTypedValuesAndResolvedStates(t *testing.T) {
	u := testUnits()

	samples := []value.Typed{
		value.NewTyped(
			value.NewBool(true),
		),

		value.NewTyped(
			value.NewInt64(42),
		),

		value.NewTyped(
			value.NewDecimal(
				dec("123.450"),
			),
		),

		value.NewTyped(
			value.NewString("board"),
		),

		value.NewTyped(
			value.NewDate(
				date.New(2026, 8, 27),
			),
		),

		value.NewTyped(
			value.NewDateTime(
				datetime.New(
					date.New(2026, 8, 27),
					12,
					30,
					0,
					0,
				),
			),
		),

		value.NewTyped(
			value.NewDuration(
				duration.FromSeconds(90),
			),
		),

		value.NewTypedWithUnit(
			value.NewDecimal(
				dec("10.50"),
			),
			u.rub,
		),
	}

	for i, v := range samples {
		if !v.IsValid() {
			t.Fatalf("sample %d invalid", i)
		}
	}

	defined := value.Defined(
		samples[len(samples)-1],
	)

	null := value.Null()

	undefined := value.Undefined(
		value.NewUndefinedInfo(
			value.UndefinedMissingValue,
		),
	)

	if !defined.IsDefined() || !defined.IsValid() {
		t.Fatal("defined state invalid")
	}

	if !null.IsNull() || !null.IsValid() {
		t.Fatal("null state invalid")
	}

	if !undefined.IsUndefined() || !undefined.IsValid() {
		t.Fatal("undefined state invalid")
	}
}

func TestE2E_UnitAlgebraBusinessCases(t *testing.T) {
	u := testUnits()

	// m * m -> m²
	area := u.meter.Mul(u.meter)

	expectedAreaDimension :=
		unit.Base("length").
			Mul(unit.Base("length"))

	if !area.Dimension().Equal(expectedAreaDimension) {
		t.Fatal("m*m must be area")
	}

	// RUB/item * item -> RUB
	cost := u.rubPerItem.Mul(u.item)

	if !cost.Equal(u.rub) {
		t.Fatal("RUB/item * item must cancel to RUB")
	}

	// RUB / kg -> RUB/kg
	rubPerKg, err := u.rub.Div(u.kilogram)
	if err != nil {
		t.Fatal(err)
	}

	expectedRubPerKgDimension :=
		unit.Base("currency").
			Div(unit.Base("mass"))

	if !rubPerKg.Dimension().Equal(
		expectedRubPerKgDimension,
	) {
		t.Fatal("RUB/kg dimension mismatch")
	}

	// kg / g -> dimensionless with scale 1000
	kgPerG, err := u.kilogram.Div(u.gram)
	if err != nil {
		t.Fatal(err)
	}

	if !kgPerG.IsDimensionless() {
		t.Fatal("kg/g must be dimensionless")
	}

	if kgPerG.Scale().Cmp(
		rational.FromInt64(1000),
	) != 0 {
		t.Fatalf(
			"kg/g scale = %s, want 1000",
			kgPerG.Scale(),
		)
	}
}

func TestE2E_ArithmeticTypeInferenceMatrix(t *testing.T) {
	u := testUnits()

	area := u.meter.Mul(u.meter)

	inverseKg, err :=
		unit.DimensionlessUnit().Div(u.kilogram)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string

		op ast.ArithmeticOperator

		left  typing.Type
		right typing.Type

		wantValue   value.Type
		wantUnit    unit.Unit
		wantHasUnit bool
	}{
		{
			name: "int+int",
			op:   ast.ArithmeticAdd,

			left:  typing.New(value.Integer),
			right: typing.New(value.Integer),

			wantValue: value.Integer,
		},
		{
			name: "int+decimal",
			op:   ast.ArithmeticAdd,

			left:  typing.New(value.Integer),
			right: typing.New(value.Decimal),

			wantValue: value.Decimal,
		},
		{
			name: "decimal-int",
			op:   ast.ArithmeticSubtract,

			left:  typing.New(value.Decimal),
			right: typing.New(value.Integer),

			wantValue: value.Decimal,
		},
		{
			name: "int*int",
			op:   ast.ArithmeticMultiply,

			left:  typing.New(value.Integer),
			right: typing.New(value.Integer),

			wantValue: value.Integer,
		},
		{
			name: "int/int promotes to decimal",
			op:   ast.ArithmeticDivide,

			left:  typing.New(value.Integer),
			right: typing.New(value.Integer),

			wantValue: value.Decimal,
		},
		{
			name: "kg+g -> kg",
			op:   ast.ArithmeticAdd,

			left: typing.NewWithUnit(
				value.Decimal,
				u.kilogram,
			),

			right: typing.NewWithUnit(
				value.Integer,
				u.gram,
			),

			wantValue:   value.Decimal,
			wantUnit:    u.kilogram,
			wantHasUnit: true,
		},
		{
			name: "g+kg -> g",
			op:   ast.ArithmeticAdd,

			left: typing.NewWithUnit(
				value.Decimal,
				u.gram,
			),

			right: typing.NewWithUnit(
				value.Integer,
				u.kilogram,
			),

			wantValue:   value.Decimal,
			wantUnit:    u.gram,
			wantHasUnit: true,
		},
		{
			name: "m*m -> m2",
			op:   ast.ArithmeticMultiply,

			left: typing.NewWithUnit(
				value.Decimal,
				u.meter,
			),

			right: typing.NewWithUnit(
				value.Decimal,
				u.meter,
			),

			wantValue:   value.Decimal,
			wantUnit:    area,
			wantHasUnit: true,
		},
		{
			name: "rub/item*item -> rub",
			op:   ast.ArithmeticMultiply,

			left: typing.NewWithUnit(
				value.Decimal,
				u.rubPerItem,
			),

			right: typing.NewWithUnit(
				value.Integer,
				u.item,
			),

			wantValue:   value.Decimal,
			wantUnit:    u.rub,
			wantHasUnit: true,
		},
		{
			name: "scalar*rub -> rub",
			op:   ast.ArithmeticMultiply,

			left: typing.New(
				value.Decimal,
			),

			right: typing.NewWithUnit(
				value.Decimal,
				u.rub,
			),

			wantValue:   value.Decimal,
			wantUnit:    u.rub,
			wantHasUnit: true,
		},
		{
			name: "rub/scalar -> rub",
			op:   ast.ArithmeticDivide,

			left: typing.NewWithUnit(
				value.Decimal,
				u.rub,
			),

			right: typing.New(
				value.Integer,
			),

			wantValue:   value.Decimal,
			wantUnit:    u.rub,
			wantHasUnit: true,
		},
		{
			name: "scalar/kg -> 1/kg",
			op:   ast.ArithmeticDivide,

			left: typing.New(
				value.Integer,
			),

			right: typing.NewWithUnit(
				value.Decimal,
				u.kilogram,
			),

			wantValue:   value.Decimal,
			wantUnit:    inverseKg,
			wantHasUnit: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := typecheck.InferArithmetic(
				tc.op,
				tc.left,
				tc.right,
			)

			if err != nil {
				t.Fatal(err)
			}

			assertType(
				t,
				got,
				tc.wantValue,
				tc.wantUnit,
				tc.wantHasUnit,
			)
		})
	}
}

func TestE2E_ArithmeticRejectsInvalidBusinessMath(
	t *testing.T,
) {
	u := testUnits()

	cases := []struct {
		name string

		op ast.ArithmeticOperator

		left  typing.Type
		right typing.Type

		want typecheck.ArithmeticErrorKind
	}{
		{
			name: "kg+meter",

			op: ast.ArithmeticAdd,

			left: typing.NewWithUnit(
				value.Decimal,
				u.kilogram,
			),

			right: typing.NewWithUnit(
				value.Decimal,
				u.meter,
			),

			want: typecheck.ArithmeticErrorIncompatibleUnits,
		},
		{
			name: "kg+unitless",

			op: ast.ArithmeticAdd,

			left: typing.NewWithUnit(
				value.Decimal,
				u.kilogram,
			),

			right: typing.New(
				value.Decimal,
			),

			want: typecheck.ArithmeticErrorIncompatibleUnits,
		},
		{
			name: "string+int",

			op: ast.ArithmeticAdd,

			left: typing.New(
				value.String,
			),

			right: typing.New(
				value.Integer,
			),

			want: typecheck.ArithmeticErrorNonNumericOperand,
		},
		{
			name: "bool*decimal",

			op: ast.ArithmeticMultiply,

			left: typing.New(
				value.Bool,
			),

			right: typing.New(
				value.Decimal,
			),

			want: typecheck.ArithmeticErrorNonNumericOperand,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := typecheck.InferArithmetic(
				tc.op,
				tc.left,
				tc.right,
			)

			if err == nil {
				t.Fatal("expected error")
			}

			var arithmeticError typecheck.ArithmeticError

			if !errors.As(
				err,
				&arithmeticError,
			) {
				t.Fatalf(
					"unexpected error %T",
					err,
				)
			}

			if arithmeticError.Kind() != tc.want {
				t.Fatalf(
					"kind got %s want %s",
					arithmeticError.Kind(),
					tc.want,
				)
			}
		})
	}
}

func TestE2E_BoardAreaSchemaToASTToStaticType(
	t *testing.T,
) {
	u := testUnits()

	boardTypeID := identity.New()

	lengthID := identity.New()
	widthID := identity.New()
	areaID := identity.New()

	// area = self.length * self.width
	areaExpr := ast.Multiply(
		ast.NewSelfPropertyReference(lengthID),
		ast.NewSelfPropertyReference(widthID),
	)

	length := schema.NewPropertyDefinition(
		lengthID,
		schema.EntityTypeOwner(boardTypeID),
		identity.UUID{},
		false,
		"Length",
		value.Decimal,
		u.meter,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	width := schema.NewPropertyDefinition(
		widthID,
		schema.EntityTypeOwner(boardTypeID),
		identity.UUID{},
		false,
		"Width",
		value.Decimal,
		u.meter,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
	)

	areaUnit := u.meter.Mul(u.meter)

	area := schema.NewPropertyDefinition(
		areaID,
		schema.EntityTypeOwner(boardTypeID),
		identity.UUID{},
		false,
		"Area",
		value.Decimal,
		areaUnit,
		true,
		schema.TemporalScalar,
		schema.DerivedComputation(areaExpr),
	)

	boardType := schema.NewEntityType(
		boardTypeID,
		"Board",
		nil,
		[]identity.UUID{
			lengthID,
			widthID,
			areaID,
		},
	)

	board := semantic.NewEntity(
		identity.New(),
		boardTypeID,
		"Board A",
		true,
	)

	checks := map[string]bool{
		"length":    length.IsValid(),
		"width":     width.IsValid(),
		"area":      area.IsValid(),
		"boardType": boardType.IsValid(),
		"board":     board.IsValid(),
		"expr":      areaExpr.IsValid(),
	}

	for name, ok := range checks {
		if !ok {
			t.Fatalf("%s invalid", name)
		}
	}

	result, err := typecheck.InferArithmetic(
		areaExpr.Operator(),

		typing.NewWithUnit(
			length.ValueType(),
			u.meter,
		),

		typing.NewWithUnit(
			width.ValueType(),
			u.meter,
		),
	)

	if err != nil {
		t.Fatal(err)
	}

	assertType(
		t,
		result,
		value.Decimal,
		areaUnit,
		true,
	)
}

func TestE2E_HeterogeneousPalletGraphAndAggregateAST(
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
	totalCostID := identity.New()

	// PRICE contract = Decimal<Currency/Count>
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

	// Anything implementing Priced must provide PRICE.
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

	// Quantity belongs to the relation itself.
	quantity := schema.NewPropertyDefinition(
		quantityID,
		schema.RelationTypeOwner(containsTypeID),
		identity.UUID{},
		false,
		"Quantity",
		value.Integer,
		u.item,
		true,
		schema.TemporalScalar,
		schema.InputComputation(),
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

	source := ast.NewTraversalSource(
		ast.TraversalDirectionOutgoing,
		containsTypeID,
	)

	// boundEntity.PRICE * boundRelation.QUANTITY
	mapExpr := ast.Multiply(
		ast.NewBoundContractPropertyReference(
			ast.BoundTargetEntity,
			priceContractID,
		),
		ast.NewBoundPropertyDefinitionReference(
			ast.BoundTargetRelation,
			quantityID,
		),
	)

	// SUM(outgoing contains, PRICE * QUANTITY)
	totalCostExpr := ast.NewMappedAggregate(
		source,
		mapExpr,
		ast.AggregateOperationSum,
	)

	totalCost := schema.NewPropertyDefinition(
		totalCostID,
		schema.EntityTypeOwner(palletTypeID),
		identity.UUID{},
		false,
		"TotalCost",
		value.Decimal,
		u.rub,
		true,
		schema.TemporalScalar,
		schema.DerivedComputation(totalCostExpr),
	)

	palletType := schema.NewEntityType(
		palletTypeID,
		"Pallet",
		nil,
		[]identity.UUID{
			totalCostID,
		},
	)

	// Pallet --contains--> anything implementing Priced.
	containsType := schema.NewRelationType(
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
		nail.ID(),
		nil,
		false,
	)

	checks := map[string]bool{
		"priceContract": priceContract.IsValid(),
		"pricedTrait":   pricedTrait.IsValid(),

		"boardPrice": boardPrice.IsValid(),
		"nailPrice":  nailPrice.IsValid(),
		"quantity":   quantity.IsValid(),

		"boardType":    boardType.IsValid(),
		"nailType":     nailType.IsValid(),
		"palletType":   palletType.IsValid(),
		"containsType": containsType.IsValid(),

		"totalCost": totalCost.IsValid(),

		"pallet": pallet.IsValid(),
		"board":  board.IsValid(),
		"nail":   nail.IsValid(),

		"r1": r1.IsValid(),
		"r2": r2.IsValid(),

		"aggregate": totalCostExpr.IsValid(),
	}

	for name, ok := range checks {
		if !ok {
			t.Fatalf("%s invalid", name)
		}
	}

	// The current typechecker can already prove the map expression:
	//
	// Decimal<RUB/item> * Integer<item>
	// -> Decimal<RUB>
	mappedType, err := typecheck.InferArithmetic(
		mapExpr.Operator(),

		typing.NewWithUnit(
			value.Decimal,
			u.rubPerItem,
		),

		typing.NewWithUnit(
			value.Integer,
			u.item,
		),
	)

	if err != nil {
		t.Fatal(err)
	}

	assertType(
		t,
		mappedType,
		value.Decimal,
		u.rub,
		true,
	)
}

func TestE2E_CountFilterConditionalAndReferencesCompose(
	t *testing.T,
) {
	u := testUnits()

	containsID := identity.New()
	quantityID := identity.New()

	source := ast.NewTraversalSource(
		ast.TraversalDirectionBoth,
		containsID,
	)

	quantityRef :=
		ast.NewBoundPropertyDefinitionReference(
			ast.BoundTargetRelation,
			quantityID,
		)

	zero := ast.NewConstant(
		value.NewTypedWithUnit(
			value.NewInt64(0),
			u.item,
		),
	)

	// boundRelation.quantity > 0
	positive := ast.Greater(
		quantityRef,
		zero,
	)

	// COUNT(contains WHERE quantity > 0)
	count := ast.NewFilteredCountAggregate(
		source,
		positive,
	)

	fallback := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(0),
		),
	)

	coalesce := ast.NewCoalesce(
		count,
		fallback,
	)

	absolute :=
		ast.NewAbsolutePropertyReference(
			semantic.NewPropertyAddress(
				semantic.EntityRef(
					identity.New(),
				),
				identity.New(),
			),
		)

	condition := ast.NewIsNull(
		absolute,
	)

	whole := ast.NewIf(
		condition,
		fallback,
		coalesce,
	)

	if !source.IsValid() {
		t.Fatal("source invalid")
	}

	if !quantityRef.IsValid() {
		t.Fatal("quantity ref invalid")
	}

	if !positive.IsValid() {
		t.Fatal("comparison invalid")
	}

	if !count.IsValid() {
		t.Fatal("count invalid")
	}

	if !coalesce.IsValid() {
		t.Fatal("coalesce invalid")
	}

	if !condition.IsValid() {
		t.Fatal("is-null invalid")
	}

	if !whole.IsValid() {
		t.Fatal("whole conditional AST invalid")
	}
}

func TestE2E_ValueBindingsForEntityAndRelationProperties(
	t *testing.T,
) {
	u := testUnits()

	entityID := identity.New()
	relationID := identity.New()

	priceID := identity.New()
	quantityID := identity.New()

	priceAddress :=
		semantic.NewPropertyAddress(
			semantic.EntityRef(entityID),
			priceID,
		)

	quantityAddress :=
		semantic.NewPropertyAddress(
			semantic.RelationRef(relationID),
			quantityID,
		)

	priceBinding := semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(
			priceAddress,
		),
		semantic.DefinedBindingValue(
			value.NewTypedWithUnit(
				value.NewDecimal(
					dec("12.50"),
				),
				u.rubPerItem,
			),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	quantityBinding := semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(
			quantityAddress,
		),
		semantic.DefinedBindingValue(
			value.NewTypedWithUnit(
				value.NewInt64(8),
				u.item,
			),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	nullBinding := semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(
			priceAddress,
		),
		semantic.NullBindingValue(),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)

	if !priceBinding.IsValid() {
		t.Fatal("price binding invalid")
	}

	if !quantityBinding.IsValid() {
		t.Fatal("quantity binding invalid")
	}

	if !nullBinding.IsValid() {
		t.Fatal("null binding invalid")
	}
}
