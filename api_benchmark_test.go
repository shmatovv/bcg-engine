package bcg_test

import (
	"testing"

	bcg "github.com/shmatovv/bcg-engine"
)

func BenchmarkPublicEvaluate(b *testing.B) {
	artifact, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		b.Fatal(err)
	}
	target := publicTarget(b)
	context := publicContext(b)
	model := artifact.Model()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := model.Evaluate(target, context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublicSessionWarmEvaluate(b *testing.B) {
	artifact, err := bcg.Unmarshal([]byte(publicArtifact))
	if err != nil {
		b.Fatal(err)
	}
	target := publicTarget(b)
	session, err := artifact.Model().NewSession(publicContext(b))
	if err != nil {
		b.Fatal(err)
	}
	if _, err := session.Evaluate(target); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := session.Evaluate(target); err != nil {
			b.Fatal(err)
		}
	}
}
