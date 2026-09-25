package bcg

import (
	"fmt"

	constraintcheck "github.com/shmatovv/bcg-engine/v2/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/v2/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	domainscenario "github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/impact"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	resolvervalue "github.com/shmatovv/bcg-engine/v2/internal/resolver/value"
	ruleeval "github.com/shmatovv/bcg-engine/v2/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	modelscenario "github.com/shmatovv/bcg-engine/v2/internal/scenario"
)

// EvaluationSnapshot is an immutable effective evaluation source: one exact
// committed Model plus one exact immutable Dataset. Together they are the
// complete runtime semantic graph used by every method on the snapshot. It
// never embeds Model, so its methods cannot accidentally fall back to
// model-only evaluation.
type EvaluationSnapshot struct {
	model   Model
	dataset Dataset
	source  model.EvaluationSource
	runtime Runtime
}

// WithDataset validates Dataset ownership, builds the combined dependency
// graph once, and rejects Dataset-induced dependency cycles.
func (m Model) WithDataset(dataset Dataset) (EvaluationSnapshot, error) {
	if !m.IsValid() || !dataset.IsValid() {
		return EvaluationSnapshot{}, fmt.Errorf("%w: committed Model and valid Dataset are required", ErrInvalidArgument)
	}
	if dataset.ModelVersionID() != m.VersionID() {
		return EvaluationSnapshot{}, fmt.Errorf("%w: Dataset belongs to a different ModelVersion", ErrInvalidArgument)
	}
	source, err := model.NewEvaluationSource(m.version, dataset.value)
	if err != nil {
		return EvaluationSnapshot{}, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return EvaluationSnapshot{model: m, dataset: dataset, source: source, runtime: DefaultRuntime()}, nil
}

func (s EvaluationSnapshot) IsValid() bool {
	return s.model.IsValid() && s.dataset.IsValid() && s.source.IsCommitted()
}
func (s EvaluationSnapshot) Model() Model       { return s.model }
func (s EvaluationSnapshot) Dataset() Dataset   { return s.dataset }
func (s EvaluationSnapshot) ModelVersionID() ID { return s.model.VersionID() }
func (s EvaluationSnapshot) DatasetID() ID      { return s.dataset.ID() }
func (s EvaluationSnapshot) runtimeValue() Runtime {
	runtime := s.runtime
	if !runtime.IsValid() {
		runtime = DefaultRuntime()
	}
	return runtime
}
func (s EvaluationSnapshot) evaluationSource() evaluator.Source {
	runtime := s.runtimeValue()
	return evaluator.BindRuntime(s.source, runtime)
}

func (s EvaluationSnapshot) validateRuntime() error {
	return s.runtimeValue().ValidateModel(s.model)
}

func (s EvaluationSnapshot) request(target PropertyAddress, context CalculationContext) (address.PropertyAddress, runtime.CalculationContext, error) {
	if !s.IsValid() {
		return address.PropertyAddress{}, runtime.CalculationContext{}, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return address.PropertyAddress{}, runtime.CalculationContext{}, err
	}
	addressValue, err := target.internal()
	if err != nil {
		return address.PropertyAddress{}, runtime.CalculationContext{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return address.PropertyAddress{}, runtime.CalculationContext{}, err
	}
	if err := resolvervalue.ValidateContext(s.source, contextValue); err != nil {
		return address.PropertyAddress{}, runtime.CalculationContext{}, publicError(err)
	}
	return addressValue, contextValue, nil
}

func (s EvaluationSnapshot) Evaluate(target PropertyAddress, context CalculationContext) (Result, error) {
	targetValue, contextValue, err := s.request(target, context)
	if err != nil {
		return Result{}, err
	}
	result, err := evaluator.Evaluate(s.evaluationSource(), targetValue, contextValue)
	if err != nil {
		return Result{}, publicError(err)
	}
	return resultFromInternal(result), nil
}
func (s EvaluationSnapshot) EvaluateMany(targets []PropertyAddress, context CalculationContext) ([]Result, error) {
	if !s.IsValid() {
		return nil, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return nil, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return nil, err
	}
	if err := resolvervalue.ValidateContext(s.source, contextValue); err != nil {
		return nil, publicError(err)
	}
	if len(targets) == 0 {
		return []Result{}, nil
	}
	values := make([]address.PropertyAddress, len(targets))
	for i, target := range targets {
		value, err := target.internal()
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	session, err := evaluator.NewSessionWithDependencyGraph(s.evaluationSource(), s.source.DependencyGraph())
	if err != nil {
		return nil, publicError(err)
	}
	results, err := session.EvaluateMany(values, contextValue)
	if err != nil {
		return nil, publicError(err)
	}
	return resultsFromInternal(results), nil
}
func (s EvaluationSnapshot) EvaluateSeries(target PropertyAddress, context CalculationContext) (SeriesResult, error) {
	targetValue, contextValue, err := s.request(target, context)
	if err != nil {
		return SeriesResult{}, err
	}
	result, err := evaluator.EvaluateSeries(s.evaluationSource(), targetValue, contextValue)
	if err != nil {
		return SeriesResult{}, publicError(err)
	}
	return seriesResultFromInternal(result), nil
}
func (s EvaluationSnapshot) Explain(target PropertyAddress, context CalculationContext) (Explanation, error) {
	targetValue, contextValue, err := s.request(target, context)
	if err != nil {
		return Explanation{}, err
	}
	result, err := evaluator.Explain(s.evaluationSource(), targetValue, contextValue)
	if err != nil {
		return Explanation{}, publicError(err)
	}
	return explanationFromInternal(result), nil
}
func (s EvaluationSnapshot) ExplainSeries(target PropertyAddress, context CalculationContext) (SeriesExplanation, error) {
	targetValue, contextValue, err := s.request(target, context)
	if err != nil {
		return SeriesExplanation{}, err
	}
	result, err := evaluator.ExplainSeries(s.evaluationSource(), targetValue, contextValue)
	if err != nil {
		return SeriesExplanation{}, publicError(err)
	}
	return seriesExplanationFromInternal(result), nil
}
func (s EvaluationSnapshot) AnalyzeImpact(mode ImpactMode, source PropertyAddress, context CalculationContext) (ImpactResult, error) {
	if !s.IsValid() {
		return ImpactResult{}, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return ImpactResult{}, err
	}
	addressValue, err := source.internal()
	if err != nil {
		return ImpactResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return ImpactResult{}, err
	}
	if err := resolvervalue.ValidateContext(s.source, contextValue); err != nil {
		return ImpactResult{}, publicError(err)
	}
	var result impact.Result
	switch mode {
	case StructuralImpact:
		result, err = impact.AnalyzeStructuralSource(s.evaluationSource(), addressValue)
	case EffectiveImpact:
		result, err = impact.AnalyzeEffectiveSource(s.evaluationSource(), addressValue, contextValue)
	case ObservedImpact:
		result, err = impact.AnalyzeObservedSource(s.evaluationSource(), addressValue, contextValue)
	default:
		return ImpactResult{}, fmt.Errorf("%w: impact mode is invalid", ErrInvalidArgument)
	}
	if err != nil {
		return ImpactResult{}, publicError(err)
	}
	return impactResultFromInternal(result), nil
}
func (s EvaluationSnapshot) CheckConstraint(id ID, context CalculationContext) (ConstraintResult, error) {
	if !s.IsValid() || id.IsZero() {
		return ConstraintResult{}, fmt.Errorf("%w: evaluation snapshot and Constraint ID are required", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return ConstraintResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return ConstraintResult{}, err
	}
	result, err := constraintcheck.One(s.evaluationSource(), id.value, contextValue)
	if err != nil {
		return ConstraintResult{}, publicError(err)
	}
	return constraintResultFromInternal(result), nil
}
func (s EvaluationSnapshot) CheckConstraints(context CalculationContext) (ConstraintReport, error) {
	if !s.IsValid() {
		return ConstraintReport{}, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return ConstraintReport{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return ConstraintReport{}, err
	}
	result, err := constraintcheck.All(s.evaluationSource(), contextValue)
	if err != nil {
		return ConstraintReport{}, publicError(err)
	}
	return constraintReportFromInternal(result), nil
}
func (s EvaluationSnapshot) CheckFeasibility(context CalculationContext) (FeasibilityResult, error) {
	if !s.IsValid() {
		return FeasibilityResult{}, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return FeasibilityResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return FeasibilityResult{}, err
	}
	result, err := feasibility.Evaluate(s.evaluationSource(), contextValue)
	if err != nil {
		return FeasibilityResult{}, publicError(err)
	}
	return feasibilityResultFromInternal(result), nil
}
func (s EvaluationSnapshot) EvaluateRule(id ID, context CalculationContext) (RuleResult, error) {
	if !s.IsValid() || id.IsZero() {
		return RuleResult{}, fmt.Errorf("%w: evaluation snapshot and Rule ID are required", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return RuleResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return RuleResult{}, err
	}
	result, err := ruleeval.One(s.evaluationSource(), id.value, contextValue)
	if err != nil {
		return RuleResult{}, publicError(err)
	}
	return ruleResultFromInternal(result), nil
}
func (s EvaluationSnapshot) EvaluateRules(context CalculationContext) (RuleReport, error) {
	if !s.IsValid() {
		return RuleReport{}, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return RuleReport{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return RuleReport{}, err
	}
	result, err := ruleeval.All(s.evaluationSource(), contextValue)
	if err != nil {
		return RuleReport{}, publicError(err)
	}
	return ruleReportFromInternal(result), nil
}
func (s EvaluationSnapshot) NewSession(context CalculationContext) (*Session, error) {
	if !s.IsValid() {
		return nil, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	if err := s.validateRuntime(); err != nil {
		return nil, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return nil, err
	}
	if err := resolvervalue.ValidateContext(s.source, contextValue); err != nil {
		return nil, publicError(err)
	}
	value, err := evaluator.NewSessionWithDependencyGraph(s.evaluationSource(), s.source.DependencyGraph())
	if err != nil {
		return nil, publicError(err)
	}
	return newDatasetSession(s.model, s.dataset, value, contextValue), nil
}

// NewScenario authorizes overrides over Dataset-owned objects and binds such a
// Scenario to this exact Dataset. Model-only override sets remain portable.
func (s EvaluationSnapshot) NewScenario(spec ScenarioSpec) (Scenario, error) {
	if !s.IsValid() {
		return Scenario{}, fmt.Errorf("%w: evaluation snapshot is invalid", ErrInvalidArgument)
	}
	id, err := spec.ID.internal()
	if err != nil {
		return Scenario{}, fmt.Errorf("%w: Scenario ID: %v", ErrInvalidArgument, err)
	}
	if spec.Name == "" {
		return Scenario{}, fmt.Errorf("%w: Scenario name must not be empty", ErrInvalidArgument)
	}
	overrides := make([]domainscenario.Override, len(spec.Overrides))
	datasetTarget := false
	for i, override := range spec.Overrides {
		if !override.IsValid() {
			return Scenario{}, fmt.Errorf("%w: Scenario override %d is invalid", ErrInvalidArgument, i)
		}
		overrides[i] = override.value
		target := override.value.Target().Object()
		if target.Kind() == address.ObjectRefEntity {
			_, datasetTarget = s.dataset.Entity(idFromInternal(target.ID()))
		} else if target.Kind() == address.ObjectRefRelation {
			_, datasetTarget = s.dataset.Relation(idFromInternal(target.ID()))
		}
		if datasetTarget {
			break
		}
	}
	candidate := domainscenario.New(id, spec.Name, s.model.version.ID(), overrides)
	if datasetTarget {
		candidate = domainscenario.NewForDataset(id, spec.Name, s.model.version.ID(), s.dataset.value.ID(), overrides)
	}
	if report := modelscenario.Validate(s.source, candidate); !report.OK() {
		return Scenario{}, fmt.Errorf("%w: Scenario is incompatible with evaluation snapshot: %s", ErrInvalidArgument, report.Error())
	}
	return scenarioFromInternal(candidate), nil
}
