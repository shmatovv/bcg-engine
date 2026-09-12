# BCG Engine

BCG Engine is a deterministic Go calculation engine for immutable business
models. A committed model is exchanged as a canonical `.bcg` JSON artifact;
the public package provides evaluation, explanation, dependency impact,
constraints, rules, and an optional incremental evaluation session.

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

The portable format contract is documented in
[`docs/bcg-format-v1.md`](docs/bcg-format-v1.md).
