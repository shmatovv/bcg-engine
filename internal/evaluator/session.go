package evaluator

import (
	"sync"
	"sync/atomic"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/plan"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// Session owns reusable in-process scalar evaluation state for exactly one
// immutable ModelVersion. It is deliberately separate from ModelVersion and
// CalculationContext: model data remains immutable and contexts remain pure
// semantic input.
//
// A Session keeps only its current calculation environment. Moving to a new
// Scope, Time or Scenario starts a fresh cache generation; changing only
// calculation overrides carries forward entries outside the union of old and
// new effective dependent closures. An in-flight evaluation retains its prior
// immutable generation and is therefore safe to complete concurrently with a
// transition.
//
// All public methods are concurrency-safe. Concurrent cold requests may
// compute the same property more than once, but only complete immutable
// Results are ever published or returned.
type Session struct {
	version Source
	graph   dependency.Graph

	mu      sync.Mutex
	current *sessionEnvironment

	hits          atomic.Uint64
	misses        atomic.Uint64
	invalidations atomic.Uint64
}

type sessionEnvironment struct {
	context runtime.CalculationContext
	plan    plan.Plan
	cache   *sessionCache
}

type sessionCache struct {
	mu      sync.RWMutex
	entries map[address.PropertyAddress]Result
	session *Session
}

// CacheStats is runtime-only diagnostic information. Hits and Misses count
// shared property-cache lookups for the lifetime of a Session. Invalidations
// counts entries actually removed by transitions or Reset; Entries is the
// number currently retained in the active environment.
type CacheStats struct {
	hits          uint64
	misses        uint64
	invalidations uint64
	entries       int
}

func (s CacheStats) Hits() uint64          { return s.hits }
func (s CacheStats) Misses() uint64        { return s.misses }
func (s CacheStats) Invalidations() uint64 { return s.invalidations }
func (s CacheStats) Entries() int          { return s.entries }

// NewSession creates a runtime cache owner for one committed immutable source.
// The concrete dependency graph is built once here and reused for every
// effective-plan transition in this Session.
func NewSession(version Source) (*Session, error) {
	if !version.IsCommitted() {
		return nil, newError(
			ErrorInvalidModelVersion,
			"evaluation session requires a committed ModelVersion",
			nil,
		)
	}

	graph, err := dependency.Build(version)
	if err != nil {
		return nil, newError(
			ErrorPlanBuild,
			"failed to build immutable dependency graph for evaluation session",
			err,
		)
	}

	return &Session{version: version, graph: graph}, nil
}

// NewSessionWithDependencyGraph binds a Session to a source whose immutable
// combined graph was already validated and built by its snapshot owner.
func NewSessionWithDependencyGraph(version Source, graph dependency.Graph) (*Session, error) {
	if !version.IsCommitted() {
		return nil, newError(ErrorInvalidModelVersion, "evaluation session requires a committed ModelVersion", nil)
	}
	return &Session{version: version, graph: graph}, nil
}

// ModelVersion returns the immutable snapshot to which s is bound.
func (s *Session) ModelVersion() model.ModelVersion {
	s.requireValid()
	return s.version.ModelVersion()
}

// Source returns the fixed effective source to which the Session is bound.
func (s *Session) Source() Source { s.requireValid(); return s.version }

// Evaluate evaluates one scalar property through the ordinary evaluator while
// reusing this Session's current-environment property cache.
func (s *Session) Evaluate(
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Result, error) {
	results, err := s.EvaluateMany(
		[]address.PropertyAddress{target},
		context,
	)
	if err != nil {
		return Result{}, err
	}
	return results[0], nil
}

// EvaluateMany evaluates scalar targets in caller order. Duplicate targets
// remain duplicate output occurrences while their concrete property result is
// computed at most once per evaluation state (or reused from the Session).
func (s *Session) EvaluateMany(
	targets []address.PropertyAddress,
	context runtime.CalculationContext,
) ([]Result, error) {
	s.requireValid()
	if len(targets) == 0 {
		return []Result{}, nil
	}

	environment, err := s.prepare(context)
	if err != nil {
		return nil, err
	}
	if !environment.plan.IsValid() {
		return nil, newError(
			ErrorInvalidPropertyAddress,
			"scalar target cannot be resolved from an empty ModelVersion",
			nil,
		)
	}

	for _, target := range targets {
		if !target.IsValid() {
			return nil, newError(
				ErrorInvalidPropertyAddress,
				"target property address is invalid",
				nil,
			)
		}
		if _, exists := environment.plan.Node(target); !exists {
			return nil, newError(
				ErrorDefinitionNotAvailable,
				"target property does not belong to the Session ModelVersion",
				nil,
			)
		}
	}

	state := newSessionEvaluationState(s.version, context, environment)
	results := make([]Result, len(targets))
	for index, target := range targets {
		result, err := state.evaluateProperty(target)
		if err != nil {
			return nil, err
		}
		results[index] = result
	}

	return results, nil
}

// Explain evaluates target through this Session and projects its recorded
// provenance without re-running the expression evaluator.
func (s *Session) Explain(
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Explanation, error) {
	result, err := s.Evaluate(target, context)
	if err != nil {
		return Explanation{}, err
	}
	return ExplainResult(target, result), nil
}

// EvaluateExpression evaluates one standalone scalar expression through the
// ordinary evaluator using cached concrete property reads. It retains the
// existing standalone contract: there is no implicit self object.
func (s *Session) EvaluateExpression(
	expression ast.Expression,
	context runtime.CalculationContext,
) (Result, error) {
	s.requireValid()
	environment, err := s.prepare(context)
	if err != nil {
		return Result{}, err
	}
	return evaluateStandaloneWithEnvironment(
		s.version,
		expression,
		context,
		environment.plan,
		environment.cache,
	)
}

// Reset drops the active runtime cache generation. It does not alter the
// immutable ModelVersion and is safe alongside concurrent evaluations: work
// already holding the retired generation completes independently.
func (s *Session) Reset() {
	s.requireValid()
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil {
		return
	}

	s.invalidations.Add(uint64(s.current.cache.Len()))
	s.current = nil
}

// Stats returns a value snapshot of runtime cache diagnostics.
func (s *Session) Stats() CacheStats {
	s.requireValid()
	s.mu.Lock()
	current := s.current
	s.mu.Unlock()

	entries := 0
	if current != nil {
		entries = current.cache.Len()
	}

	return CacheStats{
		hits:          s.hits.Load(),
		misses:        s.misses.Load(),
		invalidations: s.invalidations.Load(),
		entries:       entries,
	}
}

func (s *Session) prepare(context runtime.CalculationContext) (*sessionEnvironment, error) {
	if !context.IsValid() {
		return nil, newError(
			ErrorInvalidCalculationContext,
			"calculation context is invalid",
			nil,
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current != nil && s.current.context.Equal(context) {
		return s.current, nil
	}

	var effectivePlan plan.Plan
	if s.graph.Len() > 0 {
		var err error
		effectivePlan, err = plan.BuildWithDependencyGraph(
			s.version,
			s.graph,
			s.graph.Vertices(),
			context,
		)
		if err != nil {
			return nil, newError(
				ErrorPlanBuild,
				"failed to build Effective Evaluation Plan for session environment",
				err,
			)
		}
	}

	cache := newSessionCache(s)
	if s.current != nil && s.current.context.SameNonOverrideEnvironment(context) {
		changed := s.current.context.ChangedOverrideTargets(context)
		invalidated := effectiveAffected(changed, s.current.plan, effectivePlan)
		removed := s.current.cache.CloneExcluding(cache, invalidated)
		s.invalidations.Add(uint64(removed))
	} else if s.current != nil {
		s.invalidations.Add(uint64(s.current.cache.Len()))
	}

	environment := &sessionEnvironment{
		context: context,
		plan:    effectivePlan,
		cache:   cache,
	}
	s.current = environment
	return environment, nil
}

func newSessionEvaluationState(
	version Source,
	context runtime.CalculationContext,
	environment *sessionEnvironment,
) evaluationState {
	if environment == nil || !environment.cache.validForSession() {
		panic("evaluator: session environment is invalid")
	}

	return evaluationState{
		version:       version,
		context:       context,
		effectivePlan: environment.plan,
		memo:          make(map[address.PropertyAddress]Result),
		cache:         environment.cache,
		visiting:      make(map[address.PropertyAddress]struct{}),
	}
}

func newSessionCache(session *Session) *sessionCache {
	return &sessionCache{
		entries: make(map[address.PropertyAddress]Result),
		session: session,
	}
}

func (c *sessionCache) Load(target address.PropertyAddress) (Result, bool) {
	c.mu.RLock()
	result, found := c.entries[target]
	c.mu.RUnlock()

	if found {
		c.session.hits.Add(1)
	} else {
		c.session.misses.Add(1)
	}

	return result, found
}

func (c *sessionCache) Store(target address.PropertyAddress, result Result) {
	if !target.IsValid() || !result.IsValid() {
		panic("evaluator: session cache entry is invalid")
	}

	c.mu.Lock()
	c.entries[target] = result
	c.mu.Unlock()
}

func (c *sessionCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// CloneExcluding copies every reusable cache entry into destination and
// returns how many entries were intentionally dropped. The source remains
// valid for in-flight evaluations after this method returns.
func (c *sessionCache) CloneExcluding(
	destination *sessionCache,
	excluded map[address.PropertyAddress]struct{},
) int {
	if destination == nil || !destination.validForSession() {
		panic("evaluator: destination session cache is invalid")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	destination.mu.Lock()
	defer destination.mu.Unlock()

	removed := 0
	for target, result := range c.entries {
		if _, drop := excluded[target]; drop {
			removed++
			continue
		}
		destination.entries[target] = result
	}
	return removed
}

func (c *sessionCache) validForSession() bool {
	return c != nil && c.session != nil && c.entries != nil
}

func effectiveAffected(
	changed []address.PropertyAddress,
	plans ...plan.Plan,
) map[address.PropertyAddress]struct{} {
	result := make(map[address.PropertyAddress]struct{}, len(changed))
	if len(changed) == 0 {
		return result
	}

	for _, changedTarget := range changed {
		result[changedTarget] = struct{}{}
	}

	for _, effectivePlan := range plans {
		if !effectivePlan.IsValid() {
			continue
		}

		outgoing := make(map[address.PropertyAddress][]address.PropertyAddress)
		for _, edge := range effectivePlan.Edges() {
			outgoing[edge.From()] = append(outgoing[edge.From()], edge.To())
		}

		queue := append([]address.PropertyAddress(nil), changed...)
		seen := make(map[address.PropertyAddress]struct{}, len(changed))
		for _, changedTarget := range changed {
			seen[changedTarget] = struct{}{}
		}

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, dependent := range outgoing[current] {
				result[dependent] = struct{}{}
				if _, alreadySeen := seen[dependent]; alreadySeen {
					continue
				}
				seen[dependent] = struct{}{}
				queue = append(queue, dependent)
			}
		}
	}

	return result
}

func (s *Session) requireValid() {
	if s == nil || !s.version.IsCommitted() || s.graph.Len() < 0 {
		panic("evaluator: invalid EvaluationSession")
	}
}
