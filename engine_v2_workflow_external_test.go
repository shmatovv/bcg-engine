package bcg_test

import (
	"bytes"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// TestEngineV2CanonicalExternalConsumerWorkflow is the release-contract
// proof. It deliberately uses only github.com/shmatovv/bcg-engine/v2: no
// internal package, parallel AST, wire DTO, or test-only production hook is
// needed by an external consumer.
func TestEngineV2CanonicalExternalConsumerWorkflow(t *testing.T) {
	workflow := newCanonicalWorkflow(t)

	// 1-13: authoring is complete, inspectable, and validates before commit.
	if report, err := workflow.draft.Validate(); err != nil || !report.OK() {
		t.Fatalf("Draft.Validate = %#v, %v", report.Issues(), err)
	}
	ev1 := mustData[bcg.Model](t)(workflow.draft.Commit(mustCommitOptions(t, bcg.NewID(), "EV1", mustPolicy(t))))

	// 14-17: canonical v2 artifact IO and public lossless declarations/AST.
	artifact := mustData[bcg.Artifact](t)(bcg.NewArtifact(ev1))
	encoded := mustData[[]byte](t)(bcg.Marshal(artifact))
	decodedArtifact := mustData[bcg.Artifact](t)(bcg.Unmarshal(encoded))
	if again := mustData[[]byte](t)(bcg.Marshal(decodedArtifact)); !bytes.Equal(encoded, again) {
		t.Fatal(".bcg v2 re-encode was not byte-identical")
	}
	decoded := decodedArtifact.Model()
	if len(decoded.EnumDefinitions()) != 1 || len(decoded.EntityTypeDefinitions()) != 2 || len(decoded.RelationTypeDefinitions()) != 1 || len(decoded.EntityKeyDefinitions()) != 1 || len(decoded.InvariantDefinitions()) != 1 || len(decoded.Constraints()) != 1 || len(decoded.Rules()) != 1 || len(decoded.FunctionRequirements()) != 1 {
		t.Fatal("public declaration inspection lost canonical workflow content")
	}
	var label bcg.PropertyDefinition
	for _, declaration := range decoded.PropertyDefinitionDeclarations() {
		if declaration.ID() == workflow.labelID {
			label = declaration
			break
		}
	}
	if !label.IsValid() {
		t.Fatal("derived declaration is not public")
	}
	expression, found := label.Computation().Expression()
	if !found || expression.Kind() != bcg.ExpressionFunctionCall || len(expression.Children()) != 1 {
		t.Fatalf("derived FunctionCall AST not losslessly inspectable: %#v", expression)
	}
	call, found := expression.FunctionCall()
	if !found || call.Ref() != workflow.functionRef || len(call.Arguments()) != 1 {
		t.Fatal("FunctionCall identity or argument AST was lost")
	}
	if err := bcg.WalkExpression(expression, bcg.ExpressionVisitorFunc(func(node bcg.Expression) error {
		depth, valid := node.Depth()
		if !node.IsValid() || !valid || depth == 0 {
			t.Fatalf("WalkExpression exposed invalid node: %#v", node)
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if rebuilt, err := reconstructExpression(expression); err != nil || !rebuilt.IsValid() {
		t.Fatalf("public AST reconstruction = %#v, %v", rebuilt, err)
	}

	// 18-21: create EV2 through ordinary editing, then inspect its diff/effects.
	edit := mustData[*bcg.Draft](t)(decoded.Edit())
	noteOwner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(workflow.orderTypeID))
	optional := mustData[bcg.PropertyComputation](t)(bcg.InputComputationWithPresence(bcg.PresenceOptional))
	renamedNote := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: workflow.noteID, Owner: noteOwner, Name: "comment", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: optional}))
	if err := edit.ReplacePropertyDefinition(workflow.noteID, renamedNote); err != nil {
		t.Fatal(err)
	}
	ev2 := mustData[bcg.Model](t)(edit.Commit(mustCommitOptions(t, bcg.NewID(), "EV2", mustPolicy(t))))
	diff := mustData[bcg.ModelDiff](t)(bcg.CompareModels(decoded, ev2))
	evolution := mustData[bcg.EvolutionReport](t)(bcg.AnalyzeEvolution(decoded, ev2))
	if diff.Empty() || len(evolution.Changes()) != diff.Len() || !evolution.Has(bcg.MetadataOnly) {
		t.Fatalf("public evolution result = %#v / %#v", diff, evolution)
	}

	// 22-24: build D1, validate local mutations, and distinguish integrity from
	// structural Dataset construction.
	customer := phase12Entity(t, workflow.customerTypeID)
	order := phase12Entity(t, workflow.orderTypeID)
	relation := mustData[bcg.Relation](t)(bcg.NewRelation(bcg.RelationSpec{ID: bcg.NewID(), TypeID: workflow.relationTypeID, From: order.ID(), To: customer.ID()}))
	d1Builder := mustData[*bcg.DatasetBuilder](t)(bcg.NewDatasetBuilder(decoded, bcg.NewID()))
	for _, entity := range []bcg.Entity{customer, order} {
		if err := d1Builder.AddEntity(entity); err != nil {
			t.Fatal(err)
		}
	}
	if err := d1Builder.AddRelation(relation); err != nil {
		t.Fatal(err)
	}
	numberBinding := workflowBinding(t, order.ID(), workflow.numberID, bcg.String("order-1"))
	statusBinding := workflowBinding(t, order.ID(), workflow.statusID, mustData[bcg.Value](t)(bcg.EnumValue(workflow.enumID, workflow.openID)))
	for _, binding := range []bcg.ValueBinding{numberBinding, statusBinding} {
		if err := d1Builder.AddValueBinding(binding); err != nil {
			t.Fatal(err)
		}
	}
	d1 := mustData[bcg.Dataset](t)(d1Builder.Build())
	validator := decoded.NewRuntimeValidator()
	if report := validator.ValidateEntity(order); !report.OK() {
		t.Fatalf("local Entity validation = %#v", report.Issues())
	}
	if report := validator.ValidateRelation(relation, order, customer); !report.OK() {
		t.Fatalf("local Relation validation = %#v", report.Issues())
	}
	if report := validator.ValidateValueBinding(numberBinding, bcg.EntityRuntimeObject(order)); !report.OK() {
		t.Fatalf("local binding validation = %#v", report.Issues())
	}

	runtime := mustData[bcg.Runtime](t)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(workflowFunction{workflow.functionRef}))))
	snapshot := mustData[bcg.EvaluationSnapshot](t)(runtime.WithDataset(decoded, d1))
	if report := snapshot.CheckStructuralIntegrity(); !report.OK() {
		t.Fatalf("structural integrity = %#v", report.Issues())
	}
	context := mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-06-01T00:00:00Z"))))
	if report := snapshot.CheckIntegrity(context); !report.OK() {
		t.Fatalf("contextual integrity = %#v", report.Issues())
	}

	// 25-28: ordinary execution, provenance, impact, and static planning.
	labelAddress := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(order.ID(), workflow.labelID))
	numberAddress := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(order.ID(), workflow.numberID))
	assertWorkflowString(t, mustData[bcg.Result](t)(snapshot.Evaluate(labelAddress, context)), "order-1")
	explanation := mustData[bcg.Explanation](t)(snapshot.Explain(labelAddress, context))
	if explanation.Root().State() != bcg.Defined || len(explanation.Root().Children()) == 0 {
		t.Fatalf("Explain lost actual provenance: %#v", explanation)
	}
	impact := mustData[bcg.ImpactResult](t)(snapshot.AnalyzeImpact(bcg.StructuralImpact, numberAddress, context))
	if impact.Len() == 0 {
		t.Fatal("AnalyzeImpact did not include the FunctionCall-derived property")
	}
	target := mustData[bcg.PropertyTarget](t)(bcg.PropertyDefinitionTarget(label))
	plan := mustData[bcg.DependencyPlan](t)(decoded.PlanEvaluation([]bcg.PropertyTarget{target}))
	if len(plan.Targets()) != 1 || len(plan.DependentPropertyDefinitions()) == 0 || len(plan.FunctionRequirements()) != 1 {
		t.Fatalf("PlanEvaluation = %#v", plan)
	}

	// 29-31: a Patch creates D2; Rebase associates identical content with EV2.
	patchBuilder := mustData[*bcg.DatasetPatchBuilder](t)(bcg.NewDatasetPatchBuilder(d1.ID()))
	if err := patchBuilder.SetValueBinding(workflowBinding(t, order.ID(), workflow.numberID, bcg.String("order-2"))); err != nil {
		t.Fatal(err)
	}
	patch := mustData[bcg.DatasetPatch](t)(patchBuilder.Build())
	d2, patchReport, err := d1.Apply(decoded, bcg.NewID(), patch)
	if err != nil || !patchReport.OK() || d2.ContentDigest() == d1.ContentDigest() {
		t.Fatalf("DatasetPatch -> D2 = %#v, %v", patchReport.Issues(), err)
	}
	snapshotD2 := mustData[bcg.EvaluationSnapshot](t)(runtime.WithDataset(decoded, d2))
	assertWorkflowString(t, mustData[bcg.Result](t)(snapshotD2.Evaluate(labelAddress, context)), "order-2")
	d3, rebaseReport, err := d2.Rebase(decoded, ev2, bcg.NewID())
	if err != nil || !rebaseReport.OK() || d3.ContentDigest() != d2.ContentDigest() || d3.ModelSemanticDigest() != ev2.SemanticDigest() {
		t.Fatalf("Dataset Rebase -> D3 = %#v, %v", rebaseReport.Issues(), err)
	}

	// 32-36: independent Dataset artifact IO and ordered/duplicate-safe batch.
	datasetArtifact := mustData[bcg.DatasetArtifact](t)(bcg.NewDatasetArtifact(d3))
	datasetBytes := mustData[[]byte](t)(bcg.MarshalDataset(datasetArtifact))
	decodedDatasetArtifact := mustData[bcg.DatasetArtifact](t)(bcg.UnmarshalDataset(ev2, datasetBytes))
	if again := mustData[[]byte](t)(bcg.MarshalDataset(decodedDatasetArtifact)); !bytes.Equal(datasetBytes, again) {
		t.Fatal(".bcgd re-encode was not byte-identical")
	}
	decodedSnapshot := mustData[bcg.EvaluationSnapshot](t)(runtime.WithDataset(ev2, decodedDatasetArtifact.Dataset()))
	assertWorkflowString(t, mustData[bcg.Result](t)(decodedSnapshot.Evaluate(labelAddress, context)), "order-2")
	batch := mustData[bcg.BatchResult](t)(decodedSnapshot.EvaluateBatch([]bcg.PropertyAddress{labelAddress, labelAddress}, context))
	if batch.Len() != 2 {
		t.Fatalf("EvaluateBatch did not preserve duplicate requested targets: %d", batch.Len())
	}
	for index := 0; index < batch.Len(); index++ {
		item, _ := batch.Item(index)
		if item.Target() != labelAddress {
			t.Fatalf("batch target order changed at %d", index)
		}
		assertWorkflowString(t, item.Result(), "order-2")
	}
}

