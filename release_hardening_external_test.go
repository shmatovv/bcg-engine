package bcg_test

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// These tests deliberately use only the public package. They cover release
// boundaries that are easy to miss in individual authoring feature tests:
// half-open effective time, canonical artifact ordering, defensive copies,
// public invalid-input behavior, and feasibility precedence.

func TestPublicAuthoringEffectiveTimeAndArtifactHardening(t *testing.T) {
	fixture, v1 := publicSeriesAndSelectorModel(t)
	start := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-01T00:00:00Z"))
	end := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-02-01T00:00:00Z"))
	selector := mustData[bcg.ScopeSelector](t)(bcg.NewScopeSelector(fixture.scope, bcg.String("EU")))

	startContext := mustData[bcg.CalculationContext](t)(mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(start)).WithScopes(selector))
	endContext := mustData[bcg.CalculationContext](t)(mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(end)).WithScopes(selector))
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.scopedAddress, startContext)), "2")
	// The object candidate is [start,end), so the type default wins exactly at
	// the exclusive boundary.
	assertDecimalResult(t, mustData[bcg.Result](t)(v1.Evaluate(fixture.scopedAddress, endContext)), "1")

	// Preserve the distinct portable explicit-unbounded period form. It is not
	// the same authored state as omitting an effective selector entirely.
	edit := mustData[*bcg.Draft](t)(v1.Edit())
	var defaultBinding bcg.ValueBinding
	for _, candidate := range edit.ValueBindings() {
		if candidate.Target().Kind() == bcg.BindingTargetTypeProperty {
			defaultBinding = candidate
			break
		}
	}
	if !defaultBinding.IsValid() {
		t.Fatal("type-level default binding is missing")
	}
	if err := edit.RemoveValueBinding(defaultBinding.Key()); err != nil {
		t.Fatal(err)
	}
	openPeriod := mustData[bcg.EffectivePeriod](t)(bcg.NewEffectivePeriod(nil, nil))
	explicitDefault := mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{
		Target: defaultBinding.Target(), Value: defaultBinding.Value(), EffectivePeriod: &openPeriod,
	}))
	if err := edit.AddValueBinding(explicitDefault); err != nil {
		t.Fatal(err)
	}
	if report, err := edit.Validate(); err != nil || !report.OK() {
		t.Fatalf("explicit-unbounded binding validation = %#v, %v", report.Issues(), err)
	}
	v2 := commitProduct(t, edit, bcg.NewID(), "v2", mustPolicy(t))
	assertDecimalResult(t, mustData[bcg.Result](t)(v2.Evaluate(fixture.scopedAddress, endContext)), "1")

	foundExplicitOpenPeriod := false
	for _, candidate := range v2.ValueBindings() {
		if candidate.Target().Kind() != bcg.BindingTargetTypeProperty {
			continue
		}
		period, ok := candidate.EffectivePeriod()
		if !ok {
			continue
		}
		_, hasFrom := period.From()
		_, hasTo := period.To()
		foundExplicitOpenPeriod = !hasFrom && !hasTo
	}
	if !foundExplicitOpenPeriod {
		t.Fatal("explicit unbounded effective period was not retained")
	}

	override := mustData[bcg.ScenarioOverride](t)(bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{
		Target: fixture.scopedAddress,
		Value:  mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("3"))),
	}))
	first := mustData[bcg.Scenario](t)(bcg.NewScenario(v2, bcg.ScenarioSpec{ID: bcg.NewID(), Name: "first", Overrides: []bcg.ScenarioOverride{override}}))
	second := mustData[bcg.Scenario](t)(bcg.NewScenario(v2, bcg.ScenarioSpec{ID: bcg.NewID(), Name: "second", Overrides: []bcg.ScenarioOverride{override}}))
	artifact := mustData[bcg.Artifact](t)(bcg.NewArtifact(v2, second, first))
	scenarios := artifact.Scenarios()
	if len(scenarios) != 2 || scenarios[0].ID().String() > scenarios[1].ID().String() {
		t.Fatalf("artifact scenarios are not deterministic: %#v", scenarios)
	}
	if _, err := bcg.NewArtifact(v2, first, first); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("duplicate Scenario ID error = %T %[1]v, want ErrInvalidArgument", err)
	}
	encoded := mustData[[]byte](t)(bcg.Marshal(artifact))
	decoded := mustData[bcg.Artifact](t)(bcg.Unmarshal(encoded))
	if remarshal := mustData[[]byte](t)(bcg.Marshal(decoded)); !bytes.Equal(encoded, remarshal) {
		t.Fatal("artifact changed after canonical round trip")
	}
	var nilReader *bytes.Reader
	if _, err := bcg.Decode(nilReader); !errors.Is(err, bcg.ErrInvalidArgument) || !errors.Is(err, bcg.ErrMalformedArtifact) {
		t.Fatalf("typed-nil reader error = %T %[1]v, want ErrInvalidArgument", err)
	}
	var nilWriter *bytes.Buffer
	if err := bcg.Encode(nilWriter, artifact); !errors.Is(err, bcg.ErrInvalidArgument) {
		t.Fatalf("typed-nil writer error = %T %[1]v, want ErrInvalidArgument", err)
	}
}

