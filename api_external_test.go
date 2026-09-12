package bcg_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	bcg "github.com/shmatovv/bcg-engine"
)

// This fixture is deliberately plain v1 JSON. The external test package uses
// only the public import path and proves that consumers need no internal
// authoring or evaluator packages to load and calculate a model.
const publicArtifact = `{"formatVersion":1,"engineCompatibility":"bcg-engine-v1","artifact":{"model":{"id":"00000000-0000-4000-8000-000000000001","name":"public-fixture"},"version":{"id":"00000000-0000-4000-8000-000000000002","modelId":"00000000-0000-4000-8000-000000000001","parentVersionId":null,"label":"v1","precision":{"scale":"8","rounding":"half_even"},"entityTypes":[{"id":"00000000-0000-4000-8000-000000000003","name":"node","traits":[],"propertyDefinitions":["00000000-0000-4000-8000-000000000004"]}],"relationTypes":[],"traits":[],"propertyContracts":[],"propertyDefinitions":[{"id":"00000000-0000-4000-8000-000000000004","owner":{"kind":"entity_type","id":"00000000-0000-4000-8000-000000000003"},"contractId":null,"name":"amount","valueType":"decimal","unit":null,"temporalMode":"scalar","computation":{"kind":"input","expression":null}}],"scopeDefinitions":[],"entities":[{"id":"00000000-0000-4000-8000-000000000005","typeId":"00000000-0000-4000-8000-000000000003","name":"node-1"}],"relations":[],"valueBindings":[{"target":{"kind":"object_property","address":{"object":{"kind":"entity","id":"00000000-0000-4000-8000-000000000005"},"definitionId":"00000000-0000-4000-8000-000000000004"},"owner":null,"definitionId":null},"value":{"kind":"defined","value":{"type":"decimal","data":"10","unit":null},"series":null},"scopeSelectors":[],"effectivePeriod":null}],"constraints":[{"id":"00000000-0000-4000-8000-000000000006","name":"always","applicability":null,"assertion":{"kind":"constant","value":{"type":"bool","data":"true","unit":null}},"severity":"hard","message":"must hold"}],"rules":[{"id":"00000000-0000-4000-8000-000000000007","name":"enabled","condition":{"kind":"constant","value":{"type":"bool","data":"true","unit":null}},"metadata":[{"key":"kind","value":"test"}]}]},"scenarios":[{"id":"00000000-0000-4000-8000-000000000008","name":"raised","baseModelVersionId":"00000000-0000-4000-8000-000000000002","overrides":[{"target":{"object":{"kind":"entity","id":"00000000-0000-4000-8000-000000000005"},"definitionId":"00000000-0000-4000-8000-000000000004"},"value":{"kind":"defined","value":{"type":"decimal","data":"20","unit":null},"series":null},"scopeSelectors":[],"effectivePeriod":null}]}]}}`

