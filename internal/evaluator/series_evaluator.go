package evaluator

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/plan"
	"github.com/shmatovv/bcg-engine/internal/provenance"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/series"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	enginevalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

// EvaluateSeries evaluates one Series property at one effective-time instant.
// It keeps Effective Time in CalculationContext.At and leaves the selected
// value's Point/Interval axis untouched.
func EvaluateSeries(version model.ModelVersion, target address.PropertyAddress, context runtime.CalculationContext) (SeriesResult, error) {
	return evaluateSeries(version, target, context)
}

func evaluateSeries(version model.ModelVersion, target address.PropertyAddress, context runtime.CalculationContext) (SeriesResult, error) {
	if !version.IsCommitted() {
		return SeriesResult{}, newError(ErrorInvalidModelVersion, "Series evaluation requires a committed ModelVersion", nil)
	}
	if !target.IsValid() {
		return SeriesResult{}, newError(ErrorInvalidPropertyAddress, "Series target property address is invalid", nil)
	}
	if !context.IsValid() {
		return SeriesResult{}, newError(ErrorInvalidCalculationContext, "calculation context is invalid", nil)
	}
	if _, ok := context.TimeContext().At(); !ok {
		return SeriesResult{}, newError(ErrorUnsupportedTimeContext, "Series evaluation requires an At calculation time context", nil)
	}
	definition, ok := version.PropertyDefinition(target.DefinitionID())
	if !ok || !definition.IsValid() {
		return SeriesResult{}, newError(ErrorDefinitionNotFound, "Series target property definition could not be resolved", nil)
	}
	if definition.TemporalMode() != schema.TemporalSeries {
		return SeriesResult{}, newError(ErrorUnsupportedSeries, "EvaluateSeries requires a Series property target", nil)
	}
	effectivePlan, err := plan.Build(version, []address.PropertyAddress{target}, context)
	if err != nil {
		return SeriesResult{}, newError(ErrorPlanBuild, "failed to build Effective Evaluation Plan for Series evaluation", err)
	}
	state := newSeriesEvaluationState(version, context, effectivePlan)
	result, err := state.evaluateProperty(target, nil)
	if err != nil {
		return SeriesResult{}, err
	}
	return result.result, nil
}

type seriesEvaluationState struct {
	version model.ModelVersion
	context runtime.CalculationContext
	plan    plan.Plan

	// memo caches only direct source/override snapshots. Derived properties are
	// intentionally evaluated per demand mask: a result computed for one subset
	// must never stand in for an unevaluated sample in another subset.
	memo     map[address.PropertyAddress]SeriesResult
	visiting map[address.PropertyAddress]struct{}

	// Scalar nodes can occur as a whole-Series IF condition. The established
	// scalar evaluator retains its own memo and lazy semantics for that path.
	scalar *evaluationState
}

func newSeriesEvaluationState(version model.ModelVersion, context runtime.CalculationContext, effectivePlan plan.Plan) *seriesEvaluationState {
	scalar := &evaluationState{
		version: version, context: context, effectivePlan: effectivePlan,
		memo: make(map[address.PropertyAddress]Result), visiting: make(map[address.PropertyAddress]struct{}),
	}
	return &seriesEvaluationState{
		version: version, context: context, plan: effectivePlan,
		memo: make(map[address.PropertyAddress]SeriesResult), visiting: make(map[address.PropertyAddress]struct{}), scalar: scalar,
	}
}

