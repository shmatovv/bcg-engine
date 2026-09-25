package bcg_test

import (
	"bytes"
	"fmt"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// publicScopedArtifact is a frozen v1 artifact with a scope definition and a
// scoped/effective binding. It remains literal input, not a value produced by
// the implementation under test.
const publicScopedArtifact = `{"formatVersion":1,"engineCompatibility":"bcg-engine-v1","artifact":{"model":{"id":"00000000-0000-4000-8000-000000002001","name":"scoped-fixture"},"version":{"id":"00000000-0000-4000-8000-000000002002","modelId":"00000000-0000-4000-8000-000000002001","parentVersionId":null,"label":"v1","precision":{"scale":"8","rounding":"half_even"},"entityTypes":[{"id":"00000000-0000-4000-8000-000000002003","name":"node","traits":[],"propertyDefinitions":["00000000-0000-4000-8000-000000002004"]}],"relationTypes":[],"traits":[],"propertyContracts":[],"propertyDefinitions":[{"id":"00000000-0000-4000-8000-000000002004","owner":{"kind":"entity_type","id":"00000000-0000-4000-8000-000000002003"},"contractId":null,"name":"amount","valueType":"decimal","unit":null,"temporalMode":"scalar","computation":{"kind":"input","expression":null}}],"scopeDefinitions":[{"id":"00000000-0000-4000-8000-000000002009","name":"region","valueType":"string"}],"entities":[{"id":"00000000-0000-4000-8000-000000002005","typeId":"00000000-0000-4000-8000-000000002003","name":"node-1"}],"relations":[],"valueBindings":[{"target":{"kind":"object_property","address":{"object":{"kind":"entity","id":"00000000-0000-4000-8000-000000002005"},"definitionId":"00000000-0000-4000-8000-000000002004"},"owner":null,"definitionId":null},"value":{"kind":"defined","value":{"type":"decimal","data":"10","unit":null},"series":null},"scopeSelectors":[],"effectivePeriod":null},{"target":{"kind":"object_property","address":{"object":{"kind":"entity","id":"00000000-0000-4000-8000-000000002005"},"definitionId":"00000000-0000-4000-8000-000000002004"},"owner":null,"definitionId":null},"value":{"kind":"defined","value":{"type":"decimal","data":"11","unit":null},"series":null},"scopeSelectors":[{"definitionId":"00000000-0000-4000-8000-000000002009","value":{"type":"string","data":"eu","unit":null}}],"effectivePeriod":{"from":"2025-01-01T00:00:00Z","to":null}}],"constraints":[],"rules":[]},"scenarios":[]}}`

// The literals in api_external_test.go are frozen v1 artifacts. They
// intentionally are never marshalled by this test to produce its input: a
// future encoder cannot silently rewrite the compatibility baseline. The basic
// artifact is canonical; the others prove canonicalization is stable after
// loading legacy valid input.
func TestFrozenV1CompatibilityAndPublicIntrospection(t *testing.T) {
	for _, fixture := range []struct {
		name      string
		data      string
		target    bcg.PropertyAddress
		expected  string
		scenarios bool
		canonical bool
	}{
		{
			name:      "basic",
			data:      publicArtifact,
			target:    mustCompatibilityAddress(t, "00000000-0000-4000-8000-000000000005", "00000000-0000-4000-8000-000000000004"),
			expected:  "10",
			scenarios: true,
			canonical: true,
		},
		{
			name:      "aggregate-units",
			data:      publicAggregateArtifact,
			target:    mustCompatibilityAddress(t, "00000000-0000-4000-8000-000000001011", "00000000-0000-4000-8000-000000001010"),
			expected:  "350",
			canonical: false,
		},
		{
			name:      "scoped-effective-binding",
			data:      publicScopedArtifact,
			target:    mustCompatibilityAddress(t, "00000000-0000-4000-8000-000000002005", "00000000-0000-4000-8000-000000002004"),
			expected:  "10",
			canonical: false,
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			artifact, err := bcg.Unmarshal([]byte(fixture.data))
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := bcg.Marshal(artifact)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.canonical && !bytes.Equal(canonical, []byte(fixture.data)) {
				t.Fatal("frozen v1 fixture is no longer canonical after decode")
			}
			roundTripped, err := bcg.Unmarshal(canonical)
			if err != nil {
				t.Fatal(err)
			}
			canonicalAgain, err := bcg.Marshal(roundTripped)
			if err != nil || !bytes.Equal(canonical, canonicalAgain) {
				t.Fatalf("canonical v1 re-encode changed: %v", err)
			}

			model := artifact.Model()
			assertLosslessPublicDeclarations(t, model)
			context := mustCompatibilityContext(t)
			assertCompatibilityDecimal(t, model, fixture.target, context, fixture.expected)

			copied, err := reconstructModel(t, model)
			if err != nil {
				t.Fatal(err)
			}
			assertCompatibilityDecimal(t, copied, fixture.target, context, fixture.expected)
			if fixture.scenarios {
				copiedScenarios := reconstructScenarios(t, copied, artifact.Scenarios())
				if len(copiedScenarios) != 1 {
					t.Fatalf("copied scenarios = %d", len(copiedScenarios))
				}
				withScenario, err := context.WithScenario(copiedScenarios[0])
				if err != nil {
					t.Fatal(err)
				}
				assertCompatibilityDecimal(t, copied, fixture.target, withScenario, "20")
			}
		})
	}
}

