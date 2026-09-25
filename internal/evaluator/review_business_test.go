package evaluator_test

import (
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/v2/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	ruleeval "github.com/shmatovv/bcg-engine/v2/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func TestAggregateEmptyNullUndefinedAndSelfLoop(t *testing.T) {
	for _, operation := range []ast.AggregateOperation{ast.AggregateOperationSum, ast.AggregateOperationCount, ast.AggregateOperationAverage, ast.AggregateOperationMin, ast.AggregateOperationMax} {
		for _, mode := range []string{"empty", "defined", "null", "missing"} {
			t.Run(operation.String()+"/"+mode, func(t *testing.T) {
				f := newPipelineFixture()
				input := f.property(value.Integer, schema.InputComputation(), true)
				relationTypeID := identity.New()
				endpoint := schema.EntityTypeConstraint(f.typeID)
				f.draft.AddRelationType(schema.NewRelationType(relationTypeID, "links", endpoint, true, endpoint, true, nil))
				if mode != "empty" {
					f.draft.AddRelation(semantic.NewRelation(identity.New(), relationTypeID, f.entityID, f.entityID, boolean(true), true))
				}
				source := ast.NewTraversalSource(ast.TraversalDirectionBoth, relationTypeID)
				var expression ast.Expression = ast.NewCountAggregate(source)
				if operation != ast.AggregateOperationCount {
					expression = ast.NewMappedAggregate(source, ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, input.DefinitionID()), operation)
				}
				typ := value.Integer
				if operation == ast.AggregateOperationAverage {
					typ = value.Decimal
				}
				root := f.property(typ, schema.DerivedComputation(expression), operation != ast.AggregateOperationCount)
				v := f.commit(t)
				context := reviewContext()
				if mode == "defined" || mode == "null" {
					binding := bindingInteger(5)
					if mode == "null" {
						binding = semantic.NullBindingValue()
					}
					context = runtime.NewCalculationContext(context.TimeContext(), scope.Values{}, []runtime.Override{runtime.NewOverride(input, binding)})
				}
				result, err := evaluator.Evaluate(v, root, context)
				if err != nil {
					t.Fatal(err)
				}
				got := result.Value()
				switch {
				case operation == ast.AggregateOperationCount:
					want := int64(1)
					if mode == "empty" {
						want = 0
					}
					if !got.IsDefined() || got.Value().Raw().Integer().Int64() != want {
						t.Fatal("Count must count a self-loop once without reading map")
					}
				case mode == "null":
					if !got.IsNull() {
						t.Fatal("mapped Null ignored")
					}
				case mode == "missing":
					if !got.IsUndefined() || got.UndefinedInfo().Kind() != value.UndefinedDependencyUndefined {
						t.Fatal("mapped Undefined ignored")
					}
				case mode == "empty" && operation != ast.AggregateOperationSum:
					if !got.IsUndefined() || got.UndefinedInfo().Kind() != value.UndefinedEmptyAggregation {
						t.Fatal("wrong empty aggregation")
					}
				default:
					want := int64(5)
					if mode == "empty" {
						want = 0
					}
					if !got.IsDefined() {
						t.Fatal("expected defined aggregate")
					}
					if typ == value.Decimal {
						if got.Value().Raw().Decimal().String() != "5" {
							t.Fatal("wrong Average")
						}
					} else if got.Value().Raw().Integer().Int64() != want {
						t.Fatal("wrong aggregate value")
					}
				}
			})
		}
	}
}

