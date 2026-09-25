package evaluator

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/plan"
)

// Entry associates one concrete PropertyAddress with its evaluated Result.
type Entry struct {
	target address.PropertyAddress
	result Result
}

func newEntry(
	target address.PropertyAddress,
	result Result,
) Entry {
	if !target.IsValid() {
		panic("evaluator: result entry target is invalid")
	}

	if !result.IsValid() {
		panic("evaluator: result entry value is invalid")
	}

	return Entry{
		target: target,
		result: result,
	}
}

func (e Entry) Target() address.PropertyAddress {
	return e.target
}

func (e Entry) Result() Result {
	return e.result
}

func (e Entry) IsValid() bool {
	return e.target.IsValid() &&
		e.result.IsValid()
}

// PlanResult contains the results actually evaluated while executing one
// Effective Evaluation Plan.
//
// It intentionally does not promise that every static Plan node has been
// executed. Deterministic runtime short-circuit semantics may skip nodes that
// are statically reachable through:
//
//   - If
//   - AND
//   - OR
//   - Coalesce
//
// Every root, however, is always evaluated successfully when PlanResult is
// returned.
type PlanResult struct {
	sourceIdentity model.EffectiveSourceIdentity

	roots []address.PropertyAddress

	entries []Entry

	byTarget map[address.PropertyAddress]Result
}

func newPlanResult(
	effectivePlan plan.Plan,
	results map[address.PropertyAddress]Result,
) PlanResult {
	if !effectivePlan.IsValid() {
		panic("evaluator: cannot construct result from invalid plan")
	}

	entries := make(
		[]Entry,
		0,
		len(results),
	)

	for _, target := range effectivePlan.Order() {

		result, ok :=
			results[target]

		if !ok {
			continue
		}

		entries =
			append(
				entries,
				newEntry(
					target,
					result,
				),
			)
	}

	byTarget := make(
		map[address.PropertyAddress]Result,
		len(results),
	)

	for target, result := range results {

		if !target.IsValid() ||
			!result.IsValid() {

			panic(
				"evaluator: invalid result map",
			)
		}

		byTarget[target] =
			result
	}

	roots :=
		effectivePlan.Roots()

	for _, root := range roots {

		if _, ok :=
			byTarget[root]; !ok {

			panic(
				"evaluator: completed plan has no result for a root",
			)
		}
	}

	return PlanResult{
		sourceIdentity: effectivePlan.SourceIdentity(),
		roots:          roots,
		entries:        entries,
		byTarget:       byTarget,
	}
}

func (r PlanResult) ModelVersionID() identity.UUID {
	return r.sourceIdentity.ModelVersionID()
}

// SourceIdentity identifies the exact immutable model-only or Dataset-backed
// source whose plan produced r.
func (r PlanResult) SourceIdentity() model.EffectiveSourceIdentity { return r.sourceIdentity }

func (r PlanResult) Len() int {
	return len(r.entries)
}

func (r PlanResult) Roots() []address.PropertyAddress {
	return append(
		[]address.PropertyAddress(nil),
		r.roots...,
	)
}

func (r PlanResult) Result(
	target address.PropertyAddress,
) (Result, bool) {
	result, ok :=
		r.byTarget[target]

	return result, ok
}

// Entries returns actually executed nodes in deterministic dependency-first
// plan order.
func (r PlanResult) Entries() []Entry {
	return append(
		[]Entry(nil),
		r.entries...,
	)
}

// RootResults returns root results in the deterministic canonical root order
// stored by the Effective Evaluation Plan.
func (r PlanResult) RootResults() []Entry {
	result := make(
		[]Entry,
		0,
		len(r.roots),
	)

	for _, root := range r.roots {

		rootResult, ok :=
			r.byTarget[root]

		if !ok {
			panic(
				"evaluator: PlanResult invariant violated: root result missing",
			)
		}

		result =
			append(
				result,
				newEntry(
					root,
					rootResult,
				),
			)
	}

	return result
}

func (r PlanResult) IsValid() bool {
	if !r.sourceIdentity.IsValid() ||
		len(r.roots) == 0 ||
		len(r.entries) == 0 ||
		len(r.byTarget) == 0 {
		return false
	}

	if len(r.entries) !=
		len(r.byTarget) {
		return false
	}

	seen := make(
		map[address.PropertyAddress]struct{},
		len(r.entries),
	)

	for _, entry := range r.entries {

		if !entry.IsValid() {
			return false
		}

		if _, duplicate :=
			seen[entry.target]; duplicate {
			return false
		}

		seen[entry.target] =
			struct{}{}

		stored, ok :=
			r.byTarget[entry.target]

		if !ok ||
			!stored.IsValid() {
			return false
		}
	}

	for _, root := range r.roots {

		if !root.IsValid() {
			return false
		}

		if _, ok :=
			r.byTarget[root]; !ok {
			return false
		}
	}

	return true
}
