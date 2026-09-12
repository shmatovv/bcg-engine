package evaluator

import (
	"github.com/shmatovv/bcg-engine/internal/provenance"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// Result is one scalar evaluation result together with complete provenance
// for the path that produced it.
type Result struct {
	value value.Resolved

	provenance provenance.Node
}

func newResult(
	resolved value.Resolved,
	provenanceNode provenance.Node,
) Result {
	if !resolved.IsValid() {
		panic(
			"evaluator: result value is invalid",
		)
	}

	if !provenanceNode.IsValid() {
		panic(
			"evaluator: result provenance is invalid",
		)
	}

	return Result{
		value:      resolved,
		provenance: provenanceNode.WithResolved(resolved),
	}
}

func (r Result) Value() value.Resolved {
	return r.value
}

func (r Result) Provenance() provenance.Node {
	return r.provenance
}

func (r Result) IsValid() bool {
	return r.value.IsValid() &&
		r.provenance.IsValid()
}
