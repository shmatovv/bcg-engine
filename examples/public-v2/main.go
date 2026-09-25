package main

import (
	"fmt"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// This external module demonstrates model authoring, an explicit extension
// Runtime, Dataset evaluation, planning/batching, artifact IO, Patch, and
// Rebase without importing an internal package.
func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	modelID, versionID := bcg.NewID(), bcg.NewID()
	entityTypeID, inputID, derivedID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	logical, err := bcg.NewModel(modelID, "public-example")
	if err != nil {
		return err
	}
	draft, err := logical.Draft()
	if err != nil {
		return err
	}
	owner, err := bcg.EntityTypeOwner(entityTypeID)
	if err != nil {
		return err
	}
	ref, err := bcg.NewFunctionRef("example.text", "echo", bcg.NewSemanticVersion(1, 0, 0))
	if err != nil {
		return err
	}
	signature, err := bcg.NewFunctionSignature([]bcg.ValueType{bcg.StringType}, bcg.StringType, false)
	if err != nil {
		return err
	}
	requirement, err := bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: ref, Signature: signature})
	if err != nil {
		return err
	}
	input := bcg.InputComputation()
	inputDefinition, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: inputID, Owner: owner, Name: "input", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: input})
	if err != nil {
		return err
	}
	self, err := bcg.SelfProperty(inputID)
	if err != nil {
		return err
	}
	call, err := bcg.CallFunction(ref, self)
	if err != nil {
		return err
	}
	derived, err := bcg.DerivedComputation(call)
	if err != nil {
		return err
	}
	derivedDefinition, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: derivedID, Owner: owner, Name: "echo", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: derived})
	if err != nil {
		return err
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: entityTypeID, Name: "record", PropertyDefinitions: []bcg.ID{inputID, derivedID}})
	if err != nil {
		return err
	}
	if err = draft.AddFunctionRequirement(requirement); err != nil {
		return err
	}
	if err = draft.AddPropertyDefinition(inputDefinition); err != nil {
		return err
	}
	if err = draft.AddPropertyDefinition(derivedDefinition); err != nil {
		return err
	}
	if err = draft.AddEntityType(entityType); err != nil {
		return err
	}
	if report, err := draft.Validate(); err != nil {
		return err
	} else if !report.OK() {
		return fmt.Errorf("model validation: %#v", report.Issues())
	}
	policy, err := bcg.NewPrecisionPolicy(8, bcg.RoundHalfEven)
	if err != nil {
		return err
	}
	options, err := bcg.NewCommitOptions(versionID, "EV1", policy)
	if err != nil {
		return err
	}
	model, err := draft.Commit(options)
	if err != nil {
		return err
	}

	// .bcg always writes v2; decoding produces the same immutable public Model.
	artifact, err := bcg.NewArtifact(model)
	if err != nil {
		return err
	}
	encodedModel, err := bcg.Marshal(artifact)
	if err != nil {
		return err
	}
	decodedArtifact, err := bcg.Unmarshal(encodedModel)
	if err != nil {
		return err
	}
	model = decodedArtifact.Model()

	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: bcg.NewID(), TypeID: entityTypeID})
	if err != nil {
		return err
	}
	builder, err := bcg.NewDatasetBuilder(model, bcg.NewID())
	if err != nil {
		return err
	}
	if err = builder.AddEntity(entity); err != nil {
		return err
	}
	inputAddress, err := bcg.EntityProperty(entity.ID(), inputID)
	if err != nil {
		return err
	}
	target, err := bcg.ObjectPropertyBindingTarget(inputAddress)
	if err != nil {
		return err
	}
	bindingValue, err := bcg.DefinedBindingValue(bcg.String("hello"))
	if err != nil {
		return err
	}
	binding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: bindingValue})
	if err != nil {
		return err
	}
	if err = builder.AddValueBinding(binding); err != nil {
		return err
	}
	d1, err := builder.Build()
	if err != nil {
		return err
	}

	registry, err := bcg.NewFunctionRegistry(echoImplementation{ref: ref})
	if err != nil {
		return err
	}
	runtime, err := bcg.NewRuntime(registry)
	if err != nil {
		return err
	}
	snapshot, err := runtime.WithDataset(model, d1)
	if err != nil {
		return err
	}
	at, err := bcg.ParseDateTime("2026-01-01T00:00:00Z")
	if err != nil {
		return err
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		return err
	}
	derivedAddress, err := bcg.EntityProperty(entity.ID(), derivedID)
	if err != nil {
		return err
	}
	result, err := snapshot.Evaluate(derivedAddress, context)
	if err != nil {
		return err
	}
	value, _ := result.Value()
	text, _ := value.StringValue()
	fmt.Println(text)

	planTarget, err := bcg.PropertyDefinitionTarget(derivedDefinition)
	if err != nil {
		return err
	}
	if _, err = model.PlanEvaluation([]bcg.PropertyTarget{planTarget}); err != nil {
		return err
	}
	if _, err = snapshot.EvaluateBatch([]bcg.PropertyAddress{derivedAddress, derivedAddress}, context); err != nil {
		return err
	}

	patchBuilder, err := bcg.NewDatasetPatchBuilder(d1.ID())
	if err != nil {
		return err
	}
	changedValue, err := bcg.DefinedBindingValue(bcg.String("updated"))
	if err != nil {
		return err
	}
	changedBinding, err := bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: changedValue})
	if err != nil {
		return err
	}
	if err = patchBuilder.SetValueBinding(changedBinding); err != nil {
		return err
	}
	patch, err := patchBuilder.Build()
	if err != nil {
		return err
	}
	d2, report, err := d1.Apply(model, bcg.NewID(), patch)
	if err != nil {
		return err
	}
	if !report.OK() {
		return fmt.Errorf("patch validation: %#v", report.Issues())
	}

	// Rebase does not migrate/coerce data. This metadata-only EV2 keeps D2
	// structurally compatible and changes only the model association.
	edit, err := model.Edit()
	if err != nil {
		return err
	}
	renamed, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: inputID, Owner: owner, Name: "source", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: input})
	if err != nil {
		return err
	}
	if err = edit.ReplacePropertyDefinition(inputID, renamed); err != nil {
		return err
	}
	ev2Options, err := bcg.NewCommitOptions(bcg.NewID(), "EV2", policy)
	if err != nil {
		return err
	}
	ev2, err := edit.Commit(ev2Options)
	if err != nil {
		return err
	}
	if _, err = bcg.CompareModels(model, ev2); err != nil {
		return err
	}
	if _, err = bcg.AnalyzeEvolution(model, ev2); err != nil {
		return err
	}
	d3, report, err := d2.Rebase(model, ev2, bcg.NewID())
	if err != nil {
		return err
	}
	if !report.OK() {
		return fmt.Errorf("rebase validation: %#v", report.Issues())
	}
	datasetArtifact, err := bcg.NewDatasetArtifact(d3)
	if err != nil {
		return err
	}
	encodedDataset, err := bcg.MarshalDataset(datasetArtifact)
	if err != nil {
		return err
	}
	if _, err = bcg.UnmarshalDataset(ev2, encodedDataset); err != nil {
		return err
	}
	return nil
}

type echoImplementation struct{ ref bcg.FunctionRef }

func (f echoImplementation) Ref() bcg.FunctionRef { return f.ref }
func (echoImplementation) Bind(bcg.FunctionRequirement) (bcg.BoundFunction, error) {
	return echoBound{}, nil
}

type echoBound struct{}

func (echoBound) Call(arguments []bcg.Value) (bcg.Value, error) { return arguments[0], nil }
