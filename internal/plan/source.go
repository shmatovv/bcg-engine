package plan

import (
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
)

// Source is the composed capability required for effective planning. It does
// not introduce a new data container: ModelVersion and EvaluationSource both
// satisfy these consumer-owned contracts.
type Source interface {
	dependency.Source
	resolvervalue.ModelSource
}
