package evaluator

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/plan"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	resolvervalue "github.com/shmatovv/bcg-engine/v2/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// Evaluate is the convenient one-root evaluation API.
//
// It now follows the complete runtime path:
//
//	ModelVersion
//	+ CalculationContext
//	+ target
//	    ↓
//	Build Effective Evaluation Plan
//	    ↓
//	EvaluatePlan
//	    ↓
//	Result
//
// This guarantees that override pruning used by evaluation is the same
// semantics exposed through the explicit Plan API.
func Evaluate(
	version Source,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Result, error) {
	if !target.IsValid() {
		return Result{},
			newError(
				ErrorInvalidPropertyAddress,
				"target property address is invalid",
				nil,
			)
	}

	var effectivePlan plan.Plan
	var err error
	if graph, ok := dependencyGraph(version); ok {
		effectivePlan, err = plan.BuildWithDependencyGraph(version, graph, []address.PropertyAddress{target}, context)
	} else {
		effectivePlan, err = plan.Build(version, []address.PropertyAddress{target}, context)
	}

	if err != nil {
		return Result{},
			newError(
				ErrorPlanBuild,
				"failed to build Effective Evaluation Plan",
				err,
			)
	}

	evaluation, err :=
		EvaluatePlan(
			version,
			effectivePlan,
		)

	if err != nil {
		return Result{},
			err
	}

	result, ok :=
		evaluation.Result(
			target,
		)

	if !ok {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"completed one-root evaluation contains no root result",
				nil,
			)
	}

	return result, nil
}

// EvaluatePlan evaluates all roots of one already-built Effective Evaluation
// Plan.
//
// The Plan is context-bound, therefore no separate CalculationContext argument
// exists here. Reusing a plan with another Scenario/Scope/Time/override set is
// impossible through this API.
//
// Runtime remains lazy despite the static plan. This is necessary to preserve
// deterministic short-circuit semantics: an unused IF branch must not be
// evaluated merely because it is statically reachable in the Base Dependency
// Graph.
func EvaluatePlan(
	version Source,
	effectivePlan plan.Plan,
) (PlanResult, error) {
	if !version.IsCommitted() {
		return PlanResult{},
			newError(
				ErrorInvalidModelVersion,
				"evaluation requires a committed ModelVersion",
				nil,
			)
	}

	if !effectivePlan.IsValid() {
		return PlanResult{},
			newError(
				ErrorInvalidEvaluationPlan,
				"Effective Evaluation Plan is invalid",
				nil,
			)
	}

	if !effectivePlan.SourceIdentity().Equal(version.SourceIdentity()) {

		return PlanResult{},
			newError(
				ErrorInvalidEvaluationPlan,
				"Effective Evaluation Plan belongs to a different immutable evaluation source",
				nil,
			)
	}

	context :=
		effectivePlan.Context()

	if !context.IsValid() {
		return PlanResult{},
			newError(
				ErrorInvalidEvaluationPlan,
				"Effective Evaluation Plan contains an invalid CalculationContext",
				nil,
			)
	}

	if context.
		TimeContext().
		Kind() !=
		domaintime.ContextAt {

		return PlanResult{},
			newError(
				ErrorUnsupportedTimeContext,
				"scalar evaluator currently requires an At time context",
				nil,
			)
	}

	if err :=
		resolvervalue.ValidateContext(
			version,
			context,
		); err != nil {

		return PlanResult{},
			newError(
				ErrorResolverFailure,
				"plan CalculationContext validation failed",
				err,
			)
	}

	state :=
		evaluationState{
			version:       version,
			context:       context,
			effectivePlan: effectivePlan,

			memo: make(
				map[address.PropertyAddress]Result,
			),

			visiting: make(
				map[address.PropertyAddress]struct{},
			),
		}

	for _, root := range effectivePlan.Roots() {

		if _, err :=
			state.evaluateProperty(
				root,
			); err != nil {

			return PlanResult{},
				err
		}
	}

	result :=
		newPlanResult(
			effectivePlan,
			state.memo,
		)

	if !result.IsValid() {
		return PlanResult{},
			newError(
				ErrorInvariantViolation,
				"completed evaluation produced an invalid PlanResult",
				nil,
			)
	}

	return result, nil
}

