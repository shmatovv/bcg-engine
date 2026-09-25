package bcg

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	resolvervalue "github.com/shmatovv/bcg-engine/v2/internal/resolver/value"
)

// BatchItem preserves one requested target and its independent scalar result.
// Duplicate requested addresses intentionally produce duplicate items in the
// caller's order, even though their calculation is shared internally.
type BatchItem struct {
	target PropertyAddress
	result Result
}

func (i BatchItem) Target() PropertyAddress { return i.target }
func (i BatchItem) Result() Result          { return i.result }

// BatchResult is an immutable ordered result of EvaluateBatch.
type BatchResult struct{ items []BatchItem }

func (r BatchResult) Len() int { return len(r.items) }
func (r BatchResult) Item(index int) (BatchItem, bool) {
	if index < 0 || index >= len(r.items) {
		return BatchItem{}, false
	}
	return r.items[index], true
}
func (r BatchResult) Items() []BatchItem { return append([]BatchItem(nil), r.items...) }

// EvaluateBatch evaluates all targets using one evaluator Session and one
// fixed snapshot source/runtime identity. It preserves caller order and
// duplicate occurrences. Undefined and Null are ordinary per-item results;
// malformed arguments still return an error for the batch.
func (s EvaluationSnapshot) EvaluateBatch(targets []PropertyAddress, context CalculationContext) (BatchResult, error) {
	if !s.IsValid() {
		return BatchResult{}, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return BatchResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return BatchResult{}, err
	}
	if err := resolvervalue.ValidateContext(s.source, contextValue); err != nil {
		return BatchResult{}, publicError(err)
	}
	if len(targets) == 0 {
		return BatchResult{items: []BatchItem{}}, nil
	}
	values := make([]address.PropertyAddress, len(targets))
	for i, target := range targets {
		value, err := target.internal()
		if err != nil {
			return BatchResult{}, err
		}
		values[i] = value
	}
	session, err := evaluator.NewSessionWithDependencyGraph(s.evaluationSource(), s.source.DependencyGraph())
	if err != nil {
		return BatchResult{}, publicError(err)
	}
	results, err := session.EvaluateMany(values, contextValue)
	if err != nil {
		return BatchResult{}, publicError(err)
	}
	items := make([]BatchItem, len(targets))
	for i := range items {
		items[i] = BatchItem{target: targets[i], result: resultFromInternal(results[i])}
	}
	return BatchResult{items: items}, nil
}

// EvaluateBatch evaluates targets through this Session's reusable cache and
// its fixed model/dataset/runtime identity.
func (s *Session) EvaluateBatch(targets []PropertyAddress) (BatchResult, error) {
	results, err := s.EvaluateMany(targets)
	if err != nil {
		return BatchResult{}, err
	}
	items := make([]BatchItem, len(targets))
	for i := range items {
		items[i] = BatchItem{target: targets[i], result: results[i]}
	}
	return BatchResult{items: items}, nil
}
