package evaluator_test

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	ruleeval "github.com/shmatovv/bcg-engine/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/value"
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