func (s *seriesEvaluationState) evaluateProperty(target address.PropertyAddress, demand *seriesDemand) (seriesPartial, error) {
	if result, ok := s.memo[target]; ok {
		return newSeriesPartial(result, demand)
	}
	node, ok := s.plan.Node(target)
	if !ok {
		return seriesPartial{}, newError(ErrorInvariantViolation, "Series evaluation requested a property absent from the Effective Evaluation Plan", nil)
	}
	if _, active := s.visiting[target]; active {
		return seriesPartial{}, newError(ErrorEvaluationCycle, "Series evaluation encountered a dependency cycle", nil)
	}
	definition, err := s.scalar.definitionForTarget(target)
	if err != nil {
		return seriesPartial{}, err
	}
	if definition.TemporalMode() != schema.TemporalSeries {
		return seriesPartial{}, newError(ErrorInvariantViolation, "Series expression referenced a Scalar property as a Series operand", nil)
	}
	s.visiting[target] = struct{}{}
	defer delete(s.visiting, target)

	var result SeriesResult
	cacheResult := false
	switch node.Kind() {
	case plan.NodeCalculationOverride, plan.NodeScenarioOverride:
		outcome, found, err := resolvervalue.ResolveEffectiveSeriesOverride(s.version, target, s.context)
		if err != nil || !found {
			if err != nil {
				return seriesPartial{}, newError(ErrorResolverFailure, "Series override resolution failed", err)
			}
			return seriesPartial{}, newError(ErrorInvariantViolation, "Series override plan node has no effective override", nil)
		}
		kind, err := propertyProvenanceKind(definition)
		if err != nil {
			return seriesPartial{}, err
		}
		result, err = seriesResultFromOutcome(target, kind, outcome)
		if err != nil {
			return seriesPartial{}, err
		}
		cacheResult = true

	case plan.NodeInput:
		if definition.Computation().Kind() != schema.ComputationInput {
			return seriesPartial{}, newError(ErrorInvariantViolation, "Effective Evaluation Plan marks a non-Input Series property as Input", nil)
		}
		outcome, err := resolvervalue.ResolveSeries(s.version, target, s.context)
		if err != nil {
			return seriesPartial{}, newError(ErrorResolverFailure, "Series value resolution failed", err)
		}
		if source, hasSource := outcome.Source(); hasSource && (source.Kind() == resolvervalue.SourceCalculationOverride || source.Kind() == resolvervalue.SourceScenarioOverride) {
			return seriesPartial{}, newError(ErrorInvariantViolation, "Series resolver disagrees with Effective Evaluation Plan override decision", nil)
		}
		result, err = seriesResultFromOutcome(target, provenance.KindInputProperty, outcome)
		if err != nil {
			return seriesPartial{}, err
		}
		cacheResult = true

	case plan.NodeDerivedExpression:
		if definition.Computation().Kind() != schema.ComputationDerived {
			return seriesPartial{}, newError(ErrorInvariantViolation, "Effective Evaluation Plan marks a non-Derived Series property as DerivedExpression", nil)
		}
		expression, exists := definition.Computation().Expression()
		if !exists || expression == nil {
			return seriesPartial{}, newError(ErrorInvariantViolation, "committed Derived Series property has no expression", nil)
		}
		partial, err := s.evaluateExpression(expression, expressionContext{self: target.Object()}, demand)
		if err != nil {
			return seriesPartial{}, err
		}
		partial, err = s.normalizeToDefinition(partial, definition)
		if err != nil {
			return seriesPartial{}, err
		}
		if partial.isUndefined() {
			result = newUndefinedSeriesResult(partial.result.undefined, provenance.NewProperty(provenance.KindDerivedProperty, target, partial.result.undefinedProvenance))
			return newSeriesPartial(result, nil)
		}
		partial, err = mapPartialSeriesResults(partial, func(sample Result) (Result, error) {
			return newResult(sample.Value(), provenance.NewProperty(provenance.KindDerivedProperty, target, sample.Provenance())), nil
		})
		if err != nil {
			return seriesPartial{}, err
		}
		return partial, nil

	default:
		return seriesPartial{}, newError(ErrorInvariantViolation, "Effective Evaluation Plan contains an invalid Series node kind", nil)
	}

	if cacheResult {
		s.memo[target] = result
	}
	return newSeriesPartial(result, demand)
}

