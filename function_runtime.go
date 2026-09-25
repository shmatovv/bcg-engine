package bcg

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// SemanticVersion is an exact portable extension-function version.
type SemanticVersion struct{ major, minor, patch uint32 }

func NewSemanticVersion(major, minor, patch uint32) SemanticVersion {
	return SemanticVersion{major, minor, patch}
}
func (v SemanticVersion) Major() uint32  { return v.major }
func (v SemanticVersion) Minor() uint32  { return v.minor }
func (v SemanticVersion) Patch() uint32  { return v.patch }
func (v SemanticVersion) String() string { return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) }

// FunctionRef is an exact namespace/name/version identity.
type FunctionRef struct {
	namespace, name string
	version         SemanticVersion
}

func NewFunctionRef(namespace, name string, version SemanticVersion) (FunctionRef, error) {
	if namespace == "" || name == "" || strings.ContainsAny(namespace+name, "@/ ") {
		return FunctionRef{}, fmt.Errorf("%w: function namespace and name are required", ErrInvalidArgument)
	}
	return FunctionRef{namespace, name, version}, nil
}
func (r FunctionRef) Namespace() string        { return r.namespace }
func (r FunctionRef) Name() string             { return r.name }
func (r FunctionRef) Version() SemanticVersion { return r.version }
func (r FunctionRef) IsValid() bool            { return r.namespace != "" && r.name != "" }
func (r FunctionRef) String() string           { return r.namespace + "." + r.name + "@" + r.version.String() }

type FunctionSignature struct {
	parameters []ValueType
	result     ValueType
	nullable   bool
}

func NewFunctionSignature(parameters []ValueType, result ValueType, nullable bool) (FunctionSignature, error) {
	if !result.IsValid() {
		return FunctionSignature{}, fmt.Errorf("%w: invalid function result type", ErrInvalidArgument)
	}
	p := append([]ValueType(nil), parameters...)
	for _, v := range p {
		if !v.IsValid() {
			return FunctionSignature{}, fmt.Errorf("%w: invalid function parameter type", ErrInvalidArgument)
		}
	}
	return FunctionSignature{p, result, nullable}, nil
}
func (s FunctionSignature) ParameterTypes() []ValueType {
	return append([]ValueType(nil), s.parameters...)
}
func (s FunctionSignature) ResultType() ValueType { return s.result }
func (s FunctionSignature) NullableResult() bool  { return s.nullable }
func (s FunctionSignature) IsValid() bool         { return s.result.IsValid() }

type FunctionRequirementSpec struct {
	Ref       FunctionRef
	Signature FunctionSignature
}
type FunctionRequirement struct {
	ref       FunctionRef
	signature FunctionSignature
}

func NewFunctionRequirement(spec FunctionRequirementSpec) (FunctionRequirement, error) {
	if !spec.Ref.IsValid() || !spec.Signature.IsValid() {
		return FunctionRequirement{}, fmt.Errorf("%w: invalid function requirement", ErrInvalidArgument)
	}
	return FunctionRequirement{spec.Ref, spec.Signature}, nil
}
func (r FunctionRequirement) Ref() FunctionRef             { return r.ref }
func (r FunctionRequirement) Signature() FunctionSignature { return r.signature }
func (r FunctionRequirement) IsValid() bool                { return r.ref.IsValid() && r.signature.IsValid() }

// BoundFunction receives only defined Values and must be pure and concurrent-safe.
type BoundFunction interface {
	Call(args []Value) (Value, error)
}
type FunctionImplementation interface {
	Ref() FunctionRef
	Bind(requirement FunctionRequirement) (BoundFunction, error)
}
type FunctionRegistry struct {
	implementations map[string]FunctionImplementation
	fingerprint     [32]byte
}

func NewFunctionRegistry(implementations ...FunctionImplementation) (FunctionRegistry, error) {
	m := make(map[string]FunctionImplementation, len(implementations))
	keys := make([]string, 0, len(implementations))
	for _, impl := range implementations {
		if impl == nil || !impl.Ref().IsValid() {
			return FunctionRegistry{}, fmt.Errorf("%w: invalid function implementation", ErrInvalidArgument)
		}
		k := impl.Ref().String()
		if _, ok := m[k]; ok {
			return FunctionRegistry{}, fmt.Errorf("%w: duplicate function %s", ErrInvalidArgument, k)
		}
		m[k] = impl
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
	}
	var fp [32]byte
	copy(fp[:], h.Sum(nil))
	return FunctionRegistry{m, fp}, nil
}
func (r FunctionRegistry) Lookup(ref FunctionRef) (FunctionImplementation, bool) {
	v, ok := r.implementations[ref.String()]
	return v, ok
}
func (r FunctionRegistry) Fingerprint() []byte { return append([]byte(nil), r.fingerprint[:]...) }
func (r FunctionRegistry) IsValid() bool       { return r.implementations != nil }

type Runtime struct{ registry FunctionRegistry }

func NewRuntime(registry FunctionRegistry) (Runtime, error) {
	if !registry.IsValid() {
		return Runtime{}, fmt.Errorf("%w: invalid function registry", ErrInvalidArgument)
	}
	return Runtime{registry}, nil
}
func DefaultRuntime() Runtime                { r, _ := NewFunctionRegistry(); return Runtime{r} }
func (r Runtime) Registry() FunctionRegistry { return r.registry }
func (r Runtime) IsValid() bool              { return r.registry.IsValid() }
func (r Runtime) Bind(requirement FunctionRequirement) (BoundFunction, error) {
	if !r.IsValid() {
		return nil, fmt.Errorf("%w: invalid runtime", ErrInvalidArgument)
	}
	impl, ok := r.registry.Lookup(requirement.Ref())
	if !ok {
		return nil, fmt.Errorf("%w: required function %s is unavailable", ErrIncompatibleRuntime, requirement.Ref())
	}
	bound, err := impl.Bind(requirement)
	if err != nil || bound == nil {
		return nil, fmt.Errorf("%w: function %s is incompatible: %v", ErrIncompatibleRuntime, requirement.Ref(), err)
	}
	return bound, nil
}
