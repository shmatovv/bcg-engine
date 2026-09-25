# BCG Engine v2.0 — Semantic Kernel Specification

Status: Draft for implementation  
Purpose: Canonical specification for the next major version of `bcg-engine`. Engine v2 is implemented from this document first; BCG Platform Model Service, Records Service, and Calculation Service are designed against the frozen v2 contract afterwards.

---

## 1. Context and Purpose

BCG Engine v2 is the deterministic, infrastructure-independent semantic and calculation kernel of BCG Platform.

The Engine MUST be the authoritative source of truth for semantics shared by:

- Model Service — entity/relation types, properties, formulas, constraints, rules, invariants, and schema evolution;
- Records Service — validation of runtime objects and compatibility with a published model;
- Calculation Service — evaluation, explanation, impact analysis, dependency planning, what-if scenarios, integrity, and feasibility;
- Configuration, Assistant, and Graph Editor — construction and inspection of the same canonical semantic model without maintaining a parallel AST or type system.

The Engine is NOT Model Service, Records Service, a workflow engine, a permission engine, or a persistence layer.

Canonical boundary:

```text
Platform services own persistence, tenancy, APIs and workflows.
BCG Engine owns semantic meaning and deterministic evaluation.
```

---

## 2. Core Mental Model

The v1 lifecycle remains conceptually intact:

```text
Logical Model
    │
    ▼
Draft
    │ Validate + Commit
    ▼
Committed ModelVersion (immutable)
    │
    │ + optional immutable Dataset
    ▼
EvaluationSnapshot
    ├── Evaluate / EvaluateMany / EvaluateSeries
    ├── Explain / ExplainSeries
    ├── Impact / Dependency Planning
    ├── Constraints / Rules / Invariants / Integrity
    └── Session
```

Required properties:

- `ModelVersion` is immutable;
- `Dataset` is immutable;
- `Scenario` is immutable;
- `EvaluationSnapshot` is immutable;
- `FunctionRegistry` is immutable;
- `Runtime` is immutable;
- `Draft`, builders, and patch builders are mutable and NOT concurrent-safe;
- immutable values and runtime snapshots are concurrent-safe;
- the Engine performs no network, database, or filesystem I/O;
- the Engine has no hidden current-time dependency;
- every input affecting a result is explicit;
- equal model, dataset, context, and function runtime MUST produce deterministic results.

---

## 3. Primary v2 Goals

v2 MUST close the following capability gaps:

1. Lossless public introspection of every persistent semantic declaration.
2. First-class Enum/Choice semantics.
3. Explicit required/optional presence semantics for Input properties.
4. Business keys and uniqueness semantics for Entity records.
5. Relation cardinality.
6. Subject-scoped Invariants.
7. A richer deterministic Expression language.
8. A real pure extension-function runtime.
9. Semantic Model Diff.
10. Model Evolution / compatibility analysis.
11. Semantic Digest.
12. Lightweight runtime-object validation without building a full Dataset.
13. Clear separation of structural validation and business integrity.
14. Immutable Dataset Patch with structural sharing where possible.
15. Dataset Rebase between compatible ModelVersions.
16. Canonical Dataset Artifact format (`.bcgd`).
17. Upstream dependency / evaluation planning.
18. Full `.bcg v1` read compatibility in v2.
19. Production-grade limits, fuzz/race/compatibility coverage, and deterministic wire contracts.

---

## 4. Explicit Non-Goals

Engine v2 MUST NOT own:

- `organization_id` or tenant isolation;
- users, sessions, identities;
- organization membership;
- roles or permissions;
- UI layout, tables, forms, kanban, dashboards;
- workflows, actions, approvals;
- email, push, or notifications;
- PostgreSQL, Redis, Kafka, HTTP, or gRPC;
- audit history;
- files/object storage;
- search/indexing;
- billing/subscriptions;
- AI/LLM behavior;
- packages/templates;
- arbitrary JS/Python/Go plugins;
- record transaction/revision/optimistic-lock semantics;
- a Records query language;
- soft-delete/archive policy;
- automatic database migrations.

Repeated or nested business structures are modeled through `Entity` + `Relation`; generic first-class List/Map/JSON value types are NOT introduced in v2.

---

## 5. Go Major Version and Module Path

v2 is a real major revision of the public and semantic contract.

Canonical module path:

```text
github.com/shmatovv/bcg-engine/v2
```

The existing v1.x line remains stable and immutable. Existing tags MUST NOT be moved or retagged.

---

## 6. Wire Formats

### 6.1 `.bcg`

`.bcg` remains the portable semantic Model artifact.

Engine v2 MUST:

- read `.bcg v1`;
- read `.bcg v2`;
- write `.bcg v2` by default.

Engine v1 is not required to read `.bcg v2`.

### 6.2 `.bcgd`

Introduce a separate Dataset artifact format:

```text
.bcgd
```

It stores runtime snapshot data and MUST NOT contain schema/model declarations.

The first dataset wire contract is named independently:

```text
BCG Dataset Format v1
```

`.bcg` and `.bcgd` MUST have separate format-version constants and compatibility rules.

---

## 7. Value System v2

Existing scalar kinds remain:

- Bool;
- Integer;
- Decimal;
- String;
- Date;
- DateTime;
- Duration.

Add:

- Enum.

### 7.1 Parameterized ValueType

Primitive types keep canonical convenience values:

```go
bcg.BoolType
bcg.IntegerType
bcg.DecimalType
bcg.StringType
bcg.DateType
bcg.DateTimeType
bcg.DurationType
```

Enum type construction:

```go
func EnumValueType(enumDefinitionID ID) (ValueType, error)
```

`ValueType` MUST support lossless inspection:

```go
func (t ValueType) Kind() ValueKind
func (t ValueType) EnumDefinitionID() (ID, bool)
func (t ValueType) IsValid() bool
func (t ValueType) Equal(other ValueType) bool
```

---

## 8. First-Class Enum

### 8.1 Domain Types

```go
type EnumDefinition struct { ... }
type EnumOption struct { ... }

type EnumDefinitionSpec struct {
    ID      ID
    Name    string
    Options []EnumOptionSpec
}

type EnumOptionSpec struct {
    ID   ID
    Name string
}
```

