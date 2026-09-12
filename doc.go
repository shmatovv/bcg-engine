// Package bcg is the public consumption API for BCG Engine.
//
// A Model is an immutable, validated calculation-model version, normally
// loaded from a portable .bcg Artifact with Decode or Unmarshal. Evaluate is
// the simple stateless path; Session reuses one model's incremental runtime
// cache across calculations. Evaluation results are always one of Defined,
// Null, or Undefined. Undefined is a business outcome, not a Go error; Go
// errors indicate invalid requests or engine failures.
//
// The package intentionally exposes model capabilities and semantic results,
// not authoring drafts, expression implementation details, dependency graphs,
// or evaluator caches.
package bcg
