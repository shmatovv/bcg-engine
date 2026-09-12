package eval

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/model"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// One evaluates one committed Rule.
func One(
	version model.ModelVersion,
	ruleID identity.UUID,
	context runtime.CalculationContext,
) (Result, error) {
	if err := validateRequest(version, context); err != nil {
		return Result{}, err
	}

	modelRule, ok := version.Rule(ruleID)
	if !ok || !modelRule.IsValid() {
		return Result{},
			newError(
				ErrorRuleNotFound,
				"Rule could not be resolved from committed ModelVersion",
				nil,
			)
	}

	return evaluateRule(
		version,
		modelRule,
		context,
	)
}

// OneWithSession evaluates one Rule through a shared evaluator Session. Rule
// states remain a pure interpretation of the ordinary evaluator Result.
func OneWithSession(
	session *evaluator.Session,
	ruleID identity.UUID,
	context runtime.CalculationContext,
) (Result, error) {
	if session == nil {
		return Result{}, newError(ErrorInvalidModelVersion, "evaluation Session is nil", nil)
	}

	version := session.ModelVersion()
	if err := validateRequest(version, context); err != nil {
		return Result{}, err
	}

	modelRule, ok := version.Rule(ruleID)
	if !ok || !modelRule.IsValid() {
		return Result{}, newError(ErrorRuleNotFound, "Rule could not be resolved from committed ModelVersion", nil)
	}

	return evaluateRuleWithExpressionEvaluator(version, modelRule, context, session.EvaluateExpression)
}

// All evaluates every committed Rule in deterministic UUID order.
func All(
	version model.ModelVersion,
	context runtime.CalculationContext,
) (Report, error) {
	if err := validateRequest(version, context); err != nil {
		return Report{}, err
	}

	rules := version.Rules()

	sort.Slice(
		rules,
		func(i, j int) bool {
			return compareUUID(
				rules[i].ID(),
				rules[j].ID(),
			) < 0
		},
	)

	results := make(
		[]Result,
		0,
		len(rules),
	)

	for _, modelRule := range rules {
		result, err := evaluateRule(
			version,
			modelRule,
			context,
		)
		if err != nil {
			return Report{}, err
		}

		results = append(
			results,
			result,
		)
	}

	return newReport(results), nil
}

// AllWithSession evaluates every Rule in deterministic UUID order using one
// shared evaluator Session.
func AllWithSession(
	session *evaluator.Session,
	context runtime.CalculationContext,
) (Report, error) {
	if session == nil {
		return Report{}, newError(ErrorInvalidModelVersion, "evaluation Session is nil", nil)
	}

	version := session.ModelVersion()
	if err := validateRequest(version, context); err != nil {
		return Report{}, err
	}

	rules := version.Rules()
	sort.Slice(rules, func(i, j int) bool {
		return compareUUID(rules[i].ID(), rules[j].ID()) < 0
	})

	results := make([]Result, 0, len(rules))
	for _, modelRule := range rules {
		result, err := evaluateRuleWithExpressionEvaluator(version, modelRule, context, session.EvaluateExpression)
		if err != nil {
			return Report{}, err
		}
		results = append(results, result)
	}

	return newReport(results), nil
}

// validateRequest deliberately matches the Constraint Core's At-only scalar
// decision contract. A Period would require an explicit temporal reduction,
// which Rule Core v1 does not define.
func validateRequest(
	version model.ModelVersion,
	context runtime.CalculationContext,
) error {
	if !version.IsCommitted() {
		return newError(ErrorInvalidModelVersion, "Rule evaluation requires a committed ModelVersion", nil)
	}
	if !context.IsValid() {
		return newError(ErrorInvalidCalculationContext, "CalculationContext is invalid", nil)
	}
	if context.TimeContext().Kind() != domaintime.ContextAt {
		return newError(ErrorUnsupportedTimeContext, "Rule evaluation requires an At time context", nil)
	}
	if err := resolvervalue.ValidateContext(version, context); err != nil {
		return newError(ErrorInvalidCalculationContext, "context is not valid for ModelVersion", err)
	}
	return nil
}

func evaluateRule(
	version model.ModelVersion,
	modelRule domainrule.Rule,
	context runtime.CalculationContext,
) (Result, error) {
	return evaluateRuleWithExpressionEvaluator(
		version,
		modelRule,
		context,
		func(expression ast.Expression, context runtime.CalculationContext) (evaluator.Result, error) {
			return evaluator.EvaluateExpression(version, expression, context)
		},
	)
}

func evaluateRuleWithExpressionEvaluator(
	version model.ModelVersion,
	modelRule domainrule.Rule,
	context runtime.CalculationContext,
	evaluate func(ast.Expression, runtime.CalculationContext) (evaluator.Result, error),
) (Result, error) {
	condition, err := evaluate(modelRule.Condition(), context)
	if err != nil {
		return Result{},
			newError(
				ErrorEvaluationFailure,
				"Rule condition evaluation failed",
				err,
			)
	}

	active, undefined, err := evaluator.InterpretScalarBool(condition.Value())
	if err != nil {
		return Result{}, newError(ErrorInvariantViolation, "committed Rule condition did not evaluate to unitless Bool", err)
	}

	state := StateInactive

	if undefined {
		state = StateUndefined
	} else if active {
		state = StateActive
	}

	return newResult(
		modelRule,
		state,
		condition,
	), nil
}

func compareUUID(
	left identity.UUID,
	right identity.UUID,
) int {
	leftBytes := left.Bytes()
	rightBytes := right.Bytes()

	for index := range leftBytes {
		switch {
		case leftBytes[index] < rightBytes[index]:
			return -1
		case leftBytes[index] > rightBytes[index]:
			return 1
		}
	}

	return 0
}
