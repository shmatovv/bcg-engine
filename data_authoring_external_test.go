package bcg_test

import (
	"bytes"
	"errors"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

type productAuthoringFixture struct {
	model, version1, version2                                bcg.ID
	entityType, entity                                       bcg.ID
	quantity, price, revenue, optional                       bcg.ID
	seriesPoints, seriesIntervals                            bcg.ID
	nullContract, pointsContract, intervalContract           bcg.ID
	constraint, notApplicableConstraint, undefinedConstraint bcg.ID
	rule, undefinedRule, scenario                            bcg.ID
	policy                                                   bcg.PrecisionPolicy
	quantityAddress, priceAddress                            bcg.PropertyAddress
	revenueAddress, optionalAddress                          bcg.PropertyAddress
	pointsAddress, intervalsAddress                          bcg.PropertyAddress
}

func TestPublicProductBindingsConstraintsRulesScenarios(t *testing.T) {
	fixture, draft := newPublicProductDraft(t)
	if report, err := draft.Validate(); err != nil || !report.OK() {
		t.Fatalf("product Draft.Validate = %#v, %v", report.Issues(), err)
	}
	v1 := commitProduct(t, draft, fixture.version1, "v1", fixture.policy)
	if bindings := v1.ValueBindings(); len(bindings) != 3 || !bindings[0].Key().IsValid() {
		t.Fatalf("public base binding introspection = %#v", bindings)
	}
	if declaration, ok := v1.Constraint(fixture.constraint); !ok || !declaration.Assertion().IsValid() {
		t.Fatalf("Constraint declaration introspection = %#v, %v", declaration, ok)
	}
	if declaration, ok := v1.Rule(fixture.rule); !ok || !declaration.Condition().IsValid() {
		t.Fatalf("Rule declaration introspection = %#v, %v", declaration, ok)
	}

	at := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-15T00:00:00Z"))
	context := mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(at))
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.revenueAddress, context)), "25050")
	explanation := mustData[bcg.Explanation](t)(v1.Explain(fixture.revenueAddress, context))
	if !explanationContainsProperty(explanation.Root(), fixture.quantityAddress) || !explanationContainsProperty(explanation.Root(), fixture.priceAddress) {
		t.Fatal("revenue explanation does not retain quantity and price dependencies")
	}
	impact := mustData[bcg.ImpactResult](t)(v1.AnalyzeImpact(bcg.StructuralImpact, fixture.quantityAddress, context))
	if impact.Len() == 0 {
		t.Fatal("quantity has no structural impact on revenue")
	}

	constraint := mustData[bcg.ConstraintResult](t)(v1.CheckConstraint(fixture.constraint, context))
	if constraint.State() != bcg.ConstraintSatisfied {
		t.Fatalf("hard revenue constraint = %s", constraint.State())
	}
	feasibility := mustData[bcg.FeasibilityResult](t)(v1.CheckFeasibility(context))
	if feasibility.State() != bcg.Feasible {
		t.Fatalf("feasibility = %s", feasibility.State())
	}
	if result := mustData[bcg.ConstraintResult](t)(v1.CheckConstraint(fixture.notApplicableConstraint, context)); result.State() != bcg.ConstraintNotApplicable {
		t.Fatalf("not-applicable constraint = %s", result.State())
	}
	if result := mustData[bcg.ConstraintResult](t)(v1.CheckConstraint(fixture.undefinedConstraint, context)); result.State() != bcg.ConstraintUndefined {
		t.Fatalf("undefined constraint = %s", result.State())
	}
	rule := mustData[bcg.RuleResult](t)(v1.EvaluateRule(fixture.rule, context))
	if rule.State() != bcg.RuleInactive {
		t.Fatalf("base rule = %s, want inactive", rule.State())
	}
	if result := mustData[bcg.RuleResult](t)(v1.EvaluateRule(fixture.undefinedRule, context)); result.State() != bcg.RuleUndefined {
		t.Fatalf("undefined rule = %s", result.State())
	}
	negativeOverride := mustData[bcg.Override](t)(bcg.NewOverride(fixture.revenueAddress, decimal("-1")))
	negativeContext := mustData[bcg.CalculationContext](t)(context.WithCalculationOverrides(negativeOverride))
	if result := mustData[bcg.ConstraintResult](t)(v1.CheckConstraint(fixture.constraint, negativeContext)); result.State() != bcg.ConstraintViolated {
		t.Fatalf("violated constraint = %s", result.State())
	}
	if result := mustData[bcg.FeasibilityResult](t)(v1.CheckFeasibility(negativeContext)); result.State() != bcg.Infeasible {
		t.Fatalf("violated hard constraint feasibility = %s", result.State())
	}

	price300 := mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("300")))
	priceOverride := mustData[bcg.ScenarioOverride](t)(bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{Target: fixture.priceAddress, Value: price300}))
	nullOverride := mustData[bcg.ScenarioOverride](t)(bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{Target: fixture.optionalAddress, Value: bcg.NullBindingValue()}))
	scenario := mustData[bcg.Scenario](t)(bcg.NewScenario(v1, bcg.ScenarioSpec{ID: fixture.scenario, Name: "higher-price", Overrides: []bcg.ScenarioOverride{priceOverride, nullOverride}}))
	if len(scenario.Overrides()) != 2 || !scenario.BaseVersionID().Equal(v1.VersionID()) {
		t.Fatalf("public Scenario readback = %#v", scenario)
	}
	scenarioContext := mustData[bcg.CalculationContext](t)(context.WithScenario(scenario))
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.revenueAddress, scenarioContext)), "30000")
	if result := mustData[bcg.RuleResult](t)(v1.EvaluateRule(fixture.rule, scenarioContext)); result.State() != bcg.RuleActive {
		t.Fatalf("scenario rule = %s, want active", result.State())
	}
	if result := mustData[bcg.Result](t)(v1.Evaluate(fixture.optionalAddress, scenarioContext)); result.State() != bcg.Null {
		t.Fatalf("Scenario Null override = %s", result.State())
	}
	derivedOverride := mustData[bcg.ScenarioOverride](t)(bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{Target: fixture.revenueAddress, Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("1")))}))
	derivedScenario := mustData[bcg.Scenario](t)(bcg.NewScenario(v1, bcg.ScenarioSpec{ID: bcg.NewID(), Name: "override-derived", Overrides: []bcg.ScenarioOverride{derivedOverride}}))
	derivedContext := mustData[bcg.CalculationContext](t)(context.WithScenario(derivedScenario))
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.revenueAddress, derivedContext)), "1")

	artifact := mustData[bcg.Artifact](t)(bcg.NewArtifact(v1, scenario))
	encoded := mustData[[]byte](t)(bcg.Marshal(artifact))
	decoded := mustData[bcg.Artifact](t)(bcg.Unmarshal(encoded))
	decodedScenario, ok := decoded.Scenario(fixture.scenario)
	if !ok {
		t.Fatal("round-tripped Scenario is missing")
	}
	decodedContext := mustData[bcg.CalculationContext](t)(context.WithScenario(decodedScenario))
	assertDecimalResult(t, mustData[bcg.Result](t)(decoded.Model().Evaluate(fixture.revenueAddress, decodedContext)), "30000")
	if after := mustData[[]byte](t)(bcg.Marshal(decoded)); !bytes.Equal(encoded, after) {
		t.Fatal("product artifact changed after canonical round trip")
	}

	// A model edit independently replaces a base candidate, an expression,
	// constraint, and rule. The v1 Scenario remains intentionally foreign.
	edit := mustData[*bcg.Draft](t)(v1.Edit())
	entityName := "Product #1"
	if err := edit.ReplaceEntity(fixture.entity, mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: fixture.entity, TypeID: fixture.entityType, Name: &entityName}))); err != nil {
		t.Fatal(err)
	}
	bindings := edit.ValueBindings()
	priceBinding := bindingForTarget(t, bindings, fixture.priceAddress)
	if err := edit.ReplaceValueBinding(priceBinding.Key(), mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{
		Target: priceBinding.Target(), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("300"))),
	}))); err != nil {
		t.Fatal(err)
	}
	if err := edit.SetValueBinding(bindingForTarget(t, edit.ValueBindings(), fixture.priceAddress)); err != nil {
		t.Fatal(err)
	}
	quantityRef := mustData[bcg.Expression](t)(bcg.SelfProperty(fixture.quantity))
	priceRef := mustData[bcg.Expression](t)(bcg.SelfProperty(fixture.price))
	replacementRevenue := mustData[bcg.Expression](t)(bcg.Multiply(priceRef, quantityRef))
	owner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(fixture.entityType))
	if err := edit.ReplacePropertyDefinition(fixture.revenue, mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID: fixture.revenue, Owner: owner, Name: "revenue", ValueType: bcg.DecimalType,
		TemporalMode: bcg.TemporalScalar, Computation: mustData[bcg.PropertyComputation](t)(bcg.DerivedComputation(replacementRevenue)),
	}))); err != nil {
		t.Fatal(err)
	}
	revenueAbsolute := mustData[bcg.Expression](t)(bcg.AbsoluteProperty(fixture.revenueAddress))
	zero := mustData[bcg.Expression](t)(bcg.Constant(decimal("0")))
	if err := edit.ReplaceConstraint(fixture.constraint, mustData[bcg.Constraint](t)(bcg.NewConstraint(bcg.ConstraintSpec{
		ID: fixture.constraint, Name: "revenue-nonnegative", Assertion: mustData[bcg.Expression](t)(bcg.GreaterOrEqual(revenueAbsolute, zero)), Severity: bcg.SeverityHard,
	}))); err != nil {
		t.Fatal(err)
	}
	if err := edit.ReplaceRule(fixture.rule, mustData[bcg.Rule](t)(bcg.NewRule(bcg.RuleSpec{
		ID: fixture.rule, Name: "large-revenue", Condition: mustData[bcg.Expression](t)(bcg.Greater(revenueAbsolute, mustData[bcg.Expression](t)(bcg.Constant(decimal("26000"))))), Metadata: map[string]string{"kind": "threshold"},
	}))); err != nil {
		t.Fatal(err)
	}
	if report, err := edit.Validate(); err != nil || !report.OK() {
		t.Fatalf("edited product Validate = %#v, %v", report.Issues(), err)
	}
	v2 := commitProduct(t, edit, fixture.version2, "v2", fixture.policy)
	if parent, ok := v2.ParentVersionID(); !ok || !parent.Equal(v1.VersionID()) {
		t.Fatalf("v2 parent = %s, %v", parent, ok)
	}
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.revenueAddress, context)), "25050")
	assertDecimalResult(t, mustData[bcg.Result](t)(v2.Evaluate(fixture.revenueAddress, context)), "30000")
	if entity, ok := v2.Entity(fixture.entity); !ok {
		t.Fatal("edited Entity is missing")
	} else if name, hasName := entity.Name(); !hasName || name != "Product #1" {
		t.Fatalf("v2 Entity replacement = %q, %v", name, hasName)
	}
	if entity, ok := v1.Entity(fixture.entity); !ok {
		t.Fatal("v1 Entity is missing")
	} else if _, hasName := entity.Name(); hasName {
		t.Fatal("Entity replacement mutated v1")
	}
	if _, err := bcg.NewArtifact(v2, scenario); !errors.Is(err, bcg.ErrScenarioValidation) {
		t.Fatalf("v1 Scenario accepted for v2 = %T %[1]v", err)
	}
	removals := mustData[*bcg.Draft](t)(v1.Edit())
	if err := removals.RemoveConstraint(fixture.notApplicableConstraint); err != nil {
		t.Fatal(err)
	}
	if err := removals.RemoveRule(fixture.undefinedRule); err != nil {
		t.Fatal(err)
	}
	if report, err := removals.Validate(); err != nil || !report.OK() {
		t.Fatalf("constraint/rule removal validation = %#v, %v", report.Issues(), err)
	}
	v3 := commitProduct(t, removals, bcg.NewID(), "v3", fixture.policy)
	if len(v3.Constraints()) != len(v1.Constraints())-1 || len(v3.Rules()) != len(v1.Rules())-1 {
		t.Fatal("constraint/rule removals were not committed")
	}
	if len(v1.Constraints()) != 3 || len(v1.Rules()) != 2 {
		t.Fatal("removal edit mutated v1 declarations")
	}
}