func TestPublicAuthoringDefensiveCopies(t *testing.T) {
	traitID, replacementTraitID := bcg.NewID(), bcg.NewID()
	traits := []bcg.ID{traitID}
	entityType := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{
		ID: bcg.NewID(), Name: "copy-node", Traits: traits,
	}))
	traits[0] = replacementTraitID
	if got := entityType.Traits(); len(got) != 1 || !got[0].Equal(traitID) {
		t.Fatalf("EntityType retained caller trait slice: %#v", got)
	}
	returnedTraits := entityType.Traits()
	returnedTraits[0] = replacementTraitID
	if got := entityType.Traits(); !got[0].Equal(traitID) {
		t.Fatal("EntityType exposed its trait slice")
	}

	factors := []bcg.DimensionFactor{{ID: "length", Exponent: 1}}
	dimension := mustData[bcg.Dimension](t)(bcg.NewDimension(factors))
	factors[0] = bcg.DimensionFactor{ID: "time", Exponent: 1}
	if got := dimension.Factors(); len(got) != 1 || got[0].ID != "length" {
		t.Fatalf("Dimension retained caller factors: %#v", got)
	}
	returnedFactors := dimension.Factors()
	returnedFactors[0].ID = "time"
	if got := dimension.Factors(); got[0].ID != "length" {
		t.Fatal("Dimension exposed its factor slice")
	}
	unitFactors := []bcg.UnitFactor{{ID: "meter", Exponent: 1}}
	conversionFactors := []bcg.ConversionFactor{{ID: "length", Exponent: 1}}
	unit := mustData[bcg.Unit](t)(bcg.NewComposedUnit(dimension, unitFactors, conversionFactors, bcg.OneRational()))
	unitFactors[0].ID = "changed"
	conversionFactors[0].ID = "changed"
	if got := unit.Factors(); len(got) != 1 || got[0].ID != "meter" {
		t.Fatalf("Unit retained caller factors: %#v", got)
	}
	if got := unit.ConversionFactors(); len(got) != 1 || got[0].ID != "length" {
		t.Fatalf("Unit retained caller conversion factors: %#v", got)
	}

	at := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-01T00:00:00Z"))
	point := mustData[bcg.SourcePoint](t)(bcg.NewSourcePoint(at, mustData[bcg.SourceSample](t)(bcg.DefinedSourceSample(decimal("5")))))
	points := []bcg.SourcePoint{point}
	series := mustData[bcg.SourceSeries](t)(bcg.NewPointSourceSeries(bcg.DecimalType, nil, points))
	points[0] = mustData[bcg.SourcePoint](t)(bcg.NewSourcePoint(at, mustData[bcg.SourceSample](t)(bcg.DefinedSourceSample(decimal("9")))))
	if sample, ok := series.Points()[0].Sample().Value(); !ok || mustDecimal(t, sample) != "5" {
		t.Fatal("SourceSeries retained caller points")
	}
	returnedPoints := series.Points()
	returnedPoints[0] = points[0]
	if sample, ok := series.Points()[0].Sample().Value(); !ok || mustDecimal(t, sample) != "5" {
		t.Fatal("SourceSeries exposed its points")
	}

	target := mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(bcg.NewID(), bcg.NewID()))))
	selectorID := bcg.NewID()
	selectors := []bcg.ScopeSelector{mustData[bcg.ScopeSelector](t)(bcg.NewScopeSelector(selectorID, bcg.String("EU")))}
	binding := mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{
		Target: target, Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("1"))), ScopeSelectors: selectors,
	}))
	selectors[0] = mustData[bcg.ScopeSelector](t)(bcg.NewScopeSelector(bcg.NewID(), bcg.String("US")))
	if got := binding.ScopeSelectors(); len(got) != 1 || !got[0].DefinitionID().Equal(selectorID) {
		t.Fatalf("ValueBinding retained caller selectors: %#v", got)
	}
	returnedSelectors := binding.ScopeSelectors()
	returnedSelectors[0] = selectors[0]
	if got := binding.ScopeSelectors(); !got[0].DefinitionID().Equal(selectorID) {
		t.Fatal("ValueBinding exposed its selectors")
	}

	metadata := map[string]string{"kind": "original"}
	condition := mustData[bcg.Expression](t)(bcg.Constant(bcg.Bool(true)))
	rule := mustData[bcg.Rule](t)(bcg.NewRule(bcg.RuleSpec{ID: bcg.NewID(), Name: "copy-rule", Condition: condition, Metadata: metadata}))
	metadata["kind"] = "changed"
	returnedMetadata := rule.Metadata()
	if returnedMetadata["kind"] != "original" {
		t.Fatalf("Rule retained caller metadata: %#v", returnedMetadata)
	}
	returnedMetadata["kind"] = "changed"
	if rule.Metadata()["kind"] != "original" {
		t.Fatal("Rule exposed its metadata")
	}

	fixture, draft := newPublicProductDraft(t)
	model := commitProduct(t, draft, fixture.version1, "v1", fixture.policy)
	override := mustData[bcg.ScenarioOverride](t)(bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{
		Target: fixture.priceAddress,
		Value:  mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("300"))),
	}))
	overrides := []bcg.ScenarioOverride{override}
	scenario := mustData[bcg.Scenario](t)(bcg.NewScenario(model, bcg.ScenarioSpec{ID: bcg.NewID(), Name: "copy-scenario", Overrides: overrides}))
	overrides[0] = bcg.ScenarioOverride{}
	if got := scenario.Overrides(); len(got) != 1 || !got[0].Target().DefinitionID().Equal(fixture.price) {
		t.Fatalf("Scenario retained caller overrides: %#v", got)
	}
	returnedOverrides := scenario.Overrides()
	returnedOverrides[0] = bcg.ScenarioOverride{}
	if got := scenario.Overrides(); len(got) != 1 || !got[0].Target().DefinitionID().Equal(fixture.price) {
		t.Fatal("Scenario exposed its overrides")
	}
}

