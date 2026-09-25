# BCG Engine

[English](#english-reference) · [Русский](#русская-справка)

> **Runtime Dataset API.** This README documents the complete public Go API
> for the current engine, including immutable external runtime datasets. The
> `.bcg` wire-format contracts are documented in
> [`docs/bcg-format-v2.md`](docs/bcg-format-v2.md), retained historical
> [`docs/bcg-format-v1.md`](docs/bcg-format-v1.md), and
> [`docs/bcgd-format-v1.md`](docs/bcgd-format-v1.md).

<a id="english-reference"></a>

BCG Engine is a deterministic Go calculation engine for immutable business
models. A committed model is exchanged as a canonical `.bcg` JSON artifact;
the public package provides lifecycle authoring, evaluation, explanation,
dependency impact, constraints, rules, and an optional incremental evaluation
session.

Module: `github.com/shmatovv/bcg-engine/v2`. Requires Go 1.22 or newer.
This is a private, proprietary library.

v1.2.0 (`d7cc776`) remains the stable historical v1 release. This v2 module
reads `.bcg v1` and writes canonical `.bcg v2` while adding public
lossless inspection and Schema v2 declarations: UUID-identified Enums, Input
presence policies, Entity keys, directional Relation cardinality, and
subject-relative Invariants. Extension functions are represented by portable
requirements and calls. Dataset patches/rebases, evaluation dependency
planning, and batch evaluation are available. See the
[migration guide](docs/engine-v2-migration.md), [extension-function guide](docs/extension-functions.md),
[semantic-evolution guide](docs/semantic-evolution.md), [Dataset Patch/Rebase guide](docs/dataset-patch-rebase.md),
and the compilable [external public example](examples/public-v2/).

Expression v2 currently adds deterministic state predicates (`IsDefined`,
`IsUndefined`, `IsNull`), exact numeric transforms (`Abs`, `Round`, `Floor`,
`Ceil`, `Modulo`), Unicode string transforms (`Concat`, `Length`, `Lower`,
`Upper`, `Trim`, `Contains`, `StartsWith`, `EndsWith`), and date/time
component extraction. String length counts Unicode code points; casing uses
Go's Unicode default mappings and never the machine locale. New expression
nodes are immutable and participate in typed introspection, walking, static
validation, execution provenance, and dependency traversal.

Extension integrations use explicit immutable `FunctionRegistry` and
`Runtime` values. A function has exact namespace/name/semantic-version
identity and an immutable typed signature. There is no package-global
registration: missing or incompatible implementations return
`ErrIncompatibleRuntime` when a requirement is bound.

Schema v2 declarations round-trip in `.bcg v2`. Existing v1 artifacts continue
to decode with optional input presence and `0..N` cardinality defaults; normal
encoding migrates them to semantically equivalent canonical v2 bytes.

Enums retain definition and option UUIDs independently of names. An enum
property specifies `ValueType: bcg.EnumType` and its `EnumDefinitionID`.
`InputComputationWithPresence(bcg.PresenceRequired)` records the required base
binding policy; it is deliberately not applied to derived properties. Keys,
cardinality, and invariant declarations are all immutable and exposed through
the corresponding deterministic Model list APIs. Invariants are evaluated
purely for an Entity or Relation with `CheckInvariant` / the subject-specific
bulk helpers; they are distinct from global Constraints and named Rules.

Import the consumer API as:

```go
import bcg "github.com/shmatovv/bcg-engine/v2"
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

## Evaluation planning and batches

`Model.PlanEvaluation` accepts schema-level `PropertyTarget` values and returns
an immutable `DependencyPlan`. It is the upstream counterpart to
`AnalyzeImpact`: impact starts from a changed concrete source and finds
downstream effects, while planning starts from requested property definitions
and describes what may be needed to evaluate them. Plans expose requested and
dependent definitions, known embedded addresses, entity/relation types,
traversal direction plus filter/map dependencies, scope/effective-time needs,
and reachable function requirements. `PlanEvaluationWithContext` additionally
validates the selected context, but deliberately never drops a dependency based
on unknown Dataset values.

Traversal requirements are schema/topology requirements, not speculative
record IDs. A host may use them to build a minimal Dataset closure. Once that
Dataset is passed to `Model.WithDataset`, it is the complete evaluation world:
missing relations or bindings are evaluated normally and are never lazy-loaded.

`EvaluationSnapshot.EvaluateBatch(targets, context)` evaluates ordered
`PropertyAddress` targets through one shared evaluator session. It preserves
caller order and duplicate targets; every item independently remains Defined,
Null, or Undefined. Its per-target semantics are the same as independent
`Evaluate` calls under the same snapshot/runtime/context identity. Sessions
also expose `EvaluateBatch` for warm repeated calls. Caches remain bound to the
exact model semantic digest, dataset content, runtime function fingerprint, and
calculation environment.

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
Draft, validate, and commit with a new version UUID. The new Model has the
source version as its parent; the source Model is unchanged.
`NewArtifact(committed, scenarios...)` accepts only Scenarios compatible with
that exact committed version.

## Model evolution and semantic identity

`CompareModels(oldModel, newModel)` compares two committed versions of the
same logical `ModelID`. It returns deterministic `ModelChange` values keyed by
stable declaration IDs (or a compound identity for bindings/requirements), so
a rename is a modification rather than a remove/add pair. `AnalyzeEvolution`
returns per-change and aggregate `EvolutionEffect` flags such as
`RequiresRecalculation`, `RequiresIntegrityRecheck`,
`PotentialDataMigration`, and `PotentialDataLoss`.

Evolution analysis is static: it does not migrate data and does not inspect a
Dataset. In particular, `PotentialDataLoss` is a schema-risk classification,
not evidence that any concrete record will be lost. `Dataset.Rebase` is the
separate, concrete compatibility check for existing runtime content; it is not
a data migration. `model.SemanticDigest()` is SHA-256 over
a canonical semantic projection; it excludes version ID, parent version ID,
version label, and diagnostic names, while including schema, ASTs, units,
keys, cardinality, declarations, bindings, and function requirements.

## Runtime Dataset validation, patches, and rebase

`model.NewRuntimeValidator()` performs local structural validation of entity,
relation, and binding candidates. It deliberately cannot establish global
uniqueness, cardinality, or contextual invariant truth. A Dataset is the
complete evaluation world presented to the Engine; `snapshot.CheckStructuralIntegrity()` checks required base bindings, keys, and relation cardinality, while `snapshot.CheckIntegrity(context)` additionally evaluates invariants. Integrity findings are business results, not construction errors.

`DatasetPatchBuilder` produces an immutable patch bound to a base Dataset ID.
`dataset.Apply(model, newDatasetID, patch)` atomically creates a new Dataset;
the prior Dataset is never changed and remove operations do not cascade.
`dataset.Rebase(oldModel, newModel, newDatasetID)` validates the unchanged
concrete runtime content under a compatible logical model and creates a newly
bound Dataset only when its structural representation remains valid. Rebase is
not migration: it neither coerces values nor invents, removes, or rewrites data.

## Host-safety limits

The engine rejects malformed or excessive inputs before recursive evaluation or
unbounded collection allocation. The limits are safety ceilings, not business
rules: expression/evaluation/provenance depth is 512; `.bcg`/`.bcgd` decoded
document size is 64 MiB; JSON nesting is 2,048; a function call or signature
has at most 1,024 arguments; an Enum has at most 4,096 options; an Entity key
has at most 64 components; and dependency-plan traversal is capped at 4,096
frames. These values are exercised by boundary regression tests and fuzz smoke
tests. Hosts should still set ordinary request-size and execution-time limits
at their trust boundary; native extension callbacks are not sandboxed.

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
`PointSeries` and `IntervalSeries` are the supported source-series axes;
immutable source Series and persistent ValueBindings are authored through the
root package as described below. `PrecisionPolicy` uses an explicit scale and
one of `RoundTruncate`, `RoundFloor`, `RoundCeil`, `RoundHalfUp`, or
`RoundHalfEven`.

## Schema authoring

Schema declarations are immutable typed values. Add them to a mutable Draft,
then use `Validate` and `Commit`; unresolved references are allowed while a
Draft is assembled, but no invalid schema can be committed. IDs are explicit.

```go
dimension, err := bcg.NewBaseDimension("currency")
if err != nil {
	return err
}
credit, err := bcg.NewUnit("credit", dimension, "currency", bcg.OneRational())
if err != nil {
	return err
}
contract, err := bcg.NewPropertyContract(bcg.PropertyContractSpec{
	ID: contractID, Name: "price", ValueType: bcg.DecimalType,
	Dimension: &dimension, TemporalMode: bcg.TemporalScalar,
})
if err != nil {
	return err
}
trait, err := bcg.NewTrait(bcg.TraitSpec{
	ID: traitID, Name: "Priced", RequiredPropertyContracts: []bcg.ID{contractID},
})
if err != nil {
	return err
}
entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{
	ID: entityTypeID, Name: "Product", Traits: []bcg.ID{traitID},
	PropertyDefinitions: []bcg.ID{priceID},
})
if err != nil {
	return err
}
owner, err := bcg.EntityTypeOwner(entityTypeID)
if err != nil {
	return err
}
price, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
	ID: priceID, Owner: owner, ContractID: contractID, Name: "price",
	ValueType: bcg.DecimalType, Unit: credit, TemporalMode: bcg.TemporalScalar,
	Computation: bcg.InputComputation(),
})
if err != nil {
	return err
}
from, err := bcg.EntityTypeEndpoint(entityTypeID)
if err != nil {
	return err
}
to, err := bcg.TraitEndpoint(traitID)
if err != nil {
	return err
}
relationType, err := bcg.NewRelationType(bcg.RelationTypeSpec{
	ID: relationTypeID, Name: "offers", From: from, To: to,
})
if err != nil {
	return err
}
scope, err := bcg.NewScopeDefinition(bcg.ScopeDefinitionSpec{
	ID: scopeID, Name: "region", ValueType: bcg.StringType,
})
if err != nil {
	return err
}
if err := draft.AddPropertyContract(contract); err != nil { return err }
if err := draft.AddTrait(trait); err != nil { return err }
if err := draft.AddEntityType(entityType); err != nil { return err }
if err := draft.AddPropertyDefinition(price); err != nil { return err }
if err := draft.AddRelationType(relationType); err != nil { return err }
if err := draft.AddScopeDefinition(scope); err != nil { return err }
report, err := draft.Validate()
if err != nil {
	return err
}
if !report.OK() {
	return fmt.Errorf("schema validation failed: %v", report.Issues())
}
committed, err := draft.Commit(options)
```

Use `ReplaceEntityType`, `ReplaceRelationType`, `ReplacePropertyDefinition`,
`ReplacePropertyContract`, `ReplaceTrait`, and `ReplaceScopeDefinition` to
update an existing Draft while retaining identity. Their corresponding
`Remove...` operations never cascade; validation reports any dangling
references. Read complete immutable declarations back with `Model.EntityType`,
`Model.RelationType`, `Model.Trait`, `Model.PropertyContract`, and
`Model.ScopeDefinition`; `PropertyInfo.Schema()` provides the full property
declaration alongside the established compact read API.

`PropertyContractSpec.Dimension == nil` means no dimensional requirement; a
pointer to `DimensionlessDimension()` is an explicit dimensionless requirement.
For a property, zero `Unit` means absent and `DimensionlessUnit()` is explicit.
There is no built-in business unit catalog. BCG v1 schema objects do not carry
metadata or scope ordering fields.

## Expression authoring

`Expression` is an immutable typed AST value. Builders perform local shape
checks; `Draft.Validate` and `Draft.Commit` remain authoritative for property
references, units, value types, aggregate bindings, cycles, and depth.

```go
quantityRef, err := bcg.SelfProperty(quantityID)
if err != nil {
	return err
}
priceRef, err := bcg.SelfProperty(priceID)
if err != nil {
	return err
}
revenueExpr, err := bcg.Multiply(quantityRef, priceRef)
if err != nil {
	return err
}
revenueComputation, err := bcg.DerivedComputation(revenueExpr)
if err != nil {
	return err
}
revenue, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{
	ID: revenueID, Owner: productOwner, Name: "revenue",
	ValueType: bcg.DecimalType, TemporalMode: bcg.TemporalScalar,
	Computation: revenueComputation,
})
```

Constants accept defined `Value`s; BCG v1 has no Null literal expression,
because Null is a resolution state and is tested with `IsNull`.

`Add`, `Subtract`, `Multiply`, `Divide`, `Negate`, comparisons, `And`, `Or`,
`Not`, `If`, `IsNull`, and `Coalesce` compose Expressions. `AbsoluteProperty`
uses a `PropertyAddress`. `BoundProperty` and `BoundContractProperty` take
`BoundEntity` or `BoundRelation` and are valid only inside aggregate filter or
map expressions.

Aggregate sources are explicit and typed. For example, a sum over related
entities uses `SelfTraversal`, a bound contract reference, and a mapped
aggregate:

```go
source, err := bcg.SelfTraversal(bcg.TraverseOutgoing, containsRelationTypeID)
if err != nil {
	return err
}
price, err := bcg.BoundContractProperty(bcg.BoundEntity, priceContractID)
if err != nil {
	return err
}
total, err := bcg.Aggregate(source, bcg.AggregateSum, price)
```

`Count` and `FilteredCount` intentionally have no map argument, matching the
v1 evaluator's rule that Count never evaluates a map. `Aggregate` and
`FilteredAggregate` support `AggregateSum`, `AggregateAverage`, `AggregateMin`,
and `AggregateMax`. `SelfTraversal`, `AbsoluteEntityTraversal`, and
`BoundEntityTraversal` represent the three v1 origins; `TraverseOutgoing`,
`TraverseIncoming`, and `TraverseBoth` select direction.

Derived expressions are exposed losslessly through
`PropertyInfo.Schema().Computation().Expression()`, so `Model.Edit()` retains
them even when unrelated schema is changed. Expressions over 512 nodes may be
assembled temporarily, but Validate and Commit reject them using the shared v1
depth limit.

### Expression inspection and traversal

Every decoded or authored current expression node is publicly inspectable.
`Kind`, `IsValid`, `Depth`, and `Children` provide stable tree shape; typed
accessors expose constants (`ConstantValue`), property references
(`PropertyReference`), unary/binary operations (`UnaryOperation`,
`BinaryOperation`), control expressions (`ConditionalOperation`), and all
aggregate data (`AggregateOperation`, including its `TraversalSource`, filter,
and map). Returned child collections are caller-owned copies.

`WalkExpression(expr, visitor)` performs deterministic pre-order traversal.
Use `ExpressionVisitorFunc` for a concise visitor. The inspection data is
sufficient to recreate a semantically equivalent expression with the public
builders; it does not expose mutable internal AST nodes.

## Semantic data, bindings, and declarations

`Entity` and `Relation` are immutable semantic declarations built with
`NewEntity(EntitySpec{...})` and `NewRelation(RelationSpec{...})`. Add, replace,
or remove them through a Draft. Relation applicability is an optional public
`Expression`; endpoint and expression semantics remain validated by the model.

Persistent values are separate immutable `ValueBinding` declarations. They
target either a concrete property or a type-level default, and use a real
compound `ValueBindingKey` (target, scopes, effective period) for replacement
or removal—there is no invented binding UUID.

```go
quantityTarget, err := bcg.ObjectPropertyBindingTarget(quantityAddress)
if err != nil { return err }
quantityValue, err := bcg.DefinedBindingValue(bcg.DecimalValue(bcg.MustParseDecimal("100")))
if err != nil { return err }
quantity, err := bcg.NewValueBinding(bcg.ValueBindingSpec{
	Target: quantityTarget, Value: quantityValue,
})
if err != nil { return err }
if err := draft.AddValueBinding(quantity); err != nil { return err }
```

`NullBindingValue()` is explicit known absence, distinct from a missing
binding and runtime Undefined. `ScopeSelector` values and optional
`NewEffectivePeriod(from, to)` selectors use the same established resolution
semantics as runtime calculations. A nil effective-period pointer means no
time selector; a non-nil period may have open bounds.

For a Series input, construct immutable source data, not a runtime
`SeriesResult`:

```go
sample, err := bcg.DefinedSourceSample(bcg.DecimalValue(bcg.MustParseDecimal("5")))
if err != nil { return err }
point, err := bcg.NewSourcePoint(at, sample)
if err != nil { return err }
series, err := bcg.NewPointSourceSeries(bcg.DecimalType, nil, []bcg.SourcePoint{point})
if err != nil { return err }
seriesValue, err := bcg.SeriesBindingValue(series)
```

`NewIntervalSourceSeries` supports ordered, non-overlapping half-open
intervals. Source samples may be Defined or `NullSourceSample()` but never
Undefined. Constructors copy their input slices.

Constraints and Rules remain declarative expressions evaluated by the existing
engine; they introduce no actions or side effects:

```go
constraint, err := bcg.NewConstraint(bcg.ConstraintSpec{
	ID: constraintID, Name: "revenue-nonnegative",
	Assertion: revenueAtLeastZero, Severity: bcg.SeverityHard,
})
rule, err := bcg.NewRule(bcg.RuleSpec{
	ID: ruleID, Name: "large-revenue", Condition: revenueOverThreshold,
})
if err != nil { return err }
if err := draft.AddConstraint(constraint); err != nil { return err }
if err := draft.AddRule(rule); err != nil { return err }
```

After committing, construct a Scenario against that exact Model version. A
Scenario is immutable and carries value-only `ScenarioOverride`s; it may
override an Input or Derived property, including a source Series, but never
schema or semantic structure.

```go
override, err := bcg.NewScenarioOverride(bcg.ScenarioOverrideSpec{
	Target: priceAddress, Value: priceAt300,
})
if err != nil { return err }
scenario, err := bcg.NewScenario(model, bcg.ScenarioSpec{
	ID: scenarioID, Name: "higher-price", Overrides: []bcg.ScenarioOverride{override},
})
if err != nil { return err }
artifact, err := bcg.NewArtifact(model, scenario)
```

Use `Model.Edit()` to obtain independent copies of entities, relations,
bindings, constraints, and rules. `ReplaceValueBinding` uses the original
binding's `Key`; `Remove...` operations never cascade and the existing
validation report identifies dangling references. `Scenario.WithOverrides`
returns a new immutable Scenario and rejects a foreign base version.

An aggregate over related entities is assembled with the same expression
builders: use `SelfTraversal`, `BoundProperty(BoundEntity, childValueID)`, and
`Aggregate` or `FilteredAggregate`. This retains the existing traversal,
lexical-binding, Explain, and Impact semantics.

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

## Evaluation API and result contract

Every evaluation entry point takes a `PropertyAddress` and a
`CalculationContext`. `EntityProperty(entityID, definitionID)` and
`RelationProperty(relationID, definitionID)` construct the two address forms.
An address identifies a concrete object property; it is not a schema property
definition by itself.

`CalculationContext` supplies the temporal coordinate or requested period,
scope selections, an optional `Scenario`, and temporary calculation
overrides. Create it with `NewCalculationContextAt` for a scalar instant or
`NewCalculationContextPeriod` for a requested interval, then derive a new
immutable context with:

- `WithScopes(selectors...)`, where each selector comes from
  `NewScopeSelector(scopeDefinitionID, value)`;
- `WithScenario(scenario)` or `WithoutScenario()`;
- `WithCalculationOverrides(overrides...)`, where `NewOverride` and
  `NullOverride` create the temporary values.

Context values are immutable. An override is not a persisted binding and is
never written into a `Model`, `Dataset`, or `.bcg` artifact. At a concrete
property target the effective precedence is always:

```text
CalculationOverride > ScenarioOverride > object ValueBinding
                    > type ValueBinding > missing / Undefined