func seriesResultFromOutcome(target address.PropertyAddress, propertyKind provenance.Kind, outcome resolvervalue.SeriesOutcome) (SeriesResult, error) {
	if !outcome.IsValid() {
		return SeriesResult{}, newError(ErrorInvariantViolation, "Series resolver returned an invalid outcome", nil)
	}
	if info, undefined := outcome.UndefinedInfo(); undefined {
		node, err := seriesUndefinedProvenance(info)
		if err != nil {
			return SeriesResult{}, err
		}
		return newUndefinedSeriesResult(info, provenance.NewProperty(propertyKind, target, node)), nil
	}
	stored, ok := outcome.Series()
	if !ok {
		return SeriesResult{}, newError(ErrorInvariantViolation, "defined Series resolver outcome has no Series", nil)
	}
	source, ok := outcome.Source()
	if !ok {
		return SeriesResult{}, newError(ErrorInvariantViolation, "defined Series resolver outcome has no source", nil)
	}
	sourceNode, err := provenanceFromSource(source)
	if err != nil {
		return SeriesResult{}, err
	}
	return sourceSeriesResult(stored, func(resolved enginevalue.Resolved) (Result, error) {
		return newResult(resolved, provenance.NewProperty(propertyKind, target, sourceNode)), nil
	})
}

func seriesUndefinedProvenance(info enginevalue.UndefinedInfo) (provenance.Node, error) {
	switch info.Kind() {
	case enginevalue.UndefinedMissingValue:
		return provenance.NewMissingValue(), nil
	case enginevalue.UndefinedAmbiguousValue:
		return provenance.NewAmbiguousValue(), nil
	default:
		return provenance.Node{}, newError(ErrorInvariantViolation, "Series resolver returned unsupported source-less Undefined value", nil)
	}
}

func sourceSeriesResult(stored series.Series, wrap func(enginevalue.Resolved) (Result, error)) (SeriesResult, error) {
	convert := func(sample series.Sample) (Result, error) {
		resolved, err := sourceSampleResolved(stored, sample)
		if err != nil {
			return Result{}, err
		}
		return wrap(resolved)
	}
	switch stored.AxisKind() {
	case series.AxisPoint:
		points := stored.Points()
		results := make([]SeriesPoint, len(points))
		for index, point := range points {
			result, err := convert(point.Sample())
			if err != nil {
				return SeriesResult{}, err
			}
			results[index] = newSeriesPoint(point.At(), result)
		}
		return newDefinedPointSeriesResult(stored.ElementType(), seriesUnit(stored), stored.HasUnit(), results), nil
	case series.AxisInterval:
		intervals := stored.Intervals()
		results := make([]SeriesInterval, len(intervals))
		for index, interval := range intervals {
			result, err := convert(interval.Sample())
			if err != nil {
				return SeriesResult{}, err
			}
			results[index] = newSeriesInterval(interval.From(), interval.To(), result)
		}
		return newDefinedIntervalSeriesResult(stored.ElementType(), seriesUnit(stored), stored.HasUnit(), results), nil
	default:
		return SeriesResult{}, newError(ErrorInvariantViolation, "source Series has invalid axis", nil)
	}
}

func sourceSampleResolved(stored series.Series, sample series.Sample) (enginevalue.Resolved, error) {
	if sample.IsNull() {
		return enginevalue.Null(), nil
	}
	raw, ok := sample.Value()
	if !ok {
		return enginevalue.Resolved{}, newError(ErrorInvariantViolation, "source Series contains an invalid sample", nil)
	}
	if stored.HasUnit() {
		return enginevalue.Defined(enginevalue.NewTypedWithUnit(raw, stored.Unit())), nil
	}
	return enginevalue.Defined(enginevalue.NewTyped(raw)), nil
}

func seriesUnit(stored series.Series) unit.Unit {
	if stored.HasUnit() {
		return stored.Unit()
	}
	return unit.Unit{}
}