type evaluationState struct {
	version Source
	context runtime.CalculationContext

	effectivePlan plan.Plan

	// memo records values used by this one recursive evaluation. It preserves
	// per-call occurrence semantics even when a Session supplies a shared
	// property cache below.
	memo map[address.PropertyAddress]Result

	cache propertyResultCache

	visiting map[address.PropertyAddress]struct{}

	// expressionDepth bounds recursive expression/property evaluation. Go's
	// goroutine stack has a hard upper bound, so unbounded valid dependency
	// chains must fail as an ordinary EngineError rather than terminating the
	// process. The same guard also bounds arbitrary AST nesting.
	expressionDepth int
}

// maxEvaluationDepth is the maximum active scalar expression depth. It is a
// deliberately conservative release-safety limit: every property reference
// nests evaluator frames in addition to its expression frame. Models above
// this depth remain valid artifacts but evaluation returns
// ErrorEvaluationDepthExceeded deterministically.
const maxEvaluationDepth = ast.MaxExpressionDepth

func (s *evaluationState) enterExpression() error {
	if s.expressionDepth >= maxEvaluationDepth {
		return newError(
			ErrorEvaluationDepthExceeded,
			"scalar evaluation exceeded the maximum supported nested expression depth",
			nil,
		)
	}
	s.expressionDepth++
	return nil
}

func (s *evaluationState) leaveExpression() {
	s.expressionDepth--
	if s.expressionDepth < 0 {
		panic("evaluator: scalar expression depth underflow")
	}
}

// propertyResultCache is the narrow runtime hook used by EvaluationSession.
// Caching only concrete PropertyAddress results keeps lexical aggregate
// bindings out of cache identity and leaves all AST semantics in this
// evaluator unchanged.
type propertyResultCache interface {
	Load(address.PropertyAddress) (Result, bool)
	Store(address.PropertyAddress, Result)
}

func (s *evaluationState) evaluateProperty(
	target address.PropertyAddress,
) (Result, error) {
	if result, ok :=
		s.memo[target]; ok {

		return result, nil
	}

	if s.cache != nil {
		if result, ok := s.cache.Load(target); ok {
			s.memo[target] = result
			return result, nil
		}
	}

	planNode, ok :=
		s.effectivePlan.Node(
			target,
		)

	if !ok {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"runtime expression requested a property absent from the Effective Evaluation Plan",
				nil,
			)
	}

	if _, active :=
		s.visiting[target]; active {

		return Result{},
			newError(
				ErrorEvaluationCycle,
				"runtime evaluation encountered a dependency cycle in a committed model",
				nil,
			)
	}

	definition, err :=
		s.definitionForTarget(
			target,
		)

	if err != nil {
		return Result{}, err
	}

	if definition.TemporalMode() !=
		schema.TemporalScalar {

		return Result{},
			newError(
				ErrorUnsupportedSeries,
				"Series property evaluation is not implemented yet",
				nil,
			)
	}

	s.visiting[target] =
		struct{}{}

	defer delete(
		s.visiting,
		target,
	)

	switch planNode.Kind() {
	case plan.NodeCalculationOverride,
		plan.NodeScenarioOverride:

		outcome, exists :=
			planNode.Outcome()

		if !exists ||
			!outcome.IsValid() {

			return Result{},
				newError(
					ErrorInvariantViolation,
					"override Plan node contains no valid resolver Outcome",
					nil,
				)
		}

		propertyKind,
			kindErr :=
			propertyProvenanceKind(
				definition,
			)

		if kindErr != nil {
			return Result{},
				kindErr
		}

		result,
			resultErr :=
			resultFromOutcome(
				target,
				propertyKind,
				outcome,
			)

		if resultErr != nil {
			return Result{},
				resultErr
		}

		s.remember(target, result)

		return result, nil

	case plan.NodeInput:
		if definition.
			Computation().
			Kind() !=
			schema.ComputationInput {

			return Result{},
				newError(
					ErrorInvariantViolation,
					"Effective Evaluation Plan marks a non-Input property as Input",
					nil,
				)
		}

		result, resolveErr :=
			s.resolveStoredValue(
				target,
				provenance.KindInputProperty,
			)

		if resolveErr != nil {
			return Result{},
				resolveErr
		}

		s.remember(target, result)

		return result, nil

	case plan.NodeDerivedExpression:
		if definition.
			Computation().
			Kind() !=
			schema.ComputationDerived {

			return Result{},
				newError(
					ErrorInvariantViolation,
					"Effective Evaluation Plan marks a non-Derived property as DerivedExpression",
					nil,
				)
		}

		expression, exists :=
			definition.
				Computation().
				Expression()

		if !exists ||
			expression == nil {

			return Result{},
				newError(
					ErrorInvariantViolation,
					"committed Derived property has no expression",
					nil,
				)
		}

		expressionResult,
			evaluationErr :=
			s.evaluateExpression(
				expression,
				expressionContext{
					self: target.Object(),
				},
			)

		if evaluationErr != nil {
			return Result{},
				evaluationErr
		}

		normalized,
			normalizationErr :=
			s.normalizeDerivedResult(
				expressionResult.value,
				definition,
			)

		if normalizationErr != nil {
			return Result{},
				normalizationErr
		}

		result :=
			newResult(
				normalized,
				provenance.NewProperty(
					provenance.KindDerivedProperty,
					target,
					expressionResult.provenance,
				),
			)

		s.remember(target, result)

		return result, nil

	default:
		return Result{},
			newError(
				ErrorInvariantViolation,
				"Effective Evaluation Plan contains an invalid node kind",
				nil,
			)
	}
}