// This fixture exercises public loading and execution of a self-bound relation
// aggregate: two relation instances remain distinct and contract resolution is
// polymorphic at the traversal boundary. No internal model construction is used.
const publicAggregateArtifact = `{"formatVersion":1,"engineCompatibility":"bcg-engine-v1","artifact":{"model":{"id":"00000000-0000-4000-8000-000000001001","name":"aggregate-fixture"},"version":{"id":"00000000-0000-4000-8000-000000001002","modelId":"00000000-0000-4000-8000-000000001001","parentVersionId":null,"label":"v1","precision":{"scale":"8","rounding":"half_even"},"entityTypes":[{"id":"00000000-0000-4000-8000-000000001003","name":"root","traits":[],"propertyDefinitions":["00000000-0000-4000-8000-000000001010"]},{"id":"00000000-0000-4000-8000-000000001004","name":"part","traits":["00000000-0000-4000-8000-000000001006"],"propertyDefinitions":["00000000-0000-4000-8000-000000001008"]}],"relationTypes":[{"id":"00000000-0000-4000-8000-000000001005","name":"contains","from":{"kind":"entity_type","id":"00000000-0000-4000-8000-000000001003"},"to":{"kind":"trait","id":"00000000-0000-4000-8000-000000001006"},"propertyDefinitions":["00000000-0000-4000-8000-000000001009"]}],"traits":[{"id":"00000000-0000-4000-8000-000000001006","name":"priced","requiredPropertyContracts":["00000000-0000-4000-8000-000000001007"]}],"propertyContracts":[{"id":"00000000-0000-4000-8000-000000001007","name":"price","valueType":"decimal","dimension":{"factors":[{"id":"count","exponent":-1},{"id":"currency","exponent":1}]},"temporalMode":"scalar","nullable":false}],"propertyDefinitions":[{"id":"00000000-0000-4000-8000-000000001008","owner":{"kind":"entity_type","id":"00000000-0000-4000-8000-000000001004"},"contractId":"00000000-0000-4000-8000-000000001007","name":"price","valueType":"decimal","unit":{"dimension":{"factors":[{"id":"count","exponent":-1},{"id":"currency","exponent":1}]},"factors":[{"id":"item","exponent":-1},{"id":"rub","exponent":1}],"conversionFactors":[{"id":"item","exponent":-1},{"id":"rub","exponent":1}],"scale":{"numerator":"1","denominator":"1"}},"temporalMode":"scalar","computation":{"kind":"input","expression":null}},{"id":"00000000-0000-4000-8000-000000001009","owner":{"kind":"relation_type","id":"00000000-0000-4000-8000-000000001005"},"contractId":null,"name":"quantity","valueType":"integer","unit":{"dimension":{"factors":[{"id":"count","exponent":1}]},"factors":[{"id":"item","exponent":1}],"conversionFactors":[{"id":"item","exponent":1}],"scale":{"numerator":"1","denominator":"1"}},"temporalMode":"scalar","computation":{"kind":"input","expression":null}},{"id":"00000000-0000-4000-8000-000000001010","owner":{"kind":"entity_type","id":"00000000-0000-4000-8000-000000001003"},"contractId":null,"name":"total","valueType":"decimal","unit":{"dimension":{"factors":[{"id":"currency","exponent":1}]},"factors":[{"id":"rub","exponent":1}],"conversionFactors":[{"id":"rub","exponent":1}],"scale":{"numerator":"1","denominator":"1"}},"temporalMode":"scalar","computation":{"kind":"derived","expression":{"kind":"aggregate","source":{"origin":"self","absoluteOrigin":null,"direction":"outgoing","relationTypeId":"00000000-0000-4000-8000-000000001005"},"filter":{"kind":"comparison","operator":"greater","left":{"kind":"bound_property","target":"relation","definitionId":"00000000-0000-4000-8000-000000001009"},"right":{"kind":"constant","value":{"type":"integer","data":"0","unit":{"dimension":{"factors":[{"id":"count","exponent":1}]},"factors":[{"id":"item","exponent":1}],"conversionFactors":[{"id":"item","exponent":1}],"scale":{"numerator":"1","denominator":"1"}}}}},"map":{"kind":"arithmetic","operator":"multiply","left":{"kind":"bound_contract_property","target":"entity","contractId":"00000000-0000-4000-8000-000000001007"},"right":{"kind":"bound_property","target":"relation","definitionId":"00000000-0000-4000-8000-000000001009"}},"operation":"sum"}}}],"scopeDefinitions":[],"entities":[{"id":"00000000-0000-4000-8000-000000001011","typeId":"00000000-0000-4000-8000-000000001003","name":"root"},{"id":"00000000-0000-4000-8000-000000001012","typeId":"00000000-0000-4000-8000-000000001004","name":"part-a"},{"id":"00000000-0000-4000-8000-000000001013","typeId":"00000000-0000-4000-8000-000000001004","name":"part-b"}],"relations":[{"id":"00000000-0000-4000-8000-000000001014","typeId":"00000000-0000-4000-8000-000000001005","from":"00000000-0000-4000-8000-000000001011","to":"00000000-0000-4000-8000-000000001012","applicability":null},{"id":"00000000-0000-4000-8000-000000001015","typeId":"00000000-0000-4000-8000-000000001005","from":"00000000-0000-4000-8000-000000001011","to":"00000000-0000-4000-8000-000000001013","applicability":null}],"valueBindings":[{"target":{"kind":"object_property","address":{"object":{"kind":"entity","id":"00000000-0000-4000-8000-000000001012"},"definitionId":"00000000-0000-4000-8000-000000001008"},"owner":null,"definitionId":null},"value":{"kind":"defined","value":{"type":"decimal","data":"100","unit":{"dimension":{"factors":[{"id":"count","exponent":-1},{"id":"currency","exponent":1}]},"factors":[{"id":"item","exponent":-1},{"id":"rub","exponent":1}],"conversionFactors":[{"id":"item","exponent":-1},{"id":"rub","exponent":1}],"scale":{"numerator":"1","denominator":"1"}}},"series":null},"scopeSelectors":[],"effectivePeriod":null},{"target":{"kind":"object_property","address":{"object":{"kind":"entity","id":"00000000-0000-4000-8000-000000001013"},"definitionId":"00000000-0000-4000-8000-000000001008"},"owner":null,"definitionId":null},"value":{"kind":"defined","value":{"type":"decimal","data":"50","unit":{"dimension":{"factors":[{"id":"count","exponent":-1},{"id":"currency","exponent":1}]},"factors":[{"id":"item","exponent":-1},{"id":"rub","exponent":1}],"conversionFactors":[{"id":"item","exponent":-1},{"id":"rub","exponent":1}],"scale":{"numerator":"1","denominator":"1"}}},"series":null},"scopeSelectors":[],"effectivePeriod":null},{"target":{"kind":"object_property","address":{"object":{"kind":"relation","id":"00000000-0000-4000-8000-000000001014"},"definitionId":"00000000-0000-4000-8000-000000001009"},"owner":null,"definitionId":null},"value":{"kind":"defined","value":{"type":"integer","data":"2","unit":{"dimension":{"factors":[{"id":"count","exponent":1}]},"factors":[{"id":"item","exponent":1}],"conversionFactors":[{"id":"item","exponent":1}],"scale":{"numerator":"1","denominator":"1"}}},"series":null},"scopeSelectors":[],"effectivePeriod":null},{"target":{"kind":"object_property","address":{"object":{"kind":"relation","id":"00000000-0000-4000-8000-000000001015"},"definitionId":"00000000-0000-4000-8000-000000001009"},"owner":null,"definitionId":null},"value":{"kind":"defined","value":{"type":"integer","data":"3","unit":{"dimension":{"factors":[{"id":"count","exponent":1}]},"factors":[{"id":"item","exponent":1}],"conversionFactors":[{"id":"item","exponent":1}],"scale":{"numerator":"1","denominator":"1"}}},"series":null},"scopeSelectors":[],"effectivePeriod":null}],"constraints":[],"rules":[]},"scenarios":[]}}`

