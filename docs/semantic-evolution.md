# Semantic evolution

`CompareModels(oldModel, newModel)` compares committed versions of one logical
Model. Stable declaration IDs define identity, so renaming a declaration is a
modification rather than a removal and addition. The returned changes are in
deterministic order and cover the evaluation-relevant public semantic surface.

`AnalyzeEvolution(oldModel, newModel)` interprets that diff conservatively and
without Dataset access. Its per-change and aggregate `EvolutionEffect` flags
identify possible recalculation, integrity recheck, data migration, and data
loss consequences. This is a Dataset-independent risk analysis: it never
claims that a particular record was actually lost.

`PotentialDataLoss` means that the schema change can make existing values or
structure unacceptable. It is not a report of concrete loss. To test concrete
unchanged runtime content, call `dataset.Rebase(oldModel, newModel, newID)`.
Rebase rebuilds/validates the same Dataset under the target Model and either
returns a new Dataset association or a validation report. It is not a data
migration: it does not transform values, repair keys, or invent/delete records.

`Model.SemanticDigest()` is deterministic SHA-256 over evaluation semantics.
It intentionally excludes lifecycle metadata such as version ID, parent ID,
and version label, while including declarations, ASTs, keys, cardinality,
requirements, and other calculation semantics. The digest is stable across
canonical marshal/unmarshal and can be used with Dataset identity; it is not a
replacement for `CompareModels` when a human-readable change report is needed.
