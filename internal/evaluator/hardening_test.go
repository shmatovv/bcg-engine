package evaluator_test

import (
	"errors"
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/impact"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// TestEvaluateDeepPropertyChain is a release-hardening regression test. It
// exercises a genuinely committed model rather than a synthetic graph, so
// validation, dependency construction, evaluation, provenance and the
// Session cache all encounter the same deep concrete dependency path.
func TestEvaluateDeepPropertyChain(t *testing.T) {
	const safeDepth = 128

	fixture := newPipelineFixture()
	previous := fixture.property(value.Integer, schema.InputComputation(), false)
	fixture.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(previous),
		bindingInteger(1),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))

	for index := 0; index < safeDepth; index++ {
		previous = fixture.property(
			value.Integer,
			schema.DerivedComputation(ast.Add(ref(previous), integer(1))),
			false,
		)
	}

	version := fixture.commit(t)
	cold, err := evaluator.Evaluate(version, previous, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if got := sessionValue(t, cold); got != safeDepth+1 {
		t.Fatalf("deep cold result = %d, want %d", got, safeDepth+1)
	}

	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}
	warm, err := session.Evaluate(previous, reviewContext())
	if err != nil {
		t.Fatal(err)
	}
	if got := sessionValue(t, warm); got != safeDepth+1 {
		t.Fatalf("deep session result = %d, want %d", got, safeDepth+1)
	}
	if session.Stats().Entries() != safeDepth+1 {
		t.Fatalf("deep session entries = %d, want %d", session.Stats().Entries(), safeDepth+1)
	}
}

func TestEvaluateTooDeepPropertyChainReturnsControlledError(t *testing.T) {
	// This is deliberately much deeper than the evaluator limit. It also
	// exercises iterative effective-plan construction before evaluation reaches
	// its controlled depth boundary.
	const depth = 20_000

	fixture := newPipelineFixture()
	previous := fixture.property(value.Integer, schema.InputComputation(), false)
	fixture.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(previous),
		bindingInteger(1),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))
	for index := 0; index < depth; index++ {
		previous = fixture.property(
			value.Integer,
			schema.DerivedComputation(ast.Add(ref(previous), integer(1))),
			false,
		)
	}

	version := fixture.commit(t)
	_, err := evaluator.Evaluate(version, previous, reviewContext())
	var evaluationError evaluator.Error
	if !errors.As(err, &evaluationError) || evaluationError.Kind() != evaluator.ErrorEvaluationDepthExceeded {
		t.Fatalf("deep evaluation error = %v, want controlled EvaluationDepthExceeded", err)
	}
}

func TestValidateRejectsTooDeepExpressionBeforeRecursiveInference(t *testing.T) {
	var expression ast.Expression = integer(1)
	for index := 0; index < 5_000; index++ {
		expression = ast.NewNegate(expression)
	}

	fixture := newPipelineFixture()
	fixture.property(value.Integer, schema.DerivedComputation(expression), false)
	fixture.draft.AddEntityType(schema.NewEntityType(fixture.typeID, "item", nil, fixture.ids))
	fixture.draft.AddEntity(semantic.NewEntity(fixture.entityID, fixture.typeID, "", false))

	report := fixture.draft.Validate()
	if !report.Has(model.ValidationIssueInvalidDerivedExpression) {
		t.Fatalf("deep expression validation issues = %#v", report.Issues())
	}
}

func TestSessionWideIndependentBranchesPreserveUnrelatedEntries(t *testing.T) {
	const branches = 1_000

	fixture := newPipelineFixture()
	inputs := make([]address.PropertyAddress, branches)
	targets := make([]address.PropertyAddress, branches)
	for index := 0; index < branches; index++ {
		inputs[index] = fixture.property(value.Integer, schema.InputComputation(), false)
		targets[index] = fixture.property(
			value.Integer,
			schema.DerivedComputation(ast.Add(ref(inputs[index]), integer(1))),
			false,
		)
		fixture.draft.AddValueBinding(semantic.NewValueBinding(
			semantic.ObjectPropertyTarget(inputs[index]),
			bindingInteger(int64(index)),
			nil,
			domaintime.EffectivePeriod{},
			false,
		))
	}

	version := fixture.commit(t)
	session, err := evaluator.NewSession(version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.EvaluateMany(targets, reviewContext()); err != nil {
		t.Fatal(err)
	}
	if entries := session.Stats().Entries(); entries != 2*branches {
		t.Fatalf("wide session entries = %d, want %d", entries, 2*branches)
	}

	changed := runtime.NewCalculationContext(
		reviewContext().TimeContext(),
		reviewContext().ScopeValues(),
		[]runtime.Override{runtime.NewOverride(inputs[0], bindingInteger(10_000))},
	)
	if result, err := session.Evaluate(targets[0], changed); err != nil || sessionValue(t, result) != 10_001 {
		t.Fatalf("changed wide branch = %#v, %v", result, err)
	}
	before := session.Stats()
	if result, err := session.Evaluate(targets[branches-1], changed); err != nil || sessionValue(t, result) != branches {
		t.Fatalf("unrelated wide branch = %#v, %v", result, err)
	}
	after := session.Stats()
	if after.Hits() <= before.Hits() || after.Misses() != before.Misses() {
		t.Fatalf("unrelated wide branch was not reused: before=%#v after=%#v", before, after)
	}

	structural, err := impact.AnalyzeStructural(version, inputs[0])
	if err != nil || structural.Len() != 1 || structural.Affected()[0].Target() != targets[0] {
		t.Fatalf("wide structural impact = %#v, %v", structural, err)
	}
}
