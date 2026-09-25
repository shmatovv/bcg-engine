package bcg_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	bcg "github.com/shmatovv/bcg-engine/v2"
)

// phase34DerivedModel builds one entirely public scalar-derived model. It is
// intentionally small so each test exercises the real validation, dependency,
// evaluator, explain and session path rather than an internal test hook.
func phase34DerivedModel(t testing.TB, name string, expression bcg.Expression, valueType bcg.ValueType, requirements ...bcg.FunctionRequirement) (bcg.Model, bcg.PropertyAddress, bcg.CalculationContext) {
	t.Helper()
	logical, err := bcg.NewModel(bcg.NewID(), name)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := logical.Draft()
	if err != nil {
		t.Fatal(err)
	}
	for _, requirement := range requirements {
		if err := draft.AddFunctionRequirement(requirement); err != nil {
			t.Fatal(err)
		}
	}
	typeID, definitionID, entityID := bcg.NewID(), bcg.NewID(), bcg.NewID()
	owner, err := bcg.EntityTypeOwner(typeID)
	if err != nil {
		t.Fatal(err)
	}
	computation, err := bcg.DerivedComputation(expression)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: definitionID, Owner: owner, Name: "value", ValueType: valueType, TemporalMode: bcg.TemporalScalar, Computation: computation})
	if err != nil {
		t.Fatal(err)
	}
	entityType, err := bcg.NewEntityType(bcg.EntityTypeSpec{ID: typeID, Name: "item", PropertyDefinitions: []bcg.ID{definitionID}})
	if err != nil {
		t.Fatal(err)
	}
	entity, err := bcg.NewEntity(bcg.EntitySpec{ID: entityID, TypeID: typeID})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []any{entityType, definition, entity} {
		switch value := item.(type) {
		case bcg.EntityType:
			err = draft.AddEntityType(value)
		case bcg.PropertyDefinition:
			err = draft.AddPropertyDefinition(value)
		case bcg.Entity:
			err = draft.AddEntity(value)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	committed, err := draft.Commit(mustCommitOptions(t, bcg.NewID(), "phase34", mustPolicy(t)))
	if err != nil {
		t.Fatal(err)
	}
	target, err := bcg.EntityProperty(entityID, definitionID)
	if err != nil {
		t.Fatal(err)
	}
	at, err := bcg.ParseDateTime("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	context, err := bcg.NewCalculationContextAt(at)
	if err != nil {
		t.Fatal(err)
	}
	return committed, target, context
}

func phase34Constant(t testing.TB, value bcg.Value) bcg.Expression {
	t.Helper()
	expression, err := bcg.Constant(value)
	if err != nil {
		t.Fatal(err)
	}
	return expression
}

func TestPhase34TemporalAndScalarExpressionsExternal(t *testing.T) {
	start, err := bcg.ParseDateTime("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	delta, err := bcg.ParseDuration("PT1H2M3S")
	if err != nil {
		t.Fatal(err)
	}
	startExpression := phase34Constant(t, mustData[bcg.Value](t)(bcg.DateTimeValue(start)))
	deltaExpression := phase34Constant(t, bcg.DurationValue(delta))
	added := mustData[bcg.Expression](t)(bcg.AddDuration(startExpression, deltaExpression))
	model, target, context := phase34DerivedModel(t, "add-duration", added, bcg.DateTimeType)
	result, err := model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.Value()
	if !ok {
		t.Fatal("AddDuration did not return Defined")
	}
	actual, ok := value.DateTime()
	if !ok || actual.String() != "2026-01-01T01:02:03Z" {
		t.Fatalf("AddDuration = %v, %v", actual, ok)
	}

	subtracted := mustData[bcg.Expression](t)(bcg.SubtractDuration(added, deltaExpression))
	model, target, context = phase34DerivedModel(t, "subtract-duration", subtracted, bcg.DateTimeType)
	result, err = model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ = result.Value()
	actual, _ = value.DateTime()
	if actual.String() != start.String() {
		t.Fatalf("SubtractDuration = %s", actual)
	}

	difference := mustData[bcg.Expression](t)(bcg.Difference(added, startExpression))
	if _, err := reconstructExpression(difference); err != nil {
		t.Fatalf("temporal expression reconstruction: %v", err)
	}
	model, target, context = phase34DerivedModel(t, "difference", difference, bcg.DurationType)
	result, err = model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ = result.Value()
	duration, ok := value.Duration()
	if !ok || duration.String() != "PT1H2M3S" {
		t.Fatalf("Difference = %v, %v", duration, ok)
	}

	integerMin := mustData[bcg.Expression](t)(bcg.ScalarMin(phase34Constant(t, bcg.Int64(4)), phase34Constant(t, bcg.Int64(-9)), phase34Constant(t, bcg.Int64(3))))
	model, target, context = phase34DerivedModel(t, "integer-min", integerMin, bcg.IntegerType)
	result, err = model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ = result.Value()
	integer, _ := value.IntegerString()
	if integer != "-9" {
		t.Fatalf("ScalarMin integer = %s", integer)
	}

	decimalMax := mustData[bcg.Expression](t)(bcg.ScalarMax(phase34Constant(t, bcg.DecimalValue(bcg.MustParseDecimal("1.0000000000000000001"))), phase34Constant(t, bcg.DecimalValue(bcg.MustParseDecimal("1.00000000000000000009")))))
	model, target, context = phase34DerivedModel(t, "decimal-max", decimalMax, bcg.DecimalType)
	result, err = model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ = result.Value()
	decimal, _ := value.Decimal()
	if decimal.String() != "1.0000000000000000001" {
		t.Fatalf("ScalarMax exact decimal = %s", decimal)
	}

	date, _ := bcg.ParseDate("2026-01-01")
	invalid := mustData[bcg.Expression](t)(bcg.AddDuration(phase34Constant(t, mustData[bcg.Value](t)(bcg.DateValue(date))), deltaExpression))
	logical, _ := bcg.NewModel(bcg.NewID(), "invalid temporal")
	draft, _ := logical.Draft()
	_ = draft // the invalid expression is structurally buildable, but cannot typecheck in a model.
	if invalid.Kind() != bcg.ExpressionBinaryV2 {
		t.Fatal("invalid temporal expression lost public shape")
	}
}

type phase34Bound struct {
	call func([]bcg.Value) (bcg.Value, error)
}

func (f phase34Bound) Call(args []bcg.Value) (bcg.Value, error) { return f.call(args) }

type phase34Implementation struct {
	ref  bcg.FunctionRef
	call func([]bcg.Value) (bcg.Value, error)
}

func (f phase34Implementation) Ref() bcg.FunctionRef { return f.ref }
func (f phase34Implementation) Bind(bcg.FunctionRequirement) (bcg.BoundFunction, error) {
	return phase34Bound{call: f.call}, nil
}

type phase34ResultBound struct {
	result bcg.FunctionResult
}

func (f phase34ResultBound) Call([]bcg.Value) (bcg.Value, error) { return bcg.String("unused"), nil }
func (f phase34ResultBound) CallResult([]bcg.Value) (bcg.FunctionResult, error) {
	return f.result, nil
}

type phase34ResultImplementation struct {
	ref    bcg.FunctionRef
	result bcg.FunctionResult
}

func (f phase34ResultImplementation) Ref() bcg.FunctionRef { return f.ref }
func (f phase34ResultImplementation) Bind(bcg.FunctionRequirement) (bcg.BoundFunction, error) {
	return phase34ResultBound{result: f.result}, nil
}

func TestPhase34FunctionCallExternal(t *testing.T) {
	dottedNamespace := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example.math", "round", bcg.NewSemanticVersion(1, 0, 0)))
	dottedName := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example", "math.round", bcg.NewSemanticVersion(1, 0, 0)))
	if dottedNamespace.String() != dottedName.String() {
		t.Fatal("test setup requires the compact display forms to collide")
	}
	registryIdentity := mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(
		phase34Implementation{ref: dottedNamespace, call: func([]bcg.Value) (bcg.Value, error) { return bcg.String("namespace"), nil }},
		phase34Implementation{ref: dottedName, call: func([]bcg.Value) (bcg.Value, error) { return bcg.String("name"), nil }},
	))
	if _, ok := registryIdentity.Lookup(dottedNamespace); !ok {
		t.Fatal("registry lost dotted namespace identity")
	}
	if _, ok := registryIdentity.Lookup(dottedName); !ok {
		t.Fatal("registry lost dotted name identity")
	}

	ref := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example", "echo", bcg.NewSemanticVersion(1, 0, 0)))
	signature := mustData[bcg.FunctionSignature](t)(bcg.NewFunctionSignature([]bcg.ValueType{bcg.StringType, bcg.StringType}, bcg.StringType, false))
	requirement := mustData[bcg.FunctionRequirement](t)(bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: ref, Signature: signature}))
	call := mustData[bcg.Expression](t)(bcg.CallFunction(ref, phase34Constant(t, bcg.String("a")), phase34Constant(t, bcg.String("b"))))
	if call.Kind() != bcg.ExpressionFunctionCall || len(call.Children()) != 2 {
		t.Fatal("FunctionCall public children missing")
	}
	view, ok := call.FunctionCall()
	if !ok || view.Ref() != ref || len(view.Arguments()) != 2 {
		t.Fatal("FunctionCall public accessor lost data")
	}
	if _, err := reconstructExpression(call); err != nil {
		t.Fatalf("FunctionCall reconstruction: %v", err)
	}
	walked := 0
	if err := bcg.WalkExpression(call, bcg.ExpressionVisitorFunc(func(bcg.Expression) error { walked++; return nil })); err != nil || walked != 3 {
		t.Fatalf("walk = %d, %v", walked, err)
	}

	model, target, context := phase34DerivedModel(t, "function", call, bcg.StringType, requirement)
	if got := model.FunctionRequirements(); len(got) != 1 || got[0].Ref() != ref {
		t.Fatalf("canonical requirements = %#v", got)
	}
	reconstructed, err := reconstructModel(t, model)
	if err != nil {
		t.Fatalf("FunctionCall model reconstruction: %v", err)
	}
	if got := reconstructed.FunctionRequirements(); len(got) != 1 || got[0].Ref() != ref {
		t.Fatalf("reconstructed requirements = %#v", got)
	}
	if _, err := model.Evaluate(target, context); !errors.Is(err, bcg.ErrIncompatibleRuntime) {
		t.Fatalf("default runtime error = %v", err)
	}

	registry := mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(phase34Implementation{ref: ref, call: func(args []bcg.Value) (bcg.Value, error) {
		left, _ := args[0].StringValue()
		right, _ := args[1].StringValue()
		return bcg.String(left + right), nil
	}}))
	runtime := mustData[bcg.Runtime](t)(bcg.NewRuntime(registry))
	result, err := runtime.Evaluate(model, target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := result.Value()
	text, _ := value.StringValue()
	if text != "ab" {
		t.Fatalf("function result = %s", text)
	}

	panicRegistry := mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(phase34Implementation{ref: ref, call: func([]bcg.Value) (bcg.Value, error) { panic("host bug") }}))
	panicRuntime := mustData[bcg.Runtime](t)(bcg.NewRuntime(panicRegistry))
	if _, err := panicRuntime.Evaluate(model, target, context); !errors.Is(err, bcg.ErrFunctionPanic) {
		t.Fatalf("panic containment error = %v", err)
	}

	badRegistry := mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(phase34Implementation{ref: ref, call: func([]bcg.Value) (bcg.Value, error) { return bcg.Bool(true), nil }}))
	badRuntime := mustData[bcg.Runtime](t)(bcg.NewRuntime(badRegistry))
	if _, err := badRuntime.Evaluate(model, target, context); !errors.Is(err, bcg.ErrFunctionResultContract) {
		t.Fatalf("result contract error = %v", err)
	}

	callbackUndefined := mustData[bcg.FunctionResult](t)(bcg.UndefinedFunctionResult(bcg.MissingValue))
	undefinedRuntime := mustData[bcg.Runtime](t)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(phase34ResultImplementation{ref: ref, result: callbackUndefined}))))
	if _, err := undefinedRuntime.Evaluate(model, target, context); !errors.Is(err, bcg.ErrFunctionResultContract) {
		t.Fatalf("callback Undefined must be a contract error, got %v", err)
	}

	wrong, _ := bcg.NewFunctionRef("example", "echo", bcg.NewSemanticVersion(1, 0, 1))
	wrongRuntime := mustData[bcg.Runtime](t)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(phase34Implementation{ref: wrong, call: func([]bcg.Value) (bcg.Value, error) { return bcg.String("wrong"), nil }}))))
	if _, err := wrongRuntime.Evaluate(model, target, context); !errors.Is(err, bcg.ErrIncompatibleRuntime) {
		t.Fatalf("exact version error = %v", err)
	}

	if _, err := bcg.NewArtifact(model); err != nil {
		t.Fatalf("v2 must serialize FunctionCall model: %v", err)
	}
}

