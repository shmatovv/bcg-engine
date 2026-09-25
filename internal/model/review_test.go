package model

import (
	"strconv"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
	"github.com/shmatovv/bcg-engine/v2/internal/types/rational"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func TestDraftValidationUnitExponentOverflow(t *testing.T) {
	u := unit.NewUnit("meter", unit.Base("length"), "length", rational.One())
	for i := 0; i < strconv.IntSize-2; i++ {
		u = u.Mul(u)
	}
	constant := ast.NewConstant(value.NewTypedWithUnit(value.NewInt64(1), u))
	for _, expression := range []ast.Expression{ast.Multiply(constant, constant), ast.Divide(constant, ast.NewConstant(value.NewTypedWithUnit(value.NewInt64(1), mustInverse(t, u))))} {
		draft := NewDraft()
		typeID, propertyID := identity.New(), identity.New()
		draft.AddEntityType(schema.NewEntityType(typeID, "type", nil, []identity.UUID{propertyID}))
		draft.AddPropertyDefinition(schema.NewPropertyDefinition(propertyID, schema.EntityTypeOwner(typeID), identity.UUID{}, false, "overflow", value.Decimal, u, true, schema.TemporalScalar, schema.DerivedComputation(expression)))
		if report := draft.Validate(); !report.Has(ValidationIssueInvalidDerivedExpression) {
			t.Fatal("overflow must produce a validation issue")
		}
	}
}

func mustInverse(t *testing.T, u unit.Unit) unit.Unit {
	t.Helper()
	inverse, err := unit.DimensionlessUnit().Div(u)
	if err != nil {
		t.Fatal(err)
	}
	return inverse
}

func TestCommitPrecisionScaleContract(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("accepted unsupported committed decimal scale")
		}
	}()
	NewCommitOptions(identity.New(), "", precision.New(decimal.MaxScale+1, precision.HalfEven))
}
