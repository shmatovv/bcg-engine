package plan

import "fmt"

// ErrorKind identifies a failure while constructing an Effective Evaluation
// Plan.
//
// Business-level value states such as MissingValue or AmbiguousValue are not
// plan errors. They remain normal resolved outcomes.
type ErrorKind uint8

const (
	ErrorInvalid ErrorKind = iota

	ErrorInvalidModelVersion
	ErrorInvalidCalculationContext
	ErrorUnsupportedTimeContext
	ErrorInvalidRoot
	ErrorRootNotFound
	ErrorDependencyGraphBuild
	ErrorOverrideResolution
	ErrorDefinitionNotFound
	ErrorInvalidComputation
	ErrorCycle
	ErrorInvariantViolation
)

func (k ErrorKind) IsValid() bool {
	switch k {
	case ErrorInvalidModelVersion,
		ErrorInvalidCalculationContext,
		ErrorUnsupportedTimeContext,
		ErrorInvalidRoot,
		ErrorRootNotFound,
		ErrorDependencyGraphBuild,
		ErrorOverrideResolution,
		ErrorDefinitionNotFound,
		ErrorInvalidComputation,
		ErrorCycle,
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

	case ErrorUnsupportedTimeContext:
		return "UnsupportedTimeContext"

	case ErrorInvalidRoot:
		return "InvalidRoot"

	case ErrorRootNotFound:
		return "RootNotFound"

	case ErrorDependencyGraphBuild:
		return "DependencyGraphBuild"

	case ErrorOverrideResolution:
		return "OverrideResolution"

	case ErrorDefinitionNotFound:
		return "DefinitionNotFound"

	case ErrorInvalidComputation:
		return "InvalidComputation"

	case ErrorCycle:
		return "Cycle"

	case ErrorInvariantViolation:
		return "InvariantViolation"

	default:
		return "Invalid"
	}
}

// Error is an Effective Evaluation Plan construction failure.
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
		panic("plan: invalid error kind")
	}

	if message == "" {
		panic("plan: error message must not be empty")
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
			"plan: %s: %s",
			e.kind.String(),
			e.message,
		)
	}

	return fmt.Sprintf(
		"plan: %s: %s: %v",
		e.kind.String(),
		e.message,
		e.cause,
	)
}

var _ error = Error{}
