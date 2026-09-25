package bcg

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/v2/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/impact"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/date"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/v2/internal/types/duration"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
	"github.com/shmatovv/bcg-engine/v2/internal/types/rational"
	"github.com/shmatovv/bcg-engine/v2/internal/types/series"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func TestMinimalRoundTripIsCanonical(t *testing.T) {
	version := committedEmptyVersion(t)
	artifact, err := NewArtifact("portable model", version, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Unmarshal(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("canonical round trip differs\nfirst:  %s\nsecond: %s", first, second)
	}
	if decoded.ModelVersion().ID() != version.ID() || decoded.ModelVersion().ModelID() != version.ModelID() {
		t.Fatal("model identities changed during round trip")
	}
	var stream bytes.Buffer
	if err := Encode(&stream, artifact); err != nil {
		t.Fatal(err)
	}
	streamDecoded, err := Decode(&stream)
	if err != nil {
		t.Fatal(err)
	}
	streamBytes, err := Marshal(streamDecoded)
	if err != nil || !bytes.Equal(first, streamBytes) {
		t.Fatalf("stream codec changed canonical bytes: %v", err)
	}
}

func TestOptionalParentVersionIdentityRoundTrips(t *testing.T) {
	modelID := identity.New()
	versionID := identity.New()
	parentID := identity.New()
	options := model.NewCommitOptions(versionID, "child", precision.New(2, precision.HalfUp)).WithParent(parentID)
	version, err := model.Commit(model.NewModel(modelID, "parented"), model.NewDraft(), options)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := NewArtifact("parented", version, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Unmarshal(mustMarshal(t, artifact))
	if err != nil {
		t.Fatal(err)
	}
	parent, ok := decoded.ModelVersion().ParentVersionID()
	if !ok || parent != parentID || !decoded.ModelVersion().PrecisionPolicy().Equal(version.PrecisionPolicy()) {
		t.Fatal("parent identity or precision policy changed")
	}
}

func TestRejectsUnsupportedVersionDuplicateKeysAndTrailingData(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"formatVersion":999,"engineCompatibility":"bcg-engine-v1","artifact":{}}`),
		[]byte(`{"formatVersion":1,"formatVersion":999,"engineCompatibility":"bcg-engine-v1","artifact":{}}`),
		[]byte(`{"formatVersion":1,"engineCompatibility":"bcg-engine-v1","artifact":{}} {}`),
	} {
		if _, err := Unmarshal(data); err == nil {
			t.Fatalf("invalid document accepted: %s", data)
		}
	}
	_, err := Unmarshal([]byte(`{"formatVersion":999,"engineCompatibility":"bcg-engine-v1","artifact":{}}`))
	if !errors.Is(err, ErrUnsupportedFormatVersion) {
		t.Fatalf("unsupported version error = %v", err)
	}
}

func FuzzDecodeNeverPanics(f *testing.F) {
	version := committedEmptyVersion(f)
	artifact, err := NewArtifact("fuzz", version, nil)
	if err != nil {
		f.Fatal(err)
	}
	seed, err := Marshal(artifact)
	if err != nil {
		f.Fatal(err)
	}
	complex, _, _ := complexArtifact(f)
	complexSeed, err := Marshal(complex)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add(complexSeed)
	f.Add([]byte{})
	f.Add([]byte(`{"formatVersion":999}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := Unmarshal(data)
		if err == nil {
			if _, err := Marshal(decoded); err != nil {
				t.Fatalf("successful decode did not re-marshal: %v", err)
			}
		}
	})
}

func TestWireExpressionDepthMatchesSharedASTLimit(t *testing.T) {
	encoded := &wireExpression{
		Kind:  "constant",
		Value: &wireTyped{Type: "integer", Data: "1"},
	}
	for index := 1; index < ast.MaxExpressionDepth; index++ {
		encoded = &wireExpression{Kind: "negate", Operand: encoded}
	}

	decoded, err := encoded.toDomain("expression", 0)
	if err != nil {
		t.Fatalf("maximum supported expression depth rejected: %v", err)
	}
	if depth, valid := ast.Depth(decoded); !valid || depth != ast.MaxExpressionDepth {
		t.Fatalf("decoded expression depth = %d, valid=%t", depth, valid)
	}

	tooDeep := &wireExpression{Kind: "negate", Operand: encoded}
	if _, err := tooDeep.toDomain("expression", 0); err == nil {
		t.Fatal("expression above the shared depth limit was accepted")
	}
}

