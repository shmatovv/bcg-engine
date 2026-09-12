package evaluator_test

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

var benchmarkSessionResult evaluator.Result

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
