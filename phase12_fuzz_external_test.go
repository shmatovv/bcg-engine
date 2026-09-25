package bcg_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// FuzzPhase12BCGV1V2Decode exercises both supported .bcg compatibility
// paths from an external consumer boundary. A successful decode must remain
// marshalable and canonically stable.
func FuzzPhase12BCGV1V2Decode(f *testing.F) {
	f.Add([]byte(publicArtifact))
	f.Add([]byte(publicScopedArtifact))
	if artifact, err := bcg.Unmarshal([]byte(publicArtifact)); err == nil {
		if encoded, err := bcg.Marshal(artifact); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte(`{"formatVersion":999}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		artifact, err := bcg.Unmarshal(data)
		if err != nil {
			return
		}
		canonical, err := bcg.Marshal(artifact)
		if err != nil {
			t.Fatalf("successful .bcg decode did not marshal: %v", err)
		}
		round, err := bcg.Unmarshal(canonical)
		if err != nil {
			t.Fatalf("successful .bcg decode did not round trip: %v", err)
		}
		again, err := bcg.Marshal(round)
		if err != nil || !bytes.Equal(canonical, again) {
			t.Fatalf("canonical .bcg round trip changed: %v", err)
		}
	})
}

func FuzzPhase12ExpressionIntrospectionRoundTrip(f *testing.F) {
	f.Add([]byte("hello"), uint8(3))
	f.Add([]byte{}, uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, shape uint8) {
		if len(data) > 256 {
			data = data[:256]
		}
		expression := phase34Constant(t, bcg.String(hex.EncodeToString(data)))
		for depth := 0; depth < int(shape%8); depth++ {
			var err error
			if depth%2 == 0 {
				expression, err = bcg.Lower(expression)
			} else {
				expression, err = bcg.Concat(expression, phase34Constant(t, bcg.String("x")))
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := reconstructExpression(expression); err != nil {
			t.Fatalf("public expression reconstruction: %v", err)
		}
		model, _, _ := phase34DerivedModel(t, "fuzz-expression", expression, bcg.StringType)
		artifact := mustData[bcg.Artifact](t)(bcg.NewArtifact(model))
		encoded := mustData[[]byte](t)(bcg.Marshal(artifact))
		decoded := mustData[bcg.Artifact](t)(bcg.Unmarshal(encoded))
		if again := mustData[[]byte](t)(bcg.Marshal(decoded)); !bytes.Equal(encoded, again) {
			t.Fatal("expression artifact was not canonical")
		}
	})
}

func FuzzPhase12EnumValuesAndFunctionSignatures(f *testing.F) {
	f.Add("example", "echo", uint8(1), uint8(1))
	f.Add("", "", uint8(0), uint8(0))
	f.Fuzz(func(t *testing.T, namespace, name string, parameterCount, optionCount uint8) {
		if len(namespace) > 64 {
			namespace = namespace[:64]
		}
		if len(name) > 64 {
			name = name[:64]
		}
		ref, _ := bcg.NewFunctionRef(namespace, name, bcg.NewSemanticVersion(1, 0, 0))
		parameters := make([]bcg.ValueType, int(parameterCount%16))
		for i := range parameters {
			parameters[i] = bcg.StringType
		}
		_, _ = bcg.NewFunctionSignature(parameters, bcg.StringType, false)
		enumID := bcg.NewID()
		options := make([]bcg.EnumOptionSpec, int(optionCount%16)+1)
		for i := range options {
			options[i] = bcg.EnumOptionSpec{ID: bcg.NewID(), Name: "option"}
		}
		enum, err := bcg.NewEnumDefinition(bcg.EnumDefinitionSpec{ID: enumID, Name: "enum", Options: options})
		if err != nil {
			t.Fatal(err)
		}
		for _, option := range enum.Options() {
			value, err := bcg.EnumValue(enum.ID(), option.ID())
			if err != nil || !value.IsValid() || value.Kind() != bcg.ValueEnum {
				t.Fatalf("enum value = %#v, %v", value, err)
			}
		}
		_ = ref
	})
}

func FuzzPhase12DatasetPatchDiffEvolutionAndRebase(f *testing.F) {
	f.Add(uint8(0), "one")
	f.Add(uint8(1), "two")
	f.Fuzz(func(t *testing.T, operation uint8, text string) {
		if len(text) > 128 {
			text = text[:128]
		}
		model, typeID, propertyID, _ := datasetInputModel(t, false)
		entity := phase12Entity(t, typeID)
		builder := mustData[*bcg.DatasetBuilder](t)(bcg.NewDatasetBuilder(model, bcg.NewID()))
		if err := builder.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
		binding := workflowBinding(t, entity.ID(), propertyID, bcg.DecimalValue(bcg.MustParseDecimal("1")))
		if err := builder.AddValueBinding(binding); err != nil {
			t.Fatal(err)
		}
		base := mustData[bcg.Dataset](t)(builder.Build())
		patchBuilder := mustData[*bcg.DatasetPatchBuilder](t)(bcg.NewDatasetPatchBuilder(base.ID()))
		switch operation % 3 {
		case 0:
			_ = patchBuilder.SetValueBinding(binding)
		case 1:
			_ = patchBuilder.RemoveValueBinding(binding.Key())
		case 2:
			_ = patchBuilder.AddEntity(phase12Entity(t, typeID))
		}
		patch := mustData[bcg.DatasetPatch](t)(patchBuilder.Build())
		next, report, err := base.Apply(model, bcg.NewID(), patch)
		if err == nil && report.OK() {
			if !next.IsValid() || next.ModelSemanticDigest() != model.SemanticDigest() {
				t.Fatal("successful patch returned an invalid or mismatched Dataset")
			}
			if _, report, err := next.Rebase(model, model, bcg.NewID()); err != nil || !report.OK() {
				t.Fatalf("self rebase after successful patch = %#v, %v", report.Issues(), err)
			}
		}
		if diff, err := bcg.CompareModels(model, model); err != nil || !diff.Empty() {
			t.Fatalf("identity model diff = %#v, %v", diff, err)
		}
		if evolution, err := bcg.AnalyzeEvolution(model, model); err != nil || len(evolution.Changes()) != 0 {
			t.Fatalf("identity evolution = %#v, %v", evolution, err)
		}
		_ = text
	})
}
