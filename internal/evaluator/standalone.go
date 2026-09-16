package evaluator

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/internal/plan"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// EvaluateExpression evaluates one standalone committed-model expression in a
// CalculationContext.
//
// Standalone means there is no implicit Entity/Relation self object. Absolute
// property references and Aggregates with an explicit Entity traversal origin
// are supported. SelfPropertyReference and self-origin Aggregate traversal are
// rejected by static inference/dependency extraction.
//
// Constraint applicability/assertion evaluation uses this API.
func EvaluateExpression(
	version Source,
	expression ast.Expression,
	context runtime.CalculationContext,
) (Result, error) {
	staticType, dependencies, err := prepareStandaloneExpression(version, expression, context)
	if err != nil {
		return Result{}, err
	}

	var effectivePlan plan.Plan

	if len(dependencies) > 0 {
		if graph, ok := dependencyGraph(version); ok {
			effectivePlan, err = plan.BuildWithDependencyGraph(version, graph, dependencies, context)
		} else {
			effectivePlan, err = plan.Build(version, dependencies, context)
		}
		if err != nil {
			return Result{},
				newError(
					ErrorPlanBuild,
					"failed to build Effective Evaluation Plan for standalone expression",
					err,
				)
		}
	}

	return evaluatePreparedStandaloneExpression(
		version,
		expression,
		context,
		staticType,
		dependencies,
		effectivePlan,
		nil,
	)
}

// evaluateStandaloneWithEnvironment is the Session path for standalone
// expressions. It retains all ordinary validation and inference but reuses a
// complete effective plan and concrete-property cache owned by the runtime
// Session. It never creates an implicit self binding.
func evaluateStandaloneWithEnvironment(
	version Source,
	expression ast.Expression,
	context runtime.CalculationContext,
	effectivePlan plan.Plan,
	cache propertyResultCache,
) (Result, error) {
	staticType, dependencies, err := prepareStandaloneExpression(version, expression, context)
	if err != nil {
		return Result{}, err
	}

	return evaluatePreparedStandaloneExpression(
		version,
		expression,
		context,
		staticType,
		dependencies,
		effectivePlan,
		cache,
	)
}

func prepareStandaloneExpression(
	version Source,
	expression ast.Expression,
	context runtime.CalculationContext,
) (typing.Type, []address.PropertyAddress, error) {
	if !version.IsCommitted() {
		return typing.Type{}, nil,
			newError(
				ErrorInvalidModelVersion,
				"standalone expression evaluation requires a committed ModelVersion",
				nil,
			)
	}

	if expression == nil {
		return typing.Type{}, nil,
			newError(
				ErrorUnsupportedExpression,
				"standalone expression must not be nil",
				nil,
			)
	}

	if !context.IsValid() {
		return typing.Type{}, nil,
			newError(
				ErrorInvalidCalculationContext,
				"calculation context is invalid",
				nil,
			)
	}

	if context.TimeContext().Kind() != domaintime.ContextAt {
		return typing.Type{}, nil,
			newError(
				ErrorUnsupportedTimeContext,
				"standalone scalar expression evaluation currently requires an At time context",
				nil,
			)
	}

	if err := resolvervalue.ValidateContext(version, context); err != nil {
		return typing.Type{}, nil,
			newError(
				ErrorResolverFailure,
				"calculation context validation failed",
				err,
			)
	}

	staticType, err := inference.Infer(expression, inference.EmptyContext(), version)
	if err != nil || !staticType.IsValid() {
		return typing.Type{}, nil,
			newError(
				ErrorUnsupportedExpression,
				"standalone expression cannot be statically inferred",
				err,
			)
	}

	dependencies, err := dependency.ExtractStandaloneExpression(version, expression)
	if err != nil {
		return typing.Type{}, nil,
			newError(
				ErrorPlanBuild,
				"failed to extract standalone expression dependencies",
				err,
			)
	}

	return staticType, dependencies, nil
}

func evaluatePreparedStandaloneExpression(
	version Source,
	expression ast.Expression,
	context runtime.CalculationContext,
	staticType typing.Type,
	dependencies []address.PropertyAddress,
	effectivePlan plan.Plan,
	cache propertyResultCache,
) (Result, error) {
	if len(dependencies) > 0 {
		if !effectivePlan.IsValid() {
			return Result{}, newError(
				ErrorInvalidEvaluationPlan,
				"standalone expression dependencies require an Effective Evaluation Plan",
				nil,
			)
		}
		for _, dependencyAddress := range dependencies {
			if _, exists := effectivePlan.Node(dependencyAddress); !exists {
				return Result{}, newError(
					ErrorInvariantViolation,
					"Session Effective Evaluation Plan omits a standalone expression dependency",
					nil,
				)
			}
		}
	}

	state := evaluationState{
		version:       version,
		context:       context,
		effectivePlan: effectivePlan,
		memo:          make(map[address.PropertyAddress]Result),
		cache:         cache,
		visiting:      make(map[address.PropertyAddress]struct{}),
	}

	result, err := state.evaluateExpression(
		expression,
		expressionContext{
			static:    inference.EmptyContext(),
			hasStatic: true,
		},
	)
	if err != nil {
		return Result{}, err
	}

	normalized, err := state.normalizeResolvedToStaticType(result.value, staticType)
	if err != nil {
		return Result{}, err
	}

	return newResult(normalized, result.provenance), nil
}