func assertLosslessPublicDeclarations(t *testing.T, model bcg.Model) {
	t.Helper()
	if len(model.EntityTypeDefinitions()) == 0 || len(model.PropertyDefinitionDeclarations()) == 0 || len(model.Entities()) == 0 {
		t.Fatal("core declarations were not available through public complete-declaration lists")
	}
	for _, relation := range model.Relations() {
		if !relation.Declaration().IsValid() {
			t.Fatal("relation declaration is opaque or invalid")
		}
		if expression, ok := relation.Applicability(); ok {
			assertInspectableExpression(t, expression)
		}
	}
	for _, entity := range model.Entities() {
		if !entity.Declaration().IsValid() {
			t.Fatal("entity declaration is opaque or invalid")
		}
	}
	for _, definition := range model.PropertyDefinitionDeclarations() {
		if computation := definition.Computation(); computation.Kind() == bcg.ComputationDerived {
			expression, ok := computation.Expression()
			if !ok {
				t.Fatal("derived expression missing")
			}
			assertInspectableExpression(t, expression)
		}
	}
	for _, constraint := range model.Constraints() {
		if !constraint.Definition().IsValid() {
			t.Fatal("constraint declaration is opaque")
		}
		assertInspectableExpression(t, constraint.Definition().Assertion())
	}
	for _, rule := range model.Rules() {
		if !rule.Definition().IsValid() {
			t.Fatal("rule declaration is opaque")
		}
		assertInspectableExpression(t, rule.Definition().Condition())
	}
}

func assertInspectableExpression(t *testing.T, expression bcg.Expression) {
	t.Helper()
	if !expression.IsValid() || !expression.Kind().IsValid() {
		t.Fatal("invalid public expression")
	}
	var visited []bcg.ExpressionKind
	if err := bcg.WalkExpression(expression, bcg.ExpressionVisitorFunc(func(node bcg.Expression) error {
		visited = append(visited, node.Kind())
		if _, err := reconstructExpression(node); err != nil {
			return err
		}
		return nil
	})); err != nil {
		t.Fatalf("WalkExpression: %v", err)
	}
	if len(visited) == 0 || visited[0] != expression.Kind() {
		t.Fatalf("walk order = %v", visited)
	}
	if expression.Kind() == bcg.ExpressionAggregate {
		aggregate, ok := expression.AggregateOperation()
		if !ok || !aggregate.Source().IsValid() || !aggregate.Operation().IsValid() {
			t.Fatal("aggregate source, operation, filter, or map is not inspectable")
		}
	}
}

