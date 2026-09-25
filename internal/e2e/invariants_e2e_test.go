package e2e_test

import (
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// mustPanic verifies that invalid construction is rejected immediately.
//
// These tests intentionally exercise programmer-error invariants.
// Public constructors in the domain/AST layer use panic for states that
// must never be constructed by correct engine code.
func mustPanic(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	fn()
}

func TestE2E_ASTRejectsZeroReferenceIDs(t *testing.T) {
	t.Run("self property reference", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewSelfPropertyReference(identity.UUID{})
		})
	})

	t.Run("bound entity property definition", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewBoundPropertyDefinitionReference(
				ast.BoundTargetEntity,
				identity.UUID{},
			)
		})
	})

	t.Run("bound relation property definition", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewBoundPropertyDefinitionReference(
				ast.BoundTargetRelation,
				identity.UUID{},
			)
		})
	})

	t.Run("bound entity contract", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewBoundContractPropertyReference(
				ast.BoundTargetEntity,
				identity.UUID{},
			)
		})
	})

	t.Run("bound relation contract", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewBoundContractPropertyReference(
				ast.BoundTargetRelation,
				identity.UUID{},
			)
		})
	})
}

func TestE2E_ASTRejectsInvalidBoundTargets(t *testing.T) {
	definitionID := identity.New()
	contractID := identity.New()

	t.Run("property definition", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewBoundPropertyDefinitionReference(
				ast.BoundTargetInvalid,
				definitionID,
			)
		})
	})

	t.Run("contract", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewBoundContractPropertyReference(
				ast.BoundTargetInvalid,
				contractID,
			)
		})
	})
}

func TestE2E_TraversalRejectsInvalidStructure(t *testing.T) {
	relationTypeID := identity.New()

	t.Run("invalid direction", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewTraversalSource(
				ast.TraversalDirectionInvalid,
				relationTypeID,
			)
		})
	})

	t.Run("zero relation type", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewTraversalSource(
				ast.TraversalDirectionOutgoing,
				identity.UUID{},
			)
		})
	})
}

func TestE2E_AllTraversalDirectionsAreStructurallyValid(t *testing.T) {
	relationTypeID := identity.New()

	directions := []ast.TraversalDirection{
		ast.TraversalDirectionOutgoing,
		ast.TraversalDirectionIncoming,
		ast.TraversalDirectionBoth,
	}

	for _, direction := range directions {
		source := ast.NewTraversalSource(
			direction,
			relationTypeID,
		)

		if !source.IsValid() {
			t.Fatalf(
				"direction %s produced invalid traversal",
				direction,
			)
		}
	}
}

func TestE2E_ArithmeticRejectsInvalidASTStructure(t *testing.T) {
	left := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(1),
		),
	)

	right := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(2),
		),
	)

	t.Run("invalid operator", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewArithmetic(
				ast.ArithmeticInvalid,
				left,
				right,
			)
		})
	})

	t.Run("nil left", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewArithmetic(
				ast.ArithmeticAdd,
				nil,
				right,
			)
		})
	})

	t.Run("nil right", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewArithmetic(
				ast.ArithmeticAdd,
				left,
				nil,
			)
		})
	})

	t.Run("nil negate", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewNegate(nil)
		})
	})
}

func TestE2E_ComparisonRejectsInvalidASTStructure(t *testing.T) {
	left := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(1),
		),
	)

	right := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(2),
		),
	)

	t.Run("invalid operator", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewComparison(
				ast.ComparisonInvalid,
				left,
				right,
			)
		})
	})

	t.Run("nil left", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewComparison(
				ast.ComparisonEqual,
				nil,
				right,
			)
		})
	})

	t.Run("nil right", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewComparison(
				ast.ComparisonEqual,
				left,
				nil,
			)
		})
	})
}

func TestE2E_LogicalRejectsInvalidASTStructure(t *testing.T) {
	left := ast.NewConstant(
		value.NewTyped(
			value.NewBool(true),
		),
	)

	right := ast.NewConstant(
		value.NewTyped(
			value.NewBool(false),
		),
	)

	t.Run("invalid operator", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewLogical(
				ast.LogicalInvalid,
				left,
				right,
			)
		})
	})

	t.Run("nil left", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewLogical(
				ast.LogicalAnd,
				nil,
				right,
			)
		})
	})

	t.Run("nil right", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewLogical(
				ast.LogicalOr,
				left,
				nil,
			)
		})
	})

	t.Run("nil not", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewNot(nil)
		})
	})
}

