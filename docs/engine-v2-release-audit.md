# Engine v2 release audit

Date: 2026-09-25
Audited source: Phase 12 freeze commit `chore: harden engine v2 for release`
(parent `93739290c204799d71149fc0fee0f4e0f8d0ea95`) on
`feature/engine-v2`.
Historical v1 baseline: v1.2.0 (`d7cc776`).

## History account

The only commit between requested Phase 11 baseline `16a6360` and reported
preflight `3cdf02f` is `3cdf02f305fd0a9fe67164be7a4f5a89c99b18ae`. It moved
static dependency analysis into `internal/graph/dependency/static.go` and
made planning use it. Its successor, audited starting HEAD `9373929`, completed
the unification by making the concrete extractor delegate to the same static
analysis. There are no unexplained commits in that interval.

## Definition of Done

| # | Status | Evidence |
|---|---|---|
| 1 | PASS | `TestEngineV2CanonicalExternalConsumerWorkflow` |
| 2 | PASS | `TestFrozenV1CompatibilityAndPublicIntrospection` |
| 3 | PASS | `TestFrozenComprehensiveBCGV2Golden`; canonical workflow |
| 4 | PASS | `TestFrozenComprehensiveBCGD`; canonical workflow |
| 5 | PASS | deterministic API/order tests; `go test -shuffle=on -count=20 ./...` |
| 6 | PASS | Dataset/integrity tests, key/cardinality regressions, shuffled run |
| 7 | PASS | Schema v2, Dataset, canonical workflow, and Phase 12 enum/key/cardinality tests |
| 8 | PASS | function runtime/external workflow tests; explicit immutable registry/runtime |
| 9 | PASS | `TestExternalModelDiffEvolutionAndDigest`; fuzz and concurrency coverage |
| 10 | PASS | `TestDatasetPatchAndRebasePublicWorkflow`; exact-digest regressions |
| 11 | PASS | evaluation-plan tests, canonical workflow, dotted-FunctionRef regression |
| 12 | PASS | existing external constraint/rule/scenario/impact/explain tests plus workflow |
| 13 | PASS | immutable Dataset/snapshot tests; no loader API or evaluation I/O path |
| 14 | PASS | canonical test and external example import only `/v2` |
| 15 | PASS | package/API boundary audit; no tenant/UI/workflow/persistence/transport concepts |
| 16 | PASS | `go test ./...` |
| 17 | PASS | `go test -race ./...` |
| 18 | PASS | `go vet ./...` and external example vet |
| 19 | PASS | `go test -shuffle=on -count=20 ./...` |
| 20 | PASS | every repository fuzz target smoke-tested for one second |
| 21 | PASS | `BenchmarkPhase12*` suite, 100 ms per benchmark; no catastrophic result observed |
| 22 | PASS | final post-commit `git status --short` is empty |
| 23 | PASS | README, format references, four required guides, and external example reviewed |
| 24 | PASS | P0/P1 findings below are fixed and regression-tested |

## P0/P1 findings fixed

- **P0 cardinality:** minimum cardinality was being checked for unrelated
  entity types. Integrity now scopes each direction to entities eligible for
  that endpoint.
- **P0 keys:** Enum values had no distinct business-key sort key; distinct
  options could collide. Composite key concatenation was also ambiguous for
  legal strings. Both keys are now injective.
- **P1 Dataset identity:** Apply/Rebase accepted a Model sharing a version ID
  but not its semantic digest. Both now require the exact digest.
- **P1 local validation:** relation-owned value-binding validation now creates
  an isolated minimal endpoint world instead of falsely rejecting a valid
  relation binding.
- **P1 function identity:** planning and semantic comparison no longer use a
  lossy display string for FunctionRef identity.
- **P1 codec/API errors:** the public compatibility constant is v2; Dataset
  validation, newer `.bcgd`, and incompatible Dataset engine errors have
  stable public categories.
- **P1 host limits:** enforced/documented bounds cover Enum options (4,096),
  composite key width (64), function arity (1,024), dependency planning
  (4,096), expression/evaluation depth (512), JSON nesting (2,048), and
  artifact input (64 MiB).