func reconstructExpression(expression bcg.Expression) (bcg.Expression, error) {
	switch expression.Kind() {
	case bcg.ExpressionConstant:
		value, _ := expression.ConstantValue()
		return bcg.Constant(value)
	case bcg.ExpressionSelfPropertyReference, bcg.ExpressionAbsolutePropertyReference, bcg.ExpressionBoundPropertyReference, bcg.ExpressionBoundContractPropertyReference:
		reference, _ := expression.PropertyReference()
		switch reference.Kind() {
		case bcg.PropertyReferenceSelf:
			id, _ := reference.DefinitionID()
			return bcg.SelfProperty(id)
		case bcg.PropertyReferenceAbsolute:
			address, _ := reference.Address()
			return bcg.AbsoluteProperty(address)
		case bcg.PropertyReferenceBoundDefinition:
			binding, _ := reference.Binding()
			id, _ := reference.DefinitionID()
			return bcg.BoundProperty(binding, id)
		case bcg.PropertyReferenceBoundContract:
			binding, _ := reference.Binding()
			id, _ := reference.ContractID()
			return bcg.BoundContractProperty(binding, id)
		}
	case bcg.ExpressionNegate, bcg.ExpressionNot, bcg.ExpressionIsNull:
		operation, _ := expression.UnaryOperation()
		operand, err := reconstructExpression(operation.Operand())
		if err != nil {
			return bcg.Expression{}, err
		}
		switch operation.Kind() {
		case bcg.UnaryNegate:
			return bcg.Negate(operand)
		case bcg.UnaryNot:
			return bcg.Not(operand)
		case bcg.UnaryIsNull:
			return bcg.IsNull(operand)
		}
	case bcg.ExpressionUnaryV2:
		operation, _ := expression.UnaryV2Operation()
		operand, err := reconstructExpression(operation.Operand())
		if err != nil {
			return bcg.Expression{}, err
		}
		switch operation.Operation() {
		case bcg.UnaryIsDefined:
			return bcg.IsDefined(operand)
		case bcg.UnaryIsUndefined:
			return bcg.IsUndefined(operand)
		case bcg.UnaryAbs:
			return bcg.Abs(operand)
		case bcg.UnaryRound:
			return bcg.Round(operand)
		case bcg.UnaryFloor:
			return bcg.Floor(operand)
		case bcg.UnaryCeil:
			return bcg.Ceil(operand)
		case bcg.UnaryLength:
			return bcg.Length(operand)
		case bcg.UnaryLower:
			return bcg.Lower(operand)
		case bcg.UnaryUpper:
			return bcg.Upper(operand)
		case bcg.UnaryTrim:
			return bcg.Trim(operand)
		case bcg.UnaryYear:
			return bcg.Year(operand)
		case bcg.UnaryMonth:
			return bcg.Month(operand)
		case bcg.UnaryDay:
			return bcg.Day(operand)
		case bcg.UnaryHour:
			return bcg.Hour(operand)
		case bcg.UnaryMinute:
			return bcg.Minute(operand)
		}
	case bcg.ExpressionArithmetic, bcg.ExpressionComparison, bcg.ExpressionLogical:
		operation, _ := expression.BinaryOperation()
		left, err := reconstructExpression(operation.Left())
		if err != nil {
			return bcg.Expression{}, err
		}
		right, err := reconstructExpression(operation.Right())
		if err != nil {
			return bcg.Expression{}, err
		}
		switch operation.Kind() {
		case bcg.BinaryAdd:
			return bcg.Add(left, right)
		case bcg.BinarySubtract:
			return bcg.Subtract(left, right)
		case bcg.BinaryMultiply:
			return bcg.Multiply(left, right)
		case bcg.BinaryDivide:
			return bcg.Divide(left, right)
		case bcg.BinaryEqual:
			return bcg.Equal(left, right)
		case bcg.BinaryNotEqual:
			return bcg.NotEqual(left, right)
		case bcg.BinaryLess:
			return bcg.Less(left, right)
		case bcg.BinaryLessOrEqual:
			return bcg.LessOrEqual(left, right)
		case bcg.BinaryGreater:
			return bcg.Greater(left, right)
		case bcg.BinaryGreaterOrEqual:
			return bcg.GreaterOrEqual(left, right)
		case bcg.BinaryAnd:
			return bcg.And(left, right)
		case bcg.BinaryOr:
			return bcg.Or(left, right)
		}
	case bcg.ExpressionBinaryV2:
		operation, _ := expression.BinaryV2Operation()
		left, err := reconstructExpression(operation.Left())
		if err != nil {
			return bcg.Expression{}, err
		}
		right, err := reconstructExpression(operation.Right())
		if err != nil {
			return bcg.Expression{}, err
		}
		switch operation.Operation() {
		case bcg.BinaryModulo:
			return bcg.Modulo(left, right)
		case bcg.BinaryContains:
			return bcg.Contains(left, right)
		case bcg.BinaryStartsWith:
			return bcg.StartsWith(left, right)
		case bcg.BinaryEndsWith:
			return bcg.EndsWith(left, right)
		case bcg.BinaryAddDuration:
			return bcg.AddDuration(left, right)
		case bcg.BinarySubtractDuration:
			return bcg.SubtractDuration(left, right)
		case bcg.BinaryDifference:
			return bcg.Difference(left, right)
		}
	case bcg.ExpressionVariadicV2:
		operation, _ := expression.VariadicV2Operation()
		operands := operation.Operands()
		rebuilt := make([]bcg.Expression, len(operands))
		for index, operand := range operands {
			var err error
			rebuilt[index], err = reconstructExpression(operand)
			if err != nil {
				return bcg.Expression{}, err
			}
		}
		switch operation.Operation() {
		case bcg.VariadicConcat:
			return bcg.Concat(rebuilt...)
		case bcg.VariadicScalarMin:
			return bcg.ScalarMin(rebuilt...)
		case bcg.VariadicScalarMax:
			return bcg.ScalarMax(rebuilt...)
		}
	case bcg.ExpressionFunctionCall:
		call, _ := expression.FunctionCall()
		arguments := call.Arguments()
		rebuilt := make([]bcg.Expression, len(arguments))
		for index, argument := range arguments {
			var err error
			rebuilt[index], err = reconstructExpression(argument)
			if err != nil {
				return bcg.Expression{}, err
			}
		}
		return bcg.CallFunction(call.Ref(), rebuilt...)
	case bcg.ExpressionIf, bcg.ExpressionCoalesce:
		operation, _ := expression.ConditionalOperation()
		if operation.Kind() == bcg.ConditionalIf {
			condition, _ := operation.Condition()
			thenExpression, _ := operation.Then()
			elseExpression, _ := operation.Else()
			condition, err := reconstructExpression(condition)
			if err != nil {
				return bcg.Expression{}, err
			}
			thenExpression, err = reconstructExpression(thenExpression)
			if err != nil {
				return bcg.Expression{}, err
			}
			elseExpression, err = reconstructExpression(elseExpression)
			if err != nil {
				return bcg.Expression{}, err
			}
			return bcg.If(condition, thenExpression, elseExpression)
		}
		primary, _ := operation.Primary()
		fallback, _ := operation.Fallback()
		primary, err := reconstructExpression(primary)
		if err != nil {
			return bcg.Expression{}, err
		}
		fallback, err = reconstructExpression(fallback)
		if err != nil {
			return bcg.Expression{}, err
		}
		return bcg.Coalesce(primary, fallback)
	case bcg.ExpressionAggregate:
		aggregate, _ := expression.AggregateOperation()
		source, err := reconstructTraversal(aggregate.Source())
		if err != nil {
			return bcg.Expression{}, err
		}
		filter, hasFilter := aggregate.Filter()
		mapped, hasMap := aggregate.Map()
		if hasFilter {
			filter, err = reconstructExpression(filter)
			if err != nil {
				return bcg.Expression{}, err
			}
		}
		if hasMap {
			mapped, err = reconstructExpression(mapped)
			if err != nil {
				return bcg.Expression{}, err
			}
		}
		if aggregate.Operation() == bcg.AggregateCount {
			if hasFilter {
				return bcg.FilteredCount(source, filter)
			}
			return bcg.Count(source)
		}
		if aggregate.Operation() == bcg.AggregateExists {
			if hasFilter {
				return bcg.FilteredExists(source, filter)
			}
			return bcg.Exists(source)
		}
		if !hasMap {
			return bcg.Expression{}, fmt.Errorf("mapped aggregate has no map")
		}
		if hasFilter {
			return bcg.FilteredAggregate(source, filter, aggregate.Operation(), mapped)
		}
		return bcg.Aggregate(source, aggregate.Operation(), mapped)
	}
	return bcg.Expression{}, fmt.Errorf("unsupported expression kind %s", expression.Kind())
}