func (s *seriesEvaluationState) evaluateExpression(expression ast.Expression, context expressionContext, demand *seriesDemand) (seriesPartial, error) {
	if expression == nil || !context.IsValid() {
		return seriesPartial{}, invalidCommittedExpressionError()
	}
	switch expression := expression.(type) {
	case ast.SelfPropertyReference:
		return s.evaluateReference(address.NewPropertyAddress(context.self, expression.DefinitionID()), demand)
	case ast.AbsolutePropertyReference:
		return s.evaluateReference(expression.Address(), demand)
	case ast.Arithmetic:
		return s.evaluateArithmetic(expression, context, demand, expression.Operator().String(), func(left, right enginevalue.Resolved) (enginevalue.Resolved, error) {
			return s.scalar.evaluateArithmetic(expression.Operator(), left, right)
		})
	case ast.Negate:
		return s.evaluateUnary(expression, context, demand, "Negate", func(value enginevalue.Resolved) (enginevalue.Resolved, error) { return evaluateNegate(value) })
	case ast.Comparison:
		return s.evaluateComparison(expression, context, demand)
	case ast.Logical:
		return s.evaluateLogical(expression, context, demand)
	case ast.Not:
		return s.evaluateUnary(expression, context, demand, "Not", evaluateNot)
	case ast.If:
		return s.evaluateIf(expression, context, demand)
	case ast.IsNull:
		return s.evaluateUnary(expression, context, demand, "IsNull", func(value enginevalue.Resolved) (enginevalue.Resolved, error) { return evaluateIsNull(value), nil })
	case ast.Coalesce:
		return s.evaluateCoalesce(expression, context, demand)
	case ast.Aggregate:
		return seriesPartial{}, newError(ErrorUnsupportedSeries, "graph Aggregate over Series is not implemented", nil)
	default:
		return seriesPartial{}, newError(ErrorInvariantViolation, "committed Series expression is not Series-capable", nil)
	}
}

func (s *seriesEvaluationState) evaluateReference(target address.PropertyAddress, demand *seriesDemand) (seriesPartial, error) {
	dependency, err := s.evaluateProperty(target, demand)
	if err != nil {
		return seriesPartial{}, err
	}
	if dependency.isUndefined() {
		return operationUndefined("Reference", dependency)
	}
	return mapPartialSeriesResults(dependency, func(sample Result) (Result, error) {
		resolved := sample.Value()
		if resolved.IsUndefined() {
			resolved = dependencyUndefined()
		}
		return newResult(resolved, provenance.NewProperty(provenance.KindReference, target, sample.Provenance())), nil
	})
}

func (s *seriesEvaluationState) evaluateUnary(expression ast.Expression, context expressionContext, demand *seriesDemand, operation string, fn func(enginevalue.Resolved) (enginevalue.Resolved, error)) (seriesPartial, error) {
	var operand ast.Expression
	switch typed := expression.(type) {
	case ast.Negate:
		operand = typed.Operand()
	case ast.Not:
		operand = typed.Operand()
	case ast.IsNull:
		operand = typed.Operand()
	default:
		return seriesPartial{}, invalidCommittedExpressionError()
	}
	input, err := s.evaluateExpression(operand, context, demand)
	if err != nil {
		return seriesPartial{}, err
	}
	if input.isUndefined() {
		return operationUndefined(operation, input)
	}
	values := make([]Result, len(input.demand))
	for index, demanded := range input.demand {
		if !demanded {
			continue
		}
		sample, err := input.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		resolved, err := fn(sample.Value())
		if err != nil {
			return seriesPartial{}, err
		}
		values[index] = newResult(resolved, provenance.NewOperation(operation, sample.Provenance()))
	}
	return s.buildExpressionResult(input, values, expression, context)
}

func (s *seriesEvaluationState) evaluateArithmetic(expression ast.Arithmetic, context expressionContext, demand *seriesDemand, operation string, fn func(enginevalue.Resolved, enginevalue.Resolved) (enginevalue.Resolved, error)) (seriesPartial, error) {
	left, err := s.evaluateExpression(expression.Left(), context, demand)
	if err != nil {
		return seriesPartial{}, err
	}
	if left.isUndefined() {
		return operationUndefined(operation, left)
	}
	rightDemand, err := left.demandFor(left.demand)
	if err != nil {
		return seriesPartial{}, err
	}
	right, err := s.evaluateExpression(expression.Right(), context, rightDemand)
	if err != nil {
		return seriesPartial{}, err
	}
	if right.isUndefined() {
		return operationUndefined(operation, right)
	}
	return s.combineBinary(left, right, expression, context, operation, fn)
}

