package evaluator

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/v2/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/plan"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
)

// Source is the evaluator-owned composition of planning and inference
// capabilities. It remains intentionally consumer-scoped; no repository-wide
// mega source interface is introduced.
type Source interface {
	plan.Source
	inference.Resolver
	ModelVersion() model.ModelVersion
	PrecisionPolicy() precision.Policy
	// These declaration accessors let a runtime-bound source preserve the
	// existing Constraint/Rule entry points without carrying callbacks in the
	// semantic Model itself.
	Constraint(identity.UUID) (constraint.Constraint, bool)
	Constraints() []constraint.Constraint
	Rule(identity.UUID) (rule.Rule, bool)
	Rules() []rule.Rule
	Invariant(identity.UUID) (invariant.Invariant, bool)
	Invariants() []invariant.Invariant
}

func dependencyGraph(source Source) (dependency.Graph, bool) {
	value, ok := source.(interface{ DependencyGraph() dependency.Graph })
	if !ok {
		return dependency.Graph{}, false
	}
	return value.DependencyGraph(), true
}
