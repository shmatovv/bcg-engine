package bcg_test

import (
	"errors"
	bcg "github.com/shmatovv/bcg-engine/v2"
	"testing"
)

type testBoundFunction struct{}

func (testBoundFunction) Call(args []bcg.Value) (bcg.Value, error) { return bcg.String("ok"), nil }

type testFunction struct{ ref bcg.FunctionRef }

func (f testFunction) Ref() bcg.FunctionRef { return f.ref }
func (f testFunction) Bind(requirement bcg.FunctionRequirement) (bcg.BoundFunction, error) {
	return testBoundFunction{}, nil
}
func TestFunctionRegistryIsExplicitAndImmutable(t *testing.T) {
	ref, err := bcg.NewFunctionRef("example", "status", bcg.NewSemanticVersion(1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	sig, err := bcg.NewFunctionSignature([]bcg.ValueType{bcg.StringType}, bcg.StringType, false)
	if err != nil {
		t.Fatal(err)
	}
	requirement, err := bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: ref, Signature: sig})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := bcg.NewFunctionRegistry(testFunction{ref})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := bcg.NewRuntime(registry)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := runtime.Bind(requirement)
	if err != nil {
		t.Fatal(err)
	}
	value, err := bound.Call([]bcg.Value{bcg.String("x")})
	if err != nil || value.Kind() != bcg.ValueString {
		t.Fatalf("bound call = %#v, %v", value, err)
	}
	if _, err := bcg.DefaultRuntime().Bind(requirement); !errors.Is(err, bcg.ErrIncompatibleRuntime) {
		t.Fatalf("default runtime error = %v", err)
	}
	if _, err := bcg.NewFunctionRegistry(testFunction{ref}, testFunction{ref}); err == nil {
		t.Fatal("duplicate function must be rejected")
	}
}