```

The common scalar APIs are intentionally parallel on `Model`,
`EvaluationSnapshot`, and `Session` (a Session stores its context, so its
methods omit that argument):

| Need | Stateless Model | Model + Dataset snapshot | Reusable Session |
| --- | --- | --- | --- |
| One scalar value | `Evaluate(target, context)` | `snapshot.Evaluate(target, context)` | `session.Evaluate(target)` |
| Several scalar values | `EvaluateMany(targets, context)` | `snapshot.EvaluateMany(targets, context)` | `session.EvaluateMany(targets)` |
| A series | `EvaluateSeries(target, context)` | `snapshot.EvaluateSeries(target, context)` | `session.EvaluateSeries(target)` |
| Scalar provenance | `Explain(target, context)` | `snapshot.Explain(target, context)` | `session.Explain(target)` |
| Series provenance | `ExplainSeries(target, context)` | `snapshot.ExplainSeries(target, context)` | — |
| Downstream impact | `AnalyzeImpact(mode, target, context)` | `snapshot.AnalyzeImpact(...)` | `session.AnalyzeImpact(mode, target)` |

`Result` is a normal business outcome, not an error channel. Always branch on
`State()`:

| State | Meaning | Read it with |
| --- | --- | --- |
| `Defined` | A typed value is available. | `Value() (Value, bool)` |
| `Null` | An applicable nullable binding explicitly states known absence. | The state itself |
| `Undefined` | No usable value or an evaluable business condition prevented a value. | `Undefined() (UndefinedInfo, bool)` |

`UndefinedInfo.Kind()` distinguishes stable causes: `MissingValue`,
`AmbiguousValue`, `DependencyUndefined`, `EmptyAggregation`,
`IndeterminateCondition`, `InvalidOperation`, `ExternalUnavailable`, and
`DivisionByZero`. A non-nil Go error means a malformed call, invalid context,
or engine failure; it is not the representation for a missing business value.

`SeriesResult` is distinct from authored `SourceSeries`. A defined result has
either `PointSeries` values through `Points()` or `IntervalSeries` values
through `Intervals()`; a whole-series failure is exposed by `IsUndefined()`
and `Undefined()`. `SeriesPoint` and `SeriesInterval` each retain a scalar
`Result`, so individual samples can be Defined, Null, or Undefined.

### Explain and impact

`Explain` and `ExplainSeries` return actual execution provenance, not a
reconstructed AST. Start at `Explanation.Root()` and walk ordered
`ExplanationNode.Children()`. A node can expose its property, operation,
decision, result state/value/undefined reason, applied calculation override,
Scenario ID, or aggregate item (`relationID`, `relatedEntityID`) where
applicable. Duplicate expression occurrences are intentionally retained.

`AnalyzeImpact` produces a deterministic downstream report. Pick one mode:

- `StructuralImpact` uses all statically possible dependency edges.
- `EffectiveImpact` uses the selected time, scopes, Scenario, and override
  layer to prune dependencies that cannot currently apply.
- `ObservedImpact` uses dependencies actually observed while evaluating.

`ImpactResult` includes the exact `ModelVersionID`, the optional `DatasetID`,
the source address, ordered `Affected()` items with minimum depth, and direct
`Edges()`. `ImpactAffected.IsDirect()` is shorthand for a minimum depth of 1.

### Constraints, feasibility, and rules

Constraints and Rules are declarations owned by the committed Model. They can
read Dataset data when evaluated through a snapshot, but a Dataset does not
define new constraints, rules, or quantified "for every object" declarations.

| API | Result |
| --- | --- |
| `CheckConstraint(id, context)` | One `ConstraintResult` |
| `CheckConstraints(context)` | Deterministic `ConstraintReport` |
| `CheckFeasibility(context)` | `FeasibilityResult` over all Hard constraints |
| `EvaluateRule(id, context)` | One `RuleResult` |
| `EvaluateRules(context)` | Deterministic `RuleReport` |

The same methods are available on `EvaluationSnapshot`; `Session` provides
`CheckConstraints`, `CheckFeasibility`, `EvaluateRule`, and `EvaluateRules`
without an explicit context. Constraint states are `Satisfied`, `Violated`,
`NotApplicable`, and `Undefined`; `ConstraintResult` retains only the
applicability/assertion evaluations that ran. `FeasibilityResult.State()` is
`Feasible`, `Infeasible`, or `Undetermined` and considers only constraints
with `SeverityHard`. Rules are pure named Boolean decisions with state
`RuleActive`, `RuleInactive`, or `RuleUndefined`; they have no side effects.

## External immutable runtime datasets

Use a Dataset when the model supplies schema and calculation semantics while
concrete operational objects and their base values are supplied at runtime.
The model remains unchanged when external data changes:

```text
committed Model version + immutable Dataset = EvaluationSnapshot
```

This is deliberately not lazy loading. The embedded model data plus the
Dataset form one complete evaluation graph: a relation absent from both is
absent, and a missing object binding falls back to a model type binding or to
`Undefined` in the ordinary way.

### Dataset lifecycle

`NewDatasetBuilder(baseModel, datasetID)` requires a committed `Model` and a
non-zero caller-supplied snapshot ID. The ID identifies one immutable content
snapshot; BCG does not maintain a global ID registry, so callers **must never
reuse one Dataset ID for changed content**.

```go
builder, err := bcg.NewDatasetBuilder(model, datasetID)
if err != nil {
	return err
}