func (s *seriesEvaluationState) evaluateComparison(expression ast.Comparison, context expressionContext, demand *seriesDemand) (seriesPartial, error) {
	left, err := s.evaluateExpression(expression.Left(), context, demand)
	if err != nil {
		return seriesPartial{}, err
	}
	if left.isUndefined() {
		return operationUndefined(expression.Operator().String(), left)
	}
	rightDemand, err := left.demandFor(left.demand)
	if err != nil {
		return seriesPartial{}, err
	}
	right, err := s.evaluateExpression(expression.Right(), context, rightDemand)
	if err != nil {
		return seriesPartial{}, err
	}
	if right.isUndefined() {
		return operationUndefined(expression.Operator().String(), right)
	}
	return s.combineBinary(left, right, expression, context, expression.Operator().String(), func(left, right enginevalue.Resolved) (enginevalue.Resolved, error) {
		return evaluateComparison(expression.Operator(), left, right)
	})
}

func (s *seriesEvaluationState) combineBinary(left, right seriesPartial, expression ast.Expression, context expressionContext, operation string, fn func(enginevalue.Resolved, enginevalue.Resolved) (enginevalue.Resolved, error)) (seriesPartial, error) {
	if !compatibleFor(left, right, left.demand) {
		return seriesPartial{}, incompatibleSeriesAxisError()
	}
	values := make([]Result, len(left.demand))
	for index, demanded := range left.demand {
		if !demanded {
			continue
		}
		leftSample, err := left.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		rightSample, err := left.sampleFor(right, index)
		if err != nil {
			return seriesPartial{}, err
		}
		resolved, err := fn(leftSample.Value(), rightSample.Value())
		if err != nil {
			return seriesPartial{}, err
		}
		values[index] = newResult(resolved, provenance.NewOperation(operation, leftSample.Provenance(), rightSample.Provenance()))
	}
	return s.buildExpressionResult(left, values, expression, context)
}

func (s *seriesEvaluationState) evaluateLogical(expression ast.Logical, context expressionContext, demand *seriesDemand) (seriesPartial, error) {
	left, err := s.evaluateExpression(expression.Left(), context, demand)
	if err != nil {
		return seriesPartial{}, err
	}
	if left.isUndefined() {
		return operationUndefined(expression.Operator().String(), left)
	}
	values := make([]Result, len(left.demand))
	rightDemand := make([]bool, len(left.demand))
	for index, demanded := range left.demand {
		if !demanded {
			continue
		}
		sample, err := left.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		short, determined, err := logicalShortCircuit(expression.Operator(), sample.Value())
		if err != nil {
			return seriesPartial{}, err
		}
		if determined {
			values[index] = newResult(short, provenance.NewOperation(expression.Operator().String(), sample.Provenance()))
			continue
		}
		rightDemand[index] = true
	}
	if !hasDemand(rightDemand) {
		return s.buildExpressionResult(left, values, expression, context)
	}
	rightRequest, err := left.demandFor(rightDemand)
	if err != nil {
		return seriesPartial{}, err
	}
	right, err := s.evaluateExpression(expression.Right(), context, rightRequest)
	if err != nil {
		return seriesPartial{}, err
	}
	if right.isDefined() && !compatibleFor(left, right, rightDemand) {
		return seriesPartial{}, incompatibleSeriesAxisError()
	}
	for index, needed := range rightDemand {
		if !needed {
			continue
		}
		leftSample, err := left.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		var rightSample Result
		if right.isDefined() {
			rightSample, err = left.sampleFor(right, index)
			if err != nil {
				return seriesPartial{}, err
			}
		} else {
			rightSample = newResult(dependencyUndefined(), right.result.undefinedProvenance)
		}
		resolved, err := evaluateLogicalPair(expression.Operator(), leftSample.Value(), rightSample.Value())
		if err != nil {
			return seriesPartial{}, err
		}
		values[index] = newResult(resolved, provenance.NewOperation(expression.Operator().String(), leftSample.Provenance(), rightSample.Provenance()))
	}
	return s.buildExpressionResult(left, values, expression, context)
}

func mapPartialSeriesResults(input seriesPartial, transform func(Result) (Result, error)) (seriesPartial, error) {
	if input.isUndefined() {
		return seriesPartial{}, newError(ErrorInvariantViolation, "cannot transform a whole-Series Undefined result", nil)
	}
	values := make([]Result, len(input.demand))
	for index, demanded := range input.demand {
		if !demanded {
			continue
		}
		sample, err := input.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		result, err := transform(sample)
		if err != nil {
			return seriesPartial{}, err
		}
		values[index] = result
	}
	return input.withValues(input.result.elementType, input.result.unit, input.result.hasUnit, values)
}