func TestMaximumDepthExpressionsRoundTripInArtifact(t *testing.T) {
	var expression ast.Expression = ast.NewConstant(value.NewTyped(value.NewBool(true)))
	for depth := 1; depth < ast.MaxExpressionDepth; depth++ {
		expression = ast.NewNot(expression)
	}

	typeID, propertyID, entityID, relationTypeID := identity.New(), identity.New(), identity.New(), identity.New()
	draft := model.NewDraft()
	draft.AddEntityType(schema.NewEntityType(typeID, "node", nil, []identity.UUID{propertyID}))
	draft.AddPropertyDefinition(schema.NewPropertyDefinition(
		propertyID, schema.EntityTypeOwner(typeID), identity.UUID{}, false,
		"flag", value.Bool, unit.Unit{}, false, schema.TemporalScalar, schema.DerivedComputation(expression),
	))
	draft.AddEntity(semantic.NewEntity(entityID, typeID, "", false))
	draft.AddRelationType(schema.NewRelationType(relationTypeID, "link", schema.TypeConstraint{}, false, schema.TypeConstraint{}, false, nil))
	draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, entityID, entityID, expression, true))
	draft.AddConstraint(constraint.New(identity.New(), "check", expression, true, expression, constraint.SeverityHard, "", false))
	draft.AddRule(rule.New(identity.New(), "rule", expression))
	version, err := model.Commit(model.NewModel(identity.New(), "depth"), draft,
		model.NewCommitOptions(identity.New(), "depth", precision.New(2, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := NewArtifact("depth", version, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded := mustMarshal(t, artifact)
	decoded, err := Unmarshal(encoded)
	if err != nil {
		t.Fatalf("committed maximum-depth expressions must remain portable: %v", err)
	}
	if !bytes.Equal(encoded, mustMarshal(t, decoded)) {
		t.Fatal("maximum-depth artifact changed during canonical round trip")
	}
}

func TestExpressionCodecCoversEveryCurrentASTNode(t *testing.T) {
	definitionID := identity.MustParse("00000000-0000-4000-8000-000000000011")
	contractID := identity.MustParse("00000000-0000-4000-8000-000000000012")
	relationTypeID := identity.MustParse("00000000-0000-4000-8000-000000000013")
	entityID := identity.MustParse("00000000-0000-4000-8000-000000000014")
	relationID := identity.MustParse("00000000-0000-4000-8000-000000000015")
	boolean := ast.NewConstant(value.NewTyped(value.NewBool(true)))
	integer := ast.NewConstant(value.NewTyped(value.NewInt64(7)))

	sources := []ast.TraversalSource{
		ast.NewTraversalSource(ast.TraversalDirectionOutgoing, relationTypeID),
		ast.NewTraversalSourceFromEntity(address.EntityRef(entityID), ast.TraversalDirectionIncoming, relationTypeID),
		ast.NewTraversalSourceFromBoundEntity(ast.TraversalDirectionBoth, relationTypeID),
	}
	expressions := []ast.Expression{
		boolean,
		ast.NewArithmetic(ast.ArithmeticAdd, integer, integer),
		ast.NewArithmetic(ast.ArithmeticSubtract, integer, integer),
		ast.NewArithmetic(ast.ArithmeticMultiply, integer, integer),
		ast.NewArithmetic(ast.ArithmeticDivide, integer, integer),
		ast.NewNegate(integer),
		ast.NewComparison(ast.ComparisonEqual, integer, integer),
		ast.NewComparison(ast.ComparisonNotEqual, integer, integer),
		ast.NewComparison(ast.ComparisonLess, integer, integer),
		ast.NewComparison(ast.ComparisonLessOrEqual, integer, integer),
		ast.NewComparison(ast.ComparisonGreater, integer, integer),
		ast.NewComparison(ast.ComparisonGreaterOrEqual, integer, integer),
		ast.NewLogical(ast.LogicalAnd, boolean, boolean),
		ast.NewLogical(ast.LogicalOr, boolean, boolean),
		ast.NewNot(boolean),
		ast.NewIf(boolean, integer, integer),
		ast.NewIsNull(integer),
		ast.NewCoalesce(integer, integer),
		ast.NewSelfPropertyReference(definitionID),
		ast.NewAbsolutePropertyReference(address.NewPropertyAddress(address.EntityRef(entityID), definitionID)),
		ast.NewAbsolutePropertyReference(address.NewPropertyAddress(address.RelationRef(relationID), definitionID)),
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, definitionID),
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetRelation, definitionID),
		ast.NewBoundContractPropertyReference(ast.BoundTargetEntity, contractID),
		ast.NewBoundContractPropertyReference(ast.BoundTargetRelation, contractID),
	}
	for _, source := range sources {
		expressions = append(expressions,
			ast.NewCountAggregate(source),
			ast.NewFilteredCountAggregate(source, boolean),
			ast.NewMappedAggregate(source, integer, ast.AggregateOperationSum),
			ast.NewFilteredMappedAggregate(source, boolean, integer, ast.AggregateOperationAverage),
			ast.NewMappedAggregate(source, integer, ast.AggregateOperationMin),
			ast.NewMappedAggregate(source, integer, ast.AggregateOperationMax),
		)
	}
	for index, expression := range expressions {
		encoded, err := wireExpressionFromDomain(expression)
		if err != nil {
			t.Fatalf("expression %d encode: %v", index, err)
		}
		decoded, err := encoded.toDomain("expression", 0)
		if err != nil {
			t.Fatalf("expression %d decode: %v", index, err)
		}
		reencoded, err := wireExpressionFromDomain(decoded)
		if err != nil {
			t.Fatalf("expression %d re-encode: %v", index, err)
		}
		left, _ := json.Marshal(encoded)
		right, _ := json.Marshal(reencoded)
		if !bytes.Equal(left, right) {
			t.Fatalf("expression %d changed\n%s\n%s", index, left, right)
		}
	}
}

func TestTypedValueCodecIsLosslessForEveryValueTypeAndComposedUnit(t *testing.T) {
	dateValue, _ := date.Parse("2026-02-03")
	dateTimeValue, _ := datetime.Parse("2026-02-03T04:05:06.007008009Z")
	base := unit.NewUnit("meter", unit.Base("length"), "length", rational.One())
	second := unit.NewUnit("second", unit.Base("time"), "time", rational.One())
	compound, err := base.Div(second)
	if err != nil {
		t.Fatal(err)
	}
	values := []value.Typed{
		value.NewTyped(value.NewBool(true)),
		value.NewTyped(value.NewInteger(new(big.Int).SetUint64(^uint64(0)))),
		value.NewTypedWithUnit(value.NewDecimal(decimal.MustParse("12345678901234567890.000001")), compound),
		value.NewTyped(value.NewDecimal(decimal.MustParse("0." + strings.Repeat("0", int(decimal.MaxScale-1)) + "1"))),
		value.NewTyped(value.NewString("Русский 日本語 emoji 😀")),
		value.NewTyped(value.NewDate(dateValue)),
		value.NewTyped(value.NewDateTime(dateTimeValue)),
		value.NewTyped(value.NewDuration(duration.FromNanoseconds(big.NewInt(-1234567890123456789)))),
	}
	for index, original := range values {
		encoded, err := wireTypedFromDomain(original)
		if err != nil {
			t.Fatalf("value %d encode: %v", index, err)
		}
		decoded, err := encoded.toDomain("typed")
		if err != nil {
			t.Fatalf("value %d decode: %v", index, err)
		}
		if !original.Equal(decoded) {
			t.Fatalf("value %d changed", index)
		}
	}
	from, _ := datetime.Parse("2026-03-01T00:00:00Z")
	to, _ := datetime.Parse("2026-04-01T00:00:00Z")
	originalSeries := series.NewIntervalSeries(value.Integer, unit.Unit{}, false, []series.Interval{series.NewInterval(from, to, series.Defined(value.NewInt64(9)))})
	encodedSeries, err := wireSeriesFromDomain(originalSeries)
	if err != nil {
		t.Fatal(err)
	}
	decodedSeries, err := encodedSeries.toDomain("series")
	if err != nil || !originalSeries.Equal(decodedSeries) {
		t.Fatalf("interval series changed: %v", err)
	}
}

func TestDecodeIsStrictAndUsesNormalModelValidation(t *testing.T) {
	artifact, _, _ := complexArtifact(t)
	data, err := Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	unknownNested := []byte(strings.Replace(string(data), `"label":"complex-v1"`, `"label":"complex-v1","unexpected":true`, 1))
	if _, err := Unmarshal(unknownNested); err == nil || !errors.Is(err, ErrMalformedDocument) {
		t.Fatalf("nested unknown field error = %v", err)
	}
	document, err := newWireDocument(artifact)
	if err != nil {
		t.Fatal(err)
	}
	document.Artifact.Version.Relations[0].To = "00000000-0000-4000-8000-00000000ffff"
	corrupt, err := marshalCanonical(document)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Unmarshal(corrupt); err == nil || !errors.Is(err, ErrModelValidation) {
		t.Fatalf("corrupt relation did not reach model validation: %v", err)
	} else {
		var report model.ValidationReport
		if !errors.As(err, &report) || report.OK() {
			t.Fatalf("structured model validation report was not preserved: %v", err)
		}
	}
}

func TestMarshalCanonicalizesNonCanonicalCollectionOrder(t *testing.T) {
	artifact, _, _ := complexArtifact(t)
	canonical := mustMarshal(t, artifact)
	document, err := newWireDocument(artifact)
	if err != nil {
		t.Fatal(err)
	}
	reverse(document.Artifact.Version.Entities)
	reverse(document.Artifact.Version.Relations)
	reverse(document.Artifact.Version.Bindings)
	nonCanonical, err := marshalCanonical(document)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Unmarshal(nonCanonical)
	if err != nil {
		t.Fatal(err)
	}
	if normalized := mustMarshal(t, decoded); !bytes.Equal(canonical, normalized) {
		t.Fatal("collection input order leaked into canonical encoding")
	}
}

func TestComplexArtifactRoundTripPreservesEvaluationRulesScenariosAndSeries(t *testing.T) {
	artifact, total, context := complexArtifact(t)
	original, err := evaluator.Evaluate(artifact.ModelVersion(), total, context)
	if err != nil {
		t.Fatal(err)
	}
	originalRule, err := eval.One(artifact.ModelVersion(), artifact.ModelVersion().Rules()[0].ID(), context)
	if err != nil {
		t.Fatal(err)
	}
	warmSession, err := evaluator.NewSession(artifact.ModelVersion())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := warmSession.Evaluate(total, context); err != nil || warmSession.Stats().Entries() == 0 {
		t.Fatalf("failed to warm source session: %v", err)
	}
	bytesBefore, err := Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Unmarshal(bytesBefore)
	if err != nil {
		t.Fatal(err)
	}
	bytesAfter, err := Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytesBefore, bytesAfter) {
		t.Fatal("complex artifact was not canonical after round trip")
	}
	decodedResult, err := evaluator.Evaluate(decoded.ModelVersion(), total, context)
	if err != nil {
		t.Fatal(err)
	}
	if !original.Value().Equal(decodedResult.Value()) {
		t.Fatalf("evaluation changed: %#v != %#v", original.Value(), decodedResult.Value())
	}
	boundary, _ := datetime.Parse("2026-01-02T00:00:00Z")
	boundaryContext := context.WithTimeContext(domaintime.At(boundary))
	originalBoundary, err := evaluator.Evaluate(artifact.ModelVersion(), total, boundaryContext)
	if err != nil {
		t.Fatal(err)
	}
	decodedBoundary, err := evaluator.Evaluate(decoded.ModelVersion(), total, boundaryContext)
	if err != nil || !originalBoundary.Value().Equal(decodedBoundary.Value()) || originalBoundary.Value().Equal(original.Value()) {
		t.Fatalf("effective-time boundary semantics changed: %v", err)
	}
	selector := context.ScopeValues().Selectors()[0]
	otherScope := scope.NewValues([]scope.Selector{scope.NewSelector(selector.DefinitionID(), value.NewTyped(value.NewString("south")))})
	otherScopeContext := runtime.NewCalculationContext(context.TimeContext(), otherScope, nil)
	originalOtherScope, err := evaluator.Evaluate(artifact.ModelVersion(), total, otherScopeContext)
	if err != nil {
		t.Fatal(err)
	}
	decodedOtherScope, err := evaluator.Evaluate(decoded.ModelVersion(), total, otherScopeContext)
	if err != nil || !originalOtherScope.Value().Equal(decodedOtherScope.Value()) || originalOtherScope.Value().Equal(original.Value()) {
		t.Fatalf("scope semantics changed: %v", err)
	}
	seriesAddress := propertyAddressByNames(t, artifact.ModelVersion(), "root", "history")
	originalSeries, err := evaluator.EvaluateSeries(artifact.ModelVersion(), seriesAddress, context)
	if err != nil {
		t.Fatal(err)
	}
	decodedSeries, err := evaluator.EvaluateSeries(decoded.ModelVersion(), seriesAddress, context)
	if err != nil || !reflect.DeepEqual(originalSeries, decodedSeries) {
		t.Fatalf("series evaluation changed: %v", err)
	}
	originalExplain, err := evaluator.Explain(artifact.ModelVersion(), total, context)
	if err != nil {
		t.Fatal(err)
	}
	decodedExplain, err := evaluator.Explain(decoded.ModelVersion(), total, context)
	if err != nil || !reflect.DeepEqual(originalExplain.Root(), decodedExplain.Root()) {
		t.Fatalf("explain provenance changed: %v", err)
	}
	originalChecks, err := check.All(artifact.ModelVersion(), context)
	if err != nil {
		t.Fatal(err)
	}
	decodedChecks, err := check.All(decoded.ModelVersion(), context)
	if err != nil || !reflect.DeepEqual(originalChecks, decodedChecks) {
		t.Fatalf("constraint report changed: %v", err)
	}
	originalFeasibility, err := feasibility.Evaluate(artifact.ModelVersion(), context)
	if err != nil {
		t.Fatal(err)
	}
	decodedFeasibility, err := feasibility.Evaluate(decoded.ModelVersion(), context)
	if err != nil || !reflect.DeepEqual(originalFeasibility, decodedFeasibility) {
		t.Fatalf("feasibility changed: %v", err)
	}
	priceSource := propertyAddressByNames(t, artifact.ModelVersion(), "part one", "price")
	for _, analyze := range []func(model.ModelVersion, address.PropertyAddress, runtime.CalculationContext) (impact.Result, error){
		func(version model.ModelVersion, source address.PropertyAddress, _ runtime.CalculationContext) (impact.Result, error) {
			return impact.AnalyzeStructural(version, source)
		},
		impact.AnalyzeEffective,
		impact.AnalyzeObserved,
	} {
		originalImpact, err := analyze(artifact.ModelVersion(), priceSource, context)
		if err != nil {
			t.Fatal(err)
		}
		decodedImpact, err := analyze(decoded.ModelVersion(), priceSource, context)
		if err != nil || !reflect.DeepEqual(originalImpact.Affected(), decodedImpact.Affected()) || !reflect.DeepEqual(originalImpact.Edges(), decodedImpact.Edges()) {
			t.Fatalf("impact changed: %v", err)
		}
	}
	decodedRule, err := eval.One(decoded.ModelVersion(), decoded.ModelVersion().Rules()[0].ID(), context)
	if err != nil {
		t.Fatal(err)
	}
	if originalRule.State() != decodedRule.State() || !originalRule.Condition().Value().Equal(decodedRule.Condition().Value()) {
		t.Fatal("rule evaluation changed")
	}
	if len(decoded.Scenarios()) != 1 {
		t.Fatal("scenario was not restored")
	}
	originalScenario, err := evaluator.Evaluate(artifact.ModelVersion(), total, context.WithScenario(artifact.Scenarios()[0]))
	if err != nil {
		t.Fatal(err)
	}
	decodedScenario, err := evaluator.Evaluate(decoded.ModelVersion(), total, context.WithScenario(decoded.Scenarios()[0]))
	if err != nil {
		t.Fatal(err)
	}
	if !originalScenario.Value().Equal(decodedScenario.Value()) || originalScenario.Value().Equal(original.Value()) {
		t.Fatal("scenario override semantics changed during round trip")
	}
	session, err := evaluator.NewSession(decoded.ModelVersion())
	if err != nil {
		t.Fatalf("decoded model cannot start a clean session: %v", err)
	}
	if stats := session.Stats(); stats.Entries() != 0 || stats.Hits() != 0 || stats.Misses() != 0 {
		t.Fatal("runtime cache state leaked into portable artifact")
	}
	totalDefinition, _ := decoded.ModelVersion().PropertyDefinition(total.DefinitionID())
	totalUnit, _ := totalDefinition.Unit()
	overriddenContext := runtime.NewCalculationContext(
		context.TimeContext(), context.ScopeValues(), []runtime.Override{runtime.NewOverride(
			total,
			semantic.DefinedBindingValue(value.NewTypedWithUnit(value.NewDecimal(decimal.MustParse("999")), totalUnit)),
		)},
	)
	overridden, err := session.Evaluate(total, overriddenContext)
	if err != nil || !overridden.Value().IsDefined() || !overridden.Value().Value().Raw().Decimal().Equal(decimal.MustParse("999")) {
		t.Fatalf("decoded session override transition failed: %v", err)
	}
	reconnected, err := session.Evaluate(total, context)
	if err != nil || !reconnected.Value().Equal(decodedResult.Value()) {
		t.Fatalf("decoded session override removal failed: %v", err)
	}
}

