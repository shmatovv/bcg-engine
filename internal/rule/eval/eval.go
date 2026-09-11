package eval

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/model"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	enginevalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

// One evaluates one committed Rule.
func One(
	version model.ModelVersion,
	ruleID identity.UUID,
	context runtime.CalculationContext,
) (Result, error) {
	if !version.IsCommitted() {
		return Result{},
			newError(
				ErrorInvalidModelVersion,
				"Rule evaluation requires a committed ModelVersion",
				nil,
			)
	}

	if !context.IsValid() {
		return Result{},
			newError(
				ErrorInvalidCalculationContext,
				"CalculationContext is invalid",
				nil,
			)
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

// All evaluates every committed Rule in deterministic UUID order.
func All(
	version model.ModelVersion,
	context runtime.CalculationContext,
) (Report, error) {
	if !version.IsCommitted() {
		return Report{},
			newError(
				ErrorInvalidModelVersion,
				"Rule evaluation requires a committed ModelVersion",
				nil,
			)
	}

	if !context.IsValid() {
		return Report{},
			newError(
				ErrorInvalidCalculationContext,
				"CalculationContext is invalid",
				nil,
			)
	}

	if context.TimeContext().Kind() != domaintime.ContextAt {
		return Report{}, newError(ErrorInvalidCalculationContext, "scalar evaluation requires an At time context", nil)
	}
	if err := resolvervalue.ValidateContext(version, context); err != nil {
		return Report{}, newError(ErrorInvalidCalculationContext, "context is not valid for ModelVersion", err)
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

func evaluateRule(
	version model.ModelVersion,
	modelRule domainrule.Rule,
	context runtime.CalculationContext,
) (Result, error) {
	condition, err := evaluator.EvaluateExpression(
		version,
		modelRule.Condition(),
		context,
	)
	if err != nil {
		return Result{},
			newError(
				ErrorEvaluationFailure,
				"Rule condition evaluation failed",
				err,
			)
	}

	active, undefined, err := boolState(
		condition.Value(),
	)
	if err != nil {
		return Result{}, err
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

func boolState(
	resolved enginevalue.Resolved,
) (bool, bool, error) {
	if !resolved.IsValid() {
		return false,
			false,
			newError(
				ErrorInvariantViolation,
				"Rule condition produced an invalid resolved value",
				nil,
			)
	}

	if resolved.IsNull() ||
		resolved.IsUndefined() {
		return false, true, nil
	}

	typed := resolved.Value()

	if typed.Type() != enginevalue.Bool ||
		typed.HasUnit() {
		return false,
			false,
			newError(
				ErrorInvariantViolation,
				"committed Rule condition did not evaluate to unitless Bool",
				nil,
			)
	}

	return typed.Raw().Bool(), false, nil
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
