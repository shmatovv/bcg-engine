// Package function contains the portable semantic identity and signature of
// an extension function. It deliberately contains no Go callbacks: those
// belong to a Runtime supplied by the host.
package function

import (
	"fmt"
	"strings"

	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
)

// Version is an exact semantic function version. Compatibility is exact, not
// major-version based.
type Version struct{ Major, Minor, Patch uint32 }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// Ref is the portable identity of an extension function.
type Ref struct {
	Namespace string
	Name      string
	Version   Version
}

func NewRef(namespace, name string, version Version) (Ref, error) {
	if namespace == "" || name == "" || strings.ContainsAny(namespace+name, "@/ ") {
		return Ref{}, fmt.Errorf("function: namespace and name are required")
	}
	return Ref{Namespace: namespace, Name: name, Version: version}, nil
}

func (r Ref) IsValid() bool {
	return r.Namespace != "" && r.Name != "" && !strings.ContainsAny(r.Namespace+r.Name, "@/ ")
}
func (r Ref) String() string {
	if !r.IsValid() {
		return ""
	}
	return r.Namespace + "." + r.Name + "@" + r.Version.String()
}

// Key is an unambiguous internal map/sort identity. String intentionally
// remains the compact presentation form described by the public API, where a
// dotted namespace and a dotted name could otherwise be ambiguous.
func (r Ref) Key() string {
	if !r.IsValid() {
		return ""
	}
	return fmt.Sprintf("%d:%s%d:%s@%s", len(r.Namespace), r.Namespace, len(r.Name), r.Name, r.Version.String())
}

// Signature is a fully-qualified scalar function contract. Parameters are
// defined values at invocation; nullability is therefore meaningful only for
// the result. Units and Enum identities live in typing.Type.
type Signature struct {
	parameters []typing.Type
	result     typing.Type
	nullable   bool
}

func NewSignature(parameters []typing.Type, result typing.Type, nullable bool) (Signature, error) {
	if !validFunctionType(result) {
		return Signature{}, fmt.Errorf("function: invalid result type")
	}
	copy := append([]typing.Type(nil), parameters...)
	for _, parameter := range copy {
		if !validFunctionType(parameter) || parameter.Nullable() {
			return Signature{}, fmt.Errorf("function: invalid parameter type")
		}
	}
	return Signature{parameters: copy, result: result.WithNullable(false), nullable: nullable}, nil
}

func (s Signature) Parameters() []typing.Type { return append([]typing.Type(nil), s.parameters...) }
func (s Signature) Result() typing.Type       { return s.result }
func (s Signature) NullableResult() bool      { return s.nullable }
func (s Signature) IsValid() bool {
	if !validFunctionType(s.result) {
		return false
	}
	for _, parameter := range s.parameters {
		if !validFunctionType(parameter) || parameter.Nullable() {
			return false
		}
	}
	return true
}

func validFunctionType(t typing.Type) bool {
	return t.IsValid() && t.TemporalMode() == typing.TemporalScalar
}

// Requirement is the semantic model declaration required to evaluate a Call.
type Requirement struct {
	ref       Ref
	signature Signature
}

func NewRequirement(ref Ref, signature Signature) (Requirement, error) {
	if !ref.IsValid() || !signature.IsValid() {
		return Requirement{}, fmt.Errorf("function: invalid requirement")
	}
	return Requirement{ref: ref, signature: signature}, nil
}

func (r Requirement) Ref() Ref             { return r.ref }
func (r Requirement) Signature() Signature { return r.signature }
func (r Requirement) IsValid() bool        { return r.ref.IsValid() && r.signature.IsValid() }