### 8.2 Identity

- `EnumDefinition.ID` is a stable UUID;
- `EnumOption.ID` is a stable UUID;
- `Name` is NOT identity;
- renaming an enum or option does not change identity;
- option order is NOT semantic ordering;
- UI color/icon/display order do not belong in Engine.

### 8.3 Enum Values

```go
func EnumValue(enumDefinitionID, optionID ID) (Value, error)
```

Validation MUST enforce:

- the enum exists;
- the option exists;
- the option belongs to the declared enum;
- the property Enum type matches the Enum value type.

Allowed base operations:

- `Equal`;
- `NotEqual`;
- `IsNull` / `IsDefined` / `IsUndefined`;
- `If` / `Coalesce`.

Ordering (`<`, `>`, `<=`, `>=`) for Enum is forbidden in v2.

---

## 9. Input Presence Semantics

Presence applies ONLY to `InputComputation`.

```go
type PresencePolicy uint8

const (
    PresenceInvalid PresencePolicy = iota
    PresenceOptional
    PresenceRequired
)
```

Preferred public form:

```go
func InputComputation(presence PresencePolicy) (PropertyComputation, error)
```

Derived properties have no PresencePolicy:

```go
func DerivedComputation(expr Expression) (PropertyComputation, error)
```

### 9.1 `PresenceRequired`

`PresenceRequired` means:

> Every concrete Entity/Relation of the owning type MUST have an object-level, unqualified base binding for that Input property.

An unqualified binding has:

- no `ScopeSelector`;
- no `EffectivePeriod`.

A type-level binding/default does NOT satisfy `PresenceRequired`.

A host application MAY intentionally materialize a type-level default into an object-level base binding when creating a record.

### 9.2 Presence and Null

Presence and nullability are independent:

```text
Optional + non-nullable
  binding may be absent;
  an existing binding cannot be Null.

Required + non-nullable
  object base binding is required;
  Null is forbidden.

Required + nullable
  object base binding is required;
  explicit Null is allowed.
```

### 9.3 Temporal Semantics

For `TemporalSeries + PresenceRequired`, an unqualified `SeriesBindingValue` MUST exist.

This does NOT mean:

- the series must be non-empty;
- the series must cover every instant;
- a value must resolve for every scope/time combination.

Coverage requirements are expressed through separate Invariant/Constraint declarations.

---

## 10. Entity Key Definitions

UUID remains the canonical internal identity of each Entity.

Business identity is defined separately by `EntityKeyDefinition`.

### 10.1 API

```go
type EntityKeyDefinition struct { ... }

type EntityKeyDefinitionSpec struct {
    ID                    ID
    Name                  string
    EntityTypeID          ID
    PropertyDefinitionIDs []ID
}

func NewEntityKeyDefinition(spec EntityKeyDefinitionSpec) (EntityKeyDefinition, error)
```

Draft API:

```text
AddEntityKeyDefinition
ReplaceEntityKeyDefinition
RemoveEntityKeyDefinition
```

Model MUST expose deterministic list + lookup APIs.

### 10.2 Key Constraints

Every key component MUST be:

- a property of the target EntityType;
- `InputComputation`;
- `TemporalScalar`;
- `PresenceRequired`;
- non-nullable;
- a scalar value type.

Scoped/temporal overrides do not affect identity. A key uses the unqualified object base binding.

Composite-key component order is semantic and MUST be preserved.

One EntityType MAY define multiple alternate business keys.

### 10.3 Uniqueness

In a combined runtime world, each `EntityKeyDefinition` MUST be unique among entities of the same EntityType.

UUID identity remains separate and mandatory.

---

## 11. Relation Cardinality

Existing endpoint type/trait constraints remain.

Add stored-topology cardinality:

```go
type Cardinality struct { ... }

func NewCardinality(min uint64, max *uint64) (Cardinality, error)
func UnboundedCardinality(min uint64) Cardinality
```

`RelationTypeSpec` gains:

```go
OutgoingCardinality Cardinality
IncomingCardinality Cardinality
```

Example:

```text
Order --customer--> Customer

OutgoingCardinality = 1..1
IncomingCardinality = 0..N
```

### 11.1 Semantics

Cardinality counts stored relation instances of the given RelationType.

It MUST NOT depend on a relation instance's runtime `Applicability` expression.

Reason: topology integrity must not change meaning with `CalculationContext`.

### 11.2 `.bcg v1` Default

When reading v1 relation types:

```text
Outgoing = 0..N
Incoming = 0..N
```

---

## 12. Invariants

Constraint, Rule, and Invariant are distinct semantic declarations.

### 12.1 Purpose

An Invariant is a subject-scoped pure Boolean assertion about a concrete Entity or Relation.

Examples:

```text
OrderItem.quantity > 0
Shift.end >= Shift.start
Stock.amount >= 0
```

### 12.2 Domain API

```go
type InvariantSubjectKind uint8

type InvariantSubject struct {
    Kind   InvariantSubjectKind
    TypeID ID
}

type InvariantSpec struct {
    ID            ID
    Name          string
    Subject       InvariantSubject
    Applicability Expression // optional zero expression
    Assertion     Expression
}

func NewInvariant(spec InvariantSpec) (Invariant, error)
```

Draft API:

```text
AddInvariant
ReplaceInvariant
RemoveInvariant
```

Model API:

```go
Invariants()
Invariant(id)
```

### 12.3 Expression Scope

Inside an invariant, `SelfProperty(...)` resolves against the concrete subject object of the declared type.

For an Entity subject, `SelfProperty` may reference only properties of that EntityType / applicable contracts.

For a Relation subject, it may reference only properties of that RelationType.

Traversal from the subject is allowed through the ordinary expression language.

### 12.4 Runtime API

```go
CheckInvariant(id, subjectObjectID, context)
CheckInvariantsForObject(subjectObjectID, context)
CheckInvariants(context)
```

Result states:

- Satisfied;
- Violated;
- NotApplicable;
- Undefined.

Invariants have no side effects.

### 12.5 Important Boundary

