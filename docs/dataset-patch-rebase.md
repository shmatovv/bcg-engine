# Dataset Patch and Rebase

`Dataset` is immutable. Build a `DatasetPatch` with
`NewDatasetPatchBuilder(baseDatasetID)` and finalize it with `Build`; the patch
is immutable and is bound to that exact base ID. `dataset.Apply(model,
newDatasetID, patch)` applies all operations to a candidate final state,
validates that final state atomically, and returns a new Dataset only on
success. The original Dataset and Patch never change.

Removing an Entity or Relation never cascades. A patch that leaves dangling
relations, invalid bindings, duplicate keys, or invalid cardinality fails its
final-state validation rather than silently rewriting other objects. Local
construction/patch validity is distinct from `IntegrityReport`: structural
integrity reports required bindings, keys, and cardinality; contextual
integrity additionally evaluates invariants and is a business finding rather
than a construction error.

Every successful Apply gets the supplied new `DatasetID`; callers must not
reuse an ID for different content. `DatasetContentDigest` identifies canonical
runtime content and intentionally excludes DatasetID. A content-changing patch
changes that digest.

`dataset.Rebase(oldModel, newModel, newDatasetID)` checks that the Dataset
still has the exact old-model semantic digest, then validates its unchanged
runtime content against the new committed Model. Successful Rebase returns a
new DatasetID/model association and retains the same content digest. It never
does implicit migration, coercion, cascade deletion, or data repair. Use an
explicit host migration before rebuilding a Dataset if the target schema needs
one.