func TestE2E_ConditionalRejectsInvalidASTStructure(t *testing.T) {
	condition := ast.NewConstant(
		value.NewTyped(
			value.NewBool(true),
		),
	)

	thenExpr := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(1),
		),
	)

	elseExpr := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(2),
		),
	)

	t.Run("if nil condition", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewIf(
				nil,
				thenExpr,
				elseExpr,
			)
		})
	})

	t.Run("if nil then", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewIf(
				condition,
				nil,
				elseExpr,
			)
		})
	})

	t.Run("if nil else", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewIf(
				condition,
				thenExpr,
				nil,
			)
		})
	})

	t.Run("is null nil", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewIsNull(nil)
		})
	})

	t.Run("coalesce nil primary", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewCoalesce(
				nil,
				elseExpr,
			)
		})
	})

	t.Run("coalesce nil fallback", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewCoalesce(
				thenExpr,
				nil,
			)
		})
	})
}

func TestE2E_AggregateConstructionPreventsInvalidForms(t *testing.T) {
	source := ast.NewTraversalSource(
		ast.TraversalDirectionOutgoing,
		identity.New(),
	)

	mapExpr := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(1),
		),
	)

	filter := ast.NewConstant(
		value.NewTyped(
			value.NewBool(true),
		),
	)

	t.Run("count has no map", func(t *testing.T) {
		count := ast.NewCountAggregate(source)

		if !count.IsValid() {
			t.Fatal("count invalid")
		}

		if _, ok := count.Map(); ok {
			t.Fatal("COUNT must not have map expression")
		}
	})

	t.Run("filtered count has filter", func(t *testing.T) {
		count := ast.NewFilteredCountAggregate(
			source,
			filter,
		)

		if !count.IsValid() {
			t.Fatal("filtered count invalid")
		}

		gotFilter, ok := count.Filter()

		if !ok {
			t.Fatal("expected filter")
		}

		if gotFilter == nil {
			t.Fatal("filter must not be nil")
		}

		if _, ok := count.Map(); ok {
			t.Fatal("COUNT must not have map expression")
		}
	})

	t.Run("mapped count rejected", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewMappedAggregate(
				source,
				mapExpr,
				ast.AggregateOperationCount,
			)
		})
	})

	t.Run("mapped aggregate requires map", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewMappedAggregate(
				source,
				nil,
				ast.AggregateOperationSum,
			)
		})
	})

	t.Run("filtered count requires filter", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewFilteredCountAggregate(
				source,
				nil,
			)
		})
	})

	t.Run("filtered mapped aggregate requires filter", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewFilteredMappedAggregate(
				source,
				nil,
				mapExpr,
				ast.AggregateOperationSum,
			)
		})
	})

	t.Run("filtered mapped aggregate requires map", func(t *testing.T) {
		mustPanic(t, func() {
			ast.NewFilteredMappedAggregate(
				source,
				filter,
				nil,
				ast.AggregateOperationSum,
			)
		})
	})

	for _, operation := range []ast.AggregateOperation{
		ast.AggregateOperationSum,
		ast.AggregateOperationMin,
		ast.AggregateOperationMax,
	} {
		t.Run(operation.String(), func(t *testing.T) {
			aggregate := ast.NewMappedAggregate(
				source,
				mapExpr,
				operation,
			)

			if !aggregate.IsValid() {
				t.Fatalf(
					"%s aggregate invalid",
					operation,
				)
			}

			if _, ok := aggregate.Map(); !ok {
				t.Fatalf(
					"%s must have map",
					operation,
				)
			}
		})
	}
}

func TestE2E_NumericPromotionContract(t *testing.T) {
	numeric := []value.Type{
		value.Integer,
		value.Decimal,
	}

	for _, left := range numeric {
		for _, right := range numeric {
			t.Run(
				left.String()+"+"+right.String(),
				func(t *testing.T) {
					result, ok :=
						typecheck.PromoteAdditive(
							left,
							right,
						)

					if !ok {
						t.Fatal("numeric addition rejected")
					}

					want := value.Integer

					if left == value.Decimal ||
						right == value.Decimal {
						want = value.Decimal
					}

					if result != want {
						t.Fatalf(
							"got %s want %s",
							result,
							want,
						)
					}
				},
			)

			t.Run(
				left.String()+"*"+right.String(),
				func(t *testing.T) {
					result, ok :=
						typecheck.PromoteMultiplicative(
							left,
							right,
						)

					if !ok {
						t.Fatal("numeric multiplication rejected")
					}

					want := value.Integer

					if left == value.Decimal ||
						right == value.Decimal {
						want = value.Decimal
					}

					if result != want {
						t.Fatalf(
							"got %s want %s",
							result,
							want,
						)
					}
				},
			)

			t.Run(
				left.String()+"/"+right.String(),
				func(t *testing.T) {
					result, ok :=
						typecheck.PromoteDivision(
							left,
							right,
						)

					if !ok {
						t.Fatal("numeric division rejected")
					}

					if result != value.Decimal {
						t.Fatalf(
							"division must produce Decimal, got %s",
							result,
						)
					}
				},
			)
		}
	}
}