func TestPublicSourceSeriesAndBindingSelectors(t *testing.T) {
	fixture, model := publicSeriesAndSelectorModel(t)
	at := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-15T00:00:00Z"))
	context := mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(at))
	points := mustData[bcg.SeriesResult](t)(model.EvaluateSeries(fixture.pointsAddress, context))
	if !points.IsDefined() || points.Axis() != bcg.PointSeries || len(points.Points()) != 2 || points.Points()[1].Result().State() != bcg.Null {
		t.Fatalf("point Series = %#v", points)
	}
	if explained := mustData[bcg.SeriesExplanation](t)(model.ExplainSeries(fixture.pointsAddress, context)); len(explained.Points()) != 2 {
		t.Fatalf("point Series explanation count = %d", len(explained.Points()))
	}
	intervals := mustData[bcg.SeriesResult](t)(model.EvaluateSeries(fixture.intervalsAddress, context))
	if !intervals.IsDefined() || intervals.Axis() != bcg.IntervalSeries || len(intervals.Intervals()) != 1 {
		t.Fatalf("interval Series = %#v", intervals)
	}

	defaultResult := mustData[bcg.Result](t)(model.Evaluate(fixture.scopedAddress, context))
	assertDecimalResult(t, defaultResult, "1")
	selector := mustData[bcg.ScopeSelector](t)(bcg.NewScopeSelector(fixture.scope, bcg.String("EU")))
	scoped := mustData[bcg.CalculationContext](t)(context.WithScopes(selector))
	assertDecimalResult(t, mustData[bcg.Result](t)(model.Evaluate(fixture.scopedAddress, scoped)), "2")
	start := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-01T00:00:00Z"))
	end := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-02-01T00:00:00Z"))
	period := mustData[bcg.EffectivePeriod](t)(bcg.NewEffectivePeriod(&start, &end))
	scenarioSeries := mustData[bcg.SourceSeries](t)(bcg.NewPointSourceSeries(bcg.DecimalType, nil, []bcg.SourcePoint{
		mustData[bcg.SourcePoint](t)(bcg.NewSourcePoint(start, mustData[bcg.SourceSample](t)(bcg.DefinedSourceSample(decimal("9"))))),
	}))
	scenario := mustData[bcg.Scenario](t)(bcg.NewScenario(model, bcg.ScenarioSpec{ID: bcg.NewID(), Name: "regional-series", Overrides: []bcg.ScenarioOverride{
		mustData[bcg.ScenarioOverride](t)(bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{Target: fixture.scopedAddress, Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("3"))), ScopeSelectors: []bcg.ScopeSelector{selector}, EffectivePeriod: &period})),
		mustData[bcg.ScenarioOverride](t)(bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{Target: fixture.pointsAddress, Value: mustData[bcg.BindingValue](t)(bcg.SeriesBindingValue(scenarioSeries))})),
	}}))
	scenarioScoped := mustData[bcg.CalculationContext](t)(scoped.WithScenario(scenario))
	assertDecimalResult(t, mustData[bcg.Result](t)(model.Evaluate(fixture.scopedAddress, scenarioScoped)), "3")
	if overridden := mustData[bcg.SeriesResult](t)(model.EvaluateSeries(fixture.pointsAddress, scenarioScoped)); len(overridden.Points()) != 1 {
		t.Fatalf("Scenario Series override = %#v", overridden)
	} else {
		assertDecimalResult(t, overridden.Points()[0].Result(), "9")
	}

	artifact := mustData[bcg.Artifact](t)(bcg.NewArtifact(model))
	decoded := mustData[bcg.Artifact](t)(bcg.Unmarshal(mustData[[]byte](t)(bcg.Marshal(artifact))))
	if got := decoded.Model().ValueBindings(); len(got) != 4 {
		t.Fatalf("round-tripped public bindings = %d, want 4", len(got))
	}
	if result := mustData[bcg.SeriesResult](t)(decoded.Model().EvaluateSeries(fixture.pointsAddress, context)); len(result.Points()) != 2 || result.Points()[1].Result().State() != bcg.Null {
		t.Fatalf("round-tripped point Series = %#v", result)
	}
}

