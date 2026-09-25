package bcg

import (
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// ResultState is the semantic state of a scalar evaluation.
type ResultState string

const (
	Defined   ResultState = "Defined"
	Null      ResultState = "Null"
	Undefined ResultState = "Undefined"
)

// UndefinedKind classifies a first-class Undefined evaluation outcome.
type UndefinedKind string

const (
	MissingValue           UndefinedKind = "MissingValue"
	AmbiguousValue         UndefinedKind = "AmbiguousValue"
	DependencyUndefined    UndefinedKind = "DependencyUndefined"
	EmptyAggregation       UndefinedKind = "EmptyAggregation"
	IndeterminateCondition UndefinedKind = "IndeterminateCondition"
	InvalidOperation       UndefinedKind = "InvalidOperation"
	ExternalUnavailable    UndefinedKind = "ExternalUnavailable"
	DivisionByZero         UndefinedKind = "DivisionByZero"
)

// UndefinedInfo describes why a Result is Undefined.
type UndefinedInfo struct{ kind UndefinedKind }

// Kind returns the structured Undefined reason.
func (i UndefinedInfo) Kind() UndefinedKind { return i.kind }

// Result is an immutable scalar evaluation outcome. Value is available only
// for Defined; Null and Undefined remain explicit non-error states.
type Result struct{ value evaluator.Result }

func resultFromInternal(value evaluator.Result) Result { return Result{value: value} }

func resultsFromInternal(values []evaluator.Result) []Result {
	result := make([]Result, len(values))
	for index, value := range values {
		result[index] = resultFromInternal(value)
	}
	return result
}

// IsValid reports whether r is an evaluator-produced result.
func (r Result) IsValid() bool { return r.value.IsValid() }

// State returns Defined, Null, or Undefined. It returns an empty value for an invalid Result.
func (r Result) State() ResultState {
	if !r.IsValid() {
		return ""
	}
	return resultStateFromResolved(r.value.Value())
}

func resultStateFromResolved(value enginevalue.Resolved) ResultState {
	switch value.Kind() {
	case enginevalue.ResolvedDefined:
		return Defined
	case enginevalue.ResolvedNull:
		return Null
	case enginevalue.ResolvedUndefined:
		return Undefined
	default:
		return ""
	}
}

// Value returns the defined Value, if any.
func (r Result) Value() (Value, bool) {
	if r.State() != Defined {
		return Value{}, false
	}
	return valueFromInternal(r.value.Value().Value()), true
}

// Undefined returns the structured cause, if r is Undefined.
func (r Result) Undefined() (UndefinedInfo, bool) {
	if r.State() != Undefined {
		return UndefinedInfo{}, false
	}
	return undefinedInfoFromInternal(r.value.Value().UndefinedInfo()), true
}

func undefinedInfoFromInternal(info enginevalue.UndefinedInfo) UndefinedInfo {
	switch info.Kind() {
	case enginevalue.UndefinedMissingValue:
		return UndefinedInfo{kind: MissingValue}
	case enginevalue.UndefinedAmbiguousValue:
		return UndefinedInfo{kind: AmbiguousValue}
	case enginevalue.UndefinedDependencyUndefined:
		return UndefinedInfo{kind: DependencyUndefined}
	case enginevalue.UndefinedEmptyAggregation:
		return UndefinedInfo{kind: EmptyAggregation}
	case enginevalue.UndefinedIndeterminateCondition:
		return UndefinedInfo{kind: IndeterminateCondition}
	case enginevalue.UndefinedInvalidOperation:
		return UndefinedInfo{kind: InvalidOperation}
	case enginevalue.UndefinedExternalUnavailable:
		return UndefinedInfo{kind: ExternalUnavailable}
	case enginevalue.UndefinedDivisionByZero:
		return UndefinedInfo{kind: DivisionByZero}
	default:
		return UndefinedInfo{}
	}
}

func valueKindFromInternal(kind enginevalue.Type) ValueKind {
	switch kind {
	case enginevalue.Bool:
		return ValueBool
	case enginevalue.Integer:
		return ValueInteger
	case enginevalue.Decimal:
		return ValueDecimal
	case enginevalue.String:
		return ValueString
	case enginevalue.Date:
		return ValueDate
	case enginevalue.DateTime:
		return ValueDateTime
	case enginevalue.Duration:
		return ValueDuration
	default:
		return ""
	}
}
