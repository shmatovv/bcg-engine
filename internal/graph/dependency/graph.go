package dependency

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// Edge points from the property being read to the property whose expression
// reads it. Semantic relations alone do not create value dependencies.
type Edge struct {
	from address.PropertyAddress
	to   address.PropertyAddress
}

func newEdge(
	from address.PropertyAddress,
	to address.PropertyAddress,
) Edge {
	if !from.IsValid() {
		panic("dependency: edge source address must be valid")
	}

	if !to.IsValid() {
		panic("dependency: edge target address must be valid")
	}

	return Edge{
		from: from,
		to:   to,
	}
}

// From returns the dependency side of the edge.
func (e Edge) From() address.PropertyAddress {
	return e.from
}

// To returns the dependent side of the edge.
func (e Edge) To() address.PropertyAddress {
	return e.to
}

// IsValid reports whether e is structurally valid.
func (e Edge) IsValid() bool {
	return e.from.IsValid() &&
		e.to.IsValid()
}

// Graph is the concrete value dependency graph of one immutable model
// snapshot.
//
// Vertices are concrete PropertyAddress values.
//
// Edges are oriented:
//
//	dependency -> dependent
//
// Graph is immutable outside package dependency.
type Graph struct {
	vertices map[address.PropertyAddress]struct{}

	outgoing map[address.PropertyAddress]map[address.PropertyAddress]struct{}
	incoming map[address.PropertyAddress]map[address.PropertyAddress]struct{}
}

func newGraph() Graph {
	return Graph{
		vertices: make(
			map[address.PropertyAddress]struct{},
		),

		outgoing: make(
			map[address.PropertyAddress]map[address.PropertyAddress]struct{},
		),

		incoming: make(
			map[address.PropertyAddress]map[address.PropertyAddress]struct{},
		),
	}
}

func (g *Graph) addVertex(
	vertex address.PropertyAddress,
) {
	if g == nil {
		panic("dependency: nil graph")
	}

	if !vertex.IsValid() {
		panic("dependency: graph vertex must be valid")
	}

	g.vertices[vertex] = struct{}{}
}

func (g *Graph) addDependency(
	dependencyAddress address.PropertyAddress,
	dependentAddress address.PropertyAddress,
) {
	if g == nil {
		panic("dependency: nil graph")
	}

	if !dependencyAddress.IsValid() {
		panic("dependency: dependency address must be valid")
	}

	if !dependentAddress.IsValid() {
		panic("dependency: dependent address must be valid")
	}

	g.addVertex(
		dependencyAddress,
	)

	g.addVertex(
		dependentAddress,
	)

	dependents, ok :=
		g.outgoing[dependencyAddress]

	if !ok {
		dependents = make(
			map[address.PropertyAddress]struct{},
		)

		g.outgoing[dependencyAddress] =
			dependents
	}

	dependents[dependentAddress] =
		struct{}{}

	dependencies, ok :=
		g.incoming[dependentAddress]

	if !ok {
		dependencies = make(
			map[address.PropertyAddress]struct{},
		)

		g.incoming[dependentAddress] =
			dependencies
	}

	dependencies[dependencyAddress] =
		struct{}{}
}

// Len returns the number of concrete property vertices.
func (g Graph) Len() int {
	return len(g.vertices)
}

// EdgeLen returns the number of unique direct dependency edges.
func (g Graph) EdgeLen() int {
	count := 0

	for _, dependents := range g.outgoing {
		count += len(dependents)
	}

	return count
}

// HasVertex reports whether propertyAddress belongs to the graph.
func (g Graph) HasVertex(
	propertyAddress address.PropertyAddress,
) bool {
	_, ok := g.vertices[propertyAddress]

	return ok
}

func (g Graph) hasDirectDependency(
	from address.PropertyAddress,
	to address.PropertyAddress,
) bool {
	dependents := g.outgoing[from]

	_, ok := dependents[to]

	return ok
}

// Vertices returns every graph vertex in deterministic canonical order.
func (g Graph) Vertices() []address.PropertyAddress {
	result := make(
		[]address.PropertyAddress,
		0,
		len(g.vertices),
	)

	for vertex := range g.vertices {
		result = append(
			result,
			vertex,
		)
	}

	sortPropertyAddresses(
		result,
	)

	return result
}

// Edges returns every dependency edge in deterministic order.
func (g Graph) Edges() []Edge {
	result := make(
		[]Edge,
		0,
		g.EdgeLen(),
	)

	for from, dependents := range g.outgoing {

		for to := range dependents {
			result = append(
				result,
				newEdge(
					from,
					to,
				),
			)
		}
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			fromComparison :=
				comparePropertyAddress(
					result[i].from,
					result[j].from,
				)

			if fromComparison != 0 {
				return fromComparison < 0
			}

			return comparePropertyAddress(
				result[i].to,
				result[j].to,
			) < 0
		},
	)

	return result
}

// DependenciesOf returns every property directly read by target.
func (g Graph) DependenciesOf(
	target address.PropertyAddress,
) []address.PropertyAddress {
	dependencies :=
		g.incoming[target]

	result := make(
		[]address.PropertyAddress,
		0,
		len(dependencies),
	)

	for dependencyAddress := range dependencies {

		result = append(
			result,
			dependencyAddress,
		)
	}

	sortPropertyAddresses(
		result,
	)

	return result
}

// DependentsOf returns every property that directly reads source.
func (g Graph) DependentsOf(
	source address.PropertyAddress,
) []address.PropertyAddress {
	dependents :=
		g.outgoing[source]

	result := make(
		[]address.PropertyAddress,
		0,
		len(dependents),
	)

	for dependentAddress := range dependents {

		result = append(
			result,
			dependentAddress,
		)
	}

	sortPropertyAddresses(
		result,
	)

	return result
}

func sortPropertyAddresses(
	addresses []address.PropertyAddress,
) {
	sort.Slice(
		addresses,
		func(i, j int) bool {
			return comparePropertyAddress(
				addresses[i],
				addresses[j],
			) < 0
		},
	)
}

func comparePropertyAddress(
	left address.PropertyAddress,
	right address.PropertyAddress,
) int {
	if left.Object().Kind() <
		right.Object().Kind() {
		return -1
	}

	if left.Object().Kind() >
		right.Object().Kind() {
		return 1
	}

	if result := compareUUID(
		left.Object().ID(),
		right.Object().ID(),
	); result != 0 {
		return result
	}

	return compareUUID(
		left.DefinitionID(),
		right.DefinitionID(),
	)
}

func compareUUID(
	left identity.UUID,
	right identity.UUID,
) int {
	leftBytes := left.Bytes()
	rightBytes := right.Bytes()

	for index := range leftBytes {
		switch {
		case leftBytes[index] <
			rightBytes[index]:
			return -1

		case leftBytes[index] >
			rightBytes[index]:
			return 1
		}
	}

	return 0
}