type canonicalWorkflow struct {
	draft                                               *bcg.Draft
	enumID, openID, customerTypeID, orderTypeID         bcg.ID
	relationTypeID, numberID, noteID, statusID, labelID bcg.ID
	functionRef                                         bcg.FunctionRef
}

func newCanonicalWorkflow(t testing.TB) canonicalWorkflow {
	t.Helper()
	logical := mustData[bcg.Model](t)(bcg.NewModel(bcg.NewID(), "canonical-external"))
	draft := mustData[*bcg.Draft](t)(logical.Draft())
	enumID, openID, closedID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	customerTypeID, orderTypeID, relationTypeID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	numberID, noteID, statusID, labelID := bcg.NewID(), bcg.NewID(), bcg.NewID(), bcg.NewID()
	status := mustData[bcg.EnumDefinition](t)(bcg.NewEnumDefinition(bcg.EnumDefinitionSpec{ID: enumID, Name: "order-status", Options: []bcg.EnumOptionSpec{{ID: closedID, Name: "closed"}, {ID: openID, Name: "open"}}}))
	orderOwner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(orderTypeID))
	required := mustData[bcg.PropertyComputation](t)(bcg.InputComputationWithPresence(bcg.PresenceRequired))
	optional := mustData[bcg.PropertyComputation](t)(bcg.InputComputationWithPresence(bcg.PresenceOptional))
	functionRef := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example.workflow", "echo", bcg.NewSemanticVersion(1, 0, 0)))
	signature := mustData[bcg.FunctionSignature](t)(bcg.NewFunctionSignature([]bcg.ValueType{bcg.StringType}, bcg.StringType, false))
	requirement := mustData[bcg.FunctionRequirement](t)(bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: functionRef, Signature: signature}))
	numberReference := mustData[bcg.Expression](t)(bcg.SelfProperty(numberID))
	functionCall := mustData[bcg.Expression](t)(bcg.CallFunction(functionRef, numberReference))
	derived := mustData[bcg.PropertyComputation](t)(bcg.DerivedComputation(functionCall))
	number := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: numberID, Owner: orderOwner, Name: "number", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: required}))
	note := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: noteID, Owner: orderOwner, Name: "note", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: optional}))
	statusProperty := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: statusID, Owner: orderOwner, Name: "status", ValueType: bcg.EnumType, EnumDefinitionID: enumID, TemporalMode: bcg.TemporalScalar, Computation: required}))
	label := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: labelID, Owner: orderOwner, Name: "label", ValueType: bcg.StringType, TemporalMode: bcg.TemporalScalar, Computation: derived}))
	orderType := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: orderTypeID, Name: "order", PropertyDefinitions: []bcg.ID{statusID, labelID, noteID, numberID}}))
	customerType := mustData[bcg.EntityType](t)(bcg.NewEntityType(bcg.EntityTypeSpec{ID: customerTypeID, Name: "customer"}))
	from := mustData[bcg.EndpointConstraint](t)(bcg.EntityTypeEndpoint(orderTypeID))
	to := mustData[bcg.EndpointConstraint](t)(bcg.EntityTypeEndpoint(customerTypeID))
	one := mustData[bcg.Cardinality](t)(bcg.NewCardinality(1, 1))
	relationType := mustData[bcg.RelationType](t)(bcg.NewRelationType(bcg.RelationTypeSpec{ID: relationTypeID, Name: "belongs-to", From: from, To: to, OutgoingCardinality: one, IncomingCardinality: bcg.UnboundedCardinality(0)}))
	key := mustData[bcg.EntityKeyDefinition](t)(bcg.NewEntityKeyDefinition(bcg.EntityKeyDefinitionSpec{ID: bcg.NewID(), Name: "order-number", EntityTypeID: orderTypeID, PropertyDefinitionIDs: []bcg.ID{numberID}}))
	invariantSubject := mustData[bcg.InvariantSubject](t)(bcg.EntityInvariantSubject(orderTypeID))
	nonEmpty := mustData[bcg.Expression](t)(bcg.NotEqual(numberReference, phase34Constant(t, bcg.String(""))))
	invariant := mustData[bcg.Invariant](t)(bcg.NewInvariant(bcg.InvariantSpec{ID: bcg.NewID(), Name: "order-number-present", Subject: invariantSubject, Assertion: nonEmpty}))
	truth := phase34Constant(t, bcg.Bool(true))
	constraint := mustData[bcg.Constraint](t)(bcg.NewConstraint(bcg.ConstraintSpec{ID: bcg.NewID(), Name: "world-valid", Assertion: truth, Severity: bcg.SeverityHard}))
	rule := mustData[bcg.Rule](t)(bcg.NewRule(bcg.RuleSpec{ID: bcg.NewID(), Name: "order-enabled", Condition: truth}))
	for _, declaration := range []any{label, status, relationType, customerType, number, key, orderType, note, statusProperty} {
		switch value := declaration.(type) {
		case bcg.PropertyDefinition:
			if err := draft.AddPropertyDefinition(value); err != nil {
				t.Fatal(err)
			}
		case bcg.EnumDefinition:
			if err := draft.AddEnumDefinition(value); err != nil {
				t.Fatal(err)
			}
		case bcg.RelationType:
			if err := draft.AddRelationType(value); err != nil {
				t.Fatal(err)
			}
		case bcg.EntityType:
			if err := draft.AddEntityType(value); err != nil {
				t.Fatal(err)
			}
		case bcg.EntityKeyDefinition:
			if err := draft.AddEntityKeyDefinition(value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := draft.AddFunctionRequirement(requirement); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddInvariant(invariant); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddConstraint(constraint); err != nil {
		t.Fatal(err)
	}
	if err := draft.AddRule(rule); err != nil {
		t.Fatal(err)
	}
	return canonicalWorkflow{draft: draft, enumID: enumID, openID: openID, customerTypeID: customerTypeID, orderTypeID: orderTypeID, relationTypeID: relationTypeID, numberID: numberID, noteID: noteID, statusID: statusID, labelID: labelID, functionRef: functionRef}
}

type workflowFunction struct{ ref bcg.FunctionRef }

func (f workflowFunction) Ref() bcg.FunctionRef { return f.ref }
func (workflowFunction) Bind(bcg.FunctionRequirement) (bcg.BoundFunction, error) {
	return workflowBound{}, nil
}

type workflowBound struct{}

func (workflowBound) Call(arguments []bcg.Value) (bcg.Value, error) { return arguments[0], nil }

func workflowBinding(t testing.TB, objectID, definitionID bcg.ID, value bcg.Value) bcg.ValueBinding {
	t.Helper()
	address := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(objectID, definitionID))
	target := mustData[bcg.BindingTarget](t)(bcg.ObjectPropertyBindingTarget(address))
	bindingValue := mustData[bcg.BindingValue](t)(bcg.DefinedBindingValue(value))
	return mustData[bcg.ValueBinding](t)(bcg.NewValueBinding(bcg.ValueBindingSpec{Target: target, Value: bindingValue}))
}

func assertWorkflowString(t testing.TB, result bcg.Result, want string) {
	t.Helper()
	value, ok := result.Value()
	if !ok {
		t.Fatalf("result is %s, want Defined %q", result.State(), want)
	}
	got, ok := value.StringValue()
	if !ok || got != want {
		t.Fatalf("result value = %q / %t, want %q", got, ok, want)
	}
}