func TestConstraintRuleStatesAndFeasibility(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Bool, schema.InputComputation(), true)
	hardID, inactiveID, ruleID := identity.New(), identity.New(), identity.New()
	f.draft.AddConstraint(constraint.New(hardID, "hard", nil, false, ref(input), constraint.SeverityHard, "", false))
	f.draft.AddConstraint(constraint.New(inactiveID, "inactive", boolean(false), true, ref(input), constraint.SeverityHard, "", false))
	for _, severity := range []constraint.Severity{constraint.SeverityInfo, constraint.SeverityWarning, constraint.SeverityError} {
		f.draft.AddConstraint(constraint.New(identity.New(), "advisory", nil, false, boolean(false), severity, "", false))
	}
	f.draft.AddRule(domainrule.New(ruleID, "rule", ref(input)))
	v := f.commit(t)
	for _, mode := range []string{"true", "false", "null", "missing"} {
		t.Run(mode, func(t *testing.T) {
			context := reviewContext()
			if mode != "missing" {
				binding := semantic.NullBindingValue()
				if mode != "null" {
					binding = semantic.DefinedBindingValue(value.NewTyped(value.NewBool(mode == "true")))
				}
				context = runtime.NewCalculationContext(context.TimeContext(), scope.Values{}, []runtime.Override{runtime.NewOverride(input, binding)})
			}
			report, err := check.All(v, context)
			if err != nil {
				t.Fatal(err)
			}
			want := feasibility.StateUndetermined
			if mode == "true" {
				want = feasibility.StateFeasible
			} else if mode == "false" {
				want = feasibility.StateInfeasible
			}
			if feasibility.Assess(report).State() != want {
				t.Fatal("wrong feasibility")
			}
			inactive, err := check.One(v, inactiveID, context)
			if err != nil || inactive.State() != check.StateNotApplicable {
				t.Fatalf("inactive constraint: %v", err)
			}
			if _, evaluated := inactive.Assertion(); evaluated {
				t.Fatal("inactive assertion evaluated")
			}
			rule, err := ruleeval.One(v, ruleID, context)
			if err != nil {
				t.Fatal(err)
			}
			wantRule := ruleeval.StateUndefined
			if mode == "true" {
				wantRule = ruleeval.StateActive
			} else if mode == "false" {
				wantRule = ruleeval.StateInactive
			}
			if rule.State() != wantRule {
				t.Fatal("wrong rule state")
			}
		})
	}
}

func TestStandaloneConstraintAndRuleUseExplicitAggregateOrigin(t *testing.T) {
	f := newPipelineFixture()
	price := f.property(value.Integer, schema.InputComputation(), false)
	applicable := f.property(value.Bool, schema.InputComputation(), true)
	relationTypeID := identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(
		relationTypeID,
		"links",
		endpoint,
		true,
		endpoint,
		true,
		nil,
	))
	f.draft.AddRelation(semantic.NewRelation(
		identity.New(),
		relationTypeID,
		f.entityID,
		f.entityID,
		ref(applicable),
		true,
	))
	f.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(price),
		bindingInteger(5),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))
	f.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(applicable),
		semantic.DefinedBindingValue(value.NewTyped(value.NewBool(true))),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))

	source := ast.NewTraversalSourceFromEntity(
		address.EntityRef(f.entityID),
		ast.TraversalDirectionOutgoing,
		relationTypeID,
	)
	total := ast.NewMappedAggregate(
		source,
		ast.NewBoundPropertyDefinitionReference(
			ast.BoundTargetEntity,
			price.DefinitionID(),
		),
		ast.AggregateOperationSum,
	)
	constraintID, ruleID := identity.New(), identity.New()
	f.draft.AddConstraint(constraint.New(
		constraintID,
		"total within limit",
		nil,
		false,
		ast.LessOrEqual(total, integer(5)),
		constraint.SeverityHard,
		"",
		false,
	))
	f.draft.AddRule(domainrule.New(
		ruleID,
		"total above threshold",
		ast.Greater(total, integer(4)),
	))
	version := f.commit(t)
	context := reviewContext()
	if dependencies, ok := version.ConstraintDependencies(constraintID); !ok ||
		!containsDependency(dependencies.All(), price) ||
		!containsDependency(dependencies.All(), applicable) {
		t.Fatalf("explicit-origin constraint dependencies = %#v", dependencies)
	}
	if dependencies, ok := version.RuleDependencies(ruleID); !ok ||
		!containsDependency(dependencies.Condition(), price) ||
		!containsDependency(dependencies.Condition(), applicable) {
		t.Fatalf("explicit-origin rule dependencies = %#v", dependencies)
	}

	constraintResult, err := check.One(version, constraintID, context)
	if err != nil || constraintResult.State() != check.StateSatisfied {
		t.Fatalf("explicit-origin constraint = %s, %v", constraintResult.State(), err)
	}
	if feasibility.Assess(checkReport(t, version, context)).State() != feasibility.StateFeasible {
		t.Fatal("satisfied explicit-origin hard constraint must be feasible")
	}
	ruleResult, err := ruleeval.One(version, ruleID, context)
	if err != nil || ruleResult.State() != ruleeval.StateActive {
		t.Fatalf("explicit-origin rule = %s, %v", ruleResult.State(), err)
	}
	scenario := domainscenario.New(
		identity.New(),
		"aggregate input",
		version.ID(),
		[]domainscenario.Override{domainscenario.NewOverride(
			price,
			bindingInteger(3),
			nil,
			domaintime.EffectivePeriod{},
			false,
		)},
	)
	ruleResult, err = ruleeval.One(version, ruleID, context.WithScenario(scenario))
	if err != nil || ruleResult.State() != ruleeval.StateInactive {
		t.Fatalf("scenario aggregate override = %s, %v", ruleResult.State(), err)
	}
	calculationContext := runtime.NewCalculationContext(
		context.TimeContext(),
		scope.Values{},
		[]runtime.Override{runtime.NewOverride(price, bindingInteger(6))},
	).WithScenario(scenario)
	ruleResult, err = ruleeval.One(version, ruleID, calculationContext)
	if err != nil || ruleResult.State() != ruleeval.StateActive {
		t.Fatalf("calculation aggregate precedence = %s, %v", ruleResult.State(), err)
	}

	falseContext := runtime.NewCalculationContext(
		context.TimeContext(),
		scope.Values{},
		[]runtime.Override{runtime.NewOverride(
			applicable,
			semantic.DefinedBindingValue(value.NewTyped(value.NewBool(false))),
		)},
	)
	ruleResult, err = ruleeval.One(version, ruleID, falseContext)
	if err != nil || ruleResult.State() != ruleeval.StateInactive ||
		provenanceContains(ruleResult.Condition().Provenance(), price) {
		t.Fatalf("inactive relation evaluated map = %s, %v", ruleResult.State(), err)
	}

	nullContext := runtime.NewCalculationContext(
		context.TimeContext(),
		scope.Values{},
		[]runtime.Override{runtime.NewOverride(applicable, semantic.NullBindingValue())},
	)
	constraintResult, err = check.One(version, constraintID, nullContext)
	if err != nil || constraintResult.State() != check.StateUndefined {
		t.Fatalf("indeterminate applicability constraint = %s, %v", constraintResult.State(), err)
	}
	if feasibility.Assess(checkReport(t, version, nullContext)).State() != feasibility.StateUndetermined {
		t.Fatal("undefined explicit-origin hard constraint must be undetermined")
	}
	ruleResult, err = ruleeval.One(version, ruleID, nullContext)
	if err != nil || ruleResult.State() != ruleeval.StateUndefined {
		t.Fatalf("indeterminate applicability rule = %s, %v", ruleResult.State(), err)
	}
}

