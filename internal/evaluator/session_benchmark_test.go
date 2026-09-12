package evaluator_test

import (
	"fmt"
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/impact"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

var benchmarkSessionResult evaluator.Result
var benchmarkSessionVersion model.ModelVersion

func benchmarkCommit(f *pipelineFixture) model.ModelVersion {
	f.draft.AddEntityType(schema.NewEntityType(f.typeID, "item", nil, f.ids))
	f.draft.AddEntity(semantic.NewEntity(f.entityID, f.typeID, "", false))
	version, err := model.Commit(
		model.NewModel(identity.New(), "benchmark"),
		f.draft,
		model.NewCommitOptions(identity.New(), "benchmark", precision.New(6, precision.HalfEven)),
	)
	if err != nil {
		panic(err)
	}
	return version
}

func benchmarkDAG() (model.ModelVersion, address.PropertyAddress, address.PropertyAddress, address.PropertyAddress) {
	f := newPipelineFixture()
	a := f.property(value.Integer, schema.InputComputation(), false)
	b := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(a), integer(1))), false)
	c := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(b), ref(b))), false)
	x := f.property(value.Integer, schema.InputComputation(), false)
	y := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(x), integer(1))), false)
	z := f.property(value.Integer, schema.DerivedComputation(ast.Add(ref(y), integer(1))), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(a), bindingInteger(1), nil, domaintime.EffectivePeriod{}, false))
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(x), bindingInteger(10), nil, domaintime.EffectivePeriod{}, false))
	return benchmarkCommit(f), a, c, z
}