func (s *evaluationState) remember(target address.PropertyAddress, result Result) {
	if !target.IsValid() || !result.IsValid() {
		panic("evaluator: attempted to cache invalid property result")
	}

	s.memo[target] = result
	if s.cache != nil {
		s.cache.Store(target, result)
	}
}

func propertyProvenanceKind(
	definition schema.PropertyDefinition,
) (provenance.Kind, error) {
	switch definition.
		Computation().
		Kind() {

	case schema.ComputationInput:
		return provenance.KindInputProperty,
			nil

	case schema.ComputationDerived:
		return provenance.KindDerivedProperty,
			nil

	default:
		return provenance.KindInvalid,
			newError(
				ErrorInvariantViolation,
				"committed property has invalid computation kind",
				nil,
			)
	}
}

func (s *evaluationState) resolveStoredValue(
	target address.PropertyAddress,
	propertyKind provenance.Kind,
) (Result, error) {
	resolution, err :=
		resolvervalue.ResolveValidated(
			s.version,
			target,
			s.context,
		)

	if err != nil {
		return Result{},
			newError(
				ErrorResolverFailure,
				"value resolver failed",
				err,
			)
	}

	outcome, ok :=
		resolution.At()

	if !ok ||
		!outcome.IsValid() {

		return Result{},
			newError(
				ErrorInvariantViolation,
				"At evaluation received a non-At resolver result",
				nil,
			)
	}

	// NodeInput means the Effective Plan already proved that no Calculation or
	// Scenario override wins for this property. Seeing one now would mean the
	// Plan and Resolver disagree for the same immutable context.
	if source, hasSource :=
		outcome.Source(); hasSource {

		if source.Kind() ==
			resolvervalue.SourceCalculationOverride ||
			source.Kind() ==
				resolvervalue.SourceScenarioOverride {

			return Result{},
				newError(
					ErrorInvariantViolation,
					"Input resolver disagrees with Effective Evaluation Plan override decision",
					nil,
				)
		}
	}

	return resultFromOutcome(
		target,
		propertyKind,
		outcome,
	)
}

func resultFromOutcome(
	target address.PropertyAddress,
	propertyKind provenance.Kind,
	outcome resolvervalue.Outcome,
) (Result, error) {
	sourceNode, err :=
		provenanceFromOutcome(
			outcome,
		)

	if err != nil {
		return Result{}, err
	}

	resolved := outcome.Value()
	return newResult(
		resolved,
		provenance.NewProperty(
			propertyKind,
			target,
			sourceNode.WithResolved(resolved),
		),
	), nil
}

func provenanceFromOutcome(
	outcome resolvervalue.Outcome,
) (provenance.Node, error) {
	if source, ok :=
		outcome.Source(); ok {
		return provenanceFromSource(source)
	}

	resolved :=
		outcome.Value()

	if !resolved.IsUndefined() {
		return provenance.Node{},
			newError(
				ErrorInvariantViolation,
				"resolver outcome has neither source nor Undefined value",
				nil,
			)
	}

	switch resolved.
		UndefinedInfo().
		Kind() {

	case enginevalue.UndefinedMissingValue:
		return provenance.NewMissingValue(),
			nil

	case enginevalue.UndefinedAmbiguousValue:
		return provenance.NewAmbiguousValue(),
			nil

	default:
		return provenance.Node{},
			newError(
				ErrorInvariantViolation,
				"resolver returned unsupported source-less Undefined value",
				nil,
			)
	}
}