- Model validation validates the Invariant declaration itself;
- Dataset build is NOT required to prove every Invariant is Satisfied;
- Invariant truth is checked only by an explicit runtime check with `CalculationContext`.

This preserves the ability to evaluate and diagnose intentionally invalid what-if states.

---

## 13. Constraint vs Invariant vs Rule

### Constraint

A global assertion about the calculation world. It may participate in Feasibility.

### Invariant

An assertion about one concrete Entity or Relation subject.

### Rule

A named pure Boolean decision without actions/effects.

Engine v2 MUST NOT add side effects to any of these types.

---

## 14. Structural Validation vs Business Integrity

v2 explicitly separates two classes of checks.

### 14.1 Structural Validation

Question answered:

> Can the Engine safely and unambiguously interpret this model/dataset?

Examples of structural failures:

- duplicate IDs;
- unknown type/property;
- missing relation endpoint;
- endpoint incompatible with RelationType;
- wrong ValueType/Unit;
- base binding supplied for a Derived property;
- invalid enum option;
- ambiguous binding candidates;
- malformed expression;
- dependency cycle;
- unsupported required extension function.

A structural failure makes the Model/Dataset unusable for evaluation.

Use the existing `ValidationReport` / `ValidationIssue` / `ValidationError` model.

### 14.2 Business Integrity

Question answered:

> Does a structurally valid runtime world satisfy business schema rules?

Examples:

- required binding missing;
- duplicate business key;
- relation cardinality violation;
- invariant violated or undefined.

An integrity violation does NOT have to prevent creating an `EvaluationSnapshot`.

This is required for:

- what-if analysis;
- diagnostics;
- imported dirty-data analysis;
- migration previews;
- infeasible scenarios.

### 14.3 Integrity API

```go
type IntegrityReport struct { ... }

CheckStructuralIntegrity() // presence, key uniqueness, cardinality
CheckIntegrity(context)    // structural integrity + contextual invariants
```

Records Service may use `IntegrityReport` as a write barrier according to its own policy.

---

## 15. Public Lossless Introspection

Mandatory v2 invariant:

> Every persistent semantic declaration that can be created or deserialized MUST be losslessly inspectable through the public package.

This applies to:

- EntityType;
- RelationType;
- PropertyDefinition;
- PropertyContract;
- Trait;
- ScopeDefinition;
- EnumDefinition / EnumOption;
- EntityKeyDefinition;
- Invariant;
- Constraint;
- Rule;
- Entity;
- Relation;
- ValueBinding;
- Scenario / ScenarioOverride;
- FunctionRequirement;
- Expression.

No Platform service may need `internal/...` or a parallel canonical AST merely because a public Engine object cannot be inspected.

---

## 16. Expression Introspection

`Expression` remains immutable.

Add:

```go
type ExpressionKind uint16

func (e Expression) Kind() ExpressionKind
func (e Expression) IsValid() bool
func (e Expression) Depth() uint32
func (e Expression) Children() []Expression
```

Typed read-only accessors MUST allow lossless inspection of a concrete node:

```go
ConstantValue()
PropertyReference()
UnaryOperation()
BinaryOperation()
ConditionalOperation()
TraversalSource()
AggregateOperation()
FunctionCall()
```

Also provide:

```go
WalkExpression(expr Expression, visitor ExpressionVisitor) error
```

Requirements:

- deterministic child order;
- no mutation;
- zero-copy where safe, defensive copies for collections;
- every decoded valid AST reconstructible using public builders;
- reconstructed AST semantically equivalent to the original.

---

## 17. Expression Language v2

Existing v1 operations remain.

### 17.1 State / Control

- `IsDefined`;
- `IsUndefined`;
- `IsNull`;
- `Coalesce`;
- `If`.

### 17.2 Numeric

- Add / Subtract / Multiply / Divide / Negate;
- `Abs`;
- `Round`;
- `Floor`;
- `Ceil`;
- `Modulo`;
- `ScalarMin`;
- `ScalarMax`.

All Decimal operations MUST preserve the existing exact-arithmetic and `PrecisionPolicy` semantics.

### 17.3 Comparison

- Equal;
- NotEqual;
- Less;
- LessOrEqual;
- Greater;
- GreaterOrEqual.

Enum supports equality/non-equality only.

### 17.4 Boolean

- And;
- Or;
- Not.

### 17.5 String

- `Concat`;
- `Length`;
- `Lower`;
- `Upper`;
- `Trim`;
- `Contains`;
- `StartsWith`;
- `EndsWith`.

String semantics MUST be Unicode-defined and deterministic. Locale-sensitive collation/case behavior is out of scope for v2.

### 17.6 Date / DateTime / Duration

- `AddDuration`;
- `SubtractDuration`;
- `Difference`;
- `Year`;
- `Month`;
- `Day`;
- `Hour`;
- `Minute`.

No hidden `Now()` exists in the expression language. Current time is always supplied by `CalculationContext` or an explicit Value.

### 17.7 Aggregates

Existing:

- Count;
- Sum;
- Average;
- Min;
- Max.

Add:

- Exists;
- Any;
- All;
- DistinctCount.

Aggregate source/filter/map semantics remain typed and validated.

---

## 18. Extension Functions — Full v2 Implementation

v2 MUST fully implement a deterministic extension-function runtime.

### 18.1 FunctionRef

```go
type FunctionRef struct {
    Namespace string
    Name      string
    Version   SemanticVersion
}
```

The version is exact, not major-only.

Example:

```text
bcg.finance.npv@1.0.0
```

### 18.2 FunctionRequirement

A Model stores a portable requirement:

```go
type FunctionRequirementSpec struct {
    Ref       FunctionRef
    Signature FunctionSignature
}
```

A signature describes:

- arity;
- parameter `ValueType`s;
- result `ValueType`;
- optional unit/dimension contracts;
- nullable-result policy.

`FunctionRequirement` is semantic model content and is serialized in `.bcg v2`.

### 18.3 Function Implementation

The host registers trusted Go implementations.

```go
type FunctionImplementation interface {
    Ref() FunctionRef
    Bind(requirement FunctionRequirement) (BoundFunction, error)
}
```