func TestPublicAuthoringInvalidInputsDoNotPanic(t *testing.T) {
	validID := bcg.NewID()
	constant := mustData[bcg.Expression](t)(bcg.Constant(bcg.Bool(true)))
	at := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-01T00:00:00Z"))
	point := mustData[bcg.SourcePoint](t)(bcg.NewSourcePoint(at, mustData[bcg.SourceSample](t)(bcg.DefinedSourceSample(decimal("1")))))

	for name, call := range map[string]func() error{
		"duplicate entity type traits": func() error {
			_, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: bcg.NewID(), Name: "node", Traits: []bcg.ID{validID, validID}})
			return err
		},
		"duplicate relation properties": func() error {
			_, err := bcg.NewRelationType(bcg.RelationTypeSpec{ID: bcg.NewID(), Name: "edge", PropertyDefinitions: []bcg.ID{validID, validID}})
			return err
		},
		"duplicate trait contracts": func() error {
			_, err := bcg.NewTrait(bcg.TraitSpec{ID: bcg.NewID(), Name: "tag", RequiredPropertyContracts: []bcg.ID{validID, validID}})
			return err
		},
		"noncanonical composed unit": func() error {
			_, err := bcg.NewComposedUnit(bcg.DimensionlessDimension(), []bcg.UnitFactor{{ID: "token", Exponent: 1}, {ID: "token", Exponent: 1}}, nil, bcg.OneRational())
			return err
		},
		"duplicate point coordinate": func() error {
			_, err := bcg.NewPointSourceSeries(bcg.DecimalType, nil, []bcg.SourcePoint{point, point})
			return err
		},
		"invalid scenario override": func() error {
			_, err := bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{})
			return err
		},
		"invalid constraint severity": func() error {
			_, err := bcg.NewConstraint(bcg.ConstraintSpec{ID: bcg.NewID(), Name: "invalid", Assertion: constant})
			return err
		},
		"invalid rule condition": func() error {
			_, err := bcg.NewRule(bcg.RuleSpec{ID: bcg.NewID(), Name: "invalid"})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("public invalid input panicked: %v", recovered)
				}
			}()
			if err := call(); !errors.Is(err, bcg.ErrInvalidArgument) {
				t.Fatalf("error = %T %[1]v, want ErrInvalidArgument", err)
			}
		})
	}
}