func TestPublicAggregateSemanticDataAndDanglingRemoval(t *testing.T) {
	fixture, v1 := publicAggregateModel(t)
	at := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-15T00:00:00Z"))
	context := mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(at))
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.totalAddress, context)), "30")
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.relationAWeightAddress, context)), "2")
	if explained := mustData[bcg.Explanation](t)(v1.Explain(fixture.totalAddress, context)); len(explained.Root().Children()) == 0 {
		t.Fatal("aggregate explanation has no provenance children")
	}
	if relations := v1.Relations(); len(relations) != 2 || !relations[0].Declaration().IsValid() {
		t.Fatalf("Relation declaration introspection = %#v", relations)
	}
	if impact := mustData[bcg.ImpactResult](t)(v1.AnalyzeImpact(bcg.StructuralImpact, fixture.childAValueAddress, context)); impact.Len() == 0 {
		t.Fatal("aggregate child value has no public dependency impact")
	}
	decoded := mustData[bcg.Artifact](t)(bcg.Unmarshal(mustData[[]byte](t)(bcg.Marshal(mustData[bcg.Artifact](t)(bcg.NewArtifact(v1))))))
	assertDecimalResult(t, mustData[bcg.Result](t)(decoded.Model().Evaluate(fixture.totalAddress, context)), "30")

	invalid := mustData[*bcg.Draft](t)(v1.Edit())
	if err := invalid.RemoveEntity(fixture.childA); err != nil {
		t.Fatal(err)
	}
	if report, err := invalid.Validate(); err != nil || report.OK() {
		t.Fatalf("dangling Relation after Entity removal = %#v, %v", report.Issues(), err)
	}
	if _, err := invalid.Commit(commitOptions(t, bcg.NewID(), "invalid", fixture.policy)); !errors.Is(err, bcg.ErrModelValidation) {
		t.Fatalf("dangling Entity commit = %T %[1]v", err)
	}

	edit := mustData[*bcg.Draft](t)(v1.Edit())
	extra := mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: fixture.childType}))
	if err := edit.AddEntity(extra); err != nil {
		t.Fatal(err)
	}
	if err := edit.RemoveEntity(extra.ID()); err != nil {
		t.Fatal(err)
	}
	relation := mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: fixture.relationType, From: fixture.parent, To: fixture.childA}))
	if err := edit.AddRelation(relation); err != nil {
		t.Fatal(err)
	}
	if err := edit.RemoveRelation(relation.ID()); err != nil {
		t.Fatal(err)
	}
	relations := edit.Relations()
	if len(relations) == 0 {
		t.Fatal("aggregate edit lost Relations")
	}
	applicability, _ := relations[0].Applicability()
	if err := edit.ReplaceRelation(relations[0].ID(), mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{
		ID: relations[0].ID(), TypeID: relations[0].TypeID(), From: relations[0].From(), To: relations[0].To(), Applicability: applicability,
	}))); err != nil {
		t.Fatal(err)
	}
	childABinding := bindingForTarget(t, edit.ValueBindings(), fixture.childAValueAddress)
	if err := edit.ReplaceValueBinding(childABinding.Key(), mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{
		Target: childABinding.Target(), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("15"))),
	}))); err != nil {
		t.Fatal(err)
	}
	v2 := commitProduct(t, edit, bcg.NewID(), "v2", fixture.policy)
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.totalAddress, context)), "30")
	assertDecimalResult(t, mustData[bcg.Result](t)(v2.Evaluate(fixture.totalAddress, context)), "35")
}

