package bcg

import (
	"fmt"
	"sync"

	constraintcheck "github.com/shmatovv/bcg-engine/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/impact"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	ruleeval "github.com/shmatovv/bcg-engine/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// Session owns reusable incremental scalar evaluation state for one immutable
// model-only or Dataset-backed evaluation source. All its methods are safe for
// concurrent use. A transition obtains a new immutable CalculationContext;
// in-flight evaluations retain their prior context just as in the underlying
// evaluator Session.
type Session struct {
	model      Model
	dataset    Dataset
	hasDataset bool
	value      *evaluator.Session
	mu         sync.RWMutex
	context    runtime.CalculationContext
}

func newSession(model Model, value *evaluator.Session, context runtime.CalculationContext) *Session {
	return &Session{model: model, value: value, context: context}
}

func newDatasetSession(model Model, dataset Dataset, value *evaluator.Session, context runtime.CalculationContext) *Session {
	return &Session{model: model, dataset: dataset, hasDataset: true, value: value, context: context}
}

func (s *Session) valid() error {
	if s == nil || s.value == nil || !s.model.IsValid() {
		return fmt.Errorf("%w: session is invalid", ErrInvalidModel)
	}
	return nil
}
func (s *Session) snapshot() (runtime.CalculationContext, error) {
	if err := s.valid(); err != nil {
		return runtime.CalculationContext{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.context, nil
}

// Model returns the immutable Model bound to s.
func (s *Session) Model() Model {
	if s == nil {
		return Model{}
	}
	return s.model
}

// Dataset returns the optional fixed Dataset used by this Session.
func (s *Session) Dataset() (Dataset, bool) {
	if s == nil || !s.hasDataset {
		return Dataset{}, false
	}
	return s.dataset, true
}

// Context returns the current immutable calculation environment.
func (s *Session) Context() CalculationContext {
	context, err := s.snapshot()
	if err != nil {
		return CalculationContext{}
	}
	return CalculationContext{value: context}
}

// SetCalculationContext transitions s to context. Scope, time, and Scenario
// transitions use the core's conservative fresh generation; override-only
// transitions use its precise effective dependency invalidation.
func (s *Session) SetCalculationContext(context CalculationContext) error {
	if err := s.valid(); err != nil {
		return err
	}
	converted, err := context.internal()
	if err != nil {
		return err
	}
	if err := resolvervalue.ValidateContext(s.value.Source(), converted); err != nil {
		return publicError(err)
	}
	s.mu.Lock()
	s.context = converted
	s.mu.Unlock()
	return nil
}

// SetCalculationOverrides replaces only the calculation override set while
// retaining s's time, scope, and Scenario selection.
func (s *Session) SetCalculationOverrides(overrides ...Override) error {
	context, err := s.snapshot()
	if err != nil {
		return err
	}
	public := CalculationContext{value: context}
	next, err := public.WithCalculationOverrides(overrides...)
	if err != nil {
		return err
	}
	return s.SetCalculationContext(next)
}

// Evaluate evaluates one scalar property through the shared incremental cache.
func (s *Session) Evaluate(target PropertyAddress) (Result, error) {
	context, err := s.snapshot()
	if err != nil {
		return Result{}, err
	}
	addressValue, err := target.internal()
	if err != nil {
		return Result{}, err
	}
	result, err := s.value.Evaluate(addressValue, context)
	if err != nil {
		return Result{}, publicError(err)
	}
	return resultFromInternal(result), nil
}

// EvaluateMany evaluates scalar targets in caller order through the shared cache.
func (s *Session) EvaluateMany(targets []PropertyAddress) ([]Result, error) {
	context, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	internal := make([]address.PropertyAddress, len(targets))
	for i, target := range targets {
		converted, err := target.internal()
		if err != nil {
			return nil, err
		}
		internal[i] = converted
	}
	results, err := s.value.EvaluateMany(internal, context)
	if err != nil {
		return nil, publicError(err)
	}
	return resultsFromInternal(results), nil
}

// EvaluateSeries uses the Session's fixed effective source. Series remains
// outside the persistent scalar cache but never falls back to model-only data.
func (s *Session) EvaluateSeries(target PropertyAddress) (SeriesResult, error) {
	context, err := s.snapshot()
	if err != nil {
		return SeriesResult{}, err
	}
	addressValue, err := target.internal()
	if err != nil {
		return SeriesResult{}, err
	}
	result, err := evaluator.EvaluateSeries(s.value.Source(), addressValue, context)
	if err != nil {
		return SeriesResult{}, publicError(err)
	}
	return seriesResultFromInternal(result), nil
}

// Explain returns cached actual scalar provenance without re-running a warm property calculation.
func (s *Session) Explain(target PropertyAddress) (Explanation, error) {
	context, err := s.snapshot()
	if err != nil {
		return Explanation{}, err
	}
	addressValue, err := target.internal()
	if err != nil {
		return Explanation{}, err
	}
	explained, err := s.value.Explain(addressValue, context)
	if err != nil {
		return Explanation{}, publicError(err)
	}
	return explanationFromInternal(explained), nil
}

// AnalyzeImpact exposes structural/effective/observed impact over this
// Session's fixed effective source. Observed uses the shared evaluator.
func (s *Session) AnalyzeImpact(mode ImpactMode, source PropertyAddress) (ImpactResult, error) {
	context, err := s.snapshot()
	if err != nil {
		return ImpactResult{}, err
	}
	addressValue, err := source.internal()
	if err != nil {
		return ImpactResult{}, err
	}
	var result impact.Result
	switch mode {
	case StructuralImpact:
		result, err = impact.AnalyzeStructuralSource(s.value.Source(), addressValue)
	case EffectiveImpact:
		result, err = impact.AnalyzeEffectiveSource(s.value.Source(), addressValue, context)
	case ObservedImpact:
		result, err = impact.AnalyzeObservedWithSession(s.value, addressValue, context)
	default:
		return ImpactResult{}, fmt.Errorf("%w: impact mode is invalid", ErrInvalidArgument)
	}
	if err != nil {
		return ImpactResult{}, publicError(err)
	}
	return impactResultFromInternal(result), nil
}

// CheckConstraints checks every Constraint while sharing cached property reads.
func (s *Session) CheckConstraints() (ConstraintReport, error) {
	context, err := s.snapshot()
	if err != nil {
		return ConstraintReport{}, err
	}
	report, err := constraintcheck.AllWithSession(s.value, context)
	if err != nil {
		return ConstraintReport{}, publicError(err)
	}
	return constraintReportFromInternal(report), nil
}

// CheckFeasibility assesses all Hard Constraints through shared cached reads.
func (s *Session) CheckFeasibility() (FeasibilityResult, error) {
	context, err := s.snapshot()
	if err != nil {
		return FeasibilityResult{}, err
	}
	result, err := feasibility.EvaluateWithSession(s.value, context)
	if err != nil {
		return FeasibilityResult{}, publicError(err)
	}
	return feasibilityResultFromInternal(result), nil
}

// EvaluateRule evaluates one Rule through shared cached property reads.
func (s *Session) EvaluateRule(id ID) (RuleResult, error) {
	context, err := s.snapshot()
	if err != nil {
		return RuleResult{}, err
	}
	ruleID, err := id.internal()
	if err != nil {
		return RuleResult{}, err
	}
	result, err := ruleeval.OneWithSession(s.value, ruleID, context)
	if err != nil {
		return RuleResult{}, publicError(err)
	}
	return ruleResultFromInternal(result), nil
}

// EvaluateRules evaluates all Rules through shared cached property reads.
func (s *Session) EvaluateRules() (RuleReport, error) {
	context, err := s.snapshot()
	if err != nil {
		return RuleReport{}, err
	}
	report, err := ruleeval.AllWithSession(s.value, context)
	if err != nil {
		return RuleReport{}, publicError(err)
	}
	return ruleReportFromInternal(report), nil
}

// Reset drops the current runtime cache generation without changing Model or Context.
func (s *Session) Reset() error {
	if err := s.valid(); err != nil {
		return err
	}
	s.value.Reset()
	return nil
}

// CacheStats is a read-only diagnostic snapshot. It is not business provenance.
type CacheStats struct {
	hits, misses, invalidations uint64
	entries                     int
}

// Hits returns the number of reusable scalar property results served.
func (s CacheStats) Hits() uint64 { return s.hits }

// Misses returns the number of scalar property calculations not served from cache.
func (s CacheStats) Misses() uint64 { return s.misses }

// Invalidations returns the number of cache entries removed by transitions or Reset.
func (s CacheStats) Invalidations() uint64 { return s.invalidations }

// Entries returns the current number of cached scalar property results.
func (s CacheStats) Entries() int { return s.entries }

// CacheStats returns a current diagnostic snapshot.
func (s *Session) CacheStats() CacheStats {
	if s == nil || s.value == nil {
		return CacheStats{}
	}
	stats := s.value.Stats()
	return CacheStats{hits: stats.Hits(), misses: stats.Misses(), invalidations: stats.Invalidations(), entries: stats.Entries()}
}