func TestExternalConsumerWorkflow(t *testing.T) {
	artifact, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	canonical, err := bcg.Marshal(artifact)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	roundTripped, err := bcg.Unmarshal(canonical)
	if err != nil {
		t.Fatalf("canonical Unmarshal: %v", err)
	}
	canonicalAgain, err := bcg.Marshal(roundTripped)
	if err != nil || !bytes.Equal(canonical, canonicalAgain) {
		t.Fatalf("canonical round trip changed: %v", err)
	}

	model := artifact.Model()
	entityID := bcg.MustParseID("00000000-0000-4000-8000-000000000005")
	definitionID := bcg.MustParseID("00000000-0000-4000-8000-000000000004")
	target, err := bcg.EntityProperty(entityID, definitionID)
	if err != nil {
		t.Fatal(err)
	}
	at, _ := bcg.ParseDateTime("2026-01-02T03:04:05Z")
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}

	result, err := model.Evaluate(target, context)
	if err != nil || result.State() != bcg.Defined {
		t.Fatalf("Evaluate = %#v, %v", result.State(), err)
	}
	decimal, ok := result.Value()
	if !ok {
		t.Fatal("defined result has no value")
	}
	amount, ok := decimal.Decimal()
	if !ok || amount.String() != "10" {
		t.Fatalf("amount = %v, %v", amount, ok)
	}

	many, err := model.EvaluateMany([]bcg.PropertyAddress{target, target}, context)
	if err != nil || len(many) != 2 || many[0].State() != bcg.Defined || many[1].State() != bcg.Defined {
		t.Fatalf("EvaluateMany = %#v, %v", many, err)
	}
	explanation, err := model.Explain(target, context)
	if err != nil || explanation.Root().Kind() == "" {
		t.Fatalf("Explain = %#v, %v", explanation.Root().Kind(), err)
	}
	for _, mode := range []bcg.ImpactMode{bcg.StructuralImpact, bcg.EffectiveImpact, bcg.ObservedImpact} {
		if _, err := model.AnalyzeImpact(mode, target, context); err != nil {
			t.Fatalf("AnalyzeImpact(%s): %v", mode, err)
		}
	}
	constraints, err := model.CheckConstraints(context)
	if err != nil || constraints.Len() != 1 || constraints.Results()[0].State() != bcg.ConstraintSatisfied {
		t.Fatalf("CheckConstraints = %#v, %v", constraints, err)
	}
	feasibility, err := model.CheckFeasibility(context)
	if err != nil || feasibility.State() != bcg.Feasible {
		t.Fatalf("CheckFeasibility = %#v, %v", feasibility.State(), err)
	}
	rules, err := model.EvaluateRules(context)
	if err != nil || rules.Len() != 1 || rules.Results()[0].State() != bcg.RuleActive {
		t.Fatalf("EvaluateRules = %#v, %v", rules, err)
	}

	scenario, ok := artifact.Scenario(bcg.MustParseID("00000000-0000-4000-8000-000000000008"))
	if !ok {
		t.Fatal("Scenario not found")
	}
	scenarioContext, err := context.WithScenario(scenario)
	if err != nil {
		t.Fatal(err)
	}
	scenarioResult, err := model.Evaluate(target, scenarioContext)
	if err != nil {
		t.Fatal(err)
	}
	scenarioValue, _ := scenarioResult.Value()
	scenarioAmount, _ := scenarioValue.Decimal()
	if scenarioAmount.String() != "20" {
		t.Fatalf("scenario amount = %s", scenarioAmount)
	}

	session, err := model.NewSession(context)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Evaluate(target); err != nil {
		t.Fatal(err)
	}
	updated, _ := bcg.ParseDecimal("15")
	override, err := bcg.NewOverride(target, bcg.DecimalValue(updated))
	if err != nil {
		t.Fatal(err)
	}
	if err := session.SetCalculationOverrides(override); err != nil {
		t.Fatal(err)
	}
	after, err := session.Evaluate(target)
	if err != nil {
		t.Fatal(err)
	}
	afterValue, _ := after.Value()
	afterAmount, _ := afterValue.Decimal()
	if afterAmount.String() != "15" || session.CacheStats().Entries() == 0 {
		t.Fatalf("session result/stats = %s/%#v", afterAmount, session.CacheStats())
	}
}

