package evaluator_test

import (
	"math/big"
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
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func reviewContext() runtime.CalculationContext {
	at, _ := datetime.Parse("2026-01-01T00:00:00Z")
	return runtime.NewCalculationContext(domaintime.At(at), scope.Values{}, nil)
}

func reviewCommit(t *testing.T, draft *model.Draft) model.ModelVersion {
	t.Helper()
	v, err := model.Commit(model.NewModel(identity.New(), "review"), draft,
		model.NewCommitOptions(identity.New(), "review", precision.New(6, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func integer(n int64) ast.Constant { return ast.NewConstant(value.NewTyped(value.NewInt64(n))) }
func boolean(b bool) ast.Constant  { return ast.NewConstant(value.NewTyped(value.NewBool(b))) }

func TestConditionalRuntimePromotion(t *testing.T) {
	for _, expression := range []ast.Expression{
		ast.NewIf(boolean(true), integer(1), ast.NewConstant(value.NewTyped(value.NewDecimal(decimal.One())))),
		ast.NewCoalesce(integer(1), ast.NewConstant(value.NewTyped(value.NewDecimal(decimal.One())))),
	} {
		t.Run("derived", func(t *testing.T) {
			draft := model.NewDraft()
			typeID, propertyID, entityID := identity.New(), identity.New(), identity.New()
			draft.AddEntityType(schema.NewEntityType(typeID, "item", nil, []identity.UUID{propertyID}))
			draft.AddPropertyDefinition(schema.NewPropertyDefinition(propertyID, schema.EntityTypeOwner(typeID), identity.UUID{}, false,
				"result", value.Decimal, unit.Unit{}, false, schema.TemporalScalar, schema.DerivedComputation(ast.Multiply(expression, integer(2)))))
			draft.AddEntity(semantic.NewEntity(entityID, typeID, "", false))
			result, err := evaluator.Evaluate(reviewCommit(t, draft), address.NewPropertyAddress(address.EntityRef(entityID), propertyID), reviewContext())
			if err != nil {
				t.Fatal(err)
			}
			if result.Value().Value().Raw().Decimal().String() != "2" {
				t.Fatal("wrong promoted result")
			}
		})
	}
}

func TestConditionalNormalizesBeforeParentArithmetic(t *testing.T) {
	meter := unit.NewUnit("meter", unit.Base("length"), "length", rational.One())
	centimeter := unit.NewUnit("centimeter", unit.Base("length"), "length", rational.New(big.NewInt(1), big.NewInt(100)))
	quantity := func(n int64, u unit.Unit) ast.Expression {
		return ast.NewConstant(value.NewTypedWithUnit(value.NewInt64(n), u))
	}
	// The conditional's inferred unit is meter. Dividing by 3 must round in
	// meters, not centimeters (which would produce a different answer).
	expr := ast.Multiply(ast.Divide(ast.NewIf(boolean(false), quantity(1, meter), quantity(1, centimeter)), integer(3)), integer(3))
	result, err := evaluator.EvaluateExpression(reviewCommit(t, model.NewDraft()), expr, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Value().Value().Raw().Decimal().String(); got != "0.009999" {
		t.Fatalf("got %s", got)
	}
}

func TestDivisionByZeroReason(t *testing.T) {
	result, err := evaluator.EvaluateExpression(reviewCommit(t, model.NewDraft()), ast.Divide(integer(1), integer(0)), reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Value().IsUndefined() || result.Value().UndefinedInfo().Kind().String() != "DivisionByZero" {
		t.Fatalf("wrong reason: %v", result.Value())
	}
}

func TestInvalidUnusedCalculationOverride(t *testing.T) {
	context := runtime.NewCalculationContext(reviewContext().TimeContext(), scope.Values{}, []runtime.Override{
		runtime.NewOverride(address.NewPropertyAddress(address.EntityRef(identity.New()), identity.New()), semantic.DefinedBindingValue(value.NewTyped(value.NewInt64(1)))),
	})
	_, err := evaluator.EvaluateExpression(reviewCommit(t, model.NewDraft()), integer(1), context)
	if err == nil {
		t.Fatal("accepted override of nonexistent property")
	}
}