`Bind` lets a generic implementation prove that it supports the exact model-declared signature.

`BoundFunction` MUST be immutable and concurrent-safe.

### 18.4 FunctionRegistry

```go
func NewFunctionRegistry(implementations ...FunctionImplementation) (FunctionRegistry, error)
```

Registry requirements:

- immutable;
- concurrent-safe;
- deterministic lookup by exact `FunctionRef`;
- rejects duplicates;
- exposes a deterministic fingerprint.

Mutable package-level global registries are forbidden.

### 18.5 Runtime

```go
type Runtime struct { ... }

func NewRuntime(registry FunctionRegistry) (Runtime, error)
func DefaultRuntime() Runtime
```

`DefaultRuntime` contains Engine built-ins only and no external implementations.

Custom-function evaluation runs through Runtime-bound entry points.

Target API shape:

```go
runtime, err := bcg.NewRuntime(registry)
result, err := runtime.Evaluate(model, target, context)
snapshot, err := runtime.WithDataset(model, dataset)
session, err := runtime.NewSession(snapshot, context)
```

Existing convenience methods such as `Model.Evaluate` and `Model.WithDataset` MAY remain and use `DefaultRuntime`.

### 18.6 Call Expression

```go
func CallFunction(ref FunctionRef, args ...Expression) (Expression, error)
```

Model validation MUST validate every call against the matching `FunctionRequirement`.

### 18.7 Null / Undefined Propagation

An external implementation is invoked only when all required arguments are `Defined`.

- Undefined argument -> result `Undefined(DependencyUndefined)`;
- Null argument -> standard null propagation according to requirement/signature null policy;
- implementation receives no hidden Engine state.

### 18.8 Function Result

An extension function may return:

- a Defined typed `Value`;
- Null if the signature permits a nullable result.

It MUST NOT arbitrarily manufacture Engine `Undefined` states.

Implementation/runtime failures are represented by a stable `EngineError` category.

### 18.9 Determinism Contract

Every implementation MUST be:

- pure;
- deterministic;
- free of I/O;
- free of network access;
- free of database access;
- free of filesystem access;
- free of mutable global state;
- free of hidden wall-clock access;
- free of randomness;
- concurrent-safe.

Breaking this contract is a host-integration bug, not a supported Engine semantic.

### 18.10 Runtime Compatibility

If a Model requires a `FunctionRef` absent from the selected Runtime:

```text
ErrIncompatibleRuntime
```

This is NOT a business-level `Undefined` result.

---

## 19. Model Diff

Add:

```go
func CompareModels(oldModel, newModel Model) (ModelDiff, error)
```

Both versions MUST belong to the same logical `ModelID`.

Diff MUST use stable IDs rather than names.

### 19.1 Change Families

Diff MUST distinguish at least:

- declaration add/remove;
- diagnostic metadata rename;
- property value-type change;
- input/derived change;
- formula change;
- presence change;
- nullability change;
- unit/dimension change;
- temporal-mode change;
- contract change;
- trait change;
- relation-endpoint change;
- relation-cardinality change;
- enum option add/remove/rename;
- key add/remove/component change;
- invariant add/remove/change;
- constraint/rule change;
- function-requirement change;
- embedded Entity/Relation/ValueBinding change.

### 19.2 Determinism

`ModelDiff.Changes()` MUST return a deterministic ordered slice.

Changes MUST expose typed inspection. Human strings or generic JSON patches are insufficient as the public semantic contract.

---

## 20. Model Evolution Analysis

Add:

```go
func AnalyzeEvolution(oldModel, newModel Model) (EvolutionReport, error)
```

`EvolutionReport` is a conservative model-only analysis. It MUST NOT claim facts about a concrete Records Dataset that has not been inspected.

### 20.1 Per-Change Effects

One change MAY have multiple effects:

```text
MetadataOnly
EvaluationSemanticsChanged
RequiresDatasetStructuralRevalidation
RequiresIntegrityRecheck
RequiresRecalculation
RequiresTopologyRebuild
PotentialDataMigration
PotentialDataLoss
```

Effects SHOULD be represented as a typed set/flags rather than a single mutually-exclusive enum.

### 20.2 Examples

```text
Diagnostic Name rename
  -> MetadataOnly

Add optional Input property
  -> structurally compatible

Add Derived property
  -> EvaluationSemanticsChanged + RequiresRecalculation

Formula change
  -> EvaluationSemanticsChanged + RequiresRecalculation

Add Invariant
  -> RequiresIntegrityRecheck

Tighten cardinality
  -> RequiresIntegrityRecheck

Add required Input property
  -> PotentialDataMigration + RequiresIntegrityRecheck

Decimal -> String
  -> PotentialDataMigration

Remove property
  -> PotentialDataMigration + PotentialDataLoss

Remove EntityType
  -> PotentialDataMigration + PotentialDataLoss
```

A concrete Dataset MAY still be compatible even when model-only analysis conservatively marks a change as potentially migratory. Dataset Rebase is the factual compatibility check for one concrete Dataset.

---

## 21. Semantic Digest

Add:

```go
func (m Model) SemanticDigest() Digest
```

Algorithm:

```text
SHA-256 over a canonical semantic projection
```

### 21.1 Included in SemanticDigest

- definition IDs;
- value/schema types;
- contracts;
- computations and AST;
- units/dimensions;
- enum identities/options;
- keys;
- cardinality;
- invariants;
- constraints/rules;
- function requirements;
- embedded semantic/runtime declarations that affect evaluation semantics.

### 21.2 Excluded from SemanticDigest

- Model VersionID;
- ParentVersionID;
- version label;
- diagnostic names that do not affect semantics;
- Platform authoring/UI metadata.

An additional `Artifact.ContentDigest()` MAY exist for complete canonical wire content.

Primary uses:

- semantic no-op detection;
- release deduplication;
- cache keying;
- compatibility checks.

---

## 22. Runtime Object Local Validation

Records Service MUST be able to validate an individual mutation without building a complete Dataset.

Introduce an immutable validator:

```go
type RuntimeValidator struct { ... }

func (m Model) NewRuntimeValidator() RuntimeValidator
```