No unresolved P0/P1 issue was found. No P2/P3 item is being carried as a
release blocker.

## Compatibility and public proof

Frozen `.bcg v1` decodes with preserved semantics and normal canonical v2
output. Frozen `.bcg v2` and `.bcgd` fixtures both decode and re-encode
byte-identically. Unknown newer `.bcg`/`.bcgd` formats and incompatible Dataset
engine values fail with stable errors. The canonical public-only test executes
authoring, function requirement/runtime, v2 model artifact, evolution, Dataset
validation/integrity/evaluation/explain/impact/plan, Patch, Rebase, Dataset
artifact, and ordered duplicate-safe batch evaluation. `examples/public-v2`
also builds and runs as a separate Go module.

## Concurrency and fuzz

`TestPhase12ConcurrentImmutablePublicReads` drives concurrent Model, Dataset,
snapshot, Session, Runtime/registry, function, planning, diff/evolution,
Patch-result, digest, and batch reads; the full race suite passed.

Bounded fuzz smoke (one second target duration, eight workers) passed for:
`FuzzDecodeDatasetArtifact` (196 executions),
`FuzzPhase12BCGV1V2Decode` (10,131),
`FuzzPhase12ExpressionIntrospectionRoundTrip` (8,089),
`FuzzPhase12EnumValuesAndFunctionSignatures` (69,908),
`FuzzPhase12DatasetPatchDiffEvolutionAndRebase` (32,146),
internal BCG decode (45,990), evaluator session transitions (5,527), decimal
round-trip (246,481), and PointSeries axes (132,612). No panic, validation
bypass, or process-state corruption was observed.

## Benchmark baseline

Local Darwin/arm64 baseline, `go test -run='^$' -bench='^BenchmarkPhase12'
-benchtime=100ms .` (ns/op):

| Benchmark | ns/op |
|---|---:|
| Validate/Commit | 25,218 |
| `.bcg v1` decode into v2 | 190,813 |
| `.bcg v2` encode/decode | 377,994 |
| SemanticDigest | 13,299 |
| ModelDiff, 256-property schema | 344,768 |
| AnalyzeEvolution, 256-property schema | 345,534 |
| Dataset build | 26,635 |
| small Patch over 512 entities | 924,199 |
| Dataset Rebase | 59,537 |
| `.bcgd` encode/decode | 168,624 |
| EvaluationSnapshot build | 19,608 |
| PlanEvaluation | 5,574 |
| repeated Evaluate | 24,232 |
| EvaluateBatch (two duplicate targets) | 23,350 |
| FunctionCall evaluation | 25,920 |
| integrity over 256 keyed relations | 5,213,042 |

These are regression baselines, not latency SLAs. No suspicious catastrophic
behavior was observed in the exercised sizes.

## Documentation and limits

Created: [migration](engine-v2-migration.md),
[extension functions](extension-functions.md),
[semantic evolution](semantic-evolution.md), and
[Dataset Patch/Rebase](dataset-patch-rebase.md). Updated README and both v2
format references. Limits are documented in README and the v2 format document.
Native Go extension purity remains a host contract, not a sandbox boundary.

## Commands

```text
GOCACHE=/private/tmp/bcg-phase12-go-cache go test ./...
GOCACHE=/private/tmp/bcg-phase12-go-cache go test -race ./...
GOCACHE=/private/tmp/bcg-phase12-go-cache go vet ./...
GOCACHE=/private/tmp/bcg-phase12-go-cache go test -shuffle=on -count=20 ./...
GOCACHE=/private/tmp/bcg-phase12-go-cache go test -run='^$' -fuzz=<each target> -fuzztime=1s <package>
GOCACHE=/private/tmp/bcg-phase12-go-cache go test -run='^$' -bench='^BenchmarkPhase12' -benchtime=100ms .
(cd examples/public-v2 && GOCACHE=/private/tmp/bcg-phase12-go-cache go vet ./... && go run .)
```

All commands passed. No staticcheck/golangci-lint configuration exists in the
repository, so no unconfigured lint stack was introduced.

**Result: ENGINE V2 RELEASE AUDIT: READY FOR HUMAN TAG REVIEW.**