order, err := bcg.NewEntity(bcg.EntitySpec{ID: orderID, TypeID: orderTypeID})
if err != nil {
	return err
}
if err := builder.AddEntity(order); err != nil {
	return err
}

quantityAddress, err := bcg.EntityProperty(orderID, quantityDefinitionID)
if err != nil {
	return err
}
quantityTarget, err := bcg.ObjectPropertyBindingTarget(quantityAddress)
if err != nil {
	return err
}
quantityValue, err := bcg.DefinedBindingValue(
	bcg.DecimalValue(bcg.MustParseDecimal("10")),
)
if err != nil {
	return err
}
quantity, err := bcg.NewValueBinding(bcg.ValueBindingSpec{
	Target: quantityTarget, Value: quantityValue,
})
if err != nil {
	return err
}
if err := builder.AddValueBinding(quantity); err != nil {
	return err
}

// Add the price binding in the same way, then inspect every issue before build.
report, err := builder.Validate()
if err != nil {
	return err
}
if !report.OK() {
	return fmt.Errorf("dataset invalid: %#v", report.Issues())
}
dataset, err := builder.Build()
if err != nil {
	return err // a ValidationError carries the deterministic issues
}

snapshot, err := model.WithDataset(dataset)
if err != nil {
	return err
}
totalAddress, err := bcg.EntityProperty(orderID, totalDefinitionID)
if err != nil {
	return err
}
total, err := snapshot.Evaluate(totalAddress, calculation)
// With total = quantity * price and price = 500, total is Decimal(5000).
```

`DatasetBuilder` is mutable and not safe for concurrent use. A successful
`Build` seals it. `Dataset` is immutable and safe for concurrent reads; its
`Entities`, `Relations`, and `ValueBindings` methods return deterministic
defensive slices, while `Entity(id)` and `Relation(id)` provide indexed
lookups. `ID`, `ModelVersionID`, and `IsValid` identify/inspect the snapshot.

The builder accepts only existing public `Entity`, `Relation`, and
object-property `ValueBinding` values. Validation uses the same report and
issue machinery as a draft, and rejects:

- zero/duplicate IDs, collisions with embedded model entities or relations,
  and unknown entity/relation types;
- absent endpoints or endpoint types/traits incompatible with the
  `RelationType`;
- type-level bindings, bindings to embedded model objects, unknown or
  mismatched properties, and base bindings for Derived properties;
- scalar/series, value type, unit, nullability, scope, period, and selector
  mismatches, plus conflicting object-binding candidates;
- a Dataset relation whose two endpoints are both embedded model entities.

Dataset relations may connect Dataset-to-Dataset, Dataset-to-embedded, or
embedded-to-Dataset entities. They participate in forward, reverse, and
`TraverseBoth` relation traversal, aggregate filter/map execution, Explain,
Impact, constraints, and rules. Relation property bindings are allowed when
the relation itself is Dataset-owned. A Dataset can never override an embedded
model object's property; use a Scenario or a calculation override for a
temporary alternative.

### EvaluationSnapshot and source identity

`Model.WithDataset(dataset)` requires `dataset.ModelVersionID()` to equal the
**exact** `Model.VersionID()`. It builds and retains the combined dependency
graph once and rejects a runtime topology that introduces a dependency cycle.
It does not clone the entire Model or Dataset.

`EvaluationSnapshot` is an immutable façade with `Model()`, `Dataset()`,
`ModelVersionID()`, `DatasetID()`, and `IsValid()`, plus the full stateless
evaluation API listed above. Do not call `snapshot.Model().Evaluate(...)` if
you mean to use external data: that deliberately executes the model-only
world. Call the matching method on the snapshot.

Internally, reusable runtime work is tied to an effective source identity:

```text
model-only source:       ModelVersionID
dataset-backed source:   ModelVersionID + DatasetID
```

Consequently a plan, observed impact result, or Session from Dataset A cannot
be used for Dataset B even when both refer to the same Model version and have
coincidentally identical object IDs. A newly built Dataset always requires a
new snapshot and, for cached evaluation, a new Session.

### Dataset-aware Scenarios and sessions

`NewScenario(model, spec)` continues to create portable model-only scenarios.
`snapshot.NewScenario(spec)` is the authoring entry point when any override
targets a Dataset-owned object. A Scenario with only model-owned targets is
still model-only/portable even if it was created through a snapshot; a mixed
scenario becomes Dataset-bound. Inspect `Scenario.BaseVersionID()` and the
optional `BaseDatasetID()`.

Dataset-bound Scenarios are runtime-only: they must be evaluated with the
same Dataset ID, cannot be used with another Dataset that happens to reuse an
object ID, and `NewArtifact` rejects them. `.bcg` remains a portable
self-contained Model plus model-only Scenario artifact; Dataset bytes are not
serialized.

`Model.NewSession(context)` makes a model-only Session;
`snapshot.NewSession(context)` binds one to the snapshot's fixed Dataset.
`Session.Dataset()` reports the optional Dataset. All Session methods are safe
for concurrent use, but the Dataset cannot be replaced. `SetCalculationContext`
changes time/scopes/scenario/overrides, `SetCalculationOverrides` replaces only
temporary overrides, `Reset` drops the cache generation, and `CacheStats`
returns read-only hits/misses/invalidations/entries diagnostics.

## Public API reference

This index names every exported public concept. Constructors return immutable
value objects unless their name says `Builder`, `Draft`, or `Session`.
Collection accessors documented as owned return a copy, so callers may safely
reorder or append to it.

### Identities, values, dimensions, and units

| Area | API | Notes |
| --- | --- | --- |
| IDs | `NewID`, `ParseID`, `MustParseID`, `ID.String`, `IsZero`, `Equal` | IDs are explicit UUID value objects. Avoid `MustParseID` for untrusted input. |
| Schema value kinds | `ValueType`: `BoolType`, `IntegerType`, `DecimalType`, `StringType`, `DateType`, `DateTimeType`, `DurationType` | Use `IsValid`/`String` for validation and display. |
| Temporal schema mode | `TemporalScalar`, `TemporalSeries` | A property is scalar or series; authored source and runtime result types are separate. |
| Scalar values | `Bool`, `Integer`, `Int64`, `DecimalValue`, `String`, `DateValue`, `DateTimeValue`, `DurationValue` | `Value.Type`, `Kind`, typed accessor methods, `Unit`, `WithUnit`, and `IsValid` inspect a value. |
| Exact primitives | `ParseDecimal`, `ParseDate`, `ParseDateTime`, `ParseDuration` and `MustParseDecimal` | Decimal and conversion arithmetic avoids `float64`. |
| Dimensions | `NewBaseDimension`, `NewDimension`, `DimensionlessDimension` | `DimensionFactor` uses an integer exponent; inspect with `Factors`, `Equal`, `IsDimensionless`. |
| Rational scales | `NewRational`, `RationalFromInt64`, `OneRational` | Numerator/denominator are strings to preserve exact arbitrary precision. |
| Units | `NewUnit`, `NewComposedUnit`, `DimensionlessUnit` | Inspect with `Dimension`, `Scale`, `Factors`, `ConversionFactors`, `CanConvertTo`, `Equal`. |

`NewUnit(id, dimension, symbol, scale)` makes a simple named unit.
`NewComposedUnit(factors, conversionSpace, conversionFactors, scale, symbol)`
creates the complete canonical compound representation. A zero unit is absent;
`DimensionlessUnit()` is an explicit dimensionless unit. `UnitFactor` and
`ConversionFactor` are value structs validated by `IsValid`.

### Schema declarations

| Declaration | Construct with | Key fields and accessors |
| --- | --- | --- |
| Entity class | `NewEntityType(EntityTypeSpec)` | `ID`, `Name`, required `Traits`, available `PropertyDefinitions` |
| Relation class | `NewRelationType(RelationTypeSpec)` | `ID`, `Name`, optional `From`/`To` endpoint constraints, property definitions |
| Property ownership | `EntityTypeOwner`, `RelationTypeOwner` | `PropertyOwner` supplies `Kind`, `ID`, `IsValid` |
| Property | `NewPropertyDefinition(PropertyDefinitionSpec)` | owner, optional contract, name, `ValueType`, optional `Unit`, `TemporalMode`, `Computation` |
| Contract | `NewPropertyContract(PropertyContractSpec)` | name/type, optional dimension, temporal mode, nullable flag |
| Trait | `NewTrait(TraitSpec)` | name and required property-contract IDs |
| Scope dimension | `NewScopeDefinition(ScopeDefinitionSpec)` | name and selector value type |

`PropertyComputation` is made with `InputComputation()` or
`DerivedComputation(expression)` and exposes `Kind()` and optional
`Expression()`. Relation endpoints can be omitted, constrained with
`EntityTypeEndpoint` or `TraitEndpoint`, or explicitly unrestricted with
`AnyEntityEndpoint`; inspect `EndpointConstraint.Kind`, `ID`, `IsPresent`, and
`IsValid`. The zero endpoint means *omitted*, not `AnyEntity`.

`EntityTypeSpec`, `RelationTypeSpec`, `PropertyDefinitionSpec`,
`PropertyContractSpec`, `TraitSpec`, and `ScopeDefinitionSpec` carry IDs and
the corresponding fields above. Their constructors perform local structural
checks; cross-references, owner compatibility, expression typing, and model
invariants are checked by `Draft.Validate`/`Commit`.

### Expressions and traversals

`Expression` is opaque/immutable. Build literals and references with
`Constant`, `SelfProperty`, `AbsoluteProperty`, `BoundProperty`, and
`BoundContractProperty`; inspect only `IsValid` and `Depth`.

| Expression family | Constructors |
| --- | --- |
| Arithmetic | `Add`, `Subtract`, `Multiply`, `Divide`, `Negate` |
| Comparison | `Equal`, `NotEqual`, `Less`, `LessOrEqual`, `Greater`, `GreaterOrEqual` |
| Boolean/control | `And`, `Or`, `Not`, `If`, `IsNull`, `Coalesce` |
| Traversal sources | `SelfTraversal`, `AbsoluteEntityTraversal`, `BoundEntityTraversal` |
| Aggregation | `Count`, `FilteredCount`, `Aggregate`, `FilteredAggregate` |

`AggregateBinding` is `BoundEntity` or `BoundRelation`; bound references are
legal only in aggregate map/filter lexical scope. `TraversalDirection` is
`TraverseOutgoing`, `TraverseIncoming`, or `TraverseBoth`; `TraversalSource`
also records `TraversalOrigin` (`SelfTraversalOrigin`,
`AbsoluteEntityTraversalOrigin`, `BoundEntityTraversalOrigin`) and the
`RelationTypeID`. `AggregateOperation` is `AggregateSum`, `AggregateAverage`,
`AggregateMin`, or `AggregateMax`; `RequiresMap` is false for Count. All
cross-reference and aggregate typing checks remain authoritative at model
validation time.

### Semantic objects and persistent data

| Object | Construct with | Important behavior |
| --- | --- | --- |
| Entity | `NewEntity(EntitySpec)` | Explicit `ID`, `TypeID`, optional name. Values are separate bindings. |
| Relation | `NewRelation(RelationSpec)` | Directed `From` → `To`, type, optional Boolean applicability expression. |
| Binding target | `ObjectPropertyBindingTarget`, `TypePropertyBindingTarget` | Object target vs model type default. Inspect `Kind`, `ObjectProperty`, `TypeProperty`. |
| Binding value | `DefinedBindingValue`, `NullBindingValue`, `SeriesBindingValue` | Defined scalar, explicit null, or immutable source series. |
| Binding | `NewValueBinding(ValueBindingSpec)` | Target + binding value + canonical scopes + optional effective period. |
| Key | `ValueBinding.Key()` | A binding has no independent UUID; target/scopes/period form its identity. |

`EffectivePeriod` is half-open `[from, to)` with optional bounds; make it with
`NewEffectivePeriod`. A nil period pointer in `ValueBindingSpec`/override spec
means no time selector. Scope selector order is canonicalized, and
`ValueBindingKey.Equal` compares the canonical compound identity.

For source series, use `DefinedSourceSample`/`NullSourceSample`, then
`NewSourcePoint` or `NewSourceInterval`, and finally
`NewPointSourceSeries` or `NewIntervalSourceSeries`. Point coordinates must
strictly increase. Intervals must be ordered, non-overlapping half-open
ranges; adjacent intervals are valid. `SourceSeries` owns its input, exposes
axis/type/unit and ordered copied samples, and is not a `SeriesResult`.

### Model lifecycle, read API, and draft mutation

| Lifecycle | API | Contract |
| --- | --- | --- |
| Logical identity | `NewModel(id, name)` | Creates a non-evaluatable logical Model. |
| Editable candidate | `Model.Draft` / `Model.NewDraft` | Empty for a new model; independent copy for a committed one. |
| Edit committed content | `Model.Edit` | Requires a committed model and records its parent version. |
| Check candidate | `Draft.Validate` | Returns all deterministic `ValidationIssue`s without mutation. |
| Publish immutable version | `NewCommitOptions`, `Draft.Commit` | Explicit new version ID and decimal precision policy are required. |

Draft additions/replacements/removals follow a regular pattern:

- schema: `Add`, `Replace`, `Remove` for `EntityType`, `RelationType`,
  `PropertyDefinition`, `PropertyContract`, `Trait`, and `ScopeDefinition`;
- graph data: `AddEntity`, `ReplaceEntity`, `RemoveEntity`, and equivalent
  relation methods;
- bindings: `AddValueBinding`, `SetValueBinding`, `ReplaceValueBinding(key, value)`,
  `RemoveValueBinding(key)`;
- declarations: `AddConstraint`/`ReplaceConstraint`/`RemoveConstraint` and
  `AddRule`/`ReplaceRule`/`RemoveRule`.

Removal never cascades. Assemble temporary dangling references if needed, then
use validation before committing. Draft list accessors (`Entities`, `Relations`,
`ValueBindings`, `Constraints`, `Rules`) return deterministic owned values.

Committed `Model` offers read-only deterministic lists and direct lookups:
`Entities`/`Entity`, `Relations`/`Relation`, `ValueBindings`, `EntityTypes`/
`EntityType`, `RelationTypes`/`RelationType`, `Traits`/`Trait`,
`PropertyContracts`/`PropertyContract`, `ScopeDefinitions`/`ScopeDefinition`,
`PropertyDefinitions`/`PropertyDefinition`, `Constraints`/`Constraint`, and
`Rules`/`Rule`. `EntityInfo`, `RelationInfo`, `NamedInfo`, `PropertyInfo`, and
`ScopeDefinitionInfo` provide compact inspection; `Declaration`, `Schema`,
and `Definition` return the full immutable declaration where relevant.
For bulk lossless inspection, use `EntityTypeDefinitions`,
`RelationTypeDefinitions`, `TraitDefinitions`, `PropertyContractDefinitions`,
`ScopeDefinitionDeclarations`, and `PropertyDefinitionDeclarations`. Entities,
relations, bindings, constraints, rules, scenarios, source series, selectors,
and effective periods already return their complete immutable public values.

### Artifacts, scenarios, errors, and zero values

`NewArtifact(model, scenarios...)`, `Encode`, and `Marshal` write canonical
deterministic `.bcg` JSON. `Decode` and `Unmarshal` read it. `Artifact.Model`,
`Scenario`, and `Scenarios` provide immutable readback. `FormatVersion` and
`EngineCompatibility` name the supported portable contract. The current wire
format intentionally excludes Datasets and Dataset-bound Scenarios.

Check public errors with `errors.Is`/`errors.As`:

- `ErrInvalidArgument` and `ErrInvalidModel` classify bad public calls/wrappers;
- artifact decoding may return `ErrMalformedArtifact`, `ErrUnsupportedFormat`,
  `ErrIncompatibleEngine`, `ErrWireSemantic`, `ErrModelValidation`, or
  `ErrScenarioValidation`;
- `*ValidationError` retains deterministic `Issues`; `ValidationReport` is
  the non-error inspection form;
- `*EngineError.Kind()` exposes a stable operation failure kind.

Most public value wrappers have an intentionally invalid zero value. Prefer
their constructors, use `IsValid` before retaining optional values, and treat
the `bool` returned by optional accessors as authoritative. `Draft` and
`DatasetBuilder` are mutable and not concurrent. Committed `Model`,
declarations, `Dataset`, `EvaluationSnapshot`, results, explanations, and
artifact values are immutable. A `Session` is concurrent-safe but remains
permanently source-bound.

---

<a id="русская-справка"></a>

# Русская справка

## Что такое BCG Engine

BCG Engine — детерминированный Go-движок расчётов для неизменяемых
бизнес-моделей. Коммит модели создаёт конкретную версию, которую можно
вычислять, объяснять, анализировать на влияние, проверять ограничениями и
правилами, кэшировать в `Session` и переносить в каноническом JSON-артефакте
`.bcg`. Пакет для приложения импортируется так:

```go
import bcg "github.com/shmatovv/bcg-engine/v2"
```

Требуется Go 1.22 или новее. Формат `.bcg v1` описан отдельно в
[`docs/bcg-format-v1.md`](docs/bcg-format-v1.md). README описывает публичный
Go API, в том числе runtime Dataset; Dataset намеренно не является частью
wire format.

## Ментальная модель и неизменяемость

```text
логическая Model
    │ Draft (изменяемый, может быть невалиден)
    ▼
