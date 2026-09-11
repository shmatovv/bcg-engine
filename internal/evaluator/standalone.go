package evaluator

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/plan"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// EvaluateExpression evaluates one standalone committed-model expression in a
// CalculationContext.
//
// Standalone means there is no implicit Entity/Relation self object. Absolute
// property references are supported. SelfPropertyReference and Aggregate
// traversal that requires self are rejected by static inference/dependency
// extraction.
//
// Constraint applicability/assertion evaluation uses this API.
func EvaluateExpression(
	version model.ModelVersion,
	expression ast.Expression,
	context runtime.CalculationContext,
) (Result, error) {
	if !version.IsCommitted() {
		return Result{},
			newError(
				ErrorInvalidModelVersion,
				"standalone expression evaluation requires a committed ModelVersion",
				nil,
			)
	}

	if expression == nil {
		return Result{},
			newError(
				ErrorUnsupportedExpression,
				"standalone expression must not be nil",
				nil,
			)
	}

	if !context.IsValid() {
		return Result{},
			newError(
				ErrorInvalidCalculationContext,
				"calculation context is invalid",
				nil,
			)
	}

	if context.TimeContext().Kind() != domaintime.ContextAt {
		return Result{},
			newError(
				ErrorUnsupportedTimeContext,
				"standalone scalar expression evaluation currently requires an At time context",
				nil,
			)
	}

	if err := resolvervalue.ValidateContext(
		version,
		context,
	); err != nil {
		return Result{},
			newError(
				ErrorResolverFailure,
				"calculation context validation failed",
				err,
			)
	}

	staticType, err := inference.Infer(
		expression,
		inference.EmptyContext(),
		version,
	)
	if err != nil || !staticType.IsValid() {
		return Result{},
			newError(
				ErrorUnsupportedExpression,
				"standalone expression cannot be statically inferred",
				err,
			)
	}

	dependencies, err := dependency.ExtractStandaloneExpression(
		version,
		expression,
	)
	if err != nil {
		return Result{},
			newError(
				ErrorPlanBuild,
				"failed to extract standalone expression dependencies",
				err,
			)
	}

	var effectivePlan plan.Plan

	if len(dependencies) > 0 {
		effectivePlan, err = plan.Build(
			version,
			dependencies,
			context,
		)
		if err != nil {
			return Result{},
				newError(
					ErrorPlanBuild,
					"failed to build Effective Evaluation Plan for standalone expression",
					err,
				)
		}
	}

	state := evaluationState{
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

	normalized, err := state.normalizeResolvedToStaticType(
		result.value,
		staticType,
	)
	if err != nil {
		return Result{}, err
	}

	return newResult(
		normalized,
		result.provenance,
	), nil
}