func TestPhase34NestedFunctionCallAndRuntimeSessionIsolationExternal(t *testing.T) {
	ref := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example", "double", bcg.NewSemanticVersion(1, 0, 0)))
	signature := mustData[bcg.FunctionSignature](t)(bcg.NewFunctionSignature([]bcg.ValueType{bcg.IntegerType}, bcg.IntegerType, false))
	requirement := mustData[bcg.FunctionRequirement](t)(bcg.NewFunctionRequirement(bcg.FunctionRequirementSpec{Ref: ref, Signature: signature}))
	inner := mustData[bcg.Expression](t)(bcg.CallFunction(ref, phase34Constant(t, bcg.Int64(2))))
	nested := mustData[bcg.Expression](t)(bcg.CallFunction(ref, inner))
	model, target, context := phase34DerivedModel(t, "nested-function", nested, bcg.IntegerType, requirement)

	var calls atomic.Int32
	runtimeA := mustData[bcg.Runtime](t)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(phase34Implementation{ref: ref, call: func(args []bcg.Value) (bcg.Value, error) {
		calls.Add(1)
		text, _ := args[0].IntegerString()
		if text == "2" {
			return bcg.Int64(4), nil
		}
		return bcg.Int64(8), nil
	}}))))
	result, err := runtimeA.Evaluate(model, target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := result.Value()
	integer, _ := value.IntegerString()
	if integer != "8" {
		t.Fatalf("nested FunctionCall = %s", integer)
	}

	builder := mustData[*bcg.DatasetBuilder](t)(bcg.NewDatasetBuilder(model, bcg.NewID()))
	dataset := mustData[bcg.Dataset](t)(builder.Build())
	snapshot := mustData[bcg.EvaluationSnapshot](t)(runtimeA.WithDataset(model, dataset))
	session := mustData[*bcg.Session](t)(runtimeA.NewSession(snapshot, context))
	for i := 0; i < 2; i++ {
		if _, err := session.Evaluate(target); err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 4 { // one direct nested evaluation and one cached session evaluation
		t.Fatalf("FunctionCall cache calls = %d, want 4", got)
	}

	var group sync.WaitGroup
	for i := 0; i < 24; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			concurrent, err := session.Evaluate(target)
			if err != nil {
				t.Errorf("concurrent Session.Evaluate: %v", err)
				return
			}
			value, _ := concurrent.Value()
			actual, _ := value.IntegerString()
			if actual != "8" {
				t.Errorf("concurrent FunctionCall = %s", actual)
			}
		}()
	}
	group.Wait()
	if got := calls.Load(); got != 4 {
		t.Fatalf("warm Session recomputed FunctionCall %d times", got)
	}

	runtimeB := mustData[bcg.Runtime](t)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(phase34Implementation{ref: ref, call: func([]bcg.Value) (bcg.Value, error) {
		return bcg.Int64(99), nil
	}}))))
	other, err := runtimeB.Evaluate(model, target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ = other.Value()
	integer, _ = value.IntegerString()
	if integer != "99" {
		t.Fatalf("runtime B result = %s", integer)
	}
	stillA, err := session.Evaluate(target)
	if err != nil {
		t.Fatal(err)
	}
	value, _ = stillA.Value()
	integer, _ = value.IntegerString()
	if integer != "8" {
		t.Fatalf("session cache crossed runtimes: %s", integer)
	}

	otherRef := mustData[bcg.FunctionRef](t)(bcg.NewFunctionRef("example", "other", bcg.NewSemanticVersion(1, 0, 0)))
	incompatible := mustData[bcg.Runtime](t)(bcg.NewRuntime(mustData[bcg.FunctionRegistry](t)(bcg.NewFunctionRegistry(phase34Implementation{ref: otherRef, call: func([]bcg.Value) (bcg.Value, error) { return bcg.Int64(0), nil }}))))
	if _, err := incompatible.NewSession(snapshot, context); !errors.Is(err, bcg.ErrIncompatibleRuntime) {
		t.Fatalf("incompatible runtime session error = %v", err)
	}
}

