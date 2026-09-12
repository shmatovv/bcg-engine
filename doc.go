// Package bcg is the public API for BCG Engine.
//
// A committed Model is an immutable, validated calculation-model version. It
// can be loaded from a portable .bcg Artifact with Decode or Unmarshal, or
// authored through NewModel, Model.Draft, Draft.Validate, and Draft.Commit.
// Edit creates an independent Draft from a committed Model; committing it
// creates a new version and never mutates the source Model. NewArtifact turns
// a committed Model and compatible Scenarios into a portable artifact.
//
// Evaluate is the simple stateless path; Session reuses one model's
// incremental runtime cache across calculations. Evaluation results are always
// one of Defined, Null, or Undefined. Undefined is a business outcome, not a
// Go error; Go errors indicate invalid requests or engine failures.
//
// The package intentionally does not expose internal domain objects,
// dependency graphs, or evaluator caches. Typed schema and expression
// authoring operations are added separately from the lifecycle API.
package bcg