API:

```go
ValidateEntity(entity Entity) ValidationReport
ValidateRelation(relation Relation, from Entity, to Entity) ValidationReport
ValidateValueBinding(binding ValueBinding, object RuntimeObjectDescriptor) ValidationReport
```

Local validation checks only locally provable rules:

- known EntityType/RelationType;
- endpoint compatibility;
- known property;
- property owner;
- ValueType;
- EnumDefinition/Option;
- Unit/dimension;
- TemporalMode;
- nullability;
- prohibition of base bindings on Derived properties;
- scope selector structure/types;
- effective-period structure;
- source-series structure.

Local validation does NOT check:

- uniqueness across records;
- cardinality across records;
- contextual invariants;
- global constraints;
- relation topology beyond supplied endpoints.

---

## 23. Dataset v2 Identity

Dataset identity contains:

```text
DatasetID
ModelVersionID
ModelSemanticDigest
DatasetContentDigest
```

`DatasetID` is caller-supplied and MUST NOT be reused for different snapshot content.

`DatasetContentDigest` is deterministic over canonical dataset content.

`Model.WithDataset` / Runtime binding MUST verify exact `ModelVersionID` and `ModelSemanticDigest` compatibility.

---

## 24. Dataset Structural Semantics

A Dataset remains a complete evaluation world for the snapshot presented to the Engine.

The Engine does NOT know whether a Dataset represents:

- an entire organization;
- one order;
- a minimal dependency closure.

An absent relation means that relation does not exist in that evaluation world.

The Engine MUST NEVER call lazy DB/network loaders.

---

## 25. Dataset Patch

### 25.1 Purpose

Create a new immutable Dataset cheaply from a previous Dataset without requiring a full external rebuild/copy.

### 25.2 API

```go
type DatasetPatchBuilder struct { ... }
type DatasetPatch struct { ... }

func NewDatasetPatchBuilder(baseDatasetID ID) (*DatasetPatchBuilder, error)
```

Supported operations:

```text
AddEntity
ReplaceEntity
RemoveEntity

AddRelation
ReplaceRelation
RemoveRelation

SetValueBinding
ReplaceValueBinding
RemoveValueBinding
```

A patch builder MAY temporarily contain operations that would break references in intermediate order. Validation applies to the final candidate world.

### 25.3 Apply

```go
func (d Dataset) Apply(
    model Model,
    newDatasetID ID,
    patch DatasetPatch,
) (Dataset, ValidationReport, error)
```

Rules:

- patch MUST reference the exact base `DatasetID`;
- Apply does not change `ModelVersionID`;
- old Dataset remains unchanged;
- new Dataset receives a new `DatasetID` and content digest;
- implementation SHOULD use structural sharing / copy-on-write where possible;
- remove operations do not cascade;
- final Dataset MUST pass structural validation.

---

## 26. Dataset Rebase

Rebase attaches the SAME runtime data to a new compatible ModelVersion without coercing or transforming values.

```go
func (d Dataset) Rebase(
    oldModel Model,
    newModel Model,
    newDatasetID ID,
) (Dataset, ValidationReport, error)
```

Rebase requirements:

- same logical `ModelID`;
- no coercion or migration;
- preserve Entity/Relation IDs;
- preserve bindings unchanged;
- only model association changes when structural compatibility is proven;
- validate types/properties/endpoints/enums/bindings against `newModel`;
- produce a new immutable Dataset.

Examples:

```text
property rename with stable ID
  -> success

formula change
  -> success

add optional property
  -> success

remove unused property
  -> success

remove property with existing binding
  -> fail

Decimal -> String with existing Decimal binding
  -> fail

add required property
  -> structural rebase may succeed; IntegrityReport reports missing required value

remove EntityType used by Dataset entity
  -> fail
```

Rebase is the factual compatibility result for one concrete Dataset, unlike conservative model-only `EvolutionReport`.

---

## 27. Dataset Artifact (`.bcgd`)

### 27.1 Content

Canonical dataset artifact contains:

```text
format_version
engine_compatibility
DatasetID
ModelVersionID
ModelSemanticDigest
DatasetContentDigest
Entities
Relations
Object-level ValueBindings
```

It MUST NOT contain:

- schema;
- traits/contracts;
- scope definitions;
- constraints/rules/invariants;
- type-level bindings;
- model scenarios;
- function implementations.

### 27.2 API

```go
func NewDatasetArtifact(dataset Dataset) (DatasetArtifact, error)
func MarshalDataset(artifact DatasetArtifact) ([]byte, error)
func UnmarshalDataset(model Model, data []byte) (DatasetArtifact, error)
func EncodeDataset(w io.Writer, artifact DatasetArtifact) error
func DecodeDataset(model Model, r io.Reader) (DatasetArtifact, error)
```

Decode MUST perform structural validation against exact model identity/digest.

Serialization MUST be canonical and deterministic.

---

## 28. Integrity Checks

### 28.1 Structural Integrity

Context-independent business checks:

- `PresenceRequired`;
- EntityKey uniqueness;
- RelationCardinality.

API:

```go
func (s EvaluationSnapshot) CheckStructuralIntegrity() IntegrityReport
```

### 28.2 Contextual Integrity

```go
func (s EvaluationSnapshot) CheckIntegrity(ctx CalculationContext) IntegrityReport
```

Adds subject Invariants to structural-integrity checks.

`IntegrityReport` MUST contain deterministic ordered findings plus exact `ModelVersionID` / `DatasetID`.

Integrity violations are normal business results, not Go errors.

---

## 29. Evaluation Dependency Planning

### 29.1 Goal

Calculation and Records must be able to answer:

> Which schema/runtime dependencies may be needed to evaluate these targets?

without allowing the Engine to query a database.

### 29.2 API

```go
func (m Model) PlanEvaluation(
    targets []PropertyTarget,
) (DependencyPlan, error)
```

Optional context-aware variant:

```go
func (m Model) PlanEvaluationWithContext(
    targets []PropertyTarget,
    ctx CalculationContext,
) (DependencyPlan, error)
```

### 29.3 DependencyPlan Content

A plan includes at least:

