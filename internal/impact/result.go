package impact

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// Mode states the exact dependency semantics used to produce a Result.
//
// Structural reports every downstream dependency encoded in the immutable
// value dependency graph. Effective reports the same graph after the current
// CalculationContext's calculation and Scenario overrides have removed their
// incoming edges. Observed reports only direct reads that were actually
// recorded by the ordinary evaluator's provenance during this execution.
//
// Every mode excludes Source from Affected. Edges always point from a value
// being read to the property that reads it.
type Mode uint8

const (
	ModeInvalid Mode = iota
	ModeStructural
	ModeEffective
	ModeObserved
)

// IsValid reports whether m is a supported Impact analysis mode.
func (m Mode) IsValid() bool {
	return m == ModeStructural || m == ModeEffective || m == ModeObserved
}

// String returns m's stable semantic name.
func (m Mode) String() string {
	switch m {
	case ModeStructural:
		return "Structural"
	case ModeEffective:
		return "Effective"
	case ModeObserved:
		return "Observed"
	default:
		return "Invalid"
	}
}

// Affected is one unique downstream property in an Impact Result.
// MinimumDepth is the smallest number of dependency edges from Source to
// Target. Direct is explicit rather than order-derived and is true exactly
// when MinimumDepth is one.
type Affected struct {
	target       address.PropertyAddress
	minimumDepth int
}

// Target is the downstream property.
func (a Affected) Target() address.PropertyAddress { return a.target }

// MinimumDepth is the smallest dependency-edge distance from Result.Source.
func (a Affected) MinimumDepth() int { return a.minimumDepth }

// IsDirect reports whether Target directly reads Result.Source.
func (a Affected) IsDirect() bool { return a.minimumDepth == 1 }

// IsValid reports whether a satisfies Result invariants.
func (a Affected) IsValid() bool {
	return a.target.IsValid() && a.minimumDepth > 0
}

// Edge is one direct downstream dependency relationship in an Impact Result.
// Direction is From (dependency) to To (dependent).
type Edge struct {
	from address.PropertyAddress
	to   address.PropertyAddress
}

// From is the property whose value is read.
func (e Edge) From() address.PropertyAddress { return e.from }

// To is the property that reads From.
func (e Edge) To() address.PropertyAddress { return e.to }

// IsValid reports whether both endpoints are valid property addresses.
func (e Edge) IsValid() bool { return e.from.IsValid() && e.to.IsValid() }

// Result is an immutable normalized DAG view of one downstream Impact
// analysis. It does not duplicate targets for diamond-shaped graphs. Nodes
// are canonically ordered by minimum distance and then PropertyAddress; Edges
// are canonically ordered by From and then To.
//
// For ModeObserved and Series properties, the result is property-level: a
// relation is included only when it appeared in at least one independently
// recorded sample provenance tree. Sample trees are never paired or matched
// by index, so the result does not claim a false temporal-coordinate match.
type Result struct {
	modelVersionID identity.UUID
	source         address.PropertyAddress
	mode           Mode
	affected       []Affected
	edges          []Edge
}

func newResult(modelVersionID identity.UUID, source address.PropertyAddress, mode Mode, depths map[address.PropertyAddress]int, edges map[edgeKey]struct{}) Result {
	if modelVersionID.IsZero() || !source.IsValid() || !mode.IsValid() {
		panic("impact: invalid result metadata")
	}
	result := Result{modelVersionID: modelVersionID, source: source, mode: mode}
	for target, depth := range depths {
		if target == source {
			continue
		}
		result.affected = append(result.affected, Affected{target: target, minimumDepth: depth})
	}
	sort.Slice(result.affected, func(i, j int) bool {
		if result.affected[i].minimumDepth != result.affected[j].minimumDepth {
			return result.affected[i].minimumDepth < result.affected[j].minimumDepth
		}
		return lessAddress(result.affected[i].target, result.affected[j].target)
	})
	for edge := range edges {
		if edge.from == source || depths[edge.from] > 0 {
			if _, ok := depths[edge.to]; ok {
				result.edges = append(result.edges, Edge{from: edge.from, to: edge.to})
			}
		}
	}
	sort.Slice(result.edges, func(i, j int) bool {
		if result.edges[i].from != result.edges[j].from {
			return lessAddress(result.edges[i].from, result.edges[j].from)
		}
		return lessAddress(result.edges[i].to, result.edges[j].to)
	})
	if !result.IsValid() {
		panic("impact: constructed invalid result")
	}
	return result
}

// ModelVersionID identifies the immutable model snapshot analyzed.
func (r Result) ModelVersionID() identity.UUID { return r.modelVersionID }

// Source is the changed property whose downstream consequences were analyzed.
func (r Result) Source() address.PropertyAddress { return r.source }

// Mode returns the exact semantics used for this analysis.
func (r Result) Mode() Mode { return r.mode }

// Affected returns an owned, deterministic copy of unique affected targets.
func (r Result) Affected() []Affected { return append([]Affected(nil), r.affected...) }

// Edges returns an owned, deterministic copy of the reachable dependency DAG.
func (r Result) Edges() []Edge { return append([]Edge(nil), r.edges...) }

// Len is the number of unique downstream affected properties.
func (r Result) Len() int { return len(r.affected) }

// IsValid reports whether r satisfies all immutable normalized-DAG invariants.
func (r Result) IsValid() bool {
	if r.modelVersionID.IsZero() || !r.source.IsValid() || !r.mode.IsValid() {
		return false
	}
	depths := make(map[address.PropertyAddress]int, len(r.affected)+1)
	depths[r.source] = 0
	for index, item := range r.affected {
		if !item.IsValid() || item.target == r.source {
			return false
		}
		if prior, duplicate := depths[item.target]; duplicate && prior != 0 {
			return false
		}
		if index > 0 {
			previous := r.affected[index-1]
			if previous.minimumDepth > item.minimumDepth ||
				(previous.minimumDepth == item.minimumDepth && !lessAddress(previous.target, item.target)) {
				return false
			}
		}
		depths[item.target] = item.minimumDepth
	}
	seenEdges := make(map[edgeKey]struct{}, len(r.edges))
	for index, edge := range r.edges {
		if !edge.IsValid() || edge.to == r.source {
			return false
		}
		if _, knownFrom := depths[edge.from]; !knownFrom {
			return false
		}
		if _, knownTo := depths[edge.to]; !knownTo {
			return false
		}
		key := edgeKey{from: edge.from, to: edge.to}
		if _, duplicate := seenEdges[key]; duplicate {
			return false
		}
		seenEdges[key] = struct{}{}
		if index > 0 {
			previous := r.edges[index-1]
			if lessAddress(edge.from, previous.from) || (edge.from == previous.from && !lessAddress(previous.to, edge.to)) {
				return false
			}
		}
	}
	return true
}

type edgeKey struct{ from, to address.PropertyAddress }

func lessAddress(left, right address.PropertyAddress) bool {
	if left.Object().Kind() != right.Object().Kind() {
		return left.Object().Kind() < right.Object().Kind()
	}
	if comparison := compareUUID(left.Object().ID(), right.Object().ID()); comparison != 0 {
		return comparison < 0
	}
	return compareUUID(left.DefinitionID(), right.DefinitionID()) < 0
}

func compareUUID(left, right identity.UUID) int {
	leftBytes, rightBytes := left.Bytes(), right.Bytes()
	for index := range leftBytes {
		if leftBytes[index] < rightBytes[index] {
			return -1
		}
		if leftBytes[index] > rightBytes[index] {
			return 1
		}
	}
	return 0
}
