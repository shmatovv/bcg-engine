# BCG Dataset Format v1

`.bcgd` is the canonical JSON artifact for an immutable runtime `Dataset`.
It is separate from `.bcg`: `.bcg` carries the committed model schema and
calculation semantics, while `.bcgd` carries only runtime entities, relations
and object-level value bindings.

The envelope is `formatVersion: 1` and
`engineCompatibility: "bcg-engine-v2"`. It includes `datasetId`,
`modelVersionId`, `modelSemanticDigest`, `datasetContentDigest`, and canonical
`content`. Content is sorted deterministically and includes typed values,
units, scopes, effective periods, source series, and relation applicability.

Decoding always requires an explicit committed Model. The model-version ID and
semantic digest must both match. Decoded runtime content is structurally
validated against that model and its content digest is verified. No schema,
model declarations, scenarios, functions, callback implementations, caches,
or runtime state are serialized.

Input is strict canonical JSON: duplicate/unknown fields, trailing content,
invalid typed values, excessive nesting, malformed content, unsupported format
versions, and incompatible engine values fail clearly. `.bcgd` input is
limited to 64 MiB before decode so an artifact cannot request unbounded
allocation from a host.
