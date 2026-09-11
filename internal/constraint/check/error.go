package check

import "fmt"

type ErrorKind uint8

const (
	ErrorInvalid ErrorKind = iota
	ErrorInvalidModelVersion
	ErrorInvalidCalculationContext
	ErrorConstraintNotFound
	ErrorEvaluationFailure
	ErrorInvariantViolation
)

func (k ErrorKind) IsValid() bool {
	switch k {
	case ErrorInvalidModelVersion,
		ErrorInvalidCalculationContext,
		ErrorConstraintNotFound,
		ErrorEvaluationFailure,
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
	case ErrorConstraintNotFound:
		return "ConstraintNotFound"
	case ErrorEvaluationFailure:
		return "EvaluationFailure"
	case ErrorInvariantViolation:
		return "InvariantViolation"
	default:
		return "Invalid"
	}
}

type Error struct {
	kind    ErrorKind
	message string
	cause   error
}

func newError(
	kind ErrorKind,
	message string,
	cause error,
) Error {
	if !kind.IsValid() {
		panic("constraint/check: invalid error kind")
	}

	if message == "" {
		panic("constraint/check: error message must not be empty")
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
			"constraint/check: %s: %s",
			e.kind.String(),
			e.message,
		)
	}

	return fmt.Sprintf(
		"constraint/check: %s: %s: %v",
		e.kind.String(),
		e.message,
		e.cause,
	)
}

var _ error = Error{}
