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
Draft, validate, and commit with a new version UUID. The new Model has the
source version as its parent; the source Model is unchanged.
`NewArtifact(committed, scenarios...)` accepts only Scenarios compatible with
that exact committed version.

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