func hasDemand(demand []bool) bool {
	for _, demanded := range demand {
		if demanded {
			return true
		}
	}
	return false
}

func seriesSampleAt(input SeriesResult, index int) Result {
	switch input.axisKind {
	case series.AxisPoint:
		return input.points[index].result
	case series.AxisInterval:
		return input.intervals[index].result
	default:
		panic("evaluator: invalid Series axis")
	}
}

func makeSeriesResult(template SeriesResult, elementType enginevalue.Type, u unit.Unit, hasUnit bool, values []Result) (SeriesResult, error) {
	if !template.IsDefined() || len(values) != seriesSampleCount(template) {
		return SeriesResult{}, newError(ErrorInvariantViolation, "cannot construct Series result from incompatible template", nil)
	}
	switch template.axisKind {
	case series.AxisPoint:
		points := make([]SeriesPoint, len(template.points))
		for index, point := range template.points {
			points[index] = newSeriesPoint(point.at, values[index])
		}
		return newDefinedPointSeriesResult(elementType, u, hasUnit, points), nil
	case series.AxisInterval:
		intervals := make([]SeriesInterval, len(template.intervals))
		for index, interval := range template.intervals {
			intervals[index] = newSeriesInterval(interval.from, interval.to, values[index])
		}
		return newDefinedIntervalSeriesResult(elementType, u, hasUnit, intervals), nil
	default:
		return SeriesResult{}, newError(ErrorInvariantViolation, "Series template has an invalid axis", nil)
	}
}

func (s *seriesEvaluationState) staticType(expression ast.Expression, context expressionContext) (typing.Type, error) {
	staticContext, err := context.inferenceContext(s.scalar)
	if err != nil {
		return typing.Type{}, err
	}
	result, err := inference.Infer(expression, staticContext, s.version)
	if err != nil || !result.IsValid() {
		return typing.Type{}, newError(ErrorInvariantViolation, "committed Series expression could not be statically inferred at runtime", err)
	}
	return result, nil
}

func staticSeriesMetadata(typeInfo typing.Type) (enginevalue.Type, unit.Unit, bool, error) {
	if !typeInfo.IsValid() || typeInfo.TemporalMode() != typing.TemporalSeries {
		return enginevalue.Invalid, unit.Unit{}, false, newError(ErrorInvariantViolation, "expression did not infer a Series result", nil)
	}
	u, hasUnit := typeInfo.Unit()
	return typeInfo.ValueType(), u, hasUnit, nil
}

func (s *seriesEvaluationState) buildExpressionResult(template seriesPartial, values []Result, expression ast.Expression, context expressionContext) (seriesPartial, error) {
	typeInfo, err := s.staticType(expression, context)
	if err != nil {
		return seriesPartial{}, err
	}
	elementType, u, hasUnit, err := staticSeriesMetadata(typeInfo)
	if err != nil {
		return seriesPartial{}, err
	}
	if len(values) != len(template.demand) {
		return seriesPartial{}, newError(ErrorInvariantViolation, "expression values do not match Series demand", nil)
	}
	for index, demanded := range template.demand {
		if !demanded {
			continue
		}
		resolved, err := s.scalar.normalizeResolvedToStaticType(values[index].Value(), typeInfo)
		if err != nil {
			return seriesPartial{}, err
		}
		values[index] = newResult(resolved, values[index].Provenance())
	}
	return template.withValues(elementType, u, hasUnit, values)
}

func (s *seriesEvaluationState) normalizeToDefinition(result seriesPartial, definition schema.PropertyDefinition) (seriesPartial, error) {
	if result.isUndefined() {
		return result, nil
	}
	values := make([]Result, len(result.demand))
	for index, demanded := range result.demand {
		if !demanded {
			continue
		}
		sample, err := result.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		resolved, err := s.scalar.normalizeDerivedResult(sample.Value(), definition)
		if err != nil {
			return seriesPartial{}, err
		}
		values[index] = newResult(resolved, sample.Provenance())
	}
	u, hasUnit := definition.Unit()
	return result.withValues(definition.ValueType(), u, hasUnit, values)
}