func TestPublicErrorAndConcurrencyContracts(t *testing.T) {
	if _, err := bcg.Unmarshal([]byte(`{"formatVersion":999,"engineCompatibility":"bcg-engine-v1","artifact":{}}`)); !errors.Is(err, bcg.ErrUnsupportedFormat) {
		t.Fatalf("unsupported format error = %v", err)
	}
	artifact, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		t.Fatal(err)
	}
	model := artifact.Model()
	target, _ := bcg.EntityProperty(bcg.MustParseID("00000000-0000-4000-8000-000000000005"), bcg.MustParseID("00000000-0000-4000-8000-000000000004"))
	at, _ := bcg.ParseDateTime("2026-01-02T03:04:05Z")
	context, _ := bcg.NewCalculationContextAt(at)
	session, err := model.NewSession(context)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for index := 0; index < 16; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := session.Evaluate(target); err != nil {
				t.Error(err)
			}
			if _, err := session.Explain(target); err != nil {
				t.Error(err)
			}
			if _, err := session.CheckConstraints(); err != nil {
				t.Error(err)
			}
			if _, err := session.EvaluateRules(); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()

	foreignArtifact := foreignArtifact(t)
	scenario, ok := artifact.Scenario(bcg.MustParseID("00000000-0000-4000-8000-000000000008"))
	if !ok {
		t.Fatal("Scenario not found")
	}
	foreignContext, err := context.WithScenario(scenario)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := foreignArtifact.Model().NewSession(foreignContext); err == nil {
		t.Fatal("foreign Scenario was accepted")
	} else {
		var engine *bcg.EngineError
		if !errors.As(err, &engine) || engine.Kind() != "InvalidScenario" {
			t.Fatalf("foreign Scenario error = %T %[1]v", err)
		}
	}
}

func TestPublicNullUndefinedSeriesAndValidation(t *testing.T) {
	at, _ := bcg.ParseDateTime("2026-01-02T03:04:05Z")
	context, _ := bcg.NewCalculationContextAt(at)
	target, _ := bcg.EntityProperty(bcg.MustParseID("00000000-0000-4000-8000-000000000005"), bcg.MustParseID("00000000-0000-4000-8000-000000000004"))

	nullDocument := strings.Replace(publicArtifact, `"kind":"defined","value":{"type":"decimal","data":"10","unit":null},"series":null`, `"kind":"null","value":null,"series":null`, 1)
	nullDocument = strings.Replace(nullDocument, `"propertyContracts":[]`, `"propertyContracts":[{"id":"00000000-0000-4000-8000-000000000009","name":"nullable-amount","valueType":"decimal","dimension":null,"temporalMode":"scalar","nullable":true}]`, 1)
	nullDocument = strings.Replace(nullDocument, `"contractId":null,"name":"amount"`, `"contractId":"00000000-0000-4000-8000-000000000009","name":"amount"`, 1)
	nullArtifact, err := bcg.Unmarshal([]byte(nullDocument))
	if err != nil {
		t.Fatal(err)
	}
	nullResult, err := nullArtifact.Model().Evaluate(target, context)
	if err != nil || nullResult.State() != bcg.Null {
		t.Fatalf("Null result = %s, %v", nullResult.State(), err)
	}

	undefinedDocument := strings.Replace(publicArtifact, `"valueBindings":[{"target":{"kind":"object_property","address":{"object":{"kind":"entity","id":"00000000-0000-4000-8000-000000000005"},"definitionId":"00000000-0000-4000-8000-000000000004"},"owner":null,"definitionId":null},"value":{"kind":"defined","value":{"type":"decimal","data":"10","unit":null},"series":null},"scopeSelectors":[],"effectivePeriod":null}]`, `"valueBindings":[]`, 1)
	undefinedArtifact, err := bcg.Unmarshal([]byte(undefinedDocument))
	if err != nil {
		t.Fatal(err)
	}
	undefinedResult, err := undefinedArtifact.Model().Evaluate(target, context)
	if err != nil || undefinedResult.State() != bcg.Undefined {
		t.Fatalf("Undefined result = %s, %v", undefinedResult.State(), err)
	}
	reason, ok := undefinedResult.Undefined()
	if !ok || reason.Kind() != bcg.MissingValue {
		t.Fatalf("Undefined reason = %#v, %v", reason, ok)
	}

	seriesArtifact := seriesFixture(t)
	seriesResult, err := seriesArtifact.Model().EvaluateSeries(target, context)
	if err != nil || !seriesResult.IsDefined() || seriesResult.Axis() != bcg.PointSeries || len(seriesResult.Points()) != 1 {
		t.Fatalf("EvaluateSeries = %#v, %v", seriesResult, err)
	}
	sample, ok := seriesResult.Points()[0].Result().Value()
	if !ok {
		t.Fatal("Series sample is not defined")
	}
	seriesAmount, _ := sample.Decimal()
	if seriesAmount.String() != "10" {
		t.Fatalf("Series sample = %s", seriesAmount)
	}

	corrupt := strings.Replace(publicArtifact, `"typeId":"00000000-0000-4000-8000-000000000003","name":"node-1"`, `"typeId":"00000000-0000-4000-8000-000000000099","name":"node-1"`, 1)
	_, err = bcg.Unmarshal([]byte(corrupt))
	var validation *bcg.ValidationError
	if !errors.As(err, &validation) || len(validation.Issues()) == 0 {
		t.Fatalf("validation error = %T %[1]v", err)
	}
	if !errors.Is(err, bcg.ErrModelValidation) {
		t.Fatalf("validation error does not preserve model-validation sentinel: %v", err)
	}
}