- target PropertyDefinitions;
- dependent PropertyDefinitions;
- explicit embedded-object addresses;
- required RelationTypes;
- traversal direction;
- filter/map dependencies;
- required EntityTypes/RelationTypes;
- ScopeDefinition dependencies;
- temporal dependencies;
- function requirements.

The plan is deterministic and MAY be host-serialized if useful, but `.bcg` does not store plans.

### 29.4 Dataset Closure Contract

Records Service MAY build a minimal Dataset closure from a `DependencyPlan`.

The Dataset passed to Engine is still considered the complete world for that calculation.

The Engine MUST NOT infer that additional records exist outside the Dataset.

---

## 30. Impact and Dependency Planning

Existing downstream `AnalyzeImpact` remains.

Semantic split:

```text
AnalyzeImpact:
    source changed -> what can be affected?

PlanEvaluation:
    target requested -> what may be required?
```

Both APIs MUST use one canonical dependency-graph implementation so their interpretations cannot drift.

---

## 31. Batch Evaluation

v2 SHOULD provide an efficient repeated-plan path for many targets.

Target API shape:

```go
func (s EvaluationSnapshot) EvaluateBatch(
    targets []PropertyAddress,
    ctx CalculationContext,
) (BatchResult, error)
```

`EvaluateMany` remains supported.

Implementation SHOULD reuse compiled plans/dependency indexes rather than rebuild the graph for every target.

Semantics MUST match independent `Evaluate` calls.

---

## 32. Runtime Function Identity and Caches

Runtime/session/cache identity MUST account for:

```text
ModelVersionID
ModelSemanticDigest
optional DatasetID
optional DatasetContentDigest
FunctionRegistryFingerprint
```

Compiled plans, Sessions, or caches MUST NOT be reused across incompatible Runtime registries.

---

## 33. `.bcg v2` Semantic Content

`.bcg v2` extends v1 with:

- parameterized Enum ValueType;
- EnumDefinitions / EnumOptions;
- Input PresencePolicy;
- EntityKeyDefinitions;
- Relation cardinalities;
- Invariants;
- new Expression kinds;
- FunctionRequirements;
- all information required for lossless introspection and round-trip.

Datasets remain outside `.bcg`.

Dataset-bound Scenarios remain outside `.bcg`.

---

## 34. `.bcg v1` -> v2 Import Defaults

When reading v1:

```text
Enums                  -> none
Entity keys            -> none
Invariants             -> none
Function requirements  -> none
Relation cardinality   -> 0..N / 0..N
Input presence          -> PresenceOptional
```

Existing v1 computations, scopes, bindings, constraints, rules, scenarios, and embedded data MUST preserve their original semantics.

v1 canonical bytes do not need to equal bytes after re-encoding as v2; semantic meaning MUST remain equivalent.

---

## 35. Metadata Policy

Engine `Name` fields are diagnostic/authoring identifiers, not a user-facing localization layer.

Platform display labels, descriptions, localization, colors, iconography, graph positions, and form layouts live outside Engine.

`SemanticDigest` MUST NOT change because of metadata that does not affect evaluation semantics.

`ModelDiff` MAY still expose such metadata changes as a separate change family.

---

## 36. Defaults Policy

v2 does not add a separate `DefaultValue` schema primitive.

Existing type-level `ValueBinding` remains the runtime fallback/default layer.

It does NOT satisfy `PresenceRequired`.

If Platform wants a required record to start from a default, Records Service may:

1. resolve the type-level default;
2. materialize it as an object-level base binding;
3. persist the record.

This is intentionally host behavior.

---

## 37. Validation Issue Families v2

Exact enum names SHOULD follow existing repository naming conventions, but the capability set MUST cover at least:

```text
UnknownEnumDefinition
UnknownEnumOption
EnumValueMismatch

InvalidPresencePolicy
RequiredBaseValueMissing

UnknownEntityKeyDefinition
InvalidEntityKeyProperty
DuplicateEntityKey

InvalidCardinality
OutgoingCardinalityViolation
IncomingCardinalityViolation

UnknownInvariantSubject
InvariantExpressionTypeMismatch

UnknownFunctionRequirement
UnsupportedFunction
FunctionSignatureMismatch

DatasetModelSemanticMismatch
DatasetPatchBaseMismatch

EvolutionDifferentLogicalModel
```

Structural semantic problems remain `ValidationIssue`s rather than separate Go errors.

Go errors are used for malformed public calls, incompatible runtime/wire contracts, and Engine failures.

---

## 38. Stable Errors v2

Stable sentinels/categories MUST include at least:

```text
ErrInvalidArgument
ErrInvalidModel
ErrMalformedArtifact
ErrUnsupportedFormat
ErrIncompatibleEngine
ErrIncompatibleRuntime
ErrWireSemantic
ErrModelValidation
ErrScenarioValidation
ErrDatasetValidation
```

`EngineError.Kind()` is extended only for genuinely new operation-failure categories.

Business result states MUST NOT be converted into Go errors.

---

## 39. Immutability and Ownership

### Immutable / Concurrent-Safe

- committed Model;
- all schema declarations;
- Expression;
- FunctionRequirement;
- FunctionRegistry;
- Runtime;
- Dataset;
- DatasetPatch;
- DatasetArtifact;
- EvaluationSnapshot;
- Scenario;
- Result/Reports;
- DependencyPlan;
- ModelDiff/EvolutionReport;
- Session remains concurrent-safe as in v1.

### Mutable / NOT Concurrent-Safe

- Draft;
- DatasetBuilder;
- DatasetPatchBuilder;
- other explicitly named builders.

Returned slices/maps MUST either be owned defensive copies or documented immutable views. Caller mutation MUST NOT mutate Engine state.

---

## 40. Deterministic Ordering

Every public report/list API whose order is not user-authored semantic order MUST return deterministic ordering.

Canonical ordering is based on stable IDs/compound keys according to type-specific contracts.

This applies to at least:

- validation issues;
- integrity findings;
- diff changes;
- evolution changes;
- model declarations;
- dataset objects;
- dependency plans;
- artifact serialization.

---

## 41. Limits and Host Safety

