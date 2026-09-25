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
// Draft and losslessly inspectable on a committed Model. Expression provides
// immutable typed AST builders plus Kind, Children, typed node accessors, and
// WalkExpression; validation and evaluation retain responsibility for
// cross-model expression semantics.
// Scenario is an immutable value-only overlay constructed against one
// committed Model version.
//
// CompareModels returns a deterministic typed diff between committed versions
// of the same logical Model. AnalyzeEvolution conservatively classifies the
// semantic consequences of that diff with effect flags. It never migrates
// data and cannot know whether a concrete Dataset contains data that would be
// affected; PotentialDataLoss is risk, not proof. Dataset-aware compatibility
// is intentionally a later Dataset Rebase concern. Model.SemanticDigest is a
// SHA-256 digest of canonical semantic content, excluding version lineage and
// diagnostic metadata.
//
// Dataset is an optional fully materialized immutable external data snapshot
// bound to one exact Model version. Model.WithDataset returns an
// EvaluationSnapshot that uses the combined graph without changing the
// model-only API or portable .bcg artifacts.
//
// Model.PlanEvaluation provides the upstream, schema-level complement to
// AnalyzeImpact: it identifies property, topology, scope/time, and function
// requirements that may be needed before a Dataset is assembled. Traversal
// requirements never invent runtime records. A supplied Dataset is always the
// complete evaluation world; this package has no lazy loading or resolver I/O.
// EvaluationSnapshot.EvaluateBatch preserves ordered per-target results while
// sharing one immutable snapshot source and evaluator session.
//
// ValueType describes scalar schema metadata, while Value holds an actual
// defined value. Dimension and Unit provide exact, domain-neutral unit
// primitives; BCG has no built-in business-unit catalog. Unit conversion
// scales use Rational rather than float64. TemporalScalar and TemporalSeries
// describe property temporal mode, while PointSeries and IntervalSeries
// describe source-series axes. SourceSeries is persistent authored input;
// SeriesResult is the separate runtime evaluation output.
package bcg