func complexArtifact(t testing.TB) (Artifact, address.PropertyAddress, runtime.CalculationContext) {
	t.Helper()
	ids := func() identity.UUID { return identity.New() }
	modelID, versionID := ids(), ids()
	rootTypeID, partTypeID, relationTypeID := ids(), ids(), ids()
	scopeID := ids()
	priceContractID, seriesContractID, nullableContractID, pricedTraitID := ids(), ids(), ids(), ids()
	priceID, quantityID, totalID, seriesID, nullableID := ids(), ids(), ids(), ids(), ids()
	rootID, partOneID, partTwoID, relationOneID, relationTwoID := ids(), ids(), ids(), ids(), ids()
	constraintID, ruleID, scenarioID := ids(), ids(), ids()

	currency := unit.Base("currency")
	count := unit.Base("count")
	rub := unit.NewUnit("rub", currency, "rub", rational.One())
	item := unit.NewUnit("item", count, "count", rational.One())
	rubPerItem, err := rub.Div(item)
	if err != nil {
		t.Fatal(err)
	}
	priceContract := schema.NewPropertyContract(priceContractID, "price", value.Decimal, currency.Div(count), true, schema.TemporalScalar, false)
	seriesContract := schema.NewPropertyContract(seriesContractID, "history", value.Integer, unit.Dimension{}, false, schema.TemporalSeries, true)
	nullableContract := schema.NewPropertyContract(nullableContractID, "comment", value.String, unit.Dimension{}, false, schema.TemporalScalar, true)
	priced := schema.NewTrait(pricedTraitID, "priced", []identity.UUID{priceContractID})
	price := schema.NewPropertyDefinition(priceID, schema.EntityTypeOwner(partTypeID), priceContractID, true, "price", value.Decimal, rubPerItem, true, schema.TemporalScalar, schema.InputComputation())
	quantity := schema.NewPropertyDefinition(quantityID, schema.RelationTypeOwner(relationTypeID), identity.UUID{}, false, "quantity", value.Integer, item, true, schema.TemporalScalar, schema.InputComputation())
	source := ast.NewTraversalSource(ast.TraversalDirectionOutgoing, relationTypeID)
	totalExpression := ast.NewFilteredMappedAggregate(source,
		ast.NewComparison(ast.ComparisonGreater, ast.NewBoundPropertyDefinitionReference(ast.BoundTargetRelation, quantityID), ast.NewConstant(value.NewTypedWithUnit(value.NewInt64(0), item))),
		ast.NewArithmetic(ast.ArithmeticMultiply, ast.NewBoundContractPropertyReference(ast.BoundTargetEntity, priceContractID), ast.NewBoundPropertyDefinitionReference(ast.BoundTargetRelation, quantityID)),
		ast.AggregateOperationSum)
	total := schema.NewPropertyDefinition(totalID, schema.EntityTypeOwner(rootTypeID), identity.UUID{}, false, "total", value.Decimal, rub, true, schema.TemporalScalar, schema.DerivedComputation(totalExpression))
	seriesDefinition := schema.NewPropertyDefinition(seriesID, schema.EntityTypeOwner(rootTypeID), seriesContractID, true, "history", value.Integer, unit.Unit{}, false, schema.TemporalSeries, schema.InputComputation())
	nullableDefinition := schema.NewPropertyDefinition(nullableID, schema.EntityTypeOwner(rootTypeID), nullableContractID, true, "comment", value.String, unit.Unit{}, false, schema.TemporalScalar, schema.InputComputation())
	rootType := schema.NewEntityType(rootTypeID, "root", nil, []identity.UUID{totalID, seriesID, nullableID})
	partType := schema.NewEntityType(partTypeID, "part", []identity.UUID{pricedTraitID}, []identity.UUID{priceID})
	relationType := schema.NewRelationType(relationTypeID, "contains", schema.EntityTypeConstraint(rootTypeID), true, schema.TraitConstraint(pricedTraitID), true, []identity.UUID{quantityID})
	root := semantic.NewEntity(rootID, rootTypeID, "root", true)
	partOne := semantic.NewEntity(partOneID, partTypeID, "part one", true)
	partTwo := semantic.NewEntity(partTwoID, partTypeID, "part two", true)
	relationOne := semantic.NewRelation(relationOneID, relationTypeID, rootID, partOneID, ast.NewConstant(value.NewTyped(value.NewBool(true))), true)
	relationTwo := semantic.NewRelation(relationTwoID, relationTypeID, rootID, partTwoID, nil, false)
	region := scope.NewSelector(scopeID, value.NewTyped(value.NewString("north")))
	scopeDefinition := scope.NewDefinition(scopeID, "region", value.String)
	pointOne, _ := datetime.Parse("2026-01-01T00:00:00Z")
	pointTwo, _ := datetime.Parse("2026-01-02T00:00:00Z")
	addressOf := func(object address.ObjectRef, definition identity.UUID) address.PropertyAddress {
		return address.NewPropertyAddress(object, definition)
	}
	priceOne := addressOf(address.EntityRef(partOneID), priceID)
	priceTwo := addressOf(address.EntityRef(partTwoID), priceID)
	quantityOne := addressOf(address.RelationRef(relationOneID), quantityID)
	quantityTwo := addressOf(address.RelationRef(relationTwoID), quantityID)
	totalAddress := addressOf(address.EntityRef(rootID), totalID)
	seriesAddress := addressOf(address.EntityRef(rootID), seriesID)
	nullableAddress := addressOf(address.EntityRef(rootID), nullableID)
	bind := func(target address.PropertyAddress, bindingValue semantic.BindingValue) semantic.ValueBinding {
		return semantic.NewValueBinding(semantic.ObjectPropertyTarget(target), bindingValue, nil, domaintime.EffectivePeriod{}, false)
	}
	constraintValue := constraint.New(constraintID, "cap", nil, false, ast.NewComparison(ast.ComparisonLessOrEqual, ast.NewAbsolutePropertyReference(totalAddress), ast.NewConstant(value.NewTypedWithUnit(value.NewDecimal(decimal.MustParse("1000")), rub))), constraint.SeverityHard, "within cap", true)
	ruleValue := rule.NewWithMetadata(ruleID, "large", ast.NewComparison(ast.ComparisonGreater, ast.NewAbsolutePropertyReference(totalAddress), ast.NewConstant(value.NewTypedWithUnit(value.NewDecimal(decimal.MustParse("300")), rub))), map[string]string{"source": "round-trip"})
	draft := model.NewDraft()
	draft.AddPropertyContract(priceContract)
	draft.AddPropertyContract(seriesContract)
	draft.AddPropertyContract(nullableContract)
	draft.AddTrait(priced)
	draft.AddPropertyDefinition(price)
	draft.AddPropertyDefinition(quantity)
	draft.AddPropertyDefinition(total)
	draft.AddPropertyDefinition(seriesDefinition)
	draft.AddPropertyDefinition(nullableDefinition)
	draft.AddScopeDefinition(scopeDefinition)
	draft.AddEntityType(rootType)
	draft.AddEntityType(partType)
	draft.AddRelationType(relationType)
	draft.AddEntity(root)
	draft.AddEntity(partOne)
	draft.AddEntity(partTwo)
	draft.AddRelation(relationOne)
	draft.AddRelation(relationTwo)
	draft.AddValueBinding(semantic.NewValueBinding(
		semantic.TypePropertyTarget(schema.EntityTypeOwner(partTypeID), priceID),
		semantic.DefinedBindingValue(value.NewTypedWithUnit(value.NewDecimal(decimal.MustParse("1")), rubPerItem)),
		nil, domaintime.EffectivePeriod{}, false,
	))
	draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(priceOne),
		semantic.DefinedBindingValue(value.NewTypedWithUnit(value.NewDecimal(decimal.MustParse("100")), rubPerItem)),
		[]scope.Selector{region}, domaintime.NewEffectivePeriod(pointOne, true, pointTwo, true), true,
	))
	draft.AddValueBinding(bind(priceTwo, semantic.DefinedBindingValue(value.NewTypedWithUnit(value.NewDecimal(decimal.MustParse("50")), rubPerItem))))
	draft.AddValueBinding(bind(quantityOne, semantic.DefinedBindingValue(value.NewTypedWithUnit(value.NewInt64(2), item))))
	draft.AddValueBinding(bind(quantityTwo, semantic.DefinedBindingValue(value.NewTypedWithUnit(value.NewInt64(3), item))))
	draft.AddValueBinding(bind(seriesAddress, semantic.SeriesBindingValue(series.NewPointSeries(value.Integer, unit.Unit{}, false, []series.Point{series.NewPoint(pointOne, series.Defined(value.NewInt64(1))), series.NewPoint(pointTwo, series.Null())}))))
	draft.AddValueBinding(bind(nullableAddress, semantic.NullBindingValue()))
	draft.AddConstraint(constraintValue)
	draft.AddRule(ruleValue)
	version, err := model.Commit(model.NewModel(modelID, "complex"), draft, model.NewCommitOptions(versionID, "complex-v1", precision.New(8, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	override := scenario.NewOverride(priceOne, semantic.DefinedBindingValue(value.NewTypedWithUnit(value.NewDecimal(decimal.MustParse("200")), rubPerItem)), nil, domaintime.EffectivePeriod{}, false)
	scenarioValue := scenario.New(scenarioID, "what-if", versionID, []scenario.Override{override})
	artifact, err := NewArtifact("complex", version, []scenario.Scenario{scenarioValue})
	if err != nil {
		t.Fatal(err)
	}
	at, _ := datetime.Parse("2026-01-01T12:00:00Z")
	return artifact, totalAddress, runtime.NewCalculationContext(domaintime.At(at), scope.NewValues([]scope.Selector{region}), nil)
}

func committedEmptyVersion(t testing.TB) model.ModelVersion {
	t.Helper()
	modelID := identity.MustParse("00000000-0000-4000-8000-000000000001")
	versionID := identity.MustParse("00000000-0000-4000-8000-000000000002")
	version, err := model.Commit(model.NewModel(modelID, "fixture"), model.NewDraft(), model.NewCommitOptions(versionID, "v1", precision.New(8, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func propertyAddressByNames(t testing.TB, version model.ModelVersion, entityName, propertyName string) address.PropertyAddress {
	t.Helper()
	var entityID identity.UUID
	for _, entity := range version.Entities() {
		if name, ok := entity.Name(); ok && name == entityName {
			entityID = entity.ID()
			break
		}
	}
	if entityID.IsZero() {
		t.Fatalf("entity %q not found", entityName)
	}
	entity, _ := version.Entity(entityID)
	for _, definition := range version.PropertyDefinitions() {
		if definition.Name() == propertyName && definition.Owner() == schema.EntityTypeOwner(entity.TypeID()) {
			return address.NewPropertyAddress(address.EntityRef(entityID), definition.ID())
		}
	}
	t.Fatalf("property %q not found", propertyName)
	return address.PropertyAddress{}
}

func mustMarshal(t testing.TB, artifact Artifact) []byte {
	t.Helper()
	data, err := Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func reverse[T any](items []T) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}
