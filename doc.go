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
// dependency graphs, or evaluator caches. Typed EntityType, RelationType,
// PropertyDefinition, PropertyContract, Trait, ScopeDefinition, Entity,
// Relation, ValueBinding, Constraint, and Rule values are authored through
// Draft. Expression provides immutable typed AST builders; validation and
// evaluation retain responsibility for cross-model expression semantics.
// Scenario is an immutable value-only overlay constructed against one
// committed Model version.
//
// Dataset is an optional fully materialized immutable external data snapshot
// bound to one exact Model version. Model.WithDataset returns an
// EvaluationSnapshot that uses the combined graph without changing the
// model-only API or portable .bcg artifacts.
//
// ValueType describes scalar schema metadata, while Value holds an actual
// defined value. Dimension and Unit provide exact, domain-neutral unit
// primitives; BCG has no built-in business-unit catalog. Unit conversion
// scales use Rational rather than float64. TemporalScalar and TemporalSeries
// describe property temporal mode, while PointSeries and IntervalSeries
// describe source-series axes. SourceSeries is persistent authored input;
// SeriesResult is the separate runtime evaluation output.
package bcg
