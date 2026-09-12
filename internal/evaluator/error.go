package evaluator

import "fmt"

// ErrorKind identifies a failure of the evaluation operation itself.
//
// Business-level absence and invalid calculations such as division by zero
// are represented by value.Resolved Undefined and are not evaluator errors.
type ErrorKind uint8

const (
	ErrorInvalid ErrorKind = iota

	ErrorInvalidModelVersion
	ErrorInvalidCalculationContext
	ErrorUnsupportedScenario
	ErrorUnsupportedTimeContext
	ErrorInvalidPropertyAddress
	ErrorObjectNotFound
	ErrorDefinitionNotFound
	ErrorDefinitionNotAvailable
	ErrorUnsupportedSeries
	ErrorIncompatibleSeriesAxis
	ErrorUnsupportedExpression
	ErrorResolverFailure
	ErrorPlanBuild
	ErrorInvalidEvaluationPlan
	ErrorEvaluationCycle
	ErrorInvariantViolation
)

func (k ErrorKind) IsValid() bool {
	switch k {
	case ErrorInvalidModelVersion,
		ErrorInvalidCalculationContext,
		ErrorUnsupportedScenario,
		ErrorUnsupportedTimeContext,
		ErrorInvalidPropertyAddress,
		ErrorObjectNotFound,
		ErrorDefinitionNotFound,
		ErrorDefinitionNotAvailable,
		ErrorUnsupportedSeries,
		ErrorIncompatibleSeriesAxis,
		ErrorUnsupportedExpression,
		ErrorResolverFailure,
		ErrorPlanBuild,
		ErrorInvalidEvaluationPlan,
		ErrorEvaluationCycle,
		ErrorInvariantViolation:

		return true

	default:
		return false
	}
}

func (k ErrorKind) String() string {
	switch k {
	case ErrorInvalidModelVersion:
		return "InvalidModelVersion"

	case ErrorInvalidCalculationContext:
		return "InvalidCalculationContext"

	case ErrorUnsupportedScenario:
		return "UnsupportedScenario"

	case ErrorUnsupportedTimeContext:
		return "UnsupportedTimeContext"

	case ErrorInvalidPropertyAddress:
		return "InvalidPropertyAddress"

	case ErrorObjectNotFound:
		return "ObjectNotFound"

	case ErrorDefinitionNotFound:
		return "DefinitionNotFound"

	case ErrorDefinitionNotAvailable:
		return "DefinitionNotAvailable"

	case ErrorUnsupportedSeries:
		return "UnsupportedSeries"
	case ErrorIncompatibleSeriesAxis:
		return "IncompatibleSeriesAxis"

	case ErrorUnsupportedExpression:
		return "UnsupportedExpression"

	case ErrorResolverFailure:
		return "ResolverFailure"

	case ErrorPlanBuild:
		return "PlanBuild"

	case ErrorInvalidEvaluationPlan:
		return "InvalidEvaluationPlan"

	case ErrorEvaluationCycle:
		return "EvaluationCycle"

	case ErrorInvariantViolation:
		return "InvariantViolation"

	default:
		return "Invalid"
	}
}

type Error struct {
	kind ErrorKind

	message string
	cause   error
}

func newError(
	kind ErrorKind,
	message string,
	cause error,
) Error {
	if !kind.IsValid() {
		panic(
			"evaluator: invalid error kind",
		)
	}

	if message == "" {
		panic(
			"evaluator: error message must not be empty",
		)
	}

	return Error{
		kind:    kind,
		message: message,
		cause:   cause,
	}
}

func (e Error) Kind() ErrorKind {
	return e.kind
}

func (e Error) Unwrap() error {
	return e.cause
}

func (e Error) Error() string {
	if e.cause == nil {
		return fmt.Sprintf(
			"evaluator: %s: %s",
			e.kind.String(),
			e.message,
		)
	}

	return fmt.Sprintf(
		"evaluator: %s: %s: %v",
		e.kind.String(),
		e.message,
		e.cause,
	)
}

var _ error = Error{}