func TestPublicDataAuthoringValidationBoundaries(t *testing.T) {
	product, productDraft := newPublicProductDraft(t)
	v1 := commitProduct(t, productDraft, product.version1, "v1", product.policy)
	brokenProperty := mustData[*bcg.Draft](t)(v1.Edit())
	if err := brokenProperty.RemovePropertyDefinition(product.quantity); err != nil {
		t.Fatal(err)
	}
	assertInvalidDataDraft(t, brokenProperty, product.policy)

	aggregate, aggregateModel := publicAggregateModel(t)
	brokenEndpoint := mustData[*bcg.Draft](t)(aggregateModel.Edit())
	if err := brokenEndpoint.AddRelation(mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: aggregate.relationType, From: aggregate.parent, To: aggregate.parent}))); err != nil {
		t.Fatal(err)
	}
	assertInvalidDataDraft(t, brokenEndpoint, aggregate.policy)
	brokenTraversal := mustData[*bcg.Draft](t)(aggregateModel.Edit())
	if err := brokenTraversal.RemoveRelationType(aggregate.relationType); err != nil {
		t.Fatal(err)
	}
	assertInvalidDataDraft(t, brokenTraversal, aggregate.policy)

	series, seriesModel := publicSeriesAndSelectorModel(t)
	brokenScope := mustData[*bcg.Draft](t)(seriesModel.Edit())
	if err := brokenScope.RemoveScopeDefinition(series.scope); err != nil {
		t.Fatal(err)
	}
	assertInvalidDataDraft(t, brokenScope, mustPolicy(t))

	from := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-02-01T00:00:00Z"))
	to := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-01T00:00:00Z"))
	if _, err := bcg.NewEffectivePeriod(&from, &to); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("inverted effective period = %T %[1]v", err)
	}
	if _, err := bcg.NewSourceInterval(from, from, bcg.NullSourceSample()); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("empty source interval = %T %[1]v", err)
	}
	if _, err := bcg.NewPointSourceSeries(bcg.DecimalType, nil, []bcg.SourcePoint{
		mustData[bcg.SourcePoint](t)(bcg.NewSourcePoint(to, mustData[bcg.SourceSample](t)(bcg.DefinedSourceSample(bcg.Bool(true))))),
	}); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("source sample type mismatch = %T %[1]v", err)
	}
}

