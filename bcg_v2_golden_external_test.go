package bcg_test

import (
	"bytes"
	"os"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

type goldenFunction struct{ ref bcg.FunctionRef }

func (f goldenFunction) Ref() bcg.FunctionRef { return f.ref }
func (goldenFunction) Bind(bcg.FunctionRequirement) (bcg.BoundFunction, error) {
	return goldenBound{}, nil
}

type goldenBound struct{}

func (goldenBound) Call([]bcg.Value) (bcg.Value, error) { return bcg.String("golden"), nil }

func TestFrozenComprehensiveBCGV2Golden(t *testing.T) {
	data, err := os.ReadFile("internal/format/bcg/testdata/comprehensive-v2.bcg")
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := bcg.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	model := artifact.Model()
	if !model.IsValid() || len(model.EnumDefinitions()) != 1 || len(model.EntityKeyDefinitions()) != 1 || len(model.InvariantDefinitions()) != 1 || len(model.FunctionRequirements()) != 1 {
		t.Fatal("golden lost v2 declarations")
	}
	if len(model.RelationTypes()) != 1 || len(model.PropertyDefinitionDeclarations()) != 4 || len(model.Constraints()) != 1 || len(model.Rules()) != 1 || len(artifact.Scenarios()) != 1 {
		t.Fatal("golden lost persistent declarations")
	}
	again, err := bcg.Marshal(artifact)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("golden is not canonical: %v", err)
	}
	ref := model.FunctionRequirements()[0].Ref()
	runtime, err := bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(goldenFunction{ref})))
	if err != nil {
		t.Fatal(err)
	}
	at, err := bcg.ParseDateTime("2026-06-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	target, err := bcg.EntityProperty(bcg.MustParseID("00000000-0000-4000-8000-00000000a061"), bcg.MustParseID("00000000-0000-4000-8000-00000000a042"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Evaluate(model, target, context)
	value, ok := result.Value()
	text, textOK := value.StringValue()
	if err != nil || !ok || !textOK || text != "golden" {
		t.Fatalf("decoded FunctionCall evaluation = %#v, %v", result, err)
	}
}