The existing maximum expression depth of `512` remains unless changed only with explicit evidence and compatibility review.

v2 MUST add explicit bounds/guards for new recursive or collection surfaces, including:

- expression depth;
- function argument count;
- composite-key component count;
- enum option count per definition;
- artifact decode nesting;
- dependency-plan recursion;
- explanation depth;
- evaluation stack depth.

Concrete numeric defaults for new limits MUST be selected using implementation benchmark/fuzz evidence before freeze and documented in README/API docs.

Malformed `.bcg` / `.bcgd` input MUST NOT permit unbounded recursion, stack exhaustion, or uncontrolled allocation without limit checks.

---

## 42. Performance Requirements

This specification does not invent latency SLAs without benchmark evidence.

Benchmarks MUST exist for at least:

- model validate/commit;
- `.bcg v1` decode into v2;
- `.bcg v2` encode/decode;
- semantic digest;
- ModelDiff on large schemas;
- Evolution analysis;
- Dataset build;
- small Dataset Patch over large Dataset;
- Dataset Rebase;
- `.bcgd` encode/decode;
- combined snapshot build;
- dependency planning;
- repeated Evaluate / EvaluateBatch;
- extension-function call overhead;
- integrity checks over large relation/key sets.

Regression thresholds are set only after baseline measurements exist.

---

## 43. Required Fuzzing

Minimum fuzz targets:

- `.bcg v1/v2` decode;
- `.bcgd` decode;
- Expression builders/introspection/round-trip;
- Enum values;
- FunctionRef/signatures;
- Dataset Patch application;
- Model Diff/Evolution;
- Rebase;
- canonical marshal round-trip.

Invariant:

> Arbitrary bytes/input MUST NOT panic, corrupt process state, or bypass validation.

---

## 44. Required Race / Concurrency Coverage

`go test -race ./...` MUST cover at least:

- concurrent Model reads;
- concurrent Dataset reads;
- concurrent EvaluationSnapshot reads;
- concurrent Session calls;
- concurrent Runtime/FunctionRegistry calls;
- extension-function evaluation;
- repeated dependency planning;
- diff/evolution on shared immutable models;
- concurrent reads of Dataset-patch results.

---

## 45. Required Public External-Consumer Proof

A mandatory `package bcg_test` integration test MUST import only:

```go
github.com/shmatovv/bcg-engine/v2
```

It MUST prove the complete supported workflow without importing `internal/...`.

---

## 46. Canonical External-Consumer Scenario

A public-only test MUST perform the following end-to-end sequence:

```text
1. Create logical Model
2. Create Enum + options
3. Create EntityTypes
4. Create required/optional properties
5. Create Entity business key
6. Create RelationType with cardinality
7. Create Derived expression
8. Create Invariant
9. Create Constraint
10. Create Rule
11. Declare external FunctionRequirement
12. Use function call in Expression
13. Validate Draft
14. Commit EV1
15. Marshal .bcg v2
16. Unmarshal .bcg v2
17. Inspect every declaration/AST losslessly
18. Edit model
19. Commit EV2
20. Compare EV1/EV2
21. AnalyzeEvolution
22. Build Dataset D1
23. Validate local objects
24. Check structural integrity
25. Check contextual invariants
26. Evaluate / Explain / Impact
27. PlanEvaluation
28. Apply DatasetPatch -> D2
29. Evaluate D2
30. Rebase D2 to EV2 -> D3
31. Marshal .bcgd
32. Unmarshal .bcgd
33. Evaluate again
34. Verify deterministic equivalent results
```

If any step requires a client-owned parallel semantic representation or an `internal/...` import, v2 is NOT complete.

---

## 47. Extension Function Acceptance Scenario

A separate external test MUST:

1. register a custom pure function;
2. create a matching `FunctionRequirement`;
3. build a model expression using `CallFunction`;
4. commit, marshal, and unmarshal the model;
5. evaluate with a matching Runtime;
6. verify failure with `DefaultRuntime`;
7. verify failure for a wrong function version;
8. verify signature mismatch handling;
9. verify Null/Undefined propagation;
10. verify concurrent calls under the race detector;
11. verify registry fingerprint/cache separation.

---

## 48. Model Evolution Acceptance Matrix

Mandatory cases:

- diagnostic name change;
- add optional Input;
- add required Input;
- remove Input;
- primitive type change;
- unit/dimension change;
- Input -> Derived;
- Derived -> Input;
- formula change;
- enum option add;
- enum option remove;
- enum option rename with stable ID;
- key add/remove/component change;
- cardinality tighten/relax;
- invariant add/remove/change;
- relation-endpoint change;
- trait/contract changes;
- function-requirement add/remove/version change;
- embedded object/binding changes.

Every case MUST assert both expected `ModelDiff` and expected `EvolutionReport`.

---

## 49. Dataset Rebase Acceptance Matrix

Mandatory cases:

- metadata-only model change -> success;
- property rename with stable ID -> success;
- formula-only change -> success;
- add optional property -> success;
- add required property -> structural success + integrity missing;
- remove unused property -> success;
- remove bound property -> fail;
- incompatible type change with existing binding -> fail;
- enum option removed but unused -> success;
- enum option removed and used -> fail;
- cardinality tightened -> structural success + possible integrity violation;
- EntityType removed and used -> fail;
- RelationType removed and used -> fail;
- logical ModelID mismatch -> fail.

---

## 50. Dataset Patch Acceptance Matrix

Minimum cases:

- add entity;
- replace entity;
- remove entity;
- add/replace/remove relation;
- add/set/replace/remove binding;
- multi-operation patch with temporarily dangling intermediate state but valid final state;
- dangling final state rejected;
- wrong base DatasetID rejected;
- old Dataset unchanged;
- new DatasetID required;
- deterministic content digest;
- patch over very large Dataset benchmark;
- concurrent reads of old and new Datasets.

---

## 51. Integrity Acceptance Matrix

### Presence

- Optional missing allowed;
- Required missing produces finding;
- Required Null + nullable=true allowed;
- Required Null + nullable=false is a structural/binding violation;
- type-level default alone does not satisfy Required.

### Keys