committed ModelVersion (immutable: schema + semantics + optional embedded data)
    │                         + immutable Dataset (optional external objects/data)
    ▼
EvaluationSnapshot
    ├── Evaluate / EvaluateMany / EvaluateSeries
    ├── Explain / Impact
    ├── Constraints / Rules
    └── Session
```

`Model` после `Commit`, schema-значения, `Entity`, `Relation`,
`ValueBinding`, `Scenario`, `Dataset`, `EvaluationSnapshot`, результаты и
артефакт неизменяемы. `Draft` и `DatasetBuilder` — единственные строительные
изменяемые объекты; оба не предназначены для одновременной записи из разных
goroutine. Методы, возвращающие коллекции, возвращают принадлежащие
вызывающему коды срезов/карт, поэтому их можно изменять без изменения Engine.

Нулевые значения большинства публичных wrapper-типов невалидны. Создавайте
значения конструкторами, используйте `IsValid`, а для опциональных значений
всегда учитывайте возвращаемый `bool`.

## Быстрый путь: Model → Draft → Commit → Evaluate

1. Создайте стабильный UUID модели: `NewModel(modelID, name)`.
2. Вызовите `Draft()` (или `NewDraft()`) и добавьте schema, semantic objects,
   bindings, constraints и rules.
3. Выполните `report, err := draft.Validate()`. `err` означает неправильное
   использование публичной оболочки; `report.OK() == false` означает набор
   доменных проблем.
4. Создайте `PrecisionPolicy` через `NewPrecisionPolicy(scale, rounding)` и
   `CommitOptions` через `NewCommitOptions(versionID, label, policy)`.
5. `draft.Commit(options)` публикует новую неизменяемую `Model` и не меняет
   сам Draft. ID версии всегда задаёт вызывающий код; при редактировании
   committed version новый ID обязан отличаться от parent version ID.
6. Постройте `PropertyAddress` и `CalculationContext`, затем вызовите
   `model.Evaluate(address, context)`.

Для редактирования зафиксированной версии используйте `model.Edit()`. Он
создаёт независимый Draft, сохраняет `ModelID`, а новая версия получает
`ParentVersionID` исходной. Исходная версия никогда не меняется.

## ID, точные значения, размерности и единицы

| Область | Основной API | Семантика |
| --- | --- | --- |
| UUID | `NewID`, `ParseID`, `MustParseID`; `String`, `IsZero`, `Equal` | Все идентичности явные. `MustParseID` подходит только для констант/доверенного ввода. |
| Тип schema-значения | `BoolType`, `IntegerType`, `DecimalType`, `StringType`, `DateType`, `DateTimeType`, `DurationType` | `ValueType` описывает контракт property, а не конкретное значение. |
| Время property | `TemporalScalar`, `TemporalSeries` | Scalar и Series — разные режимы schema. |
| Значения | `Bool`, `Integer`, `Int64`, `DecimalValue`, `String`, `DateValue`, `DateTimeValue`, `DurationValue` | У `Value` есть `Type`, `Kind`, типизированные getters, `Unit`, `WithUnit`, `IsValid`. |
| Точные литералы | `ParseDecimal`, `ParseDate`, `ParseDateTime`, `ParseDuration`, `MustParseDecimal` | Для decimal и unit scale никогда не используется `float64`. |
| Размерности | `NewBaseDimension`, `NewDimension`, `DimensionlessDimension` | `DimensionFactor` содержит ID и целый показатель; есть `Factors`, `Equal`, `IsDimensionless`. |
| Рациональные масштабы | `NewRational`, `RationalFromInt64`, `OneRational` | Числитель и знаменатель строковые, точные и без ограничения `int64`. |
| Units | `NewUnit`, `NewComposedUnit`, `DimensionlessUnit` | `Dimension`, `Scale`, `Factors`, `ConversionFactors`, `CanConvertTo`, `Equal` читают unit. |

`NewUnit` создаёт простой unit из ID, размерности, символа и точного масштаба.
`NewComposedUnit` принимает каноническое составное представление.
Нулевой `Unit` означает «unit отсутствует», а `DimensionlessUnit()` —
явно заданный безразмерный unit. В BCG нет встроенного каталога валют,
килограммов или иных предметных единиц: их идентичности задаёт приложение.

## Schema API

Schema описывает классы и вычислительную семантику, а не конкретные runtime
записи.

| Сущность schema | Конструктор | Ключевые поля/API |
| --- | --- | --- |
| Класс entity | `NewEntityType(EntityTypeSpec)` | `ID`, `Name`, `Traits`, `PropertyDefinitions` |
| Класс relation | `NewRelationType(RelationTypeSpec)` | `ID`, `Name`, optional `From`/`To`, property definitions |
| Владелец property | `EntityTypeOwner`, `RelationTypeOwner` | `PropertyOwner.Kind`, `ID`, `IsValid` |
| Property | `NewPropertyDefinition(PropertyDefinitionSpec)` | owner, contract, name, value type, unit, temporal mode, computation |
| Контракт property | `NewPropertyContract(PropertyContractSpec)` | type, dimension, temporal mode, nullable |
| Trait | `NewTrait(TraitSpec)` | required property-contract IDs |
| Scope | `NewScopeDefinition(ScopeDefinitionSpec)` | имя scope и тип его selector value |

`InputComputation()` создаёт input-property, `DerivedComputation(expression)`
— derived-property. У `PropertyComputation` доступны `Kind` и, для derived,
`Expression`. Ограничение endpoint relation создают `EntityTypeEndpoint`,
`TraitEndpoint` или `AnyEntityEndpoint`. Нулевой `EndpointConstraint` означает
«ограничение не задано», а `AnyEntityEndpoint()` — явное «подходит любой
Entity»; различайте их через `IsPresent`.

Конструкторы проверяют локальную форму. Существование ссылок, совместимость
owner/contract, типы выражений, единицы, циклы и прочие межобъектные правила
проверяются только авторитетным `Draft.Validate`/`Commit`.

Для изменения Draft доступны согласованные методы:

- `Add`/`Replace`/`Remove` для `EntityType`, `RelationType`,
  `PropertyDefinition`, `PropertyContract`, `Trait`, `ScopeDefinition`;
- `AddEntity`, `ReplaceEntity`, `RemoveEntity` и соответствующие методы для
  relation;
- `AddValueBinding`, `SetValueBinding`, `ReplaceValueBinding(key, value)`,
  `RemoveValueBinding(key)`;
- `AddConstraint`, `ReplaceConstraint`, `RemoveConstraint`; `AddRule`,
  `ReplaceRule`, `RemoveRule`.

Удаление никогда не каскадирует. Можно временно собрать разорванный Draft,
но до `Commit` все проблемы будут выданы в `ValidationReport`.

Committed `Model` предоставляет детерминированные списки и lookup:
`Entities`/`Entity`, `Relations`/`Relation`, `ValueBindings`,
`EntityTypes`/`EntityType`, `RelationTypes`/`Relation`, `Traits`/`Trait`,
`PropertyContracts`/`PropertyContract`, `ScopeDefinitions`/`ScopeDefinition`,
`PropertyDefinitions`/`PropertyDefinition`, `Constraints`/`Constraint`,
`Rules`/`Rule`. Компактные `EntityInfo`, `RelationInfo`, `NamedInfo`,
`PropertyInfo`, `ScopeDefinitionInfo` содержат часто нужные поля; методы
`Declaration`, `Schema`, `Definition` возвращают полный immutable объект.

## Выражения, relation traversal и aggregates

`Expression` — непрозрачное неизменяемое typed AST значение. Для него есть
локальные строители:

| Семейство | Функции |
| --- | --- |
| Константы/ссылки | `Constant`, `SelfProperty`, `AbsoluteProperty`, `BoundProperty`, `BoundContractProperty` |
| Арифметика | `Add`, `Subtract`, `Multiply`, `Divide`, `Negate` |
| Сравнение | `Equal`, `NotEqual`, `Less`, `LessOrEqual`, `Greater`, `GreaterOrEqual` |
| Логика | `And`, `Or`, `Not`, `If`, `IsNull`, `Coalesce` |
| Источник traversal | `SelfTraversal`, `AbsoluteEntityTraversal`, `BoundEntityTraversal` |
| Aggregate | `Count`, `FilteredCount`, `Aggregate`, `FilteredAggregate` |

`TraversalDirection` принимает `TraverseOutgoing`, `TraverseIncoming` или
`TraverseBoth`. `TraversalSource` хранит origin (`SelfTraversalOrigin`,
`AbsoluteEntityTraversalOrigin`, `BoundEntityTraversalOrigin`), направление и
`RelationTypeID`. В aggregate map/filter-выражениях доступны lexical bindings
`BoundEntity` и `BoundRelation`. Операции aggregate: `AggregateSum`,
`AggregateAverage`, `AggregateMin`, `AggregateMax`; у `Count` map отсутствует
намеренно. Проверка типов, ссылок, map/filter и глубины выполняется при
валидации модели, а не только конструктором.

## Entity, Relation, Bindings и SourceSeries

`NewEntity(EntitySpec)` создаёт concrete entity с `ID`, `TypeID` и optional
name. Его property values живут отдельно. `NewRelation(RelationSpec)` создаёт
направленное ребро `From → To` c relation type и optional Boolean
`Applicability`.

`ValueBinding` — постоянный base candidate. Его target формируется
`ObjectPropertyBindingTarget(address)` для конкретного Entity/Relation или
`TypePropertyBindingTarget(owner, definitionID)` для default на типе. Value:

- `DefinedBindingValue(value)` — scalar value;
- `NullBindingValue()` — явно известное отсутствие, а не `Undefined`;
- `SeriesBindingValue(sourceSeries)` — immutable input series.

`NewValueBinding(ValueBindingSpec)` принимает target, value, selectors и
optional `*EffectivePeriod`. `NewEffectivePeriod(from, to)` задаёт
полуоткрытый интервал `[from,to)` с optional границами. `nil` period в spec
значит отсутствие time selector. `ValueBinding` не имеет UUID: его identity —
`ValueBindingKey` из target + канонических scopes + period; применяйте его в
`ReplaceValueBinding` и `RemoveValueBinding`.

Для series используйте `DefinedSourceSample` или `NullSourceSample`, затем
`NewSourcePoint` / `NewSourceInterval`, затем `NewPointSourceSeries` /
`NewIntervalSourceSeries`. Point должны строго возрастать; интервалы — быть
отсортированы, не пересекаться и быть `[from,to)`, смежные интервалы допустимы.
`SourceSeries` — authored immutable input; `SeriesResult` — отдельный runtime
output API.

## CalculationContext, результат и precedence

Постройте `PropertyAddress` через `EntityProperty(entityID, definitionID)`
или `RelationProperty(relationID, definitionID)`. Затем создайте контекст:

- `NewCalculationContextAt(at)` для scalar time point;
- `NewCalculationContextPeriod(from, to)` для запрошенного interval;
- `WithScopes(NewScopeSelector(scopeID, value), ...)`;
- `WithScenario(scenario)`, `WithoutScenario()`;
- `WithCalculationOverrides(NewOverride(...), NullOverride(...))`.

Контекст и все его производные неизменяемы. Calculation override — только
временная value-подмена: она не меняет model/dataset и не сериализуется.
Эффективный выбор значения у concrete property строго таков:

```text
CalculationOverride > ScenarioOverride > ObjectBinding > TypeBinding > Undefined
```

Dataset binding — это обычный `ObjectBinding`, нового уровня precedence нет.
`Result.State()` возвращает `Defined`, `Null` или `Undefined`. Для первого
используйте `Value()`, для последнего — `Undefined()` и его `UndefinedKind`
(`MissingValue`, `AmbiguousValue`, `DependencyUndefined`,
`EmptyAggregation`, `IndeterminateCondition`, `InvalidOperation`,
`ExternalUnavailable`, `DivisionByZero`). `Null` и `Undefined`
— нормальные бизнес-результаты, а не Go errors.

`Evaluate`, `EvaluateMany`, `EvaluateSeries`, `Explain`, `ExplainSeries`,
`AnalyzeImpact`, constraint/rule методы доступны у `Model`. Такие же методы
stateless evaluation доступны у `EvaluationSnapshot`; у `Session` context
уже сохранён и не передаётся в каждый вызов.

`SeriesResult` имеет `PointSeries` либо `IntervalSeries`. `Points()` и
`Intervals()` возвращают sample с отдельным scalar `Result`; весь series может
стать Undefined, что проверяется через `IsUndefined`/`Undefined`.

## Explain, Impact, Constraints, Rules

`Explain`/`ExplainSeries` возвращают фактический trace исполнения. Начните с
`Explanation.Root()`, обходите упорядоченные `ExplanationNode.Children()`.
Узел при наличии соответствующих данных даёт property, operation, decision,
state/value/undefined reason, calculation override target, Scenario ID и
aggregate item (`relationID`, `relatedEntityID`). Trace не реконструируется из
AST, поэтому повторные occurrence сохраняются.

`AnalyzeImpact(mode, source, context)` возвращает детерминированный
`ImpactResult`: exact `ModelVersionID`, optional `DatasetID`, source,
`Affected()` с minimum depth и прямые `Edges()`. Режимы:

- `StructuralImpact` — все структурно возможные зависимости;
- `EffectiveImpact` — после применения context/scopes/time/scenario/overrides;
- `ObservedImpact` — зависимости, реально увиденные вычислением.

Ограничения принадлежат модели. `CheckConstraint`, `CheckConstraints`,
`CheckFeasibility` работают также через snapshot/session, поэтому expression
может читать Dataset объекты по существующему языку. States Constraint:
`Satisfied`, `Violated`, `NotApplicable`, `Undefined`; только `SeverityHard`
участвует в `FeasibilityResult` (`Feasible`, `Infeasible`, `Undetermined`).

Rules — именованные чистые Boolean decisions без actions/effects. Методы
`EvaluateRule` и `EvaluateRules` дают `RuleResult`/`RuleReport` со states
`RuleActive`, `RuleInactive`, `RuleUndefined`; convenience methods `Active`,
`Inactive`, `Undefined` фильтруют report без повторного вычисления.

## External runtime Dataset

Dataset нужен, когда schema/formulas/constraints остаются в committed
`Model`, а concrete operational objects и object-level values живут отдельно.
Изменение data создаёт новый Dataset, а не ModelVersion:

```text
ModelVersion + Dataset = immutable EvaluationSnapshot
```

Dataset содержит только `Entity`, `Relation` и **object-level**
`ValueBinding`; он не содержит schema, traits, property definitions/contracts,
scope definitions, constraints, rules, type-level bindings, scenarios или
calculation context. Model embedded data остаются valid static data.

### Создание Dataset

```go
builder, err := bcg.NewDatasetBuilder(model, datasetID)
if err != nil { return err }

