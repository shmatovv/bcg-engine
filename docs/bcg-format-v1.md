# BCG portable model format v1

`.bcg` v1 is a canonical UTF-8 JSON representation of one committed logical
BCG model snapshot and optional value-only Scenario overlays. It is designed
for files, databases, object storage, and messages; the codec itself only
accepts and produces `[]byte` or `io.Reader`/`io.Writer`.

The envelope contains independent concepts:

- `formatVersion`: the JSON wire-schema version (`1`);
- `engineCompatibility`: the semantic engine generation (`bcg-engine-v1`);
- `artifact.model`: the long-lived model identity and presentation name;
- `artifact.version`: one immutable committed `ModelVersion` snapshot.

The v1 decoder is strict. It rejects unknown fields, duplicate JSON keys,
unknown enum values, non-canonical UUIDs and primitive encodings, malformed
typed values, trailing documents, invalid UTF-8, and a UTF-8 BOM. Future wire
schemas use a new `formatVersion` decoder rather than relying on ignored
fields. Required fields are explicit (including `null` for an absent optional
reference). Documents are bounded to 64 MiB, JSON nesting to 2048 levels,
and each expression tree to 512 nodes along its deepest path. Model, property,
Rule, Constraint, and Relation containers do not count toward AST depth.

All unordered model collections are encoded in canonical UUID order. Scope
selectors, bindings, Scenario overrides, and Rule metadata are also
canonicalized. Decimal, Integer, rational unit scales, Date, DateTime, and
Duration are represented as lossless strings; no business number is encoded
through `float64`. Typed values carry an explicit type tag, and model Null is
represented explicitly in binding/series sample form.

The document contains logical schema and semantic data: types, traits,
contracts, property definitions and ASTs, scope definitions, entities,
relations, bindings, constraints, rules, and Scenarios. Aggregate traversal
origins remain explicit (`self`, `absolute_entity`, or `bound_entity`), so a
decoded standalone Rule or Constraint never acquires an implicit self.

Decode is intentionally not direct struct restoration. Wire data is converted
to domain constructors, then passed through the ordinary `model.Commit`
validation path. Semantic indexes, value dependencies, relation traversal
indexes, contract indexes, Constraint/Rule indexes, and cycle checks are
rebuilt from logical data. Scenario overlays are independently validated
against the rebuilt version.

The format deliberately excludes runtime state: evaluator Sessions, caches
and cache statistics, calculation overrides and contexts, evaluation
Undefined outcomes, provenance, Explain/Impact/Feasibility outputs, locks,
memoization, effective plans, and every derived index. A decoded artifact
always starts with a clean runtime execution state.