func assertInvalidDataDraft(t *testing.T, draft *bcg.Draft, policy bcg.PrecisionPolicy) {
	t.Helper()
	if report, err := draft.Validate(); err != nil || report.OK() || report.Len() == 0 {
		t.Fatalf("invalid data Draft.Validate = %#v, %v", report.Issues(), err)
	}
	if _, err := draft.Commit(commitOptions(t, bcg.NewID(), "invalid", policy)); !errors.Is(err, bcg.ErrModelValidation) {
		t.Fatalf("invalid data Draft.Commit = %T %[1]v", err)
	}
}

type seriesSelectorFixture struct {
	pointsAddress, intervalsAddress, scopedAddress bcg.PropertyAddress
	scope                                          bcg.ID
}

func publicSeriesAndSelectorModel(t *testing.T) (seriesSelectorFixture, bcg.Model) {
	t.Helper()
	modelID, entityTypeID, entityID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	pointsID, intervalsID, scopedID, scopeID := bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID()
	pointsContractID, intervalsContractID := bcg.NewID(), bcg.NewID()
	entityType := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: entityTypeID, Name: "meter", PropertyDefinitions: []bcg.ID{pointsID, intervalsID, scopedID}}))
	draft := mustData[*bcg.Draft](t)(mustData[bcg.Model](t)(bcg.NewModel(modelID, "series-and-scope")).Draft())
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	for _, contract := range []bcg.PropertyContract{
		mustData[bcg.PropertyContract](t)(bcg.NewPropertyContract(bcg.PropertyContractSpec{ID: pointsContractID, Name: "nullable-points", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalSeries, Nullable: true})),
		mustData[bcg.PropertyContract](t)(bcg.NewPropertyContract(bcg.PropertyContractSpec{ID: intervalsContractID, Name: "nullable-intervals", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalSeries, Nullable: true})),
	} {
		if err := draft.AddPropertyContract(contract); err != nil {
			t.Fatal(err)
		}
	}
	owner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(entityTypeID))
	for _, definition := range []bcg.PropertyDefinition{
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: pointsID, Owner: owner, ContractID: pointsContractID, Name: "points", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalSeries, Computation: bcg.InputComputation()})),
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: intervalsID, Owner: owner, ContractID: intervalsContractID, Name: "intervals", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalSeries, Computation: bcg.InputComputation()})),
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: scopedID, Owner: owner, Name: "scoped", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()})),
	} {
		if err := draft.AddPropertyDefinition(definition); err != nil {
			t.Fatal(err)
		}
	}
	if err := draft.AddScopeDefinition(mustData[bcg.ScopeDefinition](t)(bcg.NewScopeDefinition(bcg.ScopeDefinitionSpec{ID: scopeID, Name: "region", ValueType: bcg.StringType}))); err != nil {
		t.Fatal(err)
	}
	entity := mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: entityTypeID}))
	if err := draft.AddEntity(entity); err != nil {
		t.Fatal(err)
	}
	pointsAddress := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(entityID, pointsID))
	intervalsAddress := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(entityID, intervalsID))
	scopedAddress := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(entityID, scopedID))
	a := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-01T00:00:00Z"))
	b := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-02-01T00:00:00Z"))
	pointSamples := []bcg.SourcePoint{
		mustData[bcg.SourcePoint](t)(bcg.NewSourcePoint(a, mustData[bcg.SourceSample](t)(bcg.DefinedSourceSample(decimal("5"))))),
		mustData[bcg.SourcePoint](t)(bcg.NewSourcePoint(b, bcg.NullSourceSample())),
	}
	pointSeries := mustData[bcg.SourceSeries](t)(bcg.NewPointSourceSeries(bcg.DecimalType, nil, pointSamples))
	pointSamples[0] = mustData[bcg.SourcePoint](t)(bcg.NewSourcePoint(a, mustData[bcg.SourceSample](t)(bcg.DefinedSourceSample(decimal("99")))))
	if sample, ok := pointSeries.Points()[0].Sample().Value(); !ok || mustDecimal(t, sample) != "5" {
		t.Fatal("source point input slice aliased")
	}
	intervalSeries := mustData[bcg.SourceSeries](t)(bcg.NewIntervalSourceSeries(bcg.DecimalType, nil, []bcg.SourceInterval{
		mustData[bcg.SourceInterval](t)(bcg.NewSourceInterval(a, b, mustData[bcg.SourceSample](t)(bcg.DefinedSourceSample(decimal("7"))))),
	}))
	effective := mustData[bcg.EffectivePeriod](t)(bcg.NewEffectivePeriod(&a, &b))
	for _, binding := range []bcg.ValueBinding{
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(pointsAddress)), Value: mustData[bcg.BindingValue](t)(bcg.SeriesBindingValue(pointSeries))})),
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(intervalsAddress)), Value: mustData[bcg.BindingValue](t)(bcg.SeriesBindingValue(intervalSeries))})),
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.TypePropertyBindingTarget(owner, scopedID)), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("1")))})),
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(scopedAddress)), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("2"))), ScopeSelectors: []bcg.ScopeSelector{mustData[bcg.ScopeSelector](t)(bcg.NewScopeSelector(scopeID, bcg.String("EU")))}, EffectivePeriod: &effective})),
	} {
		if err := draft.AddValueBinding(binding); err != nil {
			t.Fatal(err)
		}
	}
	if report, err := draft.Validate(); err != nil || !report.OK() {
		t.Fatalf("series Draft.Validate = %#v, %v", report.Issues(), err)
	}
	return seriesSelectorFixture{pointsAddress: pointsAddress, intervalsAddress: intervalsAddress, scopedAddress: scopedAddress, scope: scopeID}, commitProduct(t, draft, bcg.NewID(), "v1", mustPolicy(t))
}