order, err := bcg.NewEntity(bcg.EntitySpec{ID: orderID, TypeID: orderTypeID})
if err != nil { return err }
if err := builder.AddEntity(order); err != nil { return err }

quantityAddress, err := bcg.EntityProperty(orderID, quantityDefinitionID)
if err != nil { return err }
target, err := bcg.ObjectPropertyBindingTarget(quantityAddress)
if err != nil { return err }
value, err := bcg.DefinedBindingValue(
	bcg.DecimalValue(bcg.MustParseDecimal("10")),
)
if err != nil { return err }
binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: value})
if err != nil { return err }
if err := builder.AddValueBinding(binding); err != nil { return err }

report, err := builder.Validate()
if err != nil { return err }
if !report.OK() { return fmt.Errorf("dataset invalid: %#v", report.Issues()) }
dataset, err := builder.Build()
if err != nil { return err }

snapshot, err := model.WithDataset(dataset)
if err != nil { return err }
totalAddress, err := bcg.EntityProperty(orderID, totalDefinitionID)
if err != nil { return err }
result, err := snapshot.Evaluate(totalAddress, calculation)
```

`DatasetBuilder` не thread-safe и после успешного `Build` sealed. `Dataset`
неизменяем и безопасен для concurrent reads. `Entities`, `Relations`,
`ValueBindings` возвращают deterministic defensive copies; `Entity(id)` и
`Relation(id)` — indexed lookup. У Dataset есть `ID`, `ModelVersionID`,
`IsValid`.

`DatasetID` задаёт приложение. У движка нет глобального registry, поэтому
**нельзя повторно использовать один DatasetID для другого содержимого**. Он
должен быть exact snapshot identity.

### Dataset validation и graph

Builder переиспользует `ValidationReport`/`ValidationIssue` и отклоняет:

- duplicate Dataset IDs и collisions с embedded Model Entity/Relation;
- unknown EntityType/RelationType, missing endpoint, несовместимый endpoint
  entity type/trait;
- type-level binding, binding на embedded object, unknown/wrong property,
  base binding derived-property;
- scalar/series, value type, unit, nullable, scope, effective-period,
  selector mismatch, ambiguous/conflicting object candidates;
- Dataset relation с двумя embedded endpoints.

Разрешены отношения Dataset→Dataset, Dataset→embedded и embedded→Dataset.
Они участвуют в forward/reverse/`TraverseBoth`, aggregate filter/map, Explain,
Impact, constraints и rules. Relation-owned binding разрешён, если relation
принадлежит Dataset. Dataset не может override property embedded model object:
для временной альтернативы используйте Scenario или CalculationOverride.

Dataset вместе с embedded data образует **полный** semantic graph. Отсутствие
relation означает, что relation нет; это не «данные ещё не подгружены».
Отсутствие object binding даёт обычный fallback type binding либо Undefined.
Никакой lazy loading, network, DB callback в движке не существует.

### EvaluationSnapshot, identity, Session и Scenario

`Model.WithDataset(dataset)` требует равенство
`dataset.ModelVersionID() == model.VersionID()`, строит combined dependency
graph один раз и отвергает cycle, добавленный runtime topology. Model/Dataset
целиком не копируются. `EvaluationSnapshot` имеет `Model`, `Dataset`,
`ModelVersionID`, `DatasetID`, `IsValid`, а также полный stateless API:
`Evaluate`, `EvaluateMany`, `EvaluateSeries`, `Explain`, `ExplainSeries`,
`AnalyzeImpact`, `CheckConstraint(s)`, `CheckFeasibility`, `EvaluateRule(s)`,
`NewSession`, `NewScenario`.

Не вызывайте `snapshot.Model().Evaluate(...)`, если хотите external data: это
осознанно model-only вычисление. Вызывайте `snapshot.Evaluate(...)`.

Внутренняя source identity:

```text
только model:       ModelVersionID
model + dataset:    ModelVersionID + DatasetID
```

Поэтому plan/session/observed impact от Dataset A никогда не переиспользуется
для Dataset B, даже при одинаковом ModelVersion и совпавших object IDs.
Новый Dataset означает новый snapshot и новую Session; API замены Dataset в
Session специально отсутствует.

`Model.NewSession(context)` создаёт model-only Session;
`snapshot.NewSession(context)` — Session с фиксированным Dataset. `Session`
безопасна для concurrent use. `SetCalculationContext` меняет context,
`SetCalculationOverrides` — только override layer, `Reset` чистит generation,
`CacheStats` возвращает Hits/Misses/Invalidations/Entries.

`NewScenario(model, spec)` создаёт portable model-only Scenario. Если override
нацелен на Dataset object, используйте `snapshot.NewScenario(spec)`. Scenario
только с model-owned targets остаётся portable даже при создании через
snapshot; mixed scenario становится Dataset-bound. `BaseDatasetID()` вернёт ID
только у последнего. Dataset-bound Scenario применяется лишь к exact Dataset,
не сериализуется и будет отклонён `NewArtifact`. `.bcg` остаётся portable
self-contained Model + model-only Scenario.

## Serialization, ошибки и API compatibility

`NewArtifact(model, scenarios...)`, `Encode`, `Marshal` пишут canonical
deterministic `.bcg`; `Decode`, `Unmarshal` читают его. `Artifact` даёт
`Model`, lookup `Scenario`, отсортированный `Scenarios`, `ModelName`,
`IsValid`. `FormatVersion` и `EngineCompatibility` показывают версию
wire/semantic contract. Dataset и Dataset-bound Scenario сознательно не
сериализуются.

Dataset snapshots have a separate canonical `.bcgd` artifact: use
`NewDatasetArtifact`, `MarshalDataset`/`EncodeDataset`, and
`UnmarshalDataset`/`DecodeDataset`. A Dataset has a SHA-256
`ContentDigest()` over runtime content only (not DatasetID), while `.bcgd`
also records and verifies the exact `ModelVersionID` and
`ModelSemanticDigest`. `.bcgd` never includes model schema, scenarios,
function callbacks, caches, or platform metadata. The supplied Dataset is a
complete evaluation world; artifact decoding does not introduce loading or IO.

Проверяйте ошибки через `errors.Is`/`errors.As`:

- `ErrInvalidArgument`, `ErrInvalidModel` — ошибка публичного вызова/wrapper;
- при codec: `ErrMalformedArtifact`, `ErrUnsupportedFormat`,
  `ErrIncompatibleEngine`, `ErrWireSemantic`, `ErrModelValidation`,
  `ErrScenarioValidation`;
- `*ValidationError` содержит deterministic `Issues`; до Build/Commit
  используйте `ValidationReport` (`OK`, `Len`, `Issues`);
- `*EngineError.Kind()` даёт стабильную категорию runtime failure.

Существующий model-only API сохраняется: `Draft`, `Commit`, `Edit`,
serialization, `Evaluate`, `EvaluateMany`, `EvaluateSeries`, `Explain`,
`Impact`, `Scenario`, `Session`, constraints и rules работают без Dataset
точно как раньше. Runtime Dataset — additive capability, не новая версия
модели и не новый `.bcg` format.

## Полная краткая карта публичного API

| Группа | Типы/функции |
| --- | --- |
| Lifecycle | `Model`, `Draft`, `CommitOptions`, `PrecisionPolicy`, `RoundingMode`, `NewModel`, `NewCommitOptions`, `NewPrecisionPolicy` |
| Schema | `EntityType`, `RelationType`, `PropertyDefinition`, `PropertyContract`, `Trait`, `ScopeDefinition`, `PropertyOwner`, `EndpointConstraint`, `PropertyComputation` и все их `Spec`/constructors |
| Expressions | `Expression`, references, arithmetic, comparison, boolean, traversal и aggregate builders |
| Semantic graph | `Entity`, `Relation`, `EntitySpec`, `RelationSpec`, model/draft list+lookup API |
| Data | `ValueBinding`, `ValueBindingSpec`, `BindingTarget`, `BindingValue`, `ValueBindingKey`, `EffectivePeriod`, `SourceSeries`, samples/points/intervals |
| Runtime | `CalculationContext`, `ScopeSelector`, `Override`, `PropertyAddress`, `Result`, `SeriesResult`, `Explanation`, `ImpactResult`, `Session` |
| External runtime data | `Dataset`, `DatasetBuilder`, `NewDatasetBuilder`, `Model.WithDataset`, `EvaluationSnapshot` |
| Governance | `Constraint`, `Rule`, Scenario types, reports/results, feasibility API |
| Portable data | `Artifact`, `NewArtifact`, `Encode`, `Decode`, `Marshal`, `Unmarshal` |
| Diagnostics | validation types, sentinels `Err...`, `EngineError`, `CacheStats` |

Для точных типов параметров и всех accessor-методов используйте GoDoc вашего
редактора или `go doc github.com/shmatovv/bcg-engine/v2`; все exported declarations
также перечислены в англоязычном разделе **Public API reference** выше.