func TestPublicConstraintFeasibilityPrecedence(t *testing.T) {
	fixture, draft := newPublicProductDraft(t)
	v1 := commitProduct(t, draft, fixture.version1, "v1", fixture.policy)
	at := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-15T00:00:00Z"))
	context := mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(at))
	falseExpression := mustData[bcg.Expression](t)(bcg.Constant(bcg.Bool(false)))

	nonHard := mustData[*bcg.Draft](t)(v1.Edit())
	for _, severity := range []bcg.ConstraintSeverity{bcg.SeverityInfo, bcg.SeverityWarning, bcg.SeverityError} {
		constraint := mustData[bcg.Constraint](t)(bcg.NewConstraint(bcg.ConstraintSpec{
			ID: bcg.NewID(), Name: severity.String(), Assertion: falseExpression, Severity: severity,
		}))
		if err := nonHard.AddConstraint(constraint); err != nil {
			t.Fatal(err)
		}
	}
	vNonHard := commitProduct(t, nonHard, bcg.NewID(), "non-hard", fixture.policy)
	if result := mustData[bcg.FeasibilityResult](t)(vNonHard.CheckFeasibility(context)); result.State() != bcg.Feasible {
		t.Fatalf("non-Hard violations changed feasibility: %s", result.State())
	}

	optional := mustData[bcg.Expression](t)(bcg.AbsoluteProperty(fixture.optionalAddress))
	undefinedAssertion := mustData[bcg.Expression](t)(bcg.Greater(optional, mustData[bcg.Expression](t)(bcg.Constant(decimal("0")))))
	unknownHard := mustData[*bcg.Draft](t)(v1.Edit())
	if err := unknownHard.AddConstraint(mustData[bcg.Constraint](t)(bcg.NewConstraint(bcg.ConstraintSpec{
		ID: bcg.NewID(), Name: "unknown-hard", Assertion: undefinedAssertion, Severity: bcg.SeverityHard,
	}))); err != nil {
		t.Fatal(err)
	}
	vUnknown := commitProduct(t, unknownHard, bcg.NewID(), "unknown-hard", fixture.policy)
	if result := mustData[bcg.FeasibilityResult](t)(vUnknown.CheckFeasibility(context)); result.State() != bcg.Undetermined {
		t.Fatalf("undefined Hard feasibility = %s, want Undetermined", result.State())
	}

	winningViolation := mustData[*bcg.Draft](t)(vUnknown.Edit())
	if err := winningViolation.AddConstraint(mustData[bcg.Constraint](t)(bcg.NewConstraint(bcg.ConstraintSpec{
		ID: bcg.NewID(), Name: "violated-hard", Assertion: falseExpression, Severity: bcg.SeverityHard,
	}))); err != nil {
		t.Fatal(err)
	}
	vInfeasible := commitProduct(t, winningViolation, bcg.NewID(), "infeasible", fixture.policy)
	if result := mustData[bcg.FeasibilityResult](t)(vInfeasible.CheckFeasibility(context)); result.State() != bcg.Infeasible {
		t.Fatalf("violated Hard did not win over undefined Hard: %s", result.State())
	}
}