func provenanceFromSource(source resolvervalue.Source) (provenance.Node, error) {
	if !source.IsValid() {
		return provenance.Node{}, newError(ErrorInvariantViolation, "resolver returned an invalid source", nil)
	}

	switch source.Kind() {
	case resolvervalue.SourceCalculationOverride:
		override, exists :=
			source.Override()

		if !exists {
			return provenance.Node{},
				newError(
					ErrorInvariantViolation,
					"resolver calculation override source has no override",
					nil,
				)
		}

		return provenance.NewCalculationOverride(
			override,
		), nil

	case resolvervalue.SourceScenarioOverride:
		scenario,
			override,
			exists :=
			source.ScenarioOverride()

		if !exists {
			return provenance.Node{},
				newError(
					ErrorInvariantViolation,
					"resolver Scenario override source has no Scenario override",
					nil,
				)
		}

		return provenance.NewScenarioOverride(
			scenario,
			override,
		), nil

	case resolvervalue.SourceBaseObjectBinding:
		binding, exists :=
			source.Binding()

		if !exists {
			return provenance.Node{},
				newError(
					ErrorInvariantViolation,
					"resolver object binding source has no binding",
					nil,
				)
		}

		return provenance.NewBaseBinding(
			provenance.KindBaseObjectBinding,
			binding,
		), nil

	case resolvervalue.SourceBaseTypeBinding:
		binding, exists :=
			source.Binding()

		if !exists {
			return provenance.Node{},
				newError(
					ErrorInvariantViolation,
					"resolver type binding source has no binding",
					nil,
				)
		}

		return provenance.NewBaseBinding(
			provenance.KindBaseTypeBinding,
			binding,
		), nil

	default:
		return provenance.Node{},
			newError(
				ErrorInvariantViolation,
				"resolver returned an unknown source kind",
				nil,
			)
	}
}

func (s *evaluationState) definitionForTarget(
	target address.PropertyAddress,
) (schema.PropertyDefinition, error) {
	if !target.IsValid() {
		return schema.PropertyDefinition{},
			newError(
				ErrorInvalidPropertyAddress,
				"target property address is invalid",
				nil,
			)
	}

	definition, ok :=
		s.version.PropertyDefinition(
			target.DefinitionID(),
		)

	if !ok ||
		!definition.IsValid() {

		return schema.PropertyDefinition{},
			newError(
				ErrorDefinitionNotFound,
				"property definition could not be resolved",
				nil,
			)
	}

	var owner schema.PropertyOwner

	switch target.
		Object().
		Kind() {

	case address.ObjectRefEntity:
		entity, exists :=
			s.version.Entity(
				target.
					Object().
					ID(),
			)

		if !exists ||
			!entity.IsValid() {

			return schema.PropertyDefinition{},
				newError(
					ErrorObjectNotFound,
					"target entity could not be resolved",
					nil,
				)
		}

		entityType, exists :=
			s.version.EntityType(
				entity.TypeID(),
			)

		if !exists ||
			entityType.ID() != entity.TypeID() {

			return schema.PropertyDefinition{},
				newError(
					ErrorInvariantViolation,
					"committed entity type could not be resolved",
					nil,
				)
		}

		owner =
			schema.EntityTypeOwner(
				entityType.ID(),
			)

	case address.ObjectRefRelation:
		relation, exists :=
			s.version.Relation(
				target.
					Object().
					ID(),
			)

		if !exists ||
			!relation.IsValid() {

			return schema.PropertyDefinition{},
				newError(
					ErrorObjectNotFound,
					"target relation could not be resolved",
					nil,
				)
		}

		relationType, exists :=
			s.version.RelationType(
				relation.TypeID(),
			)

		if !exists ||
			relationType.ID() != relation.TypeID() {

			return schema.PropertyDefinition{},
				newError(
					ErrorInvariantViolation,
					"committed relation type could not be resolved",
					nil,
				)
		}

		owner =
			schema.RelationTypeOwner(
				relationType.ID(),
			)

	default:
		return schema.PropertyDefinition{},
			newError(
				ErrorInvalidPropertyAddress,
				"target object reference kind is invalid",
				nil,
			)
	}

	if definition.Owner() !=
		owner {

		return schema.PropertyDefinition{},
			newError(
				ErrorDefinitionNotAvailable,
				"property definition is not available on target object type",
				nil,
			)
	}

	return definition, nil
}
