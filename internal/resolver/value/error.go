package value

import "fmt"

// ErrorKind identifies a failure of the value resolution operation itself.
//
// Missing and ambiguous business values are not errors. They are represented
// by value.Resolved Undefined outcomes.
type ErrorKind uint8

const (
	ErrorInvalid ErrorKind = iota
	ErrorInvalidModelVersion
	ErrorInvalidCalculationContext
	ErrorInvalidScenario
	ErrorInvalidPropertyAddress
	ErrorObjectNotFound
	ErrorDefinitionNotFound
	ErrorDefinitionNotAvailable
	ErrorNotInputProperty
	ErrorUnsupportedSeriesProperty
	ErrorInvalidCalculationOverride
)

func (k ErrorKind) IsValid() bool {
	switch k {
	case ErrorInvalidModelVersion,
		ErrorInvalidCalculationContext,
		ErrorInvalidScenario,
		ErrorInvalidPropertyAddress,
		ErrorObjectNotFound,
		ErrorDefinitionNotFound,
		ErrorDefinitionNotAvailable,
		ErrorNotInputProperty,
		ErrorUnsupportedSeriesProperty,
		ErrorInvalidCalculationOverride:
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

	case ErrorInvalidScenario:
		return "InvalidScenario"

	case ErrorInvalidPropertyAddress:
		return "InvalidPropertyAddress"

	case ErrorObjectNotFound:
		return "ObjectNotFound"

	case ErrorDefinitionNotFound:
		return "DefinitionNotFound"

	case ErrorDefinitionNotAvailable:
		return "DefinitionNotAvailable"

	case ErrorNotInputProperty:
		return "NotInputProperty"

	case ErrorUnsupportedSeriesProperty:
		return "UnsupportedSeriesProperty"

	case ErrorInvalidCalculationOverride:
		return "InvalidCalculationOverride"

	default:
		return "Invalid"
	}
}

// Error describes a value resolution operation failure.
type Error struct {
	kind    ErrorKind
	message string
}

func newError(
	kind ErrorKind,
	message string,
) Error {
	if !kind.IsValid() {
		panic(
			"resolver/value: invalid error kind",
		)
	}

	if message == "" {
		panic(
			"resolver/value: error message must not be empty",
		)
	}

	return Error{
		kind:    kind,
		message: message,
	}
}

func (e Error) Kind() ErrorKind {
	return e.kind
}

func (e Error) Error() string {
	return fmt.Sprintf(
		"resolver/value: %s: %s",
		e.kind.String(),
		e.message,
	)
}

var _ error = Error{}