// The frozen public aggregate fixture has two outgoing relations. Replacing
// its one derived declaration keeps traversal, dependency extraction, Explain
// and impact on the ordinary public path while exercising v2 aggregates.
func phase34AggregateModel(t testing.TB, expression bcg.Expression, resultType bcg.ValueType) (bcg.Model, bcg.PropertyAddress, bcg.CalculationContext) {
	t.Helper()
	artifact, err := bcg.Unmarshal([]byte(publicAggregateArtifact))
	if err != nil {
		t.Fatal(err)
	}
	model := artifact.Model()
	draft, err := model.Draft()
	if err != nil {
		t.Fatal(err)
	}
	rootType := bcg.MustParseID("00000000-0000-4000-8000-000000001003")
	total := bcg.MustParseID("00000000-0000-4000-8000-000000001010")
	root := bcg.MustParseID("00000000-0000-4000-8000-000000001011")
	owner := mustData[bcg.PropertyOwner](t)(bcg.EntityTypeOwner(rootType))
	computation := mustData[bcg.PropertyComputation](t)(bcg.DerivedComputation(expression))
	definition := mustData[bcg.PropertyDefinition](t)(bcg.NewPropertyDefinition(bcg.PropertyDefinitionSpec{ID: total, Owner: owner, Name: "total", ValueType: resultType, TemporalMode: bcg.TemporalScalar, Computation: computation}))
	if err := draft.ReplacePropertyDefinition(total, definition); err != nil {
		t.Fatal(err)
	}
	committed, err := draft.Commit(mustCommitOptions(t, bcg.NewID(), "aggregate-v2", mustPolicy(t)))
	if err != nil {
		t.Fatal(err)
	}
	target := mustData[bcg.PropertyAddress](t)(bcg.EntityProperty(root, total))
	at := mustData[bcg.DateTime](t)(bcg.ParseDateTime("2026-01-01T00:00:00Z"))
	context := mustData[bcg.CalculationContext](t)(bcg.NewCalculationContextAt(at))
	return committed, target, context
}