- unique single-component key;
- duplicate single-component key;
- composite key;
- scoped override does not alter key identity;
- null/missing key component impossible after schema validation.

### Cardinality

- 0..N;
- 1..1 missing;
- 1..1 duplicate;
- 0..1 duplicate;
- incoming/outgoing checked independently;
- `Applicability` expression does not change topology count.

### Invariants

- satisfied;
- violated;
- not applicable;
- undefined;
- Entity subject;
- Relation subject;
- traversal inside invariant;
- Dataset-owned and embedded-object subjects.

---

## 52. Compatibility Policy

### Source / API

The `/v2` import path intentionally isolates the new public API from v1. Source compatibility across major paths is not required.

### Semantic

v1 artifacts imported by v2 MUST preserve v1 semantic results for all pre-existing features.

### Wire

- v2 decoder supports `.bcg v1` and `.bcg v2`;
- v2 encoder emits `.bcg v2`;
- Dataset Artifact uses its own independent wire contract;
- canonical round-trip tests are required for every format.

---

## 53. Documentation Required Before Freeze

The v2 repository MUST contain:

- top-level README documenting the actual public API;
- `docs/bcg-format-v2.md`;
- `docs/bcgd-format-v1.md`;
- `docs/engine-v2-migration.md`;
- extension-function guide;
- semantic-evolution guide;
- Dataset Patch/Rebase guide;
- public examples compilable from an external module.

README MUST describe implemented public behavior, not roadmap features.

---

## 54. Implementation Phases

Recommended implementation order:

### Phase 0 — v2 Branch / Module / Bootstrap

- create the `/v2` module path;
- freeze the v1 regression baseline;
- add v1 compatibility fixtures;
- make no semantic changes yet.

### Phase 1 — Lossless Public Introspection

- Expression introspection;
- complete declaration introspection;
- external round-trip tests.

### Phase 2 — Schema v2

- parameterized ValueType;
- Enum;
- Presence;
- EntityKeyDefinition;
- Cardinality;
- Invariant declarations.

### Phase 3 — Expression v2

- state helpers;
- numeric/string/time operations;
- Any/All/Exists/DistinctCount.

### Phase 4 — Extension Functions

- FunctionRef / Requirement / Signature;
- Registry;
- Runtime;
- CallFunction;
- compatibility/error/cache-identity semantics.

### Phase 5 — `.bcg v2`

- v2 codec;
- v1 -> v2 decoder;
- canonical compatibility tests.

### Phase 6 — Diff / Evolution / Digest

- `CompareModels`;
- `AnalyzeEvolution`;
- `SemanticDigest`;
- acceptance matrices.

### Phase 7 — Runtime Local Validation + Integrity

- RuntimeValidator;
- StructuralIntegrity;
- Invariant checks;
- key/cardinality/presence findings.

### Phase 8 — Dataset v2 Identity + Patch

- digests;
- patch builder/apply;
- structural sharing;
- benchmarks.

### Phase 9 — Dataset Rebase

- exact model compatibility;
- rebase reports;
- Evolution integration.

### Phase 10 — Dataset Artifact

- `.bcgd` codec;
- canonical round-trip;
- fuzzing.

### Phase 11 — Dependency Planning + Batch Optimization

- `PlanEvaluation`;
- unified graph implementation with Impact;
- `EvaluateBatch` benchmark path.

### Phase 12 — Hardening / Freeze

- race tests;
- fuzzing;
- shuffled/repeated tests;
- benchmarks;
- compatibility fixtures;
- external-module proof;
- documentation audit;
- API freeze.

---

## 55. Per-Phase Delivery Rules

Every implementation phase MUST finish independently with:

```text
implementation
-> focused tests
-> go test ./...
-> go test -race ./...
-> go vet ./...
-> shuffled/repeated tests
-> public external-consumer tests where relevant
-> documentation update
-> diff review
-> commit
```

The next phase MUST NOT start from a red branch.

Codex MUST NOT implement multiple future phases in one prompt unless the prompt explicitly authorizes that scope.

Each phase should preserve a clean architectural diff. Do not mix opportunistic unrelated refactors into a semantic phase unless they are required for correctness.

---

## 56. Definition of Done v2

BCG Engine v2 is ready to freeze only when ALL of the following are true:

1. the canonical public external-consumer scenario passes completely;
2. the `.bcg v1` compatibility suite passes;
3. `.bcg v2` canonical round-trip passes;
4. `.bcgd` canonical round-trip passes;
5. all structural validation is deterministic;
6. all integrity checks are deterministic;
7. Enum / Presence / Keys / Cardinality / Invariants are fully tested;
8. Extension Functions are fully implemented and Runtime-isolated;
9. Model Diff / Evolution / Digest are fully tested;
10. Dataset Patch / Rebase are fully tested;
11. dependency planning is verified against actual evaluator dependencies;
12. Explain / Impact / Constraints / Rules / Scenarios remain correct;
13. existing Dataset semantics remain immutable and contain no lazy loading;
14. no public consumer requires `internal/...`;
15. no Platform-specific tenant/UI/workflow semantics leak into Engine;
16. `go test ./...` passes;
17. `go test -race ./...` passes;
18. `go vet ./...` passes;
19. repeated shuffled tests pass;
20. configured fuzz smoke/full suites pass;
21. benchmarks exist for new hot paths and no unexplained catastrophic regression remains;
22. working tree is clean;
23. documentation describes actual implemented API;
24. final release audit finds no unresolved P0/P1 semantic/API defects.

---

## 57. Freeze Statement

After the Definition of Done is satisfied, the following becomes an architectural invariant:

> BCG Engine v2 is the canonical semantic kernel for BCG Platform Model, Records, and Calculation boundaries. Platform services MUST NOT reimplement Engine-owned schema, value, expression, validation, evolution, integrity, or runtime-calculation semantics.

After v2 freeze, architecture work moves to:

```text
Model Service
-> Records Service
-> Views / Configuration / Policy
-> Automation
-> Calculation
-> Assistant / Integration
```

Engine should change afterwards only when a real semantic capability gap is demonstrated, not merely for the convenience of a particular service implementation.