func TestPublicOwnershipAndInvalidValues(t *testing.T) {
	artifact, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		t.Fatal(err)
	}
	model := artifact.Model()
	entities := model.Entities()
	entities[0] = bcg.EntityInfo{}
	if model.Entities()[0].ID().IsZero() {
		t.Fatal("Entities exposed mutable model slice")
	}
	rules := model.Rules()
	metadata := rules[0].Metadata()
	metadata["kind"] = "mutated"
	if model.Rules()[0].Metadata()["kind"] != "test" {
		t.Fatal("Rule metadata exposed mutable model map")
	}

	if _, err := bcg.ParseID("not-a-uuid"); err == nil {
		t.Fatal("invalid ID accepted")
	}
	if _, err := bcg.Integer("01"); err == nil {
		t.Fatal("non-canonical integer accepted")
	}
	if _, err := bcg.EntityProperty(bcg.ID{}, bcg.ID{}); err == nil {
		t.Fatal("zero property address accepted")
	}
}

func TestPublicAuthoringLifecycle(t *testing.T) {
	modelID := bcg.MustParseID("00000000-0000-4000-8000-000000000101")
	version1ID := bcg.MustParseID("00000000-0000-4000-8000-000000000102")
	version2ID := bcg.MustParseID("00000000-0000-4000-8000-000000000103")
	policy, err := bcg.NewPrecisionPolicy(8, bcg.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}

	logical, err := bcg.NewModel(modelID, "public-authored")
	if err != nil {
		t.Fatal(err)
	}
	if !logical.IsLogical() || logical.IsValid() || !logical.ModelID().Equal(modelID) {
		t.Fatalf("new logical model = %#v", logical)
	}

	draft, err := logical.NewDraft()
	if err != nil {
		t.Fatal(err)
	}
	if !draft.IsValid() {
		t.Fatal("new Draft is invalid")
	}
	if _, hasParent := draft.ParentVersionID(); hasParent {
		t.Fatal("new Draft unexpectedly has a parent")
	}
	report, err := draft.Validate()
	if err != nil || !report.OK() || report.Len() != 0 || len(report.Issues()) != 0 {
		t.Fatalf("new Draft.Validate = %#v, %v", report, err)
	}

	options1, err := bcg.NewCommitOptions(version1ID, "v1", policy)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := draft.Commit(options1)
	if err != nil {
		t.Fatal(err)
	}
	if !v1.IsValid() || !v1.ModelID().Equal(modelID) || !v1.VersionID().Equal(version1ID) {
		t.Fatalf("v1 = %#v", v1)
	}
	if _, hasParent := v1.ParentVersionID(); hasParent {
		t.Fatal("v1 unexpectedly has a parent")
	}
	if got, ok := v1.PrecisionPolicy(); !ok || got.Scale() != 8 || got.Rounding() != bcg.RoundHalfEven {
		t.Fatalf("v1 precision policy = %#v, %v", got, ok)
	}

	artifact, err := bcg.NewArtifact(v1)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := bcg.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil || !decoded.Model().IsValid() || !decoded.Model().VersionID().Equal(version1ID) {
		t.Fatalf("Marshal/Unmarshal = %#v, %v", decoded, err)
	}

	edit, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	if parent, ok := edit.ParentVersionID(); !ok || !parent.Equal(version1ID) {
		t.Fatalf("edit parent = %s, %v", parent, ok)
	}
	if err := edit.SetName("public-authored-renamed"); err != nil {
		t.Fatal(err)
	}
	options2, err := bcg.NewCommitOptions(version2ID, "v2", policy)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := edit.Commit(options2)
	if err != nil {
		t.Fatal(err)
	}
	if !v2.IsValid() || !v2.ModelID().Equal(modelID) || !v2.VersionID().Equal(version2ID) {
		t.Fatalf("v2 = %#v", v2)
	}
	if parent, ok := v2.ParentVersionID(); !ok || !parent.Equal(version1ID) {
		t.Fatalf("v2 parent = %s, %v", parent, ok)
	}
	if v1.Name() != "public-authored" || v1.VersionLabel() != "v1" || v1.VersionID() != version1ID {
		t.Fatalf("v1 mutated after v2 commit: %#v", v1)
	}
	if v2.Name() != "public-authored-renamed" || v2.VersionLabel() != "v2" {
		t.Fatalf("v2 metadata = %#v", v2)
	}

	failed, err := v1.Edit()
	if err != nil {
		t.Fatal(err)
	}
	if err := failed.SetName("failed-rename"); err != nil {
		t.Fatal(err)
	}
	parentIDOptions, err := bcg.NewCommitOptions(version1ID, "must-fail", policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failed.Commit(parentIDOptions); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("commit with parent version ID = %T %[1]v", err)
	}
	if failed.Name() != "failed-rename" || v1.Name() != "public-authored" || !v1.VersionID().Equal(version1ID) {
		t.Fatal("failed Commit mutated the Draft or its source Model")
	}

	var invalid bcg.Draft
	if _, err := invalid.Validate(); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("zero Draft.Validate = %T %[1]v", err)
	}
	if _, err := invalid.Commit(options1); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("zero Draft.Commit = %T %[1]v", err)
	}

	source, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		t.Fatal(err)
	}
	sourceModel := source.Model()
	contentEdit, err := sourceModel.Edit()
	if err != nil {
		t.Fatal(err)
	}
	if err := contentEdit.SetName("public-fixture-v2"); err != nil {
		t.Fatal(err)
	}
	sourcePolicy, ok := sourceModel.PrecisionPolicy()
	if !ok {
		t.Fatal("source Model has no precision policy")
	}
	contentOptions, err := bcg.NewCommitOptions(
		bcg.MustParseID("00000000-0000-4000-8000-000000000104"),
		"v2",
		sourcePolicy,
	)
	if err != nil {
		t.Fatal(err)
	}
	contentV2, err := contentEdit.Commit(contentOptions)
	if err != nil {
		t.Fatal(err)
	}
	if parent, ok := contentV2.ParentVersionID(); !ok || !parent.Equal(sourceModel.VersionID()) {
		t.Fatalf("copied-content v2 parent = %s, %v", parent, ok)
	}
	if sourceModel.Name() != "public-fixture" || sourceModel.VersionLabel() != "v1" {
		t.Fatalf("source Model changed during content edit: %#v", sourceModel)
	}
	target, err := bcg.EntityProperty(
		bcg.MustParseID("00000000-0000-4000-8000-000000000005"),
		bcg.MustParseID("00000000-0000-4000-8000-000000000004"),
	)
	if err != nil {
		t.Fatal(err)
	}
	at, err := bcg.ParseDateTime("2026-01-02T03:04:05Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	result, err := contentV2.Evaluate(target, context)
	if err != nil || result.State() != bcg.Defined {
		t.Fatalf("edited copied-content evaluation = %#v, %v", result, err)
	}
	scenario, ok := source.Scenario(bcg.MustParseID("00000000-0000-4000-8000-000000000008"))
	if !ok {
		t.Fatal("fixture Scenario missing")
	}
	if _, err := bcg.NewArtifact(source.Model(), scenario); err != nil {
		t.Fatalf("compatible Scenario rejected: %v", err)
	}
	if _, err := bcg.NewArtifact(v1, scenario); !errors.Is(err, bcg.ErrScenarioValidation) {
		t.Fatalf("foreign Scenario accepted or wrong error: %T %[1]v", err)
	}
}