func BenchmarkSessionColdWarmDAG(b *testing.B) {
	version, _, target, _ := benchmarkDAG()
	context := reviewContext()

	b.Run("cold", func(b *testing.B) {
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			session, err := evaluator.NewSession(version)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkSessionResult, err = session.Evaluate(target, context)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("warm", func(b *testing.B) {
		session, err := evaluator.NewSession(version)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := session.Evaluate(target, context); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			benchmarkSessionResult, err = session.Evaluate(target, context)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkSessionIncrementalOneBranch(b *testing.B) {
	version, input, target, independent := benchmarkDAG()
	session, err := evaluator.NewSession(version)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := session.Evaluate(target, reviewContext()); err != nil {
		b.Fatal(err)
	}
	if _, err := session.Evaluate(independent, reviewContext()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		context := runtime.NewCalculationContext(
			reviewContext().TimeContext(),
			reviewContext().ScopeValues(),
			[]runtime.Override{runtime.NewOverride(input, bindingInteger(int64(index&1)))},
		)
		benchmarkSessionResult, err = session.Evaluate(target, context)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkSessionResult, err = session.Evaluate(independent, context)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionWarmAggregate(b *testing.B) {
	f := newPipelineFixture()
	price := f.property(value.Integer, schema.InputComputation(), false)
	relationType := identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(relationType, "links", endpoint, true, endpoint, true, nil))
	for index := 0; index < 32; index++ {
		f.draft.AddRelation(semantic.NewRelation(identity.New(), relationType, f.entityID, f.entityID, boolean(true), true))
	}
	total := f.property(value.Integer, schema.DerivedComputation(ast.NewMappedAggregate(
		ast.NewTraversalSource(ast.TraversalDirectionOutgoing, relationType),
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, price.DefinitionID()),
		ast.AggregateOperationSum,
	)), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(price), bindingInteger(5), nil, domaintime.EffectivePeriod{}, false))
	version := benchmarkCommit(f)
	session, err := evaluator.NewSession(version)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := session.Evaluate(total, reviewContext()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		benchmarkSessionResult, err = session.Evaluate(total, reviewContext())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAggregateTraversal exercises the indexed one-hop aggregate path at
// release-relevant relation counts. Cold runs include aggregate traversal;
// warm runs make cache reuse explicit. There are intentionally no timing
// assertions: the benchmark is a regression visibility tool, not a contract.
func BenchmarkAggregateTraversal(b *testing.B) {
	for _, relations := range []int{100, 1_000, 10_000, 100_000} {
		relations := relations
		version, total := benchmarkAggregateVersion(b, relations)
		context := reviewContext()

		b.Run(fmt.Sprintf("cold/%d", relations), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				session, err := evaluator.NewSession(version)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSessionResult, err = session.Evaluate(total, context)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("warm/%d", relations), func(b *testing.B) {
			session, err := evaluator.NewSession(version)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := session.Evaluate(total, context); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				benchmarkSessionResult, err = session.Evaluate(total, context)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkAggregateVersion(b *testing.B, relations int) (model.ModelVersion, address.PropertyAddress) {
	b.Helper()

	f := newPipelineFixture()
	price := f.property(value.Integer, schema.InputComputation(), false)
	relationType := identity.New()
	endpoint := schema.EntityTypeConstraint(f.typeID)
	f.draft.AddRelationType(schema.NewRelationType(relationType, "links", endpoint, true, endpoint, true, nil))
	for index := 0; index < relations; index++ {
		f.draft.AddRelation(semantic.NewRelation(identity.New(), relationType, f.entityID, f.entityID, boolean(true), true))
	}
	total := f.property(value.Integer, schema.DerivedComputation(ast.NewMappedAggregate(
		ast.NewTraversalSource(ast.TraversalDirectionOutgoing, relationType),
		ast.NewBoundPropertyDefinitionReference(ast.BoundTargetEntity, price.DefinitionID()),
		ast.AggregateOperationSum,
	)), false)
	f.draft.AddValueBinding(semantic.NewValueBinding(semantic.ObjectPropertyTarget(price), bindingInteger(5), nil, domaintime.EffectivePeriod{}, false))
	return benchmarkCommit(f), total
}

// BenchmarkCommitWideDAG100K is an explicit release-scale benchmark. It
// keeps construction outside the measured section and exercises validation,
// concrete dependency construction and cycle detection for 100,000 concrete
// scalar properties sharing one input dependency.
func BenchmarkCommitWideDAG100K(b *testing.B) {
	const nodes = 100_000

	fixture, _, _ := benchmarkWideDAGFixture(nodes)
	prepareWideDAGFixture(fixture)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		version, err := model.Commit(
			model.NewModel(identity.New(), "wide"),
			fixture.draft,
			model.NewCommitOptions(identity.New(), "wide", precision.New(6, precision.HalfEven)),
		)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkSessionVersion = version
	}
}

// BenchmarkWideDAG100KWorkflow proves the full large-model hot path without
// making a 100,000-node workload part of ordinary unit-test runtime.
func BenchmarkWideDAG100KWorkflow(b *testing.B) {
	const nodes = 100_000

	fixture, input, target := benchmarkWideDAGFixture(nodes)
	prepareWideDAGFixture(fixture)
	version, err := model.Commit(
		model.NewModel(identity.New(), "wide"),
		fixture.draft,
		model.NewCommitOptions(identity.New(), "wide", precision.New(6, precision.HalfEven)),
	)
	if err != nil {
		b.Fatal(err)
	}
	session, err := evaluator.NewSession(version)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := session.Evaluate(target, reviewContext()); err != nil {
		b.Fatal(err)
	}
	structural, err := impact.AnalyzeStructural(version, input)
	if err != nil {
		b.Fatal(err)
	}
	if structural.Len() != nodes-1 {
		b.Fatalf("wide structural impact size = %d, want %d", structural.Len(), nodes-1)
	}

	changed := runtime.NewCalculationContext(
		reviewContext().TimeContext(),
		reviewContext().ScopeValues(),
		[]runtime.Override{runtime.NewOverride(input, bindingInteger(2))},
	)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		benchmarkSessionResult, err = session.Evaluate(target, reviewContext())
		if err != nil {
			b.Fatal(err)
		}
		benchmarkSessionResult, err = session.Evaluate(target, changed)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkWideDAGFixture(nodes int) (*pipelineFixture, address.PropertyAddress, address.PropertyAddress) {
	fixture := newPipelineFixture()
	input := fixture.property(value.Integer, schema.InputComputation(), false)
	fixture.draft.AddValueBinding(semantic.NewValueBinding(
		semantic.ObjectPropertyTarget(input),
		bindingInteger(1),
		nil,
		domaintime.EffectivePeriod{},
		false,
	))
	target := input
	for index := 1; index < nodes; index++ {
		target = fixture.property(
			value.Integer,
			schema.DerivedComputation(ast.Add(ref(input), integer(1))),
			false,
		)
	}
	return fixture, input, target
}

func prepareWideDAGFixture(fixture *pipelineFixture) {
	fixture.draft.AddEntityType(schema.NewEntityType(fixture.typeID, "wide", nil, fixture.ids))
	fixture.draft.AddEntity(semantic.NewEntity(fixture.entityID, fixture.typeID, "", false))
}
