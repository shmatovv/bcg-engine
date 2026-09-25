# BCG portable model format v2

`.bcg` v2 is canonical UTF-8 JSON for one committed logical ModelVersion and its value-only Scenario overlays.

```json
{"formatVersion":2,"engineCompatibility":"bcg-engine-v2","artifact":{...}}
```

The wire format is independent of the Go module version. `bcg.Marshal` and `bcg.Encode` write v2; the decoder accepts v1 and v2.

## Semantic representation

V2 retains all v1 schema, objects, relations, bindings, effective periods, constraints, rules and scenarios. It additionally stores enum definitions/options, enum-qualified contracts and properties, input presence, entity keys, relation cardinalities, invariants (subject, applicability and assertion), and function requirements. Enum values carry both `enumDefinitionId` and `enumOptionId`.

Expressions are tagged semantic ASTs. V2 adds `unary`, `binary`, `variadic`, and `function_call` forms. A call stores only its namespace/name/exact semantic version and ordered arguments. Requirements store the exact typed signature. No callbacks, registries, closures, caches or process-local identity are stored. Normal model commit derives persisted requirements from calls.

## Canonical ordering

Declarations sort by stable UUID, never display name. Enum options sort by UUID and requirements by FunctionRef identity. Unordered IDs, selectors, metadata, bindings, overrides and scenarios use canonical ID/compound ordering. Ordered expression operands, call arguments and key components retain semantic order. Scalars use canonical strings; maps are never an ordering contract.

## Validation and compatibility

Input is untrusted: duplicate keys, unknown fields/nodes, invalid IDs, enums, functions, cardinality, values, dangling declarations, invalid scenarios, JSON nesting and AST depth are rejected. Documents are limited to 64 MiB.
JSON nesting is capped at 2,048 and expression depth at 512. An Enum has at
most 4,096 options, a composite key at most 64 components, and a function
signature or call at most 1,024 arguments. Unsupported newer format or engine
compatibility values fail explicitly; decoders never infer a version.

V1 retains its historical meaning: no enums, keys, invariants, or function requirements; optional input presence; and `0..unbounded` relation cardinalities. Re-encoding v1 reads is v2 and preserves semantics, not bytes.

Dataset content (including dataset-bound scenarios) is not part of `.bcg`. Runtime FunctionRegistry implementations, callbacks, sessions, caches, reports, provenance and derived indexes are not stored.
