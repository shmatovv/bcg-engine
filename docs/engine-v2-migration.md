# Migrating to BCG Engine v2

BCG Engine v2 is a new Go module path, not a source-compatible upgrade. Import
the public API as:

```go
import bcg "github.com/shmatovv/bcg-engine/v2"
```

The stable v1 baseline is v1.2.0 (`d7cc776`). Keep an existing v1 import path
until its call sites have been migrated and tested; `/v2` intentionally has
new immutable authoring, inspection, runtime, and result APIs.

## Artifact compatibility

V2 continues to read frozen `.bcg v1` documents with their historical
semantics. A v1 Input becomes optional and a v1 relation cardinality becomes
`0..N` in the v2 model. Normal `bcg.Marshal` output is canonical `.bcg v2`;
it does not reproduce v1 bytes. Decode a v1 artifact, inspect or edit the
resulting `Model`, then encode it normally to make the v2 representation.

`.bcg v2` adds UUID-identified Enums and enum options, Input presence,
Entity keys, directional relation cardinality, subject-relative Invariants,
function requirements, and lossless Schema v2/AST inspection. Unsupported or
newer document versions fail rather than being guessed.

## Dataset and runtime changes

Runtime data now lives in an immutable `Dataset`, separate from a committed
Model. Its identity is the application-selected `DatasetID`, `ModelVersionID`,
`ModelSemanticDigest`, and `DatasetContentDigest`. Use
`NewDatasetBuilder(model, datasetID)` and `model.WithDataset(dataset)` (or
`runtime.WithDataset`) to form an evaluation snapshot. `.bcgd v1` is the
canonical Dataset artifact; it carries and verifies those identity fields.

Extension calls are declared in a Model with `FunctionRequirement` and bound
at evaluation time by an explicit immutable `FunctionRegistry` inside a
`Runtime`. There is no global registry. A Model can be portable even when a
host has not supplied an implementation; evaluation then reports an
incompatible runtime.

## Typical migration

```go
artifact, err := bcg.Unmarshal(oldBCGBytes) // v1 or v2
if err != nil { return err }
model := artifact.Model()

draft, err := model.Edit()
if err != nil { return err }
// Add v2 declarations or make the required source/API adaptations.
next, err := draft.Commit(options)
if err != nil { return err }

nextArtifact, err := bcg.NewArtifact(next)
if err != nil { return err }
v2Bytes, err := bcg.Marshal(nextArtifact)
```

For external data, build a Dataset and use a snapshot rather than looking up
records from a callback during evaluation. Use `Dataset.Apply` for an atomic
new Dataset and `Dataset.Rebase` only to check unchanged runtime content
against another compatible Model. Rebase is not a migration and never coerces
or rewrites values. See [Dataset Patch and Rebase](dataset-patch-rebase.md).

The complete public-only example is in [`examples/public-v2`](../examples/public-v2/).
