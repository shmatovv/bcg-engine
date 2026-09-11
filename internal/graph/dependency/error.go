package dependency

import "fmt"

// BuildErrorKind identifies why a concrete dependency graph could not be
// constructed from a supposedly committed model source.
type BuildErrorKind uint8

const (
	BuildErrorInvalid BuildErrorKind = iota
	BuildErrorInvalidSourceObject
	BuildErrorTypeNotFound
	BuildErrorDefinitionNotFound
	BuildErrorDefinitionOwnerMismatch
	BuildErrorInvalidExpression
	BuildErrorInvalidReference
	BuildErrorMissingBinding
	BuildErrorContractImplementationNotFound
	BuildErrorInvalidTraversal
)

// IsValid reports whether k is a concrete build failure kind.
func (k BuildErrorKind) IsValid() bool {
	switch k {
	case BuildErrorInvalidSourceObject,
		BuildErrorTypeNotFound,
		BuildErrorDefinitionNotFound,
		BuildErrorDefinitionOwnerMismatch,
		BuildErrorInvalidExpression,
		BuildErrorInvalidReference,
		BuildErrorMissingBinding,
		BuildErrorContractImplementationNotFound,
		BuildErrorInvalidTraversal:
		return true
	default:
		return false
	}
}

// String returns the canonical stable name of k.
func (k BuildErrorKind) String() string {
	switch k {
	case BuildErrorInvalidSourceObject:
		return "InvalidSourceObject"
	case BuildErrorTypeNotFound:
		return "TypeNotFound"
	case BuildErrorDefinitionNotFound:
		return "DefinitionNotFound"
	case BuildErrorDefinitionOwnerMismatch:
		return "DefinitionOwnerMismatch"
	case BuildErrorInvalidExpression:
		return "InvalidExpression"
	case BuildErrorInvalidReference:
		return "InvalidReference"
	case BuildErrorMissingBinding:
		return "MissingBinding"
	case BuildErrorContractImplementationNotFound:
		return "ContractImplementationNotFound"
	case BuildErrorInvalidTraversal:
		return "InvalidTraversal"
	default:
		return "Invalid"
	}
}

// BuildError describes a failure to materialize the concrete value
// dependency graph.
//
// Build is intended to run only against a validated committed model. Most
// BuildError values therefore indicate that the caller supplied a corrupt or
// internally inconsistent source.
type BuildError struct {
	kind    BuildErrorKind
	message string
}

func newBuildError(
	kind BuildErrorKind,
	message string,
) BuildError {
	if !kind.IsValid() {
		panic("dependency: invalid build error kind")
	}

	if message == "" {
		panic("dependency: build error message must not be empty")
	}

	return BuildError{
		kind:    kind,
		message: message,
	}
}

// Kind returns the semantic failure class.
func (e BuildError) Kind() BuildErrorKind {
	return e.kind
}

// Error implements error.
func (e BuildError) Error() string {
	return fmt.Sprintf(
		"dependency: %s: %s",
		e.kind.String(),
		e.message,
	)
}

var _ error = BuildError{}