type aggregateFixture struct {
	parent, childA, childType, relationType bcg.ID
	totalAddress, childAValueAddress        bcg.PropertyAddress
	relationAWeightAddress                  bcg.PropertyAddress
	policy                                  bcg.PrecisionPolicy
}

func publicAggregateModel(t *testing.T) (aggregateFixture, bcg.Model) {
	t.Helper()
	modelID, parentType, childType, relationType := bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID()
	parent, childA, childB := bcg.NewID(), bcg.NewID(), bcg.NewID()
	childValue, total, relationWeight := bcg.NewID(), bcg.NewID(), bcg.NewID()
	relationA, relationB := bcg.NewID(), bcg.NewID()
	draft := mustData[*bcg.Draft](t)(mustData[bcg.Model](t)(bcg.NewModel(modelID, "aggregate")).Draft())
	parentSchema := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: parentType, Name: "parent", PropertyDefinitions: []bcg.ID{total}}))
	childSchema := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: childType, Name: "child", PropertyDefinitions: []bcg.ID{childValue}}))
	from := mustData[bcg.EndpointConstraint](t)(bcg.EntityTypeEndpoint(parentType))
	to := mustData[bcg.EndpointConstraint](t)(bcg.EntityTypeEndpoint(childType))
	relationSchema := mustData[bcg.RelationType](t)(bcg.NewRelationType(bcg.RelationTypeSpec{ID: relationType, Name: "contains", From: from, To: to, PropertyDefinitions: []bcg.ID{relationWeight}}))
	for _, item := range []bcg.EntityType{parentSchema, childSchema} {
		if err := draft.AddEntityType(item); err != nil {
			t.Fatal(err)
		}
	}
	if err := draft.AddRelationType(relationSchema); err != nil {
		t.Fatal(err)
	}
	childOwner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(childType))
	parentOwner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(parentType))
	relationOwner := mustData[bcg.PropertyOwner](t)(bcg.RelationTypeOwner(relationType))
	if err := draft.AddPropertyDefinition(mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: childValue, Owner: childOwner, Name: "value", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()}))); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddPropertyDefinition(mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: relationWeight, Owner: relationOwner, Name: "weight", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()}))); err != nil {
		t.Fatal(err)
	}
	source := mustData[bcg.TraversalSource](t)(bcg.SelfTraversal(bcg.TraverseOutgoing, relationType))
	boundValue := mustData[bcg.Expression](t)(bcg.BoundProperty(bcg.BoundEntity, childValue))
	filter := mustData[bcg.Expression](t)(bcg.Greater(boundValue, mustData[bcg.Expression](t)(bcg.Constant(decimal("0")))))
	totalExpression := mustData[bcg.Expression](t)(bcg.FilteredAggregate(source, filter, bcg.AggregateSum, boundValue))
	if err := draft.AddPropertyDefinition(mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: total, Owner: parentOwner, Name: "total", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: mustData[bcg.PropertyComputation](t)(bcg.DerivedComputation(totalExpression))}))); err != nil {
		t.Fatal(err)
	}
	for _, item := range []bcg.Entity{
		mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: parent, TypeID: parentType})),
		mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: childA, TypeID: childType})),
		mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: childB, TypeID: childType})),
	} {
		if err := draft.AddEntity(item); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []bcg.Relation{
		mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{ID: relationA, TypeID: relationType, From: parent, To: childA, Applicability: mustData[bcg.Expression](t)(bcg.Constant(bcg.Bool(true)))})),
		mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{ID: relationB, TypeID: relationType, From: parent, To: childB})),
	} {
		if err := draft.AddRelation(item); err != nil {
			t.Fatal(err)
		}
	}
	childAAddress := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(childA, childValue))
	childBAddress := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(childB, childValue))
	relationAWeightAddress := mustData[bcg.PropertyAddress](t)(bcg.RelationProperty(relationA, relationWeight))
	relationBWeightAddress := mustData[bcg.PropertyAddress](t)(bcg.RelationProperty(relationB, relationWeight))
	for _, item := range []bcg.ValueBinding{
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(childAAddress)), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("10")))})),
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(childBAddress)), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("20")))})),
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(relationAWeightAddress)), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("2")))})),
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(relationBWeightAddress)), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("3")))})),
	} {
		if err := draft.AddValueBinding(item); err != nil {
			t.Fatal(err)
		}
	}
	if report, err := draft.Validate(); err != nil || !report.OK() {
		t.Fatalf("aggregate Draft.Validate = %#v, %v", report.Issues(), err)
	}
	totalAddress := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(parent, total))
	policy := mustPolicy(t)
	return aggregateFixture{parent: parent, childA: childA, childType: childType, relationType: relationType, totalAddress: totalAddress, childAValueAddress: childAAddress, relationAWeightAddress: relationAWeightAddress, policy: policy}, commitProduct(t, draft, bcg.NewID(), "v1", policy)
}