func operationUndefined(operation string, child seriesPartial) (seriesPartial, error) {
	if !child.isUndefined() {
		return seriesPartial{}, newError(ErrorInvariantViolation, "operation Undefined requires an Undefined child", nil)
	}
	return newSeriesPartial(newUndefinedSeriesResult(enginevalue.NewUndefinedInfo(enginevalue.UndefinedDependencyUndefined), provenance.NewOperation(operation, child.result.undefinedProvenance)), nil)
}

func (s *seriesEvaluationState) evaluateIf(expression ast.If, context expressionContext, demand *seriesDemand) (seriesPartial, error) {
	conditionType, err := s.staticType(expression.Condition(), context)
	if err != nil {
		return seriesPartial{}, err
	}
	if conditionType.TemporalMode() == typing.TemporalScalar {
		condition, err := s.scalar.evaluateExpression(expression.Condition(), context)
		if err != nil {
			return seriesPartial{}, err
		}
		if condition.Value().IsNull() || condition.Value().IsUndefined() {
			return newSeriesPartial(newUndefinedSeriesResult(enginevalue.NewUndefinedInfo(enginevalue.UndefinedIndeterminateCondition), provenance.NewOperation("If", condition.Provenance())), nil)
		}
		conditionValue := condition.Value().Value()
		if conditionValue.Type() != enginevalue.Bool || conditionValue.HasUnit() {
			return seriesPartial{}, newError(ErrorInvariantViolation, "committed IF condition is not a unitless Bool", nil)
		}
		selected := expression.Else()
		if conditionValue.Raw().Bool() {
			selected = expression.Then()
		}
		branch, err := s.evaluateExpression(selected, context, demand)
		if err != nil {
			return seriesPartial{}, err
		}
		if branch.isUndefined() {
			return newSeriesPartial(newUndefinedSeriesResult(enginevalue.NewUndefinedInfo(enginevalue.UndefinedDependencyUndefined), provenance.NewOperation("If", condition.Provenance(), branch.result.undefinedProvenance)), nil)
		}
		values := make([]Result, len(branch.demand))
		for index, demanded := range branch.demand {
			if !demanded {
				continue
			}
			sample, err := branch.sample(index)
			if err != nil {
				return seriesPartial{}, err
			}
			values[index] = newResult(sample.Value(), provenance.NewOperation("If", condition.Provenance(), sample.Provenance()))
		}
		return s.buildExpressionResult(branch, values, expression, context)
	}
	if conditionType.TemporalMode() != typing.TemporalSeries {
		return seriesPartial{}, newError(ErrorInvariantViolation, "committed IF condition has an invalid temporal mode", nil)
	}
	condition, err := s.evaluateExpression(expression.Condition(), context, demand)
	if err != nil {
		return seriesPartial{}, err
	}
	if condition.isUndefined() {
		return newSeriesPartial(newUndefinedSeriesResult(enginevalue.NewUndefinedInfo(enginevalue.UndefinedIndeterminateCondition), provenance.NewOperation("If", condition.result.undefinedProvenance)), nil)
	}
	thenDemand := make([]bool, len(condition.demand))
	elseDemand := make([]bool, len(condition.demand))
	for index, demanded := range condition.demand {
		if !demanded {
			continue
		}
		sample, err := condition.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		resolved := sample.Value()
		if !resolved.IsDefined() {
			continue
		}
		if resolved.Value().Type() != enginevalue.Bool || resolved.Value().HasUnit() {
			return seriesPartial{}, newError(ErrorInvariantViolation, "committed IF condition sample is not a unitless Bool", nil)
		}
		if resolved.Value().Raw().Bool() {
			thenDemand[index] = true
		} else {
			elseDemand[index] = true
		}
	}
	var thenResult, elseResult seriesPartial
	if hasDemand(thenDemand) {
		thenRequest, demandErr := condition.demandFor(thenDemand)
		if demandErr != nil {
			return seriesPartial{}, demandErr
		}
		thenResult, err = s.evaluateExpression(expression.Then(), context, thenRequest)
		if err != nil {
			return seriesPartial{}, err
		}
		if thenResult.isDefined() && !compatibleFor(condition, thenResult, thenDemand) {
			return seriesPartial{}, incompatibleSeriesAxisError()
		}
	}
	if hasDemand(elseDemand) {
		elseRequest, demandErr := condition.demandFor(elseDemand)
		if demandErr != nil {
			return seriesPartial{}, demandErr
		}
		elseResult, err = s.evaluateExpression(expression.Else(), context, elseRequest)
		if err != nil {
			return seriesPartial{}, err
		}
		if elseResult.isDefined() && !compatibleFor(condition, elseResult, elseDemand) {
			return seriesPartial{}, incompatibleSeriesAxisError()
		}
	}
	values := make([]Result, len(condition.demand))
	for index, demanded := range condition.demand {
		if !demanded {
			continue
		}
		sample, err := condition.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		resolved := sample.Value()
		if resolved.IsNull() || resolved.IsUndefined() {
			values[index] = newResult(enginevalue.Undefined(enginevalue.NewUndefinedInfo(enginevalue.UndefinedIndeterminateCondition)), provenance.NewOperation("If", sample.Provenance()))
			continue
		}
		selected := elseResult
		if resolved.Value().Raw().Bool() {
			selected = thenResult
		}
		var selectedSample Result
		if selected.isDefined() {
			selectedSample, err = condition.sampleFor(selected, index)
			if err != nil {
				return seriesPartial{}, err
			}
		} else {
			selectedSample = newResult(dependencyUndefined(), selected.result.undefinedProvenance)
		}
		value := selectedSample.Value()
		if value.IsUndefined() {
			value = dependencyUndefined()
		}
		values[index] = newResult(value, provenance.NewOperation("If", sample.Provenance(), selectedSample.Provenance()))
	}
	return s.buildExpressionResult(condition, values, expression, context)
}

