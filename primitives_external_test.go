package bcg_test

import (
	"errors"
	"testing"

	bcg "github.com/shmatovv/bcg-engine"
)

func TestPublicScalarValuesAndValueTypes(t *testing.T) {
	date, err := bcg.ParseDate("2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	dateTime, err := bcg.ParseDateTime("2026-09-12T10:11:12.123456789+03:00")
	if err != nil {
		t.Fatal(err)
	}
	duration, err := bcg.ParseDuration("-PT1H2M3.004005006S")
	if err != nil {
		t.Fatal(err)
	}
	decimal := bcg.MustParseDecimal("123456789012345678901234567890.12345678901234567890")
	integer, err := bcg.Integer("1234567890123456789012345678901234567890")
	if err != nil {
		t.Fatal(err)
	}
	dateValue, err := bcg.DateValue(date)
	if err != nil {
		t.Fatal(err)
	}
	dateTimeValue, err := bcg.DateTimeValue(dateTime)
	if err != nil {
		t.Fatal(err)
	}

	values := []struct {
		name string
		got  bcg.Value
		want bcg.ValueType
	}{
		{"Bool", bcg.Bool(true), bcg.BoolType},
		{"Integer", integer, bcg.IntegerType},
		{"Int64", bcg.Int64(-42), bcg.IntegerType},
		{"Decimal", bcg.DecimalValue(decimal), bcg.DecimalType},
		{"String", bcg.String("public primitive"), bcg.StringType},
		{"Date", dateValue, bcg.DateType},
		{"DateTime", dateTimeValue, bcg.DateTimeType},
		{"Duration", bcg.DurationValue(duration), bcg.DurationType},
	}
	for _, test := range values {
		t.Run(test.name, func(t *testing.T) {
			if !test.got.IsValid() || test.got.Type() != test.want {
				t.Fatalf("Value = %#v, type = %s, want %s", test.got, test.got.Type(), test.want)
			}
			if !test.got.Type().IsValid() || string(test.got.Kind()) != test.want.String() {
				t.Fatalf("Value type/kind = %s/%s", test.got.Type(), test.got.Kind())
			}
		})
	}

	stored, ok := bcg.DecimalValue(decimal).Decimal()
	if !ok || stored.String() != decimal.String() || !stored.Equal(decimal) {
		t.Fatalf("Decimal round trip = %s, %v", stored, ok)
	}
	if got, ok := integer.IntegerString(); !ok || got != "1234567890123456789012345678901234567890" {
		t.Fatalf("Integer round trip = %q, %v", got, ok)
	}
	if dateTime.String() != "2026-09-12T07:11:12.123456789Z" || duration.String() != "-PT1H2M3.004005006S" {
		t.Fatalf("canonical temporal values = %s / %s", dateTime, duration)
	}

	for _, valueType := range []bcg.ValueType{
		bcg.BoolType,
		bcg.IntegerType,
		bcg.DecimalType,
		bcg.StringType,
		bcg.DateType,
		bcg.DateTimeType,
		bcg.DurationType,
	} {
		if !valueType.IsValid() || valueType.String() == "Invalid" {
			t.Fatalf("invalid public ValueType %d", valueType)
		}
	}
	if (bcg.ValueType(0)).IsValid() {
		t.Fatal("zero ValueType is valid")
	}
}

func TestPublicDimensionAndUnitAuthoring(t *testing.T) {
	dimensionless := bcg.DimensionlessDimension()
	if !dimensionless.IsValid() || !dimensionless.IsDimensionless() || !dimensionless.Equal(bcg.Dimension{}) {
		t.Fatal("dimensionless Dimension is not a valid canonical identity")
	}

	length, err := bcg.NewBaseDimension("length")
	if err != nil {
		t.Fatal(err)
	}
	composed, err := bcg.NewDimension([]bcg.DimensionFactor{
		{ID: "time", Exponent: -1},
		{ID: "length", Exponent: 1},
		{ID: "count", Exponent: 2},
		{ID: "count", Exponent: -1},
		{ID: "unused", Exponent: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !composed.IsValid() || composed.IsDimensionless() {
		t.Fatalf("composed Dimension = %#v", composed)
	}
	factors := composed.Factors()
	if len(factors) != 3 || factors[0].ID != "count" || factors[0].Exponent != 1 ||
		factors[1].ID != "length" || factors[1].Exponent != 1 ||
		factors[2].ID != "time" || factors[2].Exponent != -1 {
		t.Fatalf("canonical Dimension factors = %#v", factors)
	}
	same, err := bcg.NewDimension([]bcg.DimensionFactor{
		{ID: "length", Exponent: 1},
		{ID: "time", Exponent: -1},
		{ID: "count", Exponent: 1},
	})
	if err != nil || !same.Equal(composed) {
		t.Fatalf("Dimension canonical equality = %#v, %v", same, err)
	}
	if _, err := bcg.NewBaseDimension("Length"); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("invalid Dimension ID = %T %[1]v", err)
	}

	one := bcg.OneRational()
	centiScale, err := bcg.NewRational("100", "10000")
	if err != nil {
		t.Fatal(err)
	}
	if centiScale.String() != "1/100" || !centiScale.Equal(mustRational(t, "1", "100")) {
		t.Fatalf("canonical exact rational = %s", centiScale)
	}
	if _, err := bcg.NewRational("1", "0"); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("zero rational denominator = %T %[1]v", err)
	}
	if _, err := bcg.NewRational("", "1"); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("empty rational numerator = %T %[1]v", err)
	}

	meter, err := bcg.NewUnit("meter", length, "length", one)
	if err != nil {
		t.Fatal(err)
	}
	centimeter, err := bcg.NewUnit("centimeter", length, "length", centiScale)
	if err != nil {
		t.Fatal(err)
	}
	if !meter.IsValid() || !meter.Dimension().Equal(length) || meter.Scale().String() != "1/1" || !meter.CanConvertTo(centimeter) {
		t.Fatalf("meter = %#v", meter)
	}
	if !centimeter.Equal(centimeter) || centimeter.Equal(meter) {
		t.Fatal("Unit equality is not exact")
	}
	unitFactors := meter.Factors()
	unitFactors[0] = bcg.UnitFactor{}
	if len(meter.Factors()) != 1 || meter.Factors()[0].ID != "meter" {
		t.Fatal("Unit Factors exposed mutable storage")
	}

	count, err := bcg.NewBaseDimension("count")
	if err != nil {
		t.Fatal(err)
	}
	currency, err := bcg.NewBaseDimension("currency")
	if err != nil {
		t.Fatal(err)
	}
	currencyPerCount, err := bcg.NewDimension([]bcg.DimensionFactor{
		{ID: "currency", Exponent: 1},
		{ID: "count", Exponent: -1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !currencyPerCount.IsValid() || currency.Equal(count) {
		t.Fatal("custom dimensions were not represented independently")
	}
	composedUnit, err := bcg.NewComposedUnit(
		currencyPerCount,
		[]bcg.UnitFactor{{ID: "item", Exponent: -1}, {ID: "rub", Exponent: 1}},
		[]bcg.ConversionFactor{{ID: "count", Exponent: -1}, {ID: "rub", Exponent: 1}},
		one,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !composedUnit.IsValid() || !composedUnit.Dimension().Equal(currencyPerCount) ||
		len(composedUnit.ConversionFactors()) != 2 {
		t.Fatalf("composed Unit = %#v", composedUnit)
	}
	if _, err := bcg.NewComposedUnit(
		currencyPerCount,
		[]bcg.UnitFactor{{ID: "rub", Exponent: 1}, {ID: "item", Exponent: -1}},
		[]bcg.ConversionFactor{{ID: "count", Exponent: -1}, {ID: "rub", Exponent: 1}},
		one,
	); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("noncanonical composed Unit accepted: %T %[1]v", err)
	}
	if _, err := bcg.NewUnit("Meter", length, "length", one); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("invalid Unit ID = %T %[1]v", err)
	}
	if _, err := bcg.NewUnit("meter", length, "length", bcg.Rational{}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("zero Unit scale = %T %[1]v", err)
	}

	dimensionlessUnit := bcg.DimensionlessUnit()
	if !dimensionlessUnit.IsValid() || !dimensionlessUnit.IsDimensionless() ||
		!dimensionlessUnit.Dimension().Equal(dimensionless) || dimensionlessUnit.Scale().String() != "1/1" {
		t.Fatalf("dimensionless Unit = %#v", dimensionlessUnit)
	}
	withUnit, err := bcg.DecimalValue(bcg.MustParseDecimal("12.5")).WithUnit(centimeter)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := withUnit.Unit(); !ok || !got.Equal(centimeter) {
		t.Fatalf("Value.WithUnit = %#v, %v", got, ok)
	}
}

func TestPublicTemporalSeriesAndPrecisionPrimitives(t *testing.T) {
	if !bcg.TemporalScalar.IsValid() || bcg.TemporalScalar.String() != "Scalar" ||
		!bcg.TemporalSeries.IsValid() || bcg.TemporalSeries.String() != "Series" {
		t.Fatal("public temporal modes are incomplete")
	}
	if (bcg.TemporalMode(0)).IsValid() {
		t.Fatal("zero TemporalMode is valid")
	}
	if !bcg.PointSeries.IsValid() || !bcg.IntervalSeries.IsValid() || (bcg.SeriesAxis("")).IsValid() {
		t.Fatal("public point/interval Series primitives are invalid")
	}

	for _, mode := range []bcg.RoundingMode{
		bcg.RoundTruncate,
		bcg.RoundFloor,
		bcg.RoundCeil,
		bcg.RoundHalfUp,
		bcg.RoundHalfEven,
	} {
		policy, err := bcg.NewPrecisionPolicy(12, mode)
		if err != nil || !policy.IsValid() || policy.Rounding() != mode || policy.Scale() != 12 {
			t.Fatalf("PrecisionPolicy(%s) = %#v, %v", mode, policy, err)
		}
		logical, err := bcg.NewModel(bcg.NewID(), "precision-mapping")
		if err != nil {
			t.Fatal(err)
		}
		draft, err := logical.Draft()
		if err != nil {
			t.Fatal(err)
		}
		options, err := bcg.NewCommitOptions(bcg.NewID(), "v1", policy)
		if err != nil {
			t.Fatal(err)
		}
		committed, err := draft.Commit(options)
		if err != nil {
			t.Fatal(err)
		}
		stored, ok := committed.PrecisionPolicy()
		if !ok || stored.Scale() != 12 || stored.Rounding() != mode {
			t.Fatalf("committed PrecisionPolicy(%s) = %#v, %v", mode, stored, ok)
		}
	}
	if _, err := bcg.NewPrecisionPolicy(bcg.MaxPrecisionScale+1, bcg.RoundHalfEven); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("out-of-range PrecisionPolicy = %T %[1]v", err)
	}
}

func TestPublicUnitFacadeReconstructsDecodedV1Units(t *testing.T) {
	artifact, err := bcg.Unmarshal([]byte(publicAggregateArtifact))
	if err != nil {
		t.Fatal(err)
	}
	reconstructed := 0
	for _, property := range artifact.Model().PropertyDefinitions() {
		original, ok := property.Unit()
		if !ok {
			continue
		}
		dimension, err := bcg.NewDimension(original.Dimension().Factors())
		if err != nil {
			t.Fatalf("reconstruct Dimension for %s: %v", property.Name(), err)
		}
		copy, err := bcg.NewComposedUnit(
			dimension,
			original.Factors(),
			original.ConversionFactors(),
			original.Scale(),
		)
		if err != nil {
			t.Fatalf("reconstruct Unit for %s: %v", property.Name(), err)
		}
		if !copy.Equal(original) {
			t.Fatalf("reconstructed Unit for %s differs", property.Name())
		}
		reconstructed++
	}
	if reconstructed == 0 {
		t.Fatal("fixture contained no Units to reconstruct")
	}
}

func mustRational(t *testing.T, numerator, denominator string) bcg.Rational {
	t.Helper()
	result, err := bcg.NewRational(numerator, denominator)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