func TestPublicAggregateAndScopeTimeWorkflow(t *testing.T) {
	aggregate, err := bcg.Unmarshal([]byte(publicAggregateArtifact))
	if err != nil {
		t.Fatalf("aggregate fixture decode: %v", err)
	}
	target, err := bcg.EntityProperty(
		bcg.MustParseID("00000000-0000-4000-8000-000000001011"),
		bcg.MustParseID("00000000-0000-4000-8000-000000001010"),
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := aggregate.Model().Evaluate(target, publicContext(t))
	if err != nil {
		t.Fatal(err)
	}
	value, _ := result.Value()
	total, ok := value.Decimal()
	if !ok || total.String() != "350" {
		t.Fatalf("aggregate result = %v, %v", total, ok)
	}
	rub, ok := value.Unit()
	if !ok || len(rub.Factors()) != 1 || rub.Factors()[0].ID != "rub" {
		t.Fatalf("aggregate unit = %#v, %v", rub.Factors(), ok)
	}
	overrideValue, err := bcg.DecimalValue(bcg.MustParseDecimal("999")).WithUnit(rub)
	if err != nil {
		t.Fatal(err)
	}
	session, err := aggregate.Model().NewSession(publicContext(t))
	if err != nil {
		t.Fatal(err)
	}
	override, err := bcg.NewOverride(target, overrideValue)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.SetCalculationOverrides(override); err != nil {
		t.Fatal(err)
	}
	overridden, err := session.Evaluate(target)
	if err != nil {
		t.Fatal(err)
	}
	overriddenValue, _ := overridden.Value()
	overriddenTotal, _ := overriddenValue.Decimal()
	if overriddenTotal.String() != "999" {
		t.Fatalf("derived override = %s", overriddenTotal)
	}
	if len(aggregate.Model().Relations()) != 2 {
		t.Fatal("relation multiplicity was not exposed")
	}
	explanation, err := aggregate.Model().Explain(target, publicContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasAggregateRelation(explanation.Root(), bcg.MustParseID("00000000-0000-4000-8000-000000001014")) ||
		!hasAggregateRelation(explanation.Root(), bcg.MustParseID("00000000-0000-4000-8000-000000001015")) {
		t.Fatal("public provenance lost aggregate relation identity")
	}

	scoped := scopedAndTimedArtifact(t)
	scopedTarget := publicTarget(t)
	at, _ := bcg.ParseDateTime("2026-01-01T12:00:00Z")
	context, _ := bcg.NewCalculationContextAt(at)
	selector, err := bcg.NewScopeSelector(bcg.MustParseID("00000000-0000-4000-8000-000000000009"), bcg.String("north"))
	if err != nil {
		t.Fatal(err)
	}
	scopedContext, err := context.WithScopes(selector)
	if err != nil {
		t.Fatal(err)
	}
	assertPublicDecimal(t, scoped.Model(), scopedTarget, scopedContext, "20")
	boundary, _ := bcg.ParseDateTime("2026-01-02T00:00:00Z")
	boundaryContext, _ := bcg.NewCalculationContextAt(boundary)
	assertPublicDecimal(t, scoped.Model(), scopedTarget, boundaryContext, "30")
}

func TestPublicSessionEffectiveOverrideCutAndReconnect(t *testing.T) {
	artifact, addresses := incrementalArtifact(t)
	context := publicContext(t)
	session, err := artifact.Model().NewSession(context)
	if err != nil {
		t.Fatal(err)
	}
	assertSessionDecimal(t, session, addresses.c, "10")
	assertSessionDecimal(t, session, addresses.y, "5")

	setDecimalOverrides(t, session, overrideFor(t, addresses.a, "20"))
	assertSessionDecimal(t, session, addresses.c, "20")
	hitsBeforeY := session.CacheStats().Hits()
	assertSessionDecimal(t, session, addresses.y, "5")
	if session.CacheStats().Hits() <= hitsBeforeY {
		t.Fatal("unaffected branch was not reused after input override")
	}

	setDecimalOverrides(t, session, overrideFor(t, addresses.a, "20"), overrideFor(t, addresses.b, "30"))
	assertSessionDecimal(t, session, addresses.c, "30")
	setDecimalOverrides(t, session, overrideFor(t, addresses.a, "40"), overrideFor(t, addresses.b, "30"))
	hitsBeforeCut := session.CacheStats().Hits()
	assertSessionDecimal(t, session, addresses.c, "30")
	if session.CacheStats().Hits() <= hitsBeforeCut {
		t.Fatal("derived override did not cut the effective A -> B -> C path")
	}

	setDecimalOverrides(t, session, overrideFor(t, addresses.a, "40"))
	assertSessionDecimal(t, session, addresses.c, "40")
}

func seriesFixture(t *testing.T) bcg.Artifact {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(publicArtifact), &document); err != nil {
		t.Fatal(err)
	}
	artifact := document["artifact"].(map[string]any)
	version := artifact["version"].(map[string]any)
	properties := version["propertyDefinitions"].([]any)
	properties[0].(map[string]any)["temporalMode"] = "series"
	bindings := version["valueBindings"].([]any)
	binding := bindings[0].(map[string]any)
	binding["value"] = map[string]any{
		"kind": "series", "value": nil,
		"series": map[string]any{
			"axis": "point", "elementType": "decimal", "unit": nil,
			"points":    []any{map[string]any{"at": "2026-01-02T03:04:05Z", "sample": map[string]any{"null": false, "value": map[string]any{"type": "decimal", "data": "10"}}}},
			"intervals": []any{},
		},
	}
	artifact["scenarios"] = []any{}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func foreignArtifact(t *testing.T) bcg.Artifact {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(publicArtifact), &document); err != nil {
		t.Fatal(err)
	}
	artifact := document["artifact"].(map[string]any)
	version := artifact["version"].(map[string]any)
	version["id"] = "00000000-0000-4000-8000-000000000099"
	artifact["scenarios"] = []any{}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func scopedAndTimedArtifact(t *testing.T) bcg.Artifact {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(publicArtifact), &document); err != nil {
		t.Fatal(err)
	}
	artifact := document["artifact"].(map[string]any)
	version := artifact["version"].(map[string]any)
	version["scopeDefinitions"] = []any{map[string]any{
		"id": "00000000-0000-4000-8000-000000000009", "name": "region", "valueType": "string",
	}}
	bindings := version["valueBindings"].([]any)
	base := bindings[0].(map[string]any)
	base["effectivePeriod"] = map[string]any{"from": "2026-01-01T00:00:00Z", "to": "2026-01-02T00:00:00Z"}

	scoped := cloneBinding(t, base)
	scoped["scopeSelectors"] = []any{map[string]any{
		"definitionId": "00000000-0000-4000-8000-000000000009",
		"value":        map[string]any{"type": "string", "data": "north", "unit": nil},
	}}
	scoped["effectivePeriod"] = nil
	scoped["value"].(map[string]any)["value"].(map[string]any)["data"] = "20"

	afterBoundary := cloneBinding(t, base)
	afterBoundary["effectivePeriod"] = map[string]any{"from": "2026-01-02T00:00:00Z", "to": nil}
	afterBoundary["value"].(map[string]any)["value"].(map[string]any)["data"] = "30"
	version["valueBindings"] = []any{base, scoped, afterBoundary}
	artifact["scenarios"] = []any{}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

type incrementalAddresses struct{ a, b, c, y bcg.PropertyAddress }

func incrementalArtifact(t *testing.T) (bcg.Artifact, incrementalAddresses) {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(publicArtifact), &document); err != nil {
		t.Fatal(err)
	}
	artifact := document["artifact"].(map[string]any)
	version := artifact["version"].(map[string]any)
	entityType := version["entityTypes"].([]any)[0].(map[string]any)
	properties := version["propertyDefinitions"].([]any)
	properties[0].(map[string]any)["name"] = "a"
	entityType["propertyDefinitions"] = []any{
		"00000000-0000-4000-8000-000000000004",
		"00000000-0000-4000-8000-000000000010",
		"00000000-0000-4000-8000-000000000011",
		"00000000-0000-4000-8000-000000000012",
		"00000000-0000-4000-8000-000000000013",
	}
	properties = append(properties,
		derivedProperty("00000000-0000-4000-8000-000000000010", "b", "00000000-0000-4000-8000-000000000004"),
		derivedProperty("00000000-0000-4000-8000-000000000011", "c", "00000000-0000-4000-8000-000000000010"),
		inputProperty("00000000-0000-4000-8000-000000000012", "x"),
		derivedProperty("00000000-0000-4000-8000-000000000013", "y", "00000000-0000-4000-8000-000000000012"),
	)
	version["propertyDefinitions"] = properties
	bindings := version["valueBindings"].([]any)
	bindings = append(bindings, decimalBinding("00000000-0000-4000-8000-000000000012", "5"))
	version["valueBindings"] = bindings
	artifact["scenarios"] = []any{}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := bcg.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	address := func(definition string) bcg.PropertyAddress {
		result, err := bcg.EntityProperty(bcg.MustParseID("00000000-0000-4000-8000-000000000005"), bcg.MustParseID(definition))
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	return decoded, incrementalAddresses{
		a: address("00000000-0000-4000-8000-000000000004"),
		b: address("00000000-0000-4000-8000-000000000010"),
		c: address("00000000-0000-4000-8000-000000000011"),
		y: address("00000000-0000-4000-8000-000000000013"),
	}
}

func inputProperty(id, name string) map[string]any {
	return map[string]any{
		"id":         id,
		"owner":      map[string]any{"kind": "entity_type", "id": "00000000-0000-4000-8000-000000000003"},
		"contractId": nil, "name": name, "valueType": "decimal", "unit": nil, "temporalMode": "scalar",
		"computation": map[string]any{"kind": "input", "expression": nil},
	}
}

func derivedProperty(id, name, source string) map[string]any {
	property := inputProperty(id, name)
	property["computation"] = map[string]any{
		"kind":       "derived",
		"expression": map[string]any{"kind": "self_property", "definitionId": source},
	}
	return property
}

func decimalBinding(definition, amount string) map[string]any {
	return map[string]any{
		"target": map[string]any{
			"kind": "object_property",
			"address": map[string]any{
				"object":       map[string]any{"kind": "entity", "id": "00000000-0000-4000-8000-000000000005"},
				"definitionId": definition,
			},
			"owner": nil, "definitionId": nil,
		},
		"value":          map[string]any{"kind": "defined", "value": map[string]any{"type": "decimal", "data": amount, "unit": nil}, "series": nil},
		"scopeSelectors": []any{}, "effectivePeriod": nil,
	}
}

func overrideFor(t testing.TB, target bcg.PropertyAddress, amount string) bcg.Override {
	t.Helper()
	parsed, err := bcg.ParseDecimal(amount)
	if err != nil {
		t.Fatal(err)
	}
	override, err := bcg.NewOverride(target, bcg.DecimalValue(parsed))
	if err != nil {
		t.Fatal(err)
	}
	return override
}

func setDecimalOverrides(t testing.TB, session *bcg.Session, overrides ...bcg.Override) {
	t.Helper()
	if err := session.SetCalculationOverrides(overrides...); err != nil {
		t.Fatal(err)
	}
}

func assertSessionDecimal(t testing.TB, session *bcg.Session, target bcg.PropertyAddress, expected string) {
	t.Helper()
	result, err := session.Evaluate(target)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.Value()
	if !ok {
		t.Fatalf("expected defined %s, got %s", expected, result.State())
	}
	decimal, ok := value.Decimal()
	if !ok || decimal.String() != expected {
		t.Fatalf("value = %v, %v; want %s", decimal, ok, expected)
	}
}

func cloneBinding(t testing.TB, binding map[string]any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	var copy map[string]any
	if err := json.Unmarshal(encoded, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func hasAggregateRelation(node bcg.ExplanationNode, relationID bcg.ID) bool {
	if relation, _, ok := node.AggregateItem(); ok && relation.Equal(relationID) {
		return true
	}
	for _, child := range node.Children() {
		if hasAggregateRelation(child, relationID) {
			return true
		}
	}
	return false
}

func assertPublicDecimal(t testing.TB, model bcg.Model, target bcg.PropertyAddress, context bcg.CalculationContext, expected string) {
	t.Helper()
	result, err := model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.Value()
	if !ok {
		t.Fatalf("expected defined %s, got %s", expected, result.State())
	}
	decimal, ok := value.Decimal()
	if !ok || decimal.String() != expected {
		t.Fatalf("value = %v, %v; want %s", decimal, ok, expected)
	}
}

func publicTarget(t testing.TB) bcg.PropertyAddress {
	t.Helper()
	target, err := bcg.EntityProperty(
		bcg.MustParseID("00000000-0000-4000-8000-000000000005"),
		bcg.MustParseID("00000000-0000-4000-8000-000000000004"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func publicContext(t testing.TB) bcg.CalculationContext {
	t.Helper()
	at, err := bcg.ParseDateTime("2026-01-02T03:04:05Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	return context
}

func Example() {
	artifact, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		fmt.Println(err)
		return
	}
	model := artifact.Model()
	target, err := bcg.EntityProperty(bcg.MustParseID("00000000-0000-4000-8000-000000000005"), bcg.MustParseID("00000000-0000-4000-8000-000000000004"))
	if err != nil {
		fmt.Println(err)
		return
	}
	at, err := bcg.ParseDateTime("2026-01-02T03:04:05Z")
	if err != nil {
		fmt.Println(err)
		return
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		fmt.Println(err)
		return
	}
	result, err := model.Evaluate(target, context)
	if err != nil {
		fmt.Println(err)
		return
	}
	value, ok := result.Value()
	if !ok {
		fmt.Println(result.State())
		return
	}
	amount, ok := value.Decimal()
	if !ok {
		fmt.Println("expected Decimal")
		return
	}
	fmt.Println(amount.String())
	// Output: 10
}
