package dependency

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
)

// Cycle represents one cyclic strongly connected component of the concrete
// dependency graph.
//
// A component with one vertex is returned only when that vertex has an
// explicit self-dependency.
//
// A component with two or more vertices is cyclic by definition.
//
// Vertices are stored in deterministic canonical order.
type Cycle struct {
	vertices []address.PropertyAddress
}

func newCycle(
	vertices []address.PropertyAddress,
) Cycle {
	vertices = append(
		[]address.PropertyAddress(nil),
		vertices...,
	)

	sortPropertyAddresses(
		vertices,
	)

	return Cycle{
		vertices: vertices,
	}
}

// Vertices returns every vertex belonging to the cyclic component.
func (c Cycle) Vertices() []address.PropertyAddress {
	return append(
		[]address.PropertyAddress(nil),
		c.vertices...,
	)
}

// Len returns the number of vertices in the cyclic component.
func (c Cycle) Len() int {
	return len(c.vertices)
}

// IsSelfCycle reports whether the cycle contains one property depending
// directly on itself.
func (c Cycle) IsSelfCycle() bool {
	return len(c.vertices) == 1
}

// FindCycles finds every cyclic strongly connected component.
//
// Tarjan's algorithm is used with deterministic vertex and adjacency order,
// so the result does not depend on Go map iteration order.
func FindCycles(
	graph Graph,
) []Cycle {
	state := tarjanState{
		graph: graph,

		indices: make(
			map[address.PropertyAddress]int,
		),

		lowLinks: make(
			map[address.PropertyAddress]int,
		),

		onStack: make(
			map[address.PropertyAddress]bool,
		),
	}

	for _, vertex := range graph.Vertices() {

		if _, visited :=
			state.indices[vertex]; visited {

			continue
		}

		state.strongConnect(
			vertex,
		)
	}

	sort.Slice(
		state.cycles,
		func(i, j int) bool {
			left :=
				state.cycles[i].
					vertices

			right :=
				state.cycles[j].
					vertices

			limit := len(left)

			if len(right) < limit {
				limit = len(right)
			}

			for index := 0; index < limit; index++ {

				comparison :=
					comparePropertyAddress(
						left[index],
						right[index],
					)

				if comparison != 0 {
					return comparison < 0
				}
			}

			return len(left) <
				len(right)
		},
	)

	return state.cycles
}

type tarjanState struct {
	graph Graph

	nextIndex int

	stack []address.PropertyAddress

	indices map[address.PropertyAddress]int

	lowLinks map[address.PropertyAddress]int

	onStack map[address.PropertyAddress]bool

	cycles []Cycle
}

func (s *tarjanState) strongConnect(
	vertex address.PropertyAddress,
) {
	index := s.nextIndex

	s.nextIndex++

	s.indices[vertex] =
		index

	s.lowLinks[vertex] =
		index

	s.stack = append(
		s.stack,
		vertex,
	)

	s.onStack[vertex] =
		true

	for _, dependent := range s.graph.DependentsOf(
		vertex,
	) {

		dependentIndex, visited :=
			s.indices[dependent]

		if !visited {
			s.strongConnect(
				dependent,
			)

			if s.lowLinks[dependent] <
				s.lowLinks[vertex] {

				s.lowLinks[vertex] =
					s.lowLinks[dependent]
			}

			continue
		}

		if s.onStack[dependent] &&
			dependentIndex <
				s.lowLinks[vertex] {

			s.lowLinks[vertex] =
				dependentIndex
		}
	}

	if s.lowLinks[vertex] !=
		s.indices[vertex] {

		return
	}

	component :=
		s.popComponent(
			vertex,
		)

	if len(component) > 1 {
		s.cycles = append(
			s.cycles,
			newCycle(
				component,
			),
		)

		return
	}

	if len(component) == 1 &&
		s.graph.hasDirectDependency(
			component[0],
			component[0],
		) {

		s.cycles = append(
			s.cycles,
			newCycle(
				component,
			),
		)
	}
}

func (s *tarjanState) popComponent(
	root address.PropertyAddress,
) []address.PropertyAddress {
	component := make(
		[]address.PropertyAddress,
		0,
	)

	for len(s.stack) > 0 {
		lastIndex :=
			len(s.stack) - 1

		vertex :=
			s.stack[lastIndex]

		s.stack =
			s.stack[:lastIndex]

		s.onStack[vertex] =
			false

		component = append(
			component,
			vertex,
		)

		if vertex == root {
			break
		}
	}

	return component
}
