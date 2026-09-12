package model

import (
	"math/big"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/internal/types/rational"
	"github.com/shmatovv/bcg-engine/internal/types/series"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func testPointSeries(raw value.Raw) series.Series {
	return series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{
		series.NewPoint(mustDateTimeForSeries("2026-01-01T00:00:00Z"), series.Defined(raw)),
	})
}

func mustDateTimeForSeries(text string) datetime.DateTime {
	result, err := datetime.Parse(text)
	if err != nil {
		panic(err)
	}
	return result
}

func TestDraftValidateSeriesBindingShapeAndSamples(t *testing.T) {
	t.Run("series on scalar", func(t *testing.T) {
		fixture := newBaseValueFixture(t, schema.InputComputation(), schema.TemporalScalar, identity.UUID{}, false, value.Integer, unit.Unit{}, false)
		fixture.addObjectBinding(semantic.SeriesBindingValue(testPointSeries(value.NewInt64(1))), nil, domaintime.EffectivePeriod{}, false)
		requireValidationIssue(t, fixture.draft.Validate(), ValidationIssueTemporalValueMismatch)
	})
	t.Run("wrong element type", func(t *testing.T) {
		fixture := newBaseValueFixture(t, schema.InputComputation(), schema.TemporalSeries, identity.UUID{}, false, value.Integer, unit.Unit{}, false)
		stored := series.NewPointSeries(value.Bool, unit.Unit{}, false, []series.Point{series.NewPoint(mustDateTimeForSeries("2026-01-01T00:00:00Z"), series.Defined(value.NewBool(true)))})
		fixture.addObjectBinding(semantic.SeriesBindingValue(stored), nil, domaintime.EffectivePeriod{}, false)
		requireValidationIssue(t, fixture.draft.Validate(), ValidationIssueValueTypeMismatch)
	})
	t.Run("wrong unit", func(t *testing.T) {
		meter := unit.NewUnit("meter", unit.Base("length"), "length", rational.One())
		centimeter := unit.NewUnit("centimeter", unit.Base("length"), "length", rational.New(big.NewInt(1), big.NewInt(100)))
		fixture := newBaseValueFixture(t, schema.InputComputation(), schema.TemporalSeries, identity.UUID{}, false, value.Integer, meter, true)
		stored := series.NewPointSeries(value.Integer, centimeter, true, []series.Point{series.NewPoint(mustDateTimeForSeries("2026-01-01T00:00:00Z"), series.Defined(value.NewInt64(1)))})
		fixture.addObjectBinding(semantic.SeriesBindingValue(stored), nil, domaintime.EffectivePeriod{}, false)
		requireValidationIssue(t, fixture.draft.Validate(), ValidationIssueValueUnitMismatch)
	})
	t.Run("null sample without nullable contract", func(t *testing.T) {
		fixture := newBaseValueFixture(t, schema.InputComputation(), schema.TemporalSeries, identity.UUID{}, false, value.Integer, unit.Unit{}, false)
		stored := series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{series.NewPoint(mustDateTimeForSeries("2026-01-01T00:00:00Z"), series.Null())})
		fixture.addObjectBinding(semantic.SeriesBindingValue(stored), nil, domaintime.EffectivePeriod{}, false)
		requireValidationIssue(t, fixture.draft.Validate(), ValidationIssueNullNotAllowed)
	})
	t.Run("whole series null", func(t *testing.T) {
		fixture := newBaseValueFixture(t, schema.InputComputation(), schema.TemporalSeries, identity.UUID{}, false, value.Integer, unit.Unit{}, false)
		fixture.addObjectBinding(semantic.NullBindingValue(), nil, domaintime.EffectivePeriod{}, false)
		requireValidationIssue(t, fixture.draft.Validate(), ValidationIssueTemporalValueMismatch)
	})
}

func TestDraftValidateDerivedTemporalModeMismatch(t *testing.T) {
	fixture := newBaseValueFixture(t, schema.DerivedComputation(ast.NewConstant(value.NewTyped(value.NewInt64(1)))), schema.TemporalSeries, identity.UUID{}, false, value.Integer, unit.Unit{}, false)
	requireValidationIssue(t, fixture.draft.Validate(), ValidationIssueInvalidDerivedExpression)
}
