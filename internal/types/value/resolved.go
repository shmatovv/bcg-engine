package value

// ResolvedKind identifies the state represented by a Resolved value.
type ResolvedKind int

const (
	ResolvedInvalid ResolvedKind = iota

	ResolvedDefined
	ResolvedNull
	ResolvedUndefined

	resolvedKindLast
)

// IsValid reports whether k is a valid ResolvedKind.
func (k ResolvedKind) IsValid() bool {
	return k > ResolvedInvalid && k < resolvedKindLast
}

// String returns the canonical name of k.
//
// String returns "Invalid" for values that do not represent
// a valid ResolvedKind.
func (k ResolvedKind) String() string {
	switch k {
	case ResolvedDefined:
		return "Defined"

	case ResolvedNull:
		return "Null"

	case ResolvedUndefined:
		return "Undefined"

	default:
		return "Invalid"
	}
}

// Resolved represents the semantic result of resolving or evaluating
// an engine value.
//
// A valid Resolved is exactly one of:
//
//	Defined(Typed)
//	Null
//	Undefined(UndefinedInfo)
//
// Null represents an explicitly known absence of a value.
//
// Undefined represents a valid evaluation outcome for which no concrete
// value could be produced.
//
// Engine operation failures are intentionally not represented by Resolved.
// They belong to the EngineError layer.
//
// The zero value of Resolved is invalid.
type Resolved struct {
	kind ResolvedKind

	defined   Typed
	undefined UndefinedInfo
}

// Defined returns a Resolved containing value.
//
// Defined panics if value is invalid.
func Defined(value Typed) Resolved {
	if !value.IsValid() {
		panic("value: invalid defined value")
	}

	return Resolved{
		kind:    ResolvedDefined,
		defined: value,
	}
}

// Null returns a Resolved representing an explicitly known absence
// of a value.
func Null() Resolved {
	return Resolved{
		kind: ResolvedNull,
	}
}

// Undefined returns a Resolved representing an undefined evaluation
// outcome.
//
// Undefined panics if info is invalid.
func Undefined(info UndefinedInfo) Resolved {
	if !info.IsValid() {
		panic("value: invalid undefined info")
	}

	return Resolved{
		kind:      ResolvedUndefined,
		undefined: info,
	}
}

// Kind returns the state represented by r.
//
// ResolvedInvalid is returned for the zero value of Resolved.
func (r Resolved) Kind() ResolvedKind {
	return r.kind
}

// IsValid reports whether r represents a valid resolved value.
//
// The zero value of Resolved is invalid.
func (r Resolved) IsValid() bool {
	switch r.kind {
	case ResolvedDefined:
		return r.defined.IsValid()

	case ResolvedNull:
		return true

	case ResolvedUndefined:
		return r.undefined.IsValid()

	default:
		return false
	}
}

// IsDefined reports whether r contains a defined Typed value.
func (r Resolved) IsDefined() bool {
	return r.kind == ResolvedDefined
}

// IsNull reports whether r represents Null.
func (r Resolved) IsNull() bool {
	return r.kind == ResolvedNull
}

// IsUndefined reports whether r represents Undefined.
func (r Resolved) IsUndefined() bool {
	return r.kind == ResolvedUndefined
}

// Value returns the Typed value contained in r.
//
// Value panics unless r is Defined.
func (r Resolved) Value() Typed {
	r.requireKind(ResolvedDefined)

	return r.defined
}

// UndefinedInfo returns the UndefinedInfo contained in r.
//
// UndefinedInfo panics unless r is Undefined.
func (r Resolved) UndefinedInfo() UndefinedInfo {
	r.requireKind(ResolvedUndefined)

	return r.undefined
}

// Equal reports whether r and other represent the same resolved value.
//
// Invalid Resolved values are never equal.
func (r Resolved) Equal(other Resolved) bool {
	if !r.IsValid() || !other.IsValid() {
		return false
	}

	if r.kind != other.kind {
		return false
	}

	switch r.kind {
	case ResolvedDefined:
		return r.defined.Equal(other.defined)

	case ResolvedNull:
		return true

	case ResolvedUndefined:
		return r.undefined.Equal(other.undefined)

	default:
		return false
	}
}

// requireKind panics unless r has expected kind.
func (r Resolved) requireKind(expected ResolvedKind) {
	if r.kind != expected {
		panic(
			"value: requested " +
				expected.String() +
				" from " +
				r.kind.String() +
				" resolved value",
		)
	}
}