func TestPhase34QuantifiedAndDistinctAggregatesExternal(t *testing.T) {
	relationType := bcg.MustParseID("00000000-0000-4000-8000-000000001005")
	quantity := bcg.MustParseID("00000000-0000-4000-8000-000000001009")
	outgoing := mustData[bcg.TraversalSource](t)(bcg.SelfTraversal(bcg.TraverseOutgoing, relationType))
	quantityRef := mustData[bcg.Expression](t)(bcg.BoundProperty(bcg.BoundRelation, quantity))
	truth := mustData[bcg.Expression](t)(bcg.Equal(quantityRef, quantityRef))
	falseFilter := phase34Constant(t, bcg.Bool(false))

	for _, test := range []struct {
		name       string
		expression bcg.Expression
		want       bool
	}{
		{"exists", mustData[bcg.Expression](t)(bcg.Exists(outgoing)), true},
		{"exists-filtered-empty", mustData[bcg.Expression](t)(bcg.FilteredExists(outgoing, falseFilter)), false},
		{"any", mustData[bcg.Expression](t)(bcg.Any(outgoing, truth)), true},
		{"all", mustData[bcg.Expression](t)(bcg.All(outgoing, truth)), true},
		{"any-filtered-empty", mustData[bcg.Expression](t)(bcg.FilteredAny(outgoing, falseFilter, truth)), false},
		{"all-filtered-empty", mustData[bcg.Expression](t)(bcg.FilteredAll(outgoing, falseFilter, truth)), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := reconstructExpression(test.expression); err != nil {
				t.Fatalf("aggregate reconstruction: %v", err)
			}
			model, target, context := phase34AggregateModel(t, test.expression, bcg.BoolType)
			result, err := model.Evaluate(target, context)
			if err != nil {
				t.Fatal(err)
			}
			value, ok := result.Value()
			if !ok {
				t.Fatalf("%s was not Defined", test.name)
			}
			actual, ok := value.Bool()
			if !ok || actual != test.want {
				t.Fatalf("%s = %v, %v", test.name, actual, ok)
			}
		})
	}

	duplicates := mustData[bcg.Expression](t)(bcg.DistinctCount(outgoing, phase34Constant(t, bcg.Int64(7))))
	model, target, context := phase34AggregateModel(t, duplicates, bcg.IntegerType)
	result, err := model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := result.Value()
	count, _ := value.IntegerString()
	if count != "1" {
		t.Fatalf("DistinctCount duplicate values = %s", count)
	}
	empty := mustData[bcg.Expression](t)(bcg.FilteredDistinctCount(outgoing, falseFilter, phase34Constant(t, bcg.Int64(7))))
	model, target, context = phase34AggregateModel(t, empty, bcg.IntegerType)
	result, err = model.Evaluate(target, context)
	if err != nil {
		t.Fatal(err)
	}
	value, _ = result.Value()
	count, _ = value.IntegerString()
	if count != "0" {
		t.Fatalf("DistinctCount empty = %s", count)
	}
}