func (s *seriesEvaluationState) evaluateCoalesce(expression ast.Coalesce, context expressionContext, demand *seriesDemand) (seriesPartial, error) {
	primary, err := s.evaluateExpression(expression.Primary(), context, demand)
	if err != nil {
		return seriesPartial{}, err
	}
	if primary.isUndefined() {
		return operationUndefined("Coalesce", primary)
	}
	fallbackDemand := make([]bool, len(primary.demand))
	values := make([]Result, len(primary.demand))
	for index, demanded := range primary.demand {
		if !demanded {
			continue
		}
		sample, err := primary.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		switch {
		case sample.Value().IsUndefined():
			values[index] = newResult(dependencyUndefined(), provenance.NewOperation("Coalesce", sample.Provenance()))
		case !sample.Value().IsNull():
			values[index] = newResult(sample.Value(), provenance.NewOperation("Coalesce", sample.Provenance()))
		default:
			fallbackDemand[index] = true
		}
	}
	if !hasDemand(fallbackDemand) {
		return s.buildExpressionResult(primary, values, expression, context)
	}
	fallbackRequest, err := primary.demandFor(fallbackDemand)
	if err != nil {
		return seriesPartial{}, err
	}
	fallback, err := s.evaluateExpression(expression.Fallback(), context, fallbackRequest)
	if err != nil {
		return seriesPartial{}, err
	}
	if fallback.isDefined() && !compatibleFor(primary, fallback, fallbackDemand) {
		return seriesPartial{}, incompatibleSeriesAxisError()
	}
	for index, needed := range fallbackDemand {
		if !needed {
			continue
		}
		primarySample, err := primary.sample(index)
		if err != nil {
			return seriesPartial{}, err
		}
		var fallbackSample Result
		if fallback.isDefined() {
			fallbackSample, err = primary.sampleFor(fallback, index)
			if err != nil {
				return seriesPartial{}, err
			}
		} else {
			fallbackSample = newResult(dependencyUndefined(), fallback.result.undefinedProvenance)
		}
		resolved := fallbackSample.Value()
		if resolved.IsUndefined() {
			resolved = dependencyUndefined()
		}
		values[index] = newResult(resolved, provenance.NewOperation("Coalesce", primarySample.Provenance(), fallbackSample.Provenance()))
	}
	return s.buildExpressionResult(primary, values, expression, context)
}
