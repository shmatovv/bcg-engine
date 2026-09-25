# Extension functions

An extension function is an explicit host integration, not an Engine global.
Its identity is a `FunctionRef`: namespace, name, and exact semantic version.
The exact version is part of identity; a compatible-looking version is not a
substitute. `FunctionSignature` describes ordered argument types, result type,
and nullable-result permission. A `FunctionRequirement` combines the ref and
signature and is added to a Draft. `CallFunction(ref, args...)` creates the
portable expression; committing validates that its exact requirement exists.

At runtime, construct an immutable `FunctionRegistry` from implementations,
then `NewRuntime(registry)`. `Runtime.ValidateModel` checks requirements and
`Runtime.WithDataset` creates an evaluable snapshot. The Engine binds through
`FunctionImplementation.Bind(requirement)` and executes the resulting
`BoundFunction.Call(arguments)`. There is deliberately no package-global
registry or lazy loader.

```go
ref, _ := bcg.NewFunctionRef("example.text", "normalize", bcg.NewSemanticVersion(1, 0, 0))
signature, _ := bcg.NewFunctionSignature([]bcg.ValueType{bcg.StringType}, bcg.StringType, false)
requirement, _ := bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: ref, Signature: signature})
_ = draft.AddFunctionRequirement(requirement)
call, _ := bcg.CallFunction(ref, argument)
runtime, _ := bcg.NewRuntime(registry)
```

Functions must behave as pure, deterministic calculations for their declared
arguments and runtime identity. A host must not mutate arguments or return a
value outside the requirement's result-state/type contract. A non-nullable
signature cannot return Null; normal Undefined/Null/Defined business states
remain values, while malformed calls, a missing/incompatible implementation,
or an Engine failure return Go errors. The Engine contains a callback panic and
reports it as an error rather than allowing it to escape evaluation.

Registry/runtime identity contributes to cache identity, along with model,
Dataset, calculation context, scenario, target, and function ref. Do not use
changing external state behind a function call and expect cached values to be
safe.

Native Go purity is a host contract, not a sandbox security boundary. The
Engine does not sandbox arbitrary untrusted Go callbacks: hosts must isolate,
authorize, and resource-limit such code themselves.
