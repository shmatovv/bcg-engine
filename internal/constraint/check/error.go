package check

import "fmt"

// ErrorKind classifies an API or engine-operation failure. Constraint business
// outcomes such as Violated, NotApplicable, and Undefined are never errors.
type ErrorKind uint8

const (
	ErrorInvalid ErrorKind = iota
	ErrorInvalidModelVersion
	ErrorInvalidCalculationContext
	ErrorUnsupportedTimeContext
	ErrorConstraintNotFound
	ErrorEvaluationFailure
	ErrorInvariantViolation
)

// IsValid reports whether k is a supported constraint-checking error kind.
func (k ErrorKind) IsValid() bool {
	switch k {
	case ErrorInvalidModelVersion,
		ErrorInvalidCalculationContext,
		ErrorUnsupportedTimeContext,
		ErrorConstraintNotFound,
		ErrorEvaluationFailure,
		ErrorInvariantViolation:
		return true
	default:
		return false
	}
}

// String returns k's stable semantic name.
func (k ErrorKind) String() string {
	switch k {
	case ErrorInvalidModelVersion:
		return "InvalidModelVersion"
	case ErrorInvalidCalculationContext:
		return "InvalidCalculationContext"
	case ErrorUnsupportedTimeContext:
		return "UnsupportedTimeContext"
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

// Error is an immutable engine-operation error returned by Constraint APIs.
type Error struct {
	kind    ErrorKind
	message string
	cause   error
}

func newError(kind ErrorKind, message string, cause error) Error {
	if !kind.IsValid() {
		panic("constraint/check: invalid error kind")
	}
	if message == "" {
		panic("constraint/check: error message must not be empty")
	}
	return Error{kind: kind, message: message, cause: cause}
}

// Kind returns the structured failure classification.
func (e Error) Kind() ErrorKind { return e.kind }

// Unwrap returns the lower-level engine error when one exists.
func (e Error) Unwrap() error { return e.cause }

func (e Error) Error() string {
	if e.cause == nil {
		return fmt.Sprintf("constraint/check: %s: %s", e.kind.String(), e.message)
	}
	return fmt.Sprintf("constraint/check: %s: %s: %v", e.kind.String(), e.message, e.cause)
}

var _ error = Error{}
