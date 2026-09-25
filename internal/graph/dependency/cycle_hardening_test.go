package dependency

import (
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
)

// TestFindCyclesHandlesVeryDeepAcyclicGraph is a release-hardening regression
// test for stack safety. Concrete model graphs can be deep even when each
// individual expression has only one property reference.
func TestFindCyclesHandlesVeryDeepAcyclicGraph(t *testing.T) {
	const vertices = 100_000

	graph := newGraph()
	entity := address.EntityRef(identity.New())
	previous := address.NewPropertyAddress(entity, identity.New())
	graph.addVertex(previous)

	for index := 1; index < vertices; index++ {
		current := address.NewPropertyAddress(entity, identity.New())
		graph.addDependency(previous, current)
		previous = current
	}

	if cycles := FindCycles(graph); len(cycles) != 0 {
		t.Fatalf("deep acyclic graph reported cycles: %#v", cycles)
	}
}
