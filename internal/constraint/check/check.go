package check

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	resolvervalue "github.com/shmatovv/bcg-engine/v2/internal/resolver/value"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// One evaluates one committed Constraint using the normal scalar evaluator.
// Constraints accept only At contexts: Period × Constraint would require an
// explicit temporal reduction that Core v1 intentionally does not define.
func One(
	version Source,
	constraintID identity.UUID,
	context runtime.CalculationContext,
) (Result, error) {
	if err := validateRequest(version, context); err != nil {
		return Result{}, err
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

// OneWithSession evaluates one Constraint through a shared evaluator Session.
// It retains the ordinary Constraint decision semantics while allowing its
// concrete property reads to be reused with Rules and other Constraints.
func OneWithSession(
	session *evaluator.Session,
	constraintID identity.UUID,
	context runtime.CalculationContext,
) (Result, error) {
	if session == nil {
		return Result{}, newError(ErrorInvalidModelVersion, "evaluation Session is nil", nil)
	}

	version, ok := session.Source().(Source)
	if !ok {
		return Result{}, newError(ErrorInvalidModelVersion, "evaluation Session source cannot evaluate Constraints", nil)
	}
	if err := validateRequest(version, context); err != nil {
		return Result{}, err
	}

	modelConstraint, ok := version.Constraint(constraintID)
	if !ok || !modelConstraint.IsValid() {
		return Result{}, newError(ErrorConstraintNotFound, "Constraint could not be resolved from committed ModelVersion", nil)
	}

	return evaluateConstraintWithExpressionEvaluator(
		version,
		modelConstraint,
		context,
		session.EvaluateExpression,
	)
}

// All evaluates every committed Constraint in deterministic UUID order. All
// Constraints are evaluated even after a Hard violation so a Feasibility
// report can retain the complete set of outcomes.
func All(
	version Source,
	context runtime.CalculationContext,
) (Report, error) {
	if err := validateRequest(version, context); err != nil {
		return Report{}, err
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

// AllWithSession evaluates every Constraint in deterministic UUID order using
// one shared evaluator Session.
func AllWithSession(
	session *evaluator.Session,
	context runtime.CalculationContext,
) (Report, error) {
	if session == nil {
		return Report{}, newError(ErrorInvalidModelVersion, "evaluation Session is nil", nil)
	}

	version, ok := session.Source().(Source)
	if !ok {
		return Report{}, newError(ErrorInvalidModelVersion, "evaluation Session source cannot evaluate Constraints", nil)
	}
	if err := validateRequest(version, context); err != nil {
		return Report{}, err
	}

	constraints := version.Constraints()
	sort.Slice(constraints, func(i, j int) bool {
		return compareUUID(constraints[i].ID(), constraints[j].ID()) < 0
	})

	results := make([]Result, 0, len(constraints))
	for _, modelConstraint := range constraints {
		result, err := evaluateConstraintWithExpressionEvaluator(
			version,
			modelConstraint,
			context,
			session.EvaluateExpression,
		)
		if err != nil {
			return Report{}, err
		}
		results = append(results, result)
	}

	return newReport(results), nil
}

func validateRequest(
	version Source,
	context runtime.CalculationContext,
) error {
	if !version.IsCommitted() {
		return newError(
			ErrorInvalidModelVersion,
			"Constraint checking requires a committed ModelVersion",
			nil,
		)
	}

	if !context.IsValid() {
		return newError(
			ErrorInvalidCalculationContext,
			"CalculationContext is invalid",
			nil,
		)
	}

	if context.TimeContext().Kind() != domaintime.ContextAt {
		return newError(
			ErrorUnsupportedTimeContext,
			"Constraint evaluation requires an At time context",
			nil,
		)
	}

	if err := resolvervalue.ValidateContext(version, context); err != nil {
		return newError(
			ErrorInvalidCalculationContext,
			"context is not valid for ModelVersion",
			err,
		)
	}

	return nil
}

func evaluateConstraint(
	version Source,
	modelConstraint constraint.Constraint,
	context runtime.CalculationContext,
) (Result, error) {
	return evaluateConstraintWithExpressionEvaluator(
		version,
		modelConstraint,
		context,
		func(expression ast.Expression, context runtime.CalculationContext) (evaluator.Result, error) {
			return evaluator.EvaluateExpression(version, expression, context)
		},
	)
}

func evaluateConstraintWithExpressionEvaluator(
	version Source,
	modelConstraint constraint.Constraint,
	context runtime.CalculationContext,
	evaluate func(ast.Expression, runtime.CalculationContext) (evaluator.Result, error),
) (Result, error) {
	var applicabilityResult evaluator.Result
	hasApplicabilityResult := false

	if applicability, hasApplicability := modelConstraint.Applicability(); hasApplicability {
		result, err := evaluate(
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

	assertionResult, err := evaluate(
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

// boolState preserves the evaluator's Null/Undefined distinction until the
// constraint decision boundary. Neither state is Boolean false; both make a
// Constraint indeterminate and preserve its evaluator Result/provenance.
func boolState(
	resolved enginevalue.Resolved,
) (bool, bool, error) {
	decision, indeterminate, err := evaluator.InterpretScalarBool(resolved)
	if err != nil {
		return false, false, newError(
			ErrorInvariantViolation,
			"committed Constraint expression did not evaluate to unitless Bool",
			err,
		)
	}
	return decision, indeterminate, nil
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