func TestE2E_AllNonNumericTypesRejectedByNumericPromotion(
	t *testing.T,
) {
	nonNumeric := []value.Type{
		value.Bool,
		value.String,
		value.Date,
		value.DateTime,
		value.Duration,
	}

	for _, typ := range nonNumeric {
		if typecheck.IsNumeric(typ) {
			t.Fatalf(
				"%s unexpectedly considered numeric",
				typ,
			)
		}

		if _, ok := typecheck.PromoteAdditive(
			typ,
			value.Integer,
		); ok {
			t.Fatalf(
				"%s + Integer unexpectedly accepted",
				typ,
			)
		}

		if _, ok := typecheck.PromoteMultiplicative(
			value.Decimal,
			typ,
		); ok {
			t.Fatalf(
				"Decimal * %s unexpectedly accepted",
				typ,
			)
		}

		if _, ok := typecheck.PromoteDivision(
			typ,
			value.Decimal,
		); ok {
			t.Fatalf(
				"%s / Decimal unexpectedly accepted",
				typ,
			)
		}
	}
}

func TestE2E_ArithmeticTypeInferenceIsOrderAwareForAddition(
	t *testing.T,
) {
	u := testUnits()

	kgPlusG, err := typecheck.InferArithmetic(
		ast.ArithmeticAdd,
		typing.NewWithUnit(
			value.Decimal,
			u.kilogram,
		),
		typing.NewWithUnit(
			value.Decimal,
			u.gram,
		),
	)

	if err != nil {
		t.Fatal(err)
	}

	gPlusKg, err := typecheck.InferArithmetic(
		ast.ArithmeticAdd,
		typing.NewWithUnit(
			value.Decimal,
			u.gram,
		),
		typing.NewWithUnit(
			value.Decimal,
			u.kilogram,
		),
	)

	if err != nil {
		t.Fatal(err)
	}

	kgResult, ok := kgPlusG.Unit()
	if !ok {
		t.Fatal("kg+g must have unit")
	}

	gResult, ok := gPlusKg.Unit()
	if !ok {
		t.Fatal("g+kg must have unit")
	}

	if !kgResult.Equal(u.kilogram) {
		t.Fatal("kg+g result must use left unit kg")
	}

	if !gResult.Equal(u.gram) {
		t.Fatal("g+kg result must use left unit g")
	}
}

func TestE2E_DeepASTCanBeComposedWithoutSemanticLeakage(
	t *testing.T,
) {
	containsID := identity.New()
	quantityID := identity.New()
	priceContractID := identity.New()

	source := ast.NewTraversalSource(
		ast.TraversalDirectionOutgoing,
		containsID,
	)

	price :=
		ast.NewBoundContractPropertyReference(
			ast.BoundTargetEntity,
			priceContractID,
		)

	quantity :=
		ast.NewBoundPropertyDefinitionReference(
			ast.BoundTargetRelation,
			quantityID,
		)

	zero := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(0),
		),
	)

	positiveQuantity := ast.Greater(
		quantity,
		zero,
	)

	lineCost := ast.Multiply(
		price,
		quantity,
	)

	sum := ast.NewFilteredMappedAggregate(
		source,
		positiveQuantity,
		lineCost,
		ast.AggregateOperationSum,
	)

	fallback := ast.NewConstant(
		value.NewTyped(
			value.NewInt64(0),
		),
	)

	result := ast.NewCoalesce(
		sum,
		fallback,
	)

	if !source.IsValid() ||
		!price.IsValid() ||
		!quantity.IsValid() ||
		!positiveQuantity.IsValid() ||
		!lineCost.IsValid() ||
		!sum.IsValid() ||
		!result.IsValid() {
		t.Fatal("deep AST composition invalid")
	}
}