func reconstructTraversal(source bcg.TraversalSource) (bcg.TraversalSource, error) {
	switch source.Origin() {
	case bcg.TraversalSelf:
		return bcg.SelfTraversal(source.Direction(), source.RelationTypeID())
	case bcg.TraversalAbsoluteEntity:
		id, _ := source.AbsoluteEntityID()
		return bcg.AbsoluteEntityTraversal(id, source.Direction(), source.RelationTypeID())
	case bcg.TraversalBoundEntity:
		return bcg.BoundEntityTraversal(source.Direction(), source.RelationTypeID())
	default:
		return bcg.TraversalSource{}, fmt.Errorf("invalid traversal origin")
	}
}

func reconstructModel(t *testing.T, source bcg.Model) (bcg.Model, error) {
	t.Helper()
	logical, err := bcg.NewModel(source.ModelID(), source.Name())
	if err != nil {
		return bcg.Model{}, err
	}
	draft, err := logical.Draft()
	if err != nil {
		return bcg.Model{}, err
	}
	for _, requirement := range source.FunctionRequirements() {
		if err := draft.AddFunctionRequirement(requirement); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, value := range source.PropertyContractDefinitions() {
		if err := draft.AddPropertyContract(rebuildContract(value)); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, value := range source.TraitDefinitions() {
		rebuilt, err := bcg.NewTrait(bcg.TraitSpec{ID: value.ID(), Name: value.Name(), RequiredPropertyContracts: value.RequiredPropertyContracts()})
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddTrait(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, value := range source.EntityTypeDefinitions() {
		rebuilt, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: value.ID(), Name: value.Name(), Traits: value.Traits(), PropertyDefinitions: value.PropertyDefinitions()})
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddEntityType(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, value := range source.RelationTypeDefinitions() {
		rebuilt, err := rebuildRelationType(value)
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddRelationType(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, value := range source.ScopeDefinitionDeclarations() {
		rebuilt, err := bcg.NewScopeDefinition(bcg.ScopeDefinitionSpec{ID: value.ID(), Name: value.Name(), ValueType: value.ValueType()})
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddScopeDefinition(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, value := range source.PropertyDefinitionDeclarations() {
		rebuilt, err := rebuildProperty(value)
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddPropertyDefinition(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, info := range source.Entities() {
		value := info.Declaration()
		var name *string
		if text, ok := value.Name(); ok {
			name = &text
		}
		rebuilt, err := bcg.NewEntity(bcg.EntitySpec{ID: value.ID(), TypeID: value.TypeID(), Name: name})
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddEntity(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, info := range source.Relations() {
		value := info.Declaration()
		applicability, ok := value.Applicability()
		if ok {
			applicability, err = reconstructExpression(applicability)
			if err != nil {
				return bcg.Model{}, err
			}
		}
		rebuilt, err := bcg.NewRelation(bcg.RelationSpec{ID: value.ID(), TypeID: value.TypeID(), From: value.From(), To: value.To(), Applicability: applicability})
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddRelation(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, value := range source.ValueBindings() {
		rebuilt, err := rebuildBinding(value)
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddValueBinding(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, info := range source.Constraints() {
		value := info.Definition()
		assertion, err := reconstructExpression(value.Assertion())
		if err != nil {
			return bcg.Model{}, err
		}
		applicability, ok := value.Applicability()
		if ok {
			applicability, err = reconstructExpression(applicability)
			if err != nil {
				return bcg.Model{}, err
			}
		}
		rebuilt, err := bcg.NewConstraint(bcg.ConstraintSpec{ID: value.ID(), Name: value.Name(), Applicability: applicability, Assertion: assertion, Severity: value.Severity(), Message: optionalString(value.Message())})
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddConstraint(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	for _, info := range source.Rules() {
		value := info.Definition()
		condition, err := reconstructExpression(value.Condition())
		if err != nil {
			return bcg.Model{}, err
		}
		rebuilt, err := bcg.NewRule(bcg.RuleSpec{ID: value.ID(), Name: value.Name(), Condition: condition, Metadata: value.Metadata()})
		if err != nil {
			return bcg.Model{}, err
		}
		if err := draft.AddRule(rebuilt); err != nil {
			return bcg.Model{}, err
		}
	}
	policy, ok := source.PrecisionPolicy()
	if !ok {
		return bcg.Model{}, fmt.Errorf("source model has no precision policy")
	}
	return draft.Commit(mustCompatibilityOptions(t, bcg.NewID(), "reconstructed", policy))
}

func rebuildContract(value bcg.PropertyContract) bcg.PropertyContract {
	dimension, ok := value.Dimension()
	var pointer *bcg.Dimension
	if ok {
		pointer = &dimension
	}
	result, err := bcg.NewPropertyContract(bcg.PropertyContractSpec{ID: value.ID(), Name: value.Name(), ValueType: value.ValueType(), Dimension: pointer, TemporalMode: value.TemporalMode(), Nullable: value.Nullable()})
	if err != nil {
		panic(err)
	}
	return result
}
func rebuildRelationType(value bcg.RelationType) (bcg.RelationType, error) {
	from, _ := value.From()
	to, _ := value.To()
	return bcg.NewRelationType(bcg.RelationTypeSpec{ID: value.ID(), Name: value.Name(), From: from, To: to, PropertyDefinitions: value.PropertyDefinitions()})
}
func rebuildProperty(value bcg.PropertyDefinition) (bcg.PropertyDefinition, error) {
	contract, hasContract := value.ContractID()
	unit, hasUnit := value.Unit()
	computation := value.Computation()
	if computation.Kind() == bcg.ComputationDerived {
		expression, _ := computation.Expression()
		rebuilt, err := reconstructExpression(expression)
		if err != nil {
			return bcg.PropertyDefinition{}, err
		}
		computation, err = bcg.DerivedComputation(rebuilt)
		if err != nil {
			return bcg.PropertyDefinition{}, err
		}
	}
	if !hasContract {
		contract = bcg.ID{}
	}
	if !hasUnit {
		unit = bcg.Unit{}
	}
	return bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: value.ID(), Owner: value.Owner(), ContractID: contract, Name: value.Name(), ValueType: value.ValueType(), Unit: unit, TemporalMode: value.TemporalMode(), Computation: computation})
}
func rebuildBinding(value bcg.ValueBinding) (bcg.ValueBinding, error) {
	period, hasPeriod := value.EffectivePeriod()
	var pointer *bcg.EffectivePeriod
	if hasPeriod {
		pointer = &period
	}
	return bcg.NewValueBinding(bcg.ValueBindingSpec{Target: value.Target(), Value: value.Value(), ScopeSelectors: value.ScopeSelectors(), EffectivePeriod: pointer})
}
func optionalString(value string, ok bool) string {
	if ok {
		return value
	}
	return ""
}
func reconstructScenarios(t *testing.T, model bcg.Model, source []bcg.Scenario) []bcg.Scenario {
	t.Helper()
	result := make([]bcg.Scenario, len(source))
	for i, value := range source {
		overrides := value.Overrides()
		rebuilt := make([]bcg.ScenarioOverride, len(overrides))
		for j, override := range overrides {
			period, hasPeriod := override.EffectivePeriod()
			var pointer *bcg.EffectivePeriod
			if hasPeriod {
				pointer = &period
			}
			candidate, err := bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{Target: override.Target(), Value: override.Value(), ScopeSelectors: override.ScopeSelectors(), EffectivePeriod: pointer})
			if err != nil {
				t.Fatal(err)
			}
			rebuilt[j] = candidate
		}
		candidate, err := bcg.NewScenario(model, bcg.ScenarioSpec{ID: value.ID(), Name: value.Name(), Overrides: rebuilt})
		if err != nil {
			t.Fatal(err)
		}
		result[i] = candidate
	}
	return result
}
func mustCompatibilityAddress(t *testing.T, entity, definition string) bcg.PropertyAddress {
	t.Helper()
	value, err := bcg.EntityProperty(bcg.MustParseID(entity), bcg.MustParseID(definition))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustCompatibilityContext(t *testing.T) bcg.CalculationContext {
	t.Helper()
	at, err := bcg.ParseDateTime("2026-01-02T03:04:05Z")
	if err != nil {
		t.Fatal(err)
	}
	value, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustCompatibilityOptions(t *testing.T, id bcg.ID, label string, policy bcg.PrecisionPolicy) bcg.CommitOptions {
	t.Helper()
	value, err := bcg.NewCommitOptions(id, label, policy)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func assertCompatibilityDecimal(t *testing.T, model bcg.Model, target bcg.PropertyAddress, context bcg.CalculationContext, expected string) {
	t.Helper()
	result, err := model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.Value()
	if !ok {
		t.Fatalf("result is not defined: %s", result.State())
	}
	decimal, ok := value.Decimal()
	if !ok || decimal.String() != expected {
		t.Fatalf("decimal = %v, %t; want %s", decimal, ok, expected)
	}
}