func checkReport(
	t *testing.T,
	version model.ModelVersion,
	context runtime.CalculationContext,
) check.Report {
	t.Helper()
	report, err := check.All(version, context)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestNestedAggregateUsesBoundEntityOriginAndPreservesMultiplicity(t *testing.T) {
	draft := model.NewDraft()
	rootTypeID, outerTypeID, innerTypeID := identity.New(), identity.New(), identity.New()
	outerRelationTypeID, innerRelationTypeID := identity.New(), identity.New()
	innerValueID, totalID := identity.New(), identity.New()
	rootID, outerOneID, outerTwoID := identity.New(), identity.New(), identity.New()
	innerOneID, innerTwoID, innerThreeID := identity.New(), identity.New(), identity.New()

	innerValue := schema.NewPropertyDefinition(
		innerValueID,
		schema.EntityTypeOwner(innerTypeID),
		identity.UUID{},
		false,
		"value",
		value.Integer,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.InputComputation(),
	)
	innerAggregate := ast.NewMappedAggregate(
		ast.NewTraversalSourceFromBoundEntity(
			ast.TraversalDirectionOutgoing,
			innerRelationTypeID,
		),
		ast.NewBoundPropertyDefinitionReference(
			ast.BoundTargetEntity,
			innerValueID,
		),
		ast.AggregateOperationSum,
	)
	total := schema.NewPropertyDefinition(
		totalID,
		schema.EntityTypeOwner(rootTypeID),
		identity.UUID{},
		false,
		"total",
		value.Integer,
		unit.Unit{},
		false,
		schema.TemporalScalar,
		schema.DerivedComputation(ast.NewMappedAggregate(
			ast.NewTraversalSource(ast.TraversalDirectionOutgoing, outerRelationTypeID),
			innerAggregate,
			ast.AggregateOperationSum,
		)),
	)
	draft.AddEntityType(schema.NewEntityType(rootTypeID, "root", nil, []identity.UUID{totalID}))
	draft.AddEntityType(schema.NewEntityType(outerTypeID, "outer", nil, nil))
	draft.AddEntityType(schema.NewEntityType(innerTypeID, "inner", nil, []identity.UUID{innerValueID}))
	draft.AddPropertyDefinition(innerValue)
	draft.AddPropertyDefinition(total)
	draft.AddRelationType(schema.NewRelationType(
		outerRelationTypeID,
		"root-outer",
		schema.EntityTypeConstraint(rootTypeID),
		true,
		schema.EntityTypeConstraint(outerTypeID),
		true,
		nil,
	))
	draft.AddRelationType(schema.NewRelationType(
		innerRelationTypeID,
		"outer-inner",
		schema.EntityTypeConstraint(outerTypeID),
		true,
		schema.EntityTypeConstraint(innerTypeID),
		true,
		nil,
	))
	draft.AddEntity(semantic.NewEntity(rootID, rootTypeID, "", false))
	draft.AddEntity(semantic.NewEntity(outerOneID, outerTypeID, "", false))
	draft.AddEntity(semantic.NewEntity(outerTwoID, outerTypeID, "", false))
	draft.AddEntity(semantic.NewEntity(innerOneID, innerTypeID, "", false))
	draft.AddEntity(semantic.NewEntity(innerTwoID, innerTypeID, "", false))
	draft.AddEntity(semantic.NewEntity(innerThreeID, innerTypeID, "", false))
	// Distinct Relation identities to outerOne intentionally count twice.
	draft.AddRelation(semantic.NewRelation(identity.New(), outerRelationTypeID, rootID, outerOneID, nil, false))
	draft.AddRelation(semantic.NewRelation(identity.New(), outerRelationTypeID, rootID, outerTwoID, nil, false))
	draft.AddRelation(semantic.NewRelation(identity.New(), outerRelationTypeID, rootID, outerOneID, nil, false))
	draft.AddRelation(semantic.NewRelation(identity.New(), innerRelationTypeID, outerOneID, innerOneID, nil, false))
	draft.AddRelation(semantic.NewRelation(identity.New(), innerRelationTypeID, outerOneID, innerTwoID, nil, false))
	draft.AddRelation(semantic.NewRelation(identity.New(), innerRelationTypeID, outerTwoID, innerThreeID, nil, false))
	for _, binding := range []struct {
		entityID identity.UUID
		number   int64
	}{
		{innerOneID, 2},
		{innerTwoID, 3},
		{innerThreeID, 4},
	} {
		draft.AddValueBinding(semantic.NewValueBinding(
			semantic.ObjectPropertyTarget(address.NewPropertyAddress(address.EntityRef(binding.entityID), innerValueID)),
			bindingInteger(binding.number),
			nil,
			domaintime.EffectivePeriod{},
			false,
		))
	}

	version := reviewCommit(t, draft)
	result, err := evaluator.Evaluate(
		version,
		address.NewPropertyAddress(address.EntityRef(rootID), totalID),
		reviewContext(),
	)
	if err != nil || !result.Value().IsDefined() || result.Value().Value().Raw().Integer().Int64() != 14 {
		t.Fatalf("nested aggregate = %#v, %v", result.Value(), err)
	}
	if got := countAggregateItems(result.Provenance()); got != 8 {
		t.Fatalf("aggregate provenance items = %d, want 8", got)
	}
}

func TestAggregateProvenanceRetainsRelationAndEntityIdentity(t *testing.T) {
	f := newPipelineFixture()
	input := f.property(value.Integer, schema.InputComputation(), false)
	relationTypeID, relationID := identity.New(), identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(relationTypeID, "links", endpoint, true, endpoint, true, nil))
	f.draft.AddRelation(semantic.NewRelation(relationID, relationTypeID, f.entityID, f.entityID, nil, false))
	total := f.property(value.Integer, schema.DerivedComputation(ast.NewMappedAggregate(
		ast.NewTraversalSource(ast.TraversalDirectionBoth, relationTypeID),
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, input.DefinitionID()),
		ast.AggregateOperationSum,
	)), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(input),
		bindingInteger(7),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))
	result, err := evaluator.Evaluate(f.commit(t), total, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	relation, entity, found := firstAggregateItem(result.Provenance())
	if !found || relation != address.RelationRef(relationID) || entity != address.EntityRef(f.entityID) {
		t.Fatalf("aggregate item identity = %v / %v / %v", relation, entity, found)
	}
}

func firstAggregateItem(node provenance.Node) (address.ObjectRef, address.ObjectRef, bool) {
	if node.Kind() == provenance.KindAggregateItem {
		return node.AggregateItem()
	}
	for _, child := range node.Children() {
		if relation, entity, found := firstAggregateItem(child); found {
			return relation, entity, true
		}
	}
	return address.ObjectRef{}, address.ObjectRef{}, false
}

func countAggregateItems(node provenance.Node) int {
	count := 0
	if node.Kind() == provenance.KindAggregateItem {
		count++
	}
	for _, child := range node.Children() {
		count += countAggregateItems(child)
	}
	return count
}

func containsDependency(
	dependencies []address.PropertyAddress,
	target address.PropertyAddress,
) bool {
	for _, dependency := range dependencies {
		if dependency == target {
			return true
		}
	}
	return false
}
