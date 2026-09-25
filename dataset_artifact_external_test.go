package bcg_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

func bcgdGoldenModel(t testing.TB) bcg.Model {
	t.Helper()
	b, err := os.ReadFile("internal/format/bcg/testdata/comprehensive-v2.bcg")
	if err != nil {
		t.Fatal(err)
	}
	a, err := bcg.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return a.Model()
}

func TestFrozenComprehensiveBCGD(t *testing.T) {
	m := bcgdGoldenModel(t)
	b, err := os.ReadFile("testdata/comprehensive-v1.bcgd")
	if err != nil {
		t.Fatal(err)
	}
	a, err := bcg.UnmarshalDataset(m, b)
	if err != nil {
		t.Fatal(err)
	}
	if !a.IsValid() || !a.ModelVersionID().Equal(m.VersionID()) || a.ModelSemanticDigest() != m.SemanticDigest() || a.DatasetContentDigest() != a.Dataset().ContentDigest() {
		t.Fatal("artifact identity lost")
	}
	again, err := bcg.MarshalDataset(a)
	if err != nil || !bytes.Equal(b, again) {
		t.Fatalf("canonical .bcgd round trip: %v", err)
	}
	ctxAt, _ := bcg.ParseDateTime("2026-06-01T00:00:00Z")
	ctx, _ := bcg.NewCalculationContextAt(ctxAt)
	child := bcg.MustParseID("00000000-0000-4000-8000-00000000d102")
	target, _ := bcg.EntityProperty(child, bcg.MustParseID("00000000-0000-4000-8000-00000000a042"))
	runtime, err := bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(goldenFunction{m.FunctionRequirements()[0].Ref()})))
	if err != nil {
		t.Fatal(err)
	}
	s, err := runtime.WithDataset(m, a.Dataset())
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Evaluate(target, ctx)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := r.Value()
	text, ok2 := v.StringValue()
	if !ok || !ok2 || text != "golden" {
		t.Fatalf("decoded dataset evaluation = %#v", r)
	}
}

func TestDatasetArtifactRejectsIdentityCorruption(t *testing.T) {
	m := bcgdGoldenModel(t)
	b, err := os.ReadFile("testdata/comprehensive-v1.bcgd")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{
		strings.Replace(string(b), `"modelVersionId":"00000000-0000-4000-8000-00000000a002"`, `"modelVersionId":"00000000-0000-4000-8000-00000000ffff"`, 1),
		strings.Replace(string(b), `"modelSemanticDigest":"`, `"modelSemanticDigest":"00`, 1),
		strings.Replace(string(b), `"datasetContentDigest":"`, `"datasetContentDigest":"00`, 1),
		`{`, `{"formatVersion":1,"formatVersion":1}`,
	} {
		if _, err := bcg.UnmarshalDataset(m, []byte(change)); err == nil {
			t.Fatal("corrupt .bcgd accepted")
		}
	}
}

func TestDatasetArtifactRejectsUnknownFormatAndEngineClearly(t *testing.T) {
	model := bcgdGoldenModel(t)
	fixture, err := os.ReadFile("testdata/comprehensive-v1.bcgd")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		data string
		want error
	}{
		{"newer format", strings.Replace(string(fixture), `"formatVersion":1`, `"formatVersion":999`, 1), bcg.ErrUnsupportedFormat},
		{"wrong engine", strings.Replace(string(fixture), `"engineCompatibility":"bcg-engine-v2"`, `"engineCompatibility":"future-engine"`, 1), bcg.ErrIncompatibleEngine},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := bcg.UnmarshalDataset(model, []byte(test.data)); !errors.Is(err, test.want) {
				t.Fatalf("UnmarshalDataset error = %v, want %v", err, test.want)
			}
		})
	}
}

func FuzzDecodeDatasetArtifact(f *testing.F) {
	b, err := os.ReadFile("testdata/comprehensive-v1.bcgd")
	if err == nil {
		f.Add(b)
	}
	f.Add([]byte("{"))
	f.Fuzz(func(t *testing.T, data []byte) { m := bcgdGoldenModel(t); _, _ = bcg.UnmarshalDataset(m, data) })
}
