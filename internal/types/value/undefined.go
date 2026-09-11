package value

// UndefinedKind identifies the reason why evaluation did not produce
// a defined value.
type UndefinedKind int

const (
	UndefinedInvalid UndefinedKind = iota

	UndefinedMissingValue
	UndefinedAmbiguousValue
	UndefinedDependencyUndefined
	UndefinedEmptyAggregation
	UndefinedIndeterminateCondition
	UndefinedInvalidOperation
	UndefinedExternalUnavailable
	UndefinedDivisionByZero

	undefinedKindLast
)

// IsValid reports whether k is a valid UndefinedKind.
func (k UndefinedKind) IsValid() bool {
	return k > UndefinedInvalid && k < undefinedKindLast
}

// String returns the canonical name of k.
//
// String returns "Invalid" for values that do not represent
// a valid UndefinedKind.
func (k UndefinedKind) String() string {
	switch k {
	case UndefinedMissingValue:
		return "MissingValue"

	case UndefinedAmbiguousValue:
		return "AmbiguousValue"

	case UndefinedDependencyUndefined:
		return "DependencyUndefined"

	case UndefinedEmptyAggregation:
		return "EmptyAggregation"

	case UndefinedIndeterminateCondition:
		return "IndeterminateCondition"

	case UndefinedInvalidOperation:
		return "InvalidOperation"

	case UndefinedExternalUnavailable:
		return "ExternalUnavailable"
	case UndefinedDivisionByZero:
		return "DivisionByZero"

	default:
		return "Invalid"
	}
}

// UndefinedInfo describes why evaluation produced Undefined.
//
// Provenance is intentionally not stored here. UndefinedInfo describes
// the semantic reason; provenance belongs to the evaluation result layer.
//
// The zero value of UndefinedInfo is invalid.
type UndefinedInfo struct {
	kind UndefinedKind
}

// NewUndefinedInfo returns UndefinedInfo with the specified kind.
//
// NewUndefinedInfo panics if kind is invalid.
func NewUndefinedInfo(kind UndefinedKind) UndefinedInfo {
	if !kind.IsValid() {
		panic("value: invalid undefined kind")
	}

	return UndefinedInfo{
		kind: kind,
	}
}

// Kind returns the reason represented by i.
func (i UndefinedInfo) Kind() UndefinedKind {
	return i.kind
}

// IsValid reports whether i represents valid Undefined information.
func (i UndefinedInfo) IsValid() bool {
	return i.kind.IsValid()
}

// Equal reports whether i and other describe the same Undefined reason.
//
// Invalid UndefinedInfo values are never equal.
func (i UndefinedInfo) Equal(other UndefinedInfo) bool {
	if !i.IsValid() || !other.IsValid() {
		return false
	}

	return i.kind == other.kind
}