func newPublicProductDraft(t *testing.T) (productAuthoringFixture, *bcg.Draft) {
	t.Helper()
	fixture := productAuthoringFixture{model: bcg.NewID(), version1: bcg.NewID(), version2: bcg.NewID(), entityType: bcg.NewID(), entity: bcg.NewID(), quantity: bcg.NewID(), price: bcg.NewID(), revenue: bcg.NewID(), optional: bcg.NewID(), seriesPoints: bcg.NewID(), seriesIntervals: bcg.NewID(), nullContract: bcg.NewID(), pointsContract: bcg.NewID(), intervalContract: bcg.NewID(), constraint: bcg.NewID(), notApplicableConstraint: bcg.NewID(), undefinedConstraint: bcg.NewID(), rule: bcg.NewID(), undefinedRule: bcg.NewID(), scenario: bcg.NewID(), policy: mustPolicy(t)}
	model := mustData[bcg.Model](t)(bcg.NewModel(fixture.model, "products"))
	draft := mustData[*bcg.Draft](t)(model.Draft())
	entityType := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: fixture.entityType, Name: "Product", PropertyDefinitions: []bcg.ID{fixture.quantity, fixture.price, fixture.revenue, fixture.optional, fixture.seriesPoints, fixture.seriesIntervals}}))
	if err := draft.AddEntityType(entityType); err != nil {
		t.Fatal(err)
	}
	for _, item := range []bcg.PropertyContract{
		mustData[bcg.PropertyContract](t)(bcg.NewPropertyContract(bcg.PropertyContractSpec{ID: fixture.nullContract, Name: "optional", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Nullable: true})),
		mustData[bcg.PropertyContract](t)(bcg.NewPropertyContract(bcg.PropertyContractSpec{ID: fixture.pointsContract, Name: "points", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalSeries, Nullable: true})),
		mustData[bcg.PropertyContract](t)(bcg.NewPropertyContract(bcg.PropertyContractSpec{ID: fixture.intervalContract, Name: "intervals", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalSeries, Nullable: true})),
	} {
		if err := draft.AddPropertyContract(item); err != nil {
			t.Fatal(err)
		}
	}
	owner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(fixture.entityType))
	quantityRef := mustData[bcg.Expression](t)(bcg.SelfProperty(fixture.quantity))
	priceRef := mustData[bcg.Expression](t)(bcg.SelfProperty(fixture.price))
	for _, item := range []bcg.PropertyDefinition{
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: fixture.quantity, Owner: owner, Name: "quantity", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()})),
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: fixture.price, Owner: owner, Name: "price", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()})),
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: fixture.revenue, Owner: owner, Name: "revenue", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: mustData[bcg.PropertyComputation](t)(bcg.DerivedComputation(mustData[bcg.Expression](t)(bcg.Multiply(quantityRef, priceRef))))})),
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: fixture.optional, Owner: owner, ContractID: fixture.nullContract, Name: "optional", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation()})),
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: fixture.seriesPoints, Owner: owner, ContractID: fixture.pointsContract, Name: "points", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalSeries, Computation: bcg.InputComputation()})),
		mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: fixture.seriesIntervals, Owner: owner, ContractID: fixture.intervalContract, Name: "intervals", ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalSeries, Computation: bcg.InputComputation()})),
	} {
		if err := draft.AddPropertyDefinition(item); err != nil {
			t.Fatal(err)
		}
	}
	if err := draft.AddEntity(mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: fixture.entity, TypeID: fixture.entityType}))); err != nil {
		t.Fatal(err)
	}
	fixture.quantityAddress = mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(fixture.entity, fixture.quantity))
	fixture.priceAddress = mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(fixture.entity, fixture.price))
	fixture.revenueAddress = mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(fixture.entity, fixture.revenue))
	fixture.optionalAddress = mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(fixture.entity, fixture.optional))
	fixture.pointsAddress = mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(fixture.entity, fixture.seriesPoints))
	fixture.intervalsAddress = mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(fixture.entity, fixture.seriesIntervals))
	for _, item := range []bcg.ValueBinding{
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(fixture.quantityAddress)), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("100")))})),
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(fixture.priceAddress)), Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("250.50")))})),
		mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(fixture.optionalAddress)), Value: bcg.NullBindingValue()})),
	} {
		if err := draft.AddValueBinding(item); err != nil {
			t.Fatal(err)
		}
	}
	revenueAbsolute := mustData[bcg.Expression](t)(bcg.AbsoluteProperty(fixture.revenueAddress))
	optionalAbsolute := mustData[bcg.Expression](t)(bcg.AbsoluteProperty(fixture.optionalAddress))
	zero := mustData[bcg.Expression](t)(bcg.Constant(decimal("0")))
	if err := draft.AddConstraint(mustData[bcg.Constraint](t)(bcg.NewConstraint(bcg.ConstraintSpec{ID: fixture.constraint, Name: "revenue-nonnegative", Assertion: mustData[bcg.Expression](t)(bcg.GreaterOrEqual(revenueAbsolute, mustData[bcg.Expression](t)(bcg.Constant(decimal("0"))))), Severity: bcg.SeverityHard, Message: "revenue must be nonnegative"}))); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddConstraint(mustData[bcg.Constraint](t)(bcg.NewConstraint(bcg.ConstraintSpec{ID: fixture.notApplicableConstraint, Name: "not-applicable", Applicability: mustData[bcg.Expression](t)(bcg.Constant(bcg.Bool(false))), Assertion: mustData[bcg.Expression](t)(bcg.Constant(bcg.Bool(true))), Severity: bcg.SeverityInfo}))); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddConstraint(mustData[bcg.Constraint](t)(bcg.NewConstraint(bcg.ConstraintSpec{ID: fixture.undefinedConstraint, Name: "undefined", Assertion: mustData[bcg.Expression](t)(bcg.Greater(optionalAbsolute, zero)), Severity: bcg.SeverityInfo}))); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddRule(mustData[bcg.Rule](t)(bcg.NewRule(bcg.RuleSpec{ID: fixture.rule, Name: "large-revenue", Condition: mustData[bcg.Expression](t)(bcg.Greater(revenueAbsolute, mustData[bcg.Expression](t)(bcg.Constant(decimal("26000"))))), Metadata: map[string]string{"kind": "threshold"}}))); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddRule(mustData[bcg.Rule](t)(bcg.NewRule(bcg.RuleSpec{ID: fixture.undefinedRule, Name: "undefined", Condition: mustData[bcg.Expression](t)(bcg.Greater(optionalAbsolute, zero))}))); err != nil {
		t.Fatal(err)
	}
	return fixture, draft
}

