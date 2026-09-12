// Package impact analyzes downstream property consequences in one immutable
// BCG model snapshot.
package impact

import "fmt"

// ErrorKind identifies an Impact analysis operation failure. Normal business
// outcomes, including Null and Undefined, never become Impact errors.
type ErrorKind uint8

const (
	ErrorInvalid ErrorKind = iota
	ErrorInvalidModelVersion
	ErrorInvalidSource
	ErrorSourceNotFound
	ErrorDependencyGraphBuild
	ErrorEffectivePlanBuild
	ErrorEvaluation
	ErrorInvariantViolation
)

// IsValid reports whether k is a supported Impact error kind.
func (k ErrorKind) IsValid() bool {
	switch k {
	case ErrorInvalidModelVersion, ErrorInvalidSource, ErrorSourceNotFound,
		ErrorDependencyGraphBuild, ErrorEffectivePlanBuild, ErrorEvaluation,
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
	case ErrorInvalidSource:
		return "InvalidSource"
	case ErrorSourceNotFound:
		return "SourceNotFound"
	case ErrorDependencyGraphBuild:
		return "DependencyGraphBuild"
	case ErrorEffectivePlanBuild:
		return "EffectivePlanBuild"
	case ErrorEvaluation:
		return "Evaluation"
	case ErrorInvariantViolation:
		return "InvariantViolation"
	default:
		return "Invalid"
	}
}

// Error is an engine-operation failure during Impact analysis.
type Error struct {
	kind    ErrorKind
	message string
	cause   error
}

func newError(kind ErrorKind, message string, cause error) Error {
	if !kind.IsValid() {
		panic("impact: invalid error kind")
	}
	if message == "" {
		panic("impact: error message must not be empty")
	}
	return Error{kind: kind, message: message, cause: cause}
}

// Kind classifies this engine-operation failure.
func (e Error) Kind() ErrorKind { return e.kind }

// Unwrap returns the underlying graph, plan, or evaluator failure when present.
func (e Error) Unwrap() error { return e.cause }

// Error implements error.
func (e Error) Error() string {
	if e.cause == nil {
		return fmt.Sprintf("impact: %s: %s", e.kind, e.message)
	}
	return fmt.Sprintf("impact: %s: %s: %v", e.kind, e.message, e.cause)
}

var _ error = Error{}