func TestPublicAuthoringDeterministicReadOrdering(t *testing.T) {
	fixture, draft := newPublicProductDraft(t)
	v1 := commitProduct(t, draft, fixture.version1, "v1", fixture.policy)
	bindings := v1.ValueBindings()
	keys := make([]string, len(bindings))
	for index, binding := range bindings {
		target := binding.Key().Target()
		keys[index] = target.Kind().String()
		if address, ok := target.ObjectProperty(); ok {
			keys[index] += ":" + string(address.ObjectKind()) + ":" + address.ObjectID().String() + ":" + address.DefinitionID().String()
		}
		if owner, definitionID, ok := target.TypeProperty(); ok {
			keys[index] += ":" + owner.Kind().String() + ":" + owner.ID().String() + ":" + definitionID.String()
		}
	}
	if !sort.StringsAreSorted(keys) {
		t.Fatalf("ValueBindings are not deterministically ordered: %v", keys)
	}
}

func TestPublicAuthoringScaleSmoke(t *testing.T) {
	const (
		chainLength    = 32
		entityCount    = 64
		auxiliaryTypes = 8
		scenarioCount  = 8
	)

	modelID, nodeTypeID, relationTypeID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	definitionIDs := make([]bcg.ID, chainLength+1)
	for index := range definitionIDs {
		definitionIDs[index] = bcg.NewID()
	}
	inputID, aggregateID := definitionIDs[0], definitionIDs[len(definitionIDs)-1]
	entityIDs := make([]bcg.ID, entityCount)
	for index := range entityIDs {
		entityIDs[index] = bcg.NewID()
	}

	logical := mustData[bcg.Model](t)(bcg.NewModel(modelID, "authoring-scale"))
	draft := mustData[*bcg.Draft](t)(logical.Draft())
	if err := draft.AddEntityType(mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{
		ID: nodeTypeID, Name: "node", PropertyDefinitions: definitionIDs,
	}))); err != nil {
		t.Fatal(err)
	}
	from := mustData[bcg.EndpointConstraint](t)(bcg.EntityTypeEndpoint(nodeTypeID))
	to := mustData[bcg.EndpointConstraint](t)(bcg.EntityTypeEndpoint(nodeTypeID))
	if err := draft.AddRelationType(mustData[bcg.RelationType](t)(bcg.NewRelationType(bcg.RelationTypeSpec{
		ID: relationTypeID, Name: "next", From: from, To: to,
	}))); err != nil {
		t.Fatal(err)
	}
	nodeOwner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(nodeTypeID))
	if err := draft.AddPropertyDefinition(mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID: inputID, Owner: nodeOwner, Name: "input", ValueType: bcg.DecimalType,
		TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation(),
	}))); err != nil {
		t.Fatal(err)
	}
	for index := 1; index < chainLength; index++ {
		previous := mustData[bcg.Expression](t)(bcg.SelfProperty(definitionIDs[index-1]))
		next := mustData[bcg.Expression](t)(bcg.Add(previous, mustData[bcg.Expression](t)(bcg.Constant(decimal("1")))))
		if err := draft.AddPropertyDefinition(mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
			ID: definitionIDs[index], Owner: nodeOwner, Name: fmt.Sprintf("derived-%02d", index), ValueType: bcg.DecimalType,
			TemporalMode: bcg.TemporalScalar, Computation: mustData[bcg.PropertyComputation](t)(bcg.DerivedComputation(next)),
		}))); err != nil {
			t.Fatal(err)
		}
	}
	source := mustData[bcg.TraversalSource](t)(bcg.SelfTraversal(bcg.TraverseOutgoing, relationTypeID))
	aggregate := mustData[bcg.Expression](t)(bcg.Aggregate(source, bcg.AggregateSum, mustData[bcg.Expression](t)(bcg.BoundProperty(bcg.BoundEntity, inputID))))
	if err := draft.AddPropertyDefinition(mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
		ID: aggregateID, Owner: nodeOwner, Name: "next-input-sum", ValueType: bcg.DecimalType,
		TemporalMode: bcg.TemporalScalar, Computation: mustData[bcg.PropertyComputation](t)(bcg.DerivedComputation(aggregate)),
	}))); err != nil {
		t.Fatal(err)
	}

	// Auxiliary schema objects ensure collection indexing stays healthy beyond
	// the single-type Product-style examples used by the functional tests.
	for index := 0; index < auxiliaryTypes; index++ {
		typeID, propertyID := bcg.NewID(), bcg.NewID()
		if err := draft.AddEntityType(mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{
			ID: typeID, Name: fmt.Sprintf("aux-%02d", index), PropertyDefinitions: []bcg.ID{propertyID},
		}))); err != nil {
			t.Fatal(err)
		}
		owner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(typeID))
		if err := draft.AddPropertyDefinition(mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
			ID: propertyID, Owner: owner, Name: "input", ValueType: bcg.DecimalType,
			TemporalMode: bcg.TemporalScalar, Computation: bcg.InputComputation(),
		}))); err != nil {
			t.Fatal(err)
		}
	}
	for index, entityID := range entityIDs {
		if err := draft.AddEntity(mustData[bcg.Entity](t)(bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: nodeTypeID}))); err != nil {
			t.Fatal(err)
		}
		address := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(entityID, inputID))
		binding := mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{
			Target: mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(address)),
			Value:  mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("1"))),
		}))
		if err := draft.AddValueBinding(binding); err != nil {
			t.Fatal(err)
		}
		if index+1 < len(entityIDs) {
			if err := draft.AddRelation(mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{
				ID: bcg.NewID(), TypeID: relationTypeID, From: entityID, To: entityIDs[index+1],
			}))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if report, err := draft.Validate(); err != nil || !report.OK() {
		t.Fatalf("scale Draft.Validate = %#v, %v", report.Issues(), err)
	}
	policy := mustPolicy(t)
	model := commitProduct(t, draft, bcg.NewID(), "v1", policy)
	at := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-15T00:00:00Z"))
	context := mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(at))
	chainTarget := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(entityIDs[0], definitionIDs[chainLength-1]))
	assertDecimalResult(t, mustData[bcg.Result](t)(model.Evaluate(chainTarget, context)), fmt.Sprintf("%d", chainLength))
	aggregateTarget := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(entityIDs[0], aggregateID))
	assertDecimalResult(t, mustData[bcg.Result](t)(model.Evaluate(aggregateTarget, context)), "1")

	scenarios := make([]bcg.Scenario, scenarioCount)
	for index := range scenarios {
		target := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(entityIDs[index], inputID))
		override := mustData[bcg.ScenarioOverride](t)(bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{
			Target: target, Value: mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(decimal("2"))),
		}))
		scenarios[index] = mustData[bcg.Scenario](t)(bcg.NewScenario(model, bcg.ScenarioSpec{
			ID: bcg.NewID(), Name: fmt.Sprintf("what-if-%02d", index), Overrides: []bcg.ScenarioOverride{override},
		}))
	}
	artifact := mustData[bcg.Artifact](t)(bcg.NewArtifact(model, scenarios...))
	decoded := mustData[bcg.Artifact](t)(bcg.Unmarshal(mustData[[]byte](t)(bcg.Marshal(artifact))))
	assertDecimalResult(t, mustData[bcg.Result](t)(decoded.Model().Evaluate(chainTarget, context)), fmt.Sprintf("%d", chainLength))
}
