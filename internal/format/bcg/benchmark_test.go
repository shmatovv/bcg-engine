package bcg

import "testing"

func BenchmarkMarshalComplexArtifact(b *testing.B) {
	artifact, _, _ := complexArtifact(b)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := Marshal(artifact); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalComplexArtifact(b *testing.B) {
	artifact, _, _ := complexArtifact(b)
	data, err := Marshal(artifact)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := Unmarshal(data); err != nil {
			b.Fatal(err)
		}
	}
}
