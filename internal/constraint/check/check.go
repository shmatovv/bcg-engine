package check

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
	"github.com/shmatovv/bcg-engine/internal/model"
	resolvervalue "github.com/shmatovv/bcg-engine/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	enginevalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

func One(
	version model.ModelVersion,
	constraintID identity.UUID,
	context runtime.CalculationContext,
) (Result, error) {
	if !version.IsCommitted() {
		return Result{},
			newError(
				ErrorInvalidModelVersion,
				"Constraint checking requires a committed ModelVersion",
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

	modelConstraint, ok := version.Constraint(constraintID)
	if !ok || !modelConstraint.IsValid() {
		return Result{},
			newError(
				ErrorConstraintNotFound,
				"Constraint could not be resolved from committed ModelVersion",
				nil,
			)
	}

	return evaluateConstraint(
		version,
		modelConstraint,
		context,
	)
}

func All(
	version model.ModelVersion,
	context runtime.CalculationContext,
) (Report, error) {
	if !version.IsCommitted() {
		return Report{},
			newError(
				ErrorInvalidModelVersion,
				"Constraint checking requires a committed ModelVersion",
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

	constraints := version.Constraints()

	sort.Slice(
		constraints,
		func(i, j int) bool {
			return compareUUID(
				constraints[i].ID(),
				constraints[j].ID(),
			) < 0
		},
	)

	results := make(
		[]Result,
		0,
		len(constraints),
	)

	for _, modelConstraint := range constraints {
		result, err := evaluateConstraint(
			version,
			modelConstraint,
			context,
		)
		if err != nil {
			return Report{}, err
		}

		results = append(results, result)
	}

	return newReport(results), nil
}

func evaluateConstraint(
	version model.ModelVersion,
	modelConstraint constraint.Constraint,
	context runtime.CalculationContext,
) (Result, error) {
	var applicabilityResult evaluator.Result
	hasApplicabilityResult := false

	if applicability, hasApplicability := modelConstraint.Applicability(); hasApplicability {
		result, err := evaluator.EvaluateExpression(
			version,
			applicability,
			context,
		)
		if err != nil {
			return Result{},
				newError(
					ErrorEvaluationFailure,
					"Constraint applicability evaluation failed",
					err,
				)
		}

		applicabilityResult = result
		hasApplicabilityResult = true

		applicable, undefined, err := boolState(result.Value())
		if err != nil {
			return Result{}, err
		}

		if undefined {
			return newResult(
				modelConstraint,
				StateUndefined,
				applicabilityResult,
				true,
				evaluator.Result{},
				false,
			), nil
		}

		if !applicable {
			return newResult(
				modelConstraint,
				StateNotApplicable,
				applicabilityResult,
				true,
				evaluator.Result{},
				false,
			), nil
		}
	}

	assertionResult, err := evaluator.EvaluateExpression(
		version,
		modelConstraint.Assertion(),
		context,
	)
	if err != nil {
		return Result{},
			newError(
				ErrorEvaluationFailure,
				"Constraint assertion evaluation failed",
				err,
			)
	}

	satisfied, undefined, err := boolState(assertionResult.Value())
	if err != nil {
		return Result{}, err
	}

	if undefined {
		return newResult(
			modelConstraint,
			StateUndefined,
			applicabilityResult,
			hasApplicabilityResult,
			assertionResult,
			true,
		), nil
	}

	state := StateViolated
	if satisfied {
		state = StateSatisfied
	}

	return newResult(
		modelConstraint,
		state,
		applicabilityResult,
		hasApplicabilityResult,
		assertionResult,
		true,
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
				"Constraint expression produced an invalid resolved value",
				nil,
			)
	}

	if resolved.IsNull() || resolved.IsUndefined() {
		return false, true, nil
	}

	typed := resolved.Value()

	if typed.Type() != enginevalue.Bool || typed.HasUnit() {
		return false,
			false,
			newError(
				ErrorInvariantViolation,
				"committed Constraint expression did not evaluate to unitless Bool",
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
