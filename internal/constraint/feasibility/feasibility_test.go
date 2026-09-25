package feasibility_test

import (
	"sync"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/v2/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func testContext(t *testing.T) runtime.CalculationContext {
	t.Helper()
	instant, err := datetime.Parse("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return runtime.NewCalculationContext(domaintime.At(instant), scope.Values{}, nil)
}

func commit(t *testing.T, constraints ...constraint.Constraint) model.ModelVersion {
	t.Helper()
	draft := model.NewDraft()
	for _, modelConstraint := range constraints {
		draft.AddConstraint(modelConstraint)
	}
	version, err := model.Commit(model.NewModel(identity.New(), "feasibility"), draft, model.NewCommitOptions(identity.New(), "feasibility", precision.New(6, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func boolExpr(flag bool) ast.Expression { return ast.NewConstant(value.NewTyped(value.NewBool(flag))) }
func intExpr(number int64) ast.Expression {
	return ast.NewConstant(value.NewTyped(value.NewInt64(number)))
}
func makeConstraint(id identity.UUID, severity constraint.Severity, applicability ast.Expression, hasApplicability bool, assertion ast.Expression) constraint.Constraint {
	return constraint.New(id, severity.String(), applicability, hasApplicability, assertion, severity, "", false)
}

func TestEvaluateFeasibilityStatesAndCompleteReport(t *testing.T) {
	t.Run("no constraints", func(t *testing.T) {
		result, err := feasibility.Evaluate(commit(t), testContext(t))
		if err != nil || result.State() != feasibility.StateFeasible || result.Report().Len() != 0 || !result.IsValid() {
			t.Fatalf("result = %#v, %v", result, err)
		}
	})
	t.Run("only advisory violations and inactive hard are feasible", func(t *testing.T) {
		version := commit(t,
			makeConstraint(identity.New(), constraint.SeverityInfo, nil, false, boolExpr(false)),
			makeConstraint(identity.New(), constraint.SeverityWarning, nil, false, boolExpr(false)),
			makeConstraint(identity.New(), constraint.SeverityError, nil, false, boolExpr(false)),
			makeConstraint(identity.New(), constraint.SeverityHard, boolExpr(false), true, boolExpr(false)),
		)
		result, err := feasibility.Evaluate(version, testContext(t))
		if err != nil || result.State() != feasibility.StateFeasible {
			t.Fatalf("advisory/inactive result = %v, %v", result.State(), err)
		}
	})

	undefinedID, violatedID := identity.New(), identity.New()
	version := commit(t,
		makeConstraint(identity.New(), constraint.SeverityInfo, nil, false, boolExpr(false)),
		makeConstraint(identity.New(), constraint.SeverityWarning, nil, false, boolExpr(false)),
		makeConstraint(identity.New(), constraint.SeverityError, nil, false, boolExpr(false)),
		makeConstraint(identity.New(), constraint.SeverityHard, nil, false, boolExpr(true)),
		makeConstraint(identity.New(), constraint.SeverityHard, boolExpr(false), true, ast.Greater(ast.Divide(intExpr(1), intExpr(0)), intExpr(0))),
		makeConstraint(undefinedID, constraint.SeverityHard, nil, false, ast.Greater(ast.Divide(intExpr(1), intExpr(0)), intExpr(0))),
		makeConstraint(violatedID, constraint.SeverityHard, nil, false, boolExpr(false)),
	)
	result, err := feasibility.Evaluate(version, testContext(t))
	if err != nil || result.State() != feasibility.StateInfeasible || !result.IsValid() {
		t.Fatalf("infeasible result = %#v, %v", result, err)
	}
	if result.Report().Len() != 7 {
		t.Fatalf("incomplete report: %d", result.Report().Len())
	}
	if got := result.ViolatedHard(); len(got) != 1 || got[0] != violatedID {
		t.Fatalf("violated witnesses = %v", got)
	}
	if got := result.UndefinedHard(); len(got) != 1 || got[0] != undefinedID {
		t.Fatalf("undefined witnesses = %v", got)
	}
	for _, item := range result.Report().Results() {
		if item.Constraint().ID() == violatedID && item.State() != check.StateViolated {
			t.Fatal("violated hard omitted or misclassified")
		}
	}

	undetermined, err := feasibility.Evaluate(commit(t, makeConstraint(undefinedID, constraint.SeverityHard, nil, false, ast.Greater(ast.Divide(intExpr(1), intExpr(0)), intExpr(0)))), testContext(t))
	if err != nil || undetermined.State() != feasibility.StateUndetermined {
		t.Fatalf("undetermined = %v, %v", undetermined.State(), err)
	}
	nonHardUndefined, err := feasibility.Evaluate(commit(t, makeConstraint(identity.New(), constraint.SeverityError, nil, false, ast.Greater(ast.Divide(intExpr(1), intExpr(0)), intExpr(0)))), testContext(t))
	if err != nil || nonHardUndefined.State() != feasibility.StateFeasible {
		t.Fatalf("non-hard Undefined = %v, %v", nonHardUndefined.State(), err)
	}
}

func TestFeasibilityOwnershipDeterminismAndConcurrency(t *testing.T) {
	constraints := make([]constraint.Constraint, 0, 48)
	for index := 0; index < 48; index++ {
		constraints = append(constraints, makeConstraint(identity.New(), constraint.SeverityHard, nil, false, boolExpr(index%3 != 0)))
	}
	version := commit(t, constraints...)
	context := testContext(t)
	result, err := feasibility.Evaluate(version, context)
	if err != nil || result.State() != feasibility.StateInfeasible {
		t.Fatalf("result = %v, %v", result.State(), err)
	}
	mutated := result.ViolatedHard()
	mutated[0] = identity.UUID{}
	if !result.IsValid() || result.ViolatedHard()[0].IsZero() {
		t.Fatal("feasibility exposes mutable witnesses")
	}
	for index := 0; index < 100; index++ {
		again, err := feasibility.Evaluate(version, context)
		if err != nil || again.State() != result.State() || len(again.ViolatedHard()) != len(result.ViolatedHard()) {
			t.Fatalf("nondeterministic feasibility: %v", err)
		}
	}
	var group sync.WaitGroup
	for index := 0; index < 32; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := feasibility.Evaluate(version, context); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
}