func decimal(text string) bcg.Value { return bcg.DecimalValue(bcg.MustParseDecimal(text)) }

func mustDecimal(t *testing.T, result bcg.Value) string {
	t.Helper()
	value, ok := result.Decimal()
	if !ok {
		t.Fatalf("Value is not Decimal: %#v", result)
	}
	return value.String()
}

func assertDecimalResult(t *testing.T, result bcg.Result, want string) {
	t.Helper()
	if result.State() != bcg.Defined {
		t.Fatalf("Result state = %s, want defined", result.State())
	}
	value, ok := result.Value()
	if !ok || mustDecimal(t, value) != want {
		t.Fatalf("Result = %#v, want %s", value, want)
	}
}

func explanationContainsProperty(node bcg.ExplanationNode, target bcg.PropertyAddress) bool {
	if property, ok := node.Property(); ok && property.ObjectKind() == target.ObjectKind() && property.ObjectID().Equal(target.ObjectID()) && property.DefinitionID().Equal(target.DefinitionID()) {
		return true
	}
	for _, child := range node.Children() {
		if explanationContainsProperty(child, target) {
			return true
		}
	}
	return false
}

func bindingForTarget(t *testing.T, bindings []bcg.ValueBinding, address bcg.PropertyAddress) bcg.ValueBinding {
	t.Helper()
	for _, binding := range bindings {
		target, ok := binding.Target().ObjectProperty()
		if ok && target.ObjectKind() == address.ObjectKind() && target.ObjectID().Equal(address.ObjectID()) && target.DefinitionID().Equal(address.DefinitionID()) {
			return binding
		}
	}
	t.Fatalf("binding for %s not found", address)
	return bcg.ValueBinding{}
}

func mustPolicy(t *testing.T) bcg.PrecisionPolicy {
	return mustData[bcg.PrecisionPolicy](t)(bcg.NewPrecisionPolicy(8, bcg.RoundHalfEven))
}

func commitProduct(t *testing.T, draft *bcg.Draft, version bcg.ID, label string, policy bcg.PrecisionPolicy) bcg.Model {
	t.Helper()
	return mustData[bcg.Model](t)(draft.Commit(commitOptions(t, version, label, policy)))
}

func commitOptions(t *testing.T, version bcg.ID, label string, policy bcg.PrecisionPolicy) bcg.CommitOptions {
	t.Helper()
	return mustData[bcg.CommitOptions](t)(bcg.NewCommitOptions(version, label, policy))
}

func mustData[T any](t *testing.T) func(T, error) T {
	t.Helper()
	return func(value T, err error) T {
		if err != nil {
			t.Helper()
			t.Fatal(err)
		}
		return value
	}
}
