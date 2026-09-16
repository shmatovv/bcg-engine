package evaluator

import (
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/plan"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
)

// Source is the evaluator-owned composition of planning and inference
// capabilities. It remains intentionally consumer-scoped; no repository-wide
// mega source interface is introduced.
type Source interface {
	plan.Source
	inference.Resolver
	ModelVersion() model.ModelVersion
	PrecisionPolicy() precision.Policy
}

func dependencyGraph(source Source) (dependency.Graph, bool) {
	value, ok := source.(interface{ DependencyGraph() dependency.Graph })
	if !ok {
		return dependency.Graph{}, false
	}
	return value.DependencyGraph(), true
}
