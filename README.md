# BCG Engine

BCG Engine is a deterministic Go calculation engine for immutable business
models. A committed model is exchanged as a canonical `.bcg` JSON artifact;
the public package provides lifecycle authoring, evaluation, explanation,
dependency impact, constraints, rules, and an optional incremental evaluation
session.

Module: `github.com/shmatovv/bcg-engine`. Requires Go 1.22 or newer.
This is a private, proprietary library.

Import the consumer API as:

```go
import bcg "github.com/shmatovv/bcg-engine"
```

```go
artifact, err := bcg.Unmarshal(data)
if err != nil {
	return err
}

model := artifact.Model()
target, err := bcg.EntityProperty(entityID, propertyDefinitionID)
if err != nil {
	return err
}
calculation, err := bcg.NewCalculationContextAt(at)
if err != nil {
	return err
}

result, err := model.Evaluate(target, calculation)
if err != nil {
	return err
}
switch result.State() {
case bcg.Defined:
	value, _ := result.Value()
	// consume the typed value
case bcg.Null:
	// explicit known absence
case bcg.Undefined:
	reason, _ := result.Undefined()
	// inspect the structured reason
}
```

Use `model.NewSession(calculation)` for repeated scalar evaluations and
calculation-override what-if analysis. Defined, Null, and Undefined are normal
result states; a Go error indicates an invalid call or an engine failure.
Select an artifact's Scenario with `calculation.WithScenario(scenario)`;
evaluation validates that it belongs to the model version.

## Public model lifecycle

Create a logical model with an explicit long-lived UUID, make a Draft, then
validate and commit it with an explicit version UUID and arithmetic policy.
Only a successfully committed `Model` can be evaluated or serialized.

```go
policy, err := bcg.NewPrecisionPolicy(8, bcg.RoundHalfEven)
if err != nil {
	return err
}
logical, err := bcg.NewModel(modelID, "pricing")
if err != nil {
	return err
}
draft, err := logical.Draft()
if err != nil {
	return err
}
report, err := draft.Validate()
if err != nil {
	return err
}
if !report.OK() {
	return fmt.Errorf("draft has %d validation issues", report.Len())
}
options, err := bcg.NewCommitOptions(versionID, "v1", policy)
if err != nil {
	return err
}
committed, err := draft.Commit(options) // validates again before publishing
if err != nil {
	return err
}
artifact, err := bcg.NewArtifact(committed)
if err != nil {
	return err
}
data, err := bcg.Marshal(artifact)
```

To edit a committed version, use `draft, err := committed.Edit()`, update the
Draft (this step includes `SetName`), validate, and commit with a new version
UUID. The new Model has the source version as its parent; the source Model is
unchanged. `NewArtifact(committed, scenarios...)` accepts only Scenarios
compatible with that exact committed version.

This release establishes the lifecycle boundary. Typed schema, expression,
entity, relation, value-binding, constraint, rule, and Scenario authoring are
intentionally introduced in later v1.1 steps; callers never need wire JSON or
an `internal/...` import for the lifecycle itself.

## Authoring primitives

`ValueType` is schema metadata (`bcg.DecimalType`, `bcg.BoolType`, and the
other scalar v1 types); `Value` is an actual defined value (`DecimalValue`,
`Bool`, `Integer`, and so on). Decimal, integer, duration, and unit-scale
construction are exact and never use `float64`.

Use `NewBaseDimension` or `NewDimension` to describe generic dimensional
meaning, then `NewUnit` for one concrete symbol and exact conversion scale.
`NewComposedUnit` accepts the full canonical `.bcg v1` unit representation
when a compound unit is required. Dimensions and unit IDs are generic symbols;
BCG provides no built-in catalog of currencies, weights, lengths, or other
business units.

`TemporalScalar` and `TemporalSeries` describe property temporal mode.
`PointSeries` and `IntervalSeries` are the supported source-series axes. They
are primitives only in this release; source-series and value-binding authoring
arrive in later steps. `PrecisionPolicy` uses an explicit scale and one of
`RoundTruncate`, `RoundFloor`, `RoundCeil`, `RoundHalfUp`, or `RoundHalfEven`.

For host-process safety, schema validation and `.bcg` decoding reject ASTs
deeper than 512 nodes. Scalar and Series evaluation also bound active
expression/property nesting to 512 frames; a deeper valid dependency chain
returns an `EngineError` with kind `EvaluationDepthExceeded` instead of
exhausting the host process stack.

The portable format contract is documented in
[`docs/bcg-format-v1.md`](docs/bcg-format-v1.md).

Development checks:

```sh
go build ./...
go test ./...
go test -race ./...
go vet ./...
go test -shuffle=on -count=20 ./...
```
