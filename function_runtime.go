package bcg

import (
	"crypto/sha256"
	"fmt"
	"sort"

	constraintcheck "github.com/shmatovv/bcg-engine/v2/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/v2/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	internalfunction "github.com/shmatovv/bcg-engine/v2/internal/function"
	"github.com/shmatovv/bcg-engine/v2/internal/impact"
	ruleeval "github.com/shmatovv/bcg-engine/v2/internal/rule/eval"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// SemanticVersion is an exact portable extension-function version.
type SemanticVersion struct{ major, minor, patch uint32 }

// MaxFunctionArguments is the maximum signature and call arity accepted by
// Engine v2. It bounds host-controlled allocation during artifact decode; it
// is not a business-language recommendation.
const MaxFunctionArguments = internalfunction.MaxArguments

func NewSemanticVersion(major, minor, patch uint32) SemanticVersion {
	return SemanticVersion{major, minor, patch}
}
func (v SemanticVersion) Major() uint32  { return v.major }
func (v SemanticVersion) Minor() uint32  { return v.minor }
func (v SemanticVersion) Patch() uint32  { return v.patch }
func (v SemanticVersion) String() string { return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) }
func (v SemanticVersion) internal() internalfunction.Version {
	return internalfunction.Version{Major: v.major, Minor: v.minor, Patch: v.patch}
}
func semanticVersionFromInternal(v internalfunction.Version) SemanticVersion {
	return NewSemanticVersion(v.Major, v.Minor, v.Patch)
}

// FunctionRef is an exact namespace/name/version identity.
type FunctionRef struct {
	namespace, name string
	version         SemanticVersion
}

func NewFunctionRef(namespace, name string, version SemanticVersion) (FunctionRef, error) {
	ref, err := internalfunction.NewRef(namespace, name, version.internal())
	if err != nil {
		return FunctionRef{}, fmt.Errorf("%w: function namespace and name are required", ErrInvalidArgument)
	}
	return functionRefFromInternal(ref), nil
}
func (r FunctionRef) Namespace() string        { return r.namespace }
func (r FunctionRef) Name() string             { return r.name }
func (r FunctionRef) Version() SemanticVersion { return r.version }
func (r FunctionRef) IsValid() bool            { _, err := r.internal(); return err == nil }
func (r FunctionRef) String() string {
	ref, err := r.internal()
	if err != nil {
		return ""
	}
	return ref.String()
}
func (r FunctionRef) key() string {
	ref, err := r.internal()
	if err != nil {
		return ""
	}
	return ref.Key()
}
func (r FunctionRef) internal() (internalfunction.Ref, error) {
	ref, err := internalfunction.NewRef(r.namespace, r.name, r.version.internal())
	if err != nil {
		return internalfunction.Ref{}, fmt.Errorf("%w: function ref is invalid", ErrInvalidArgument)
	}
	return ref, nil
}
func functionRefFromInternal(ref internalfunction.Ref) FunctionRef {
	return FunctionRef{namespace: ref.Namespace, name: ref.Name, version: semanticVersionFromInternal(ref.Version)}
}

// FunctionValueType is a fully-qualified scalar function type. Enum identity
// and exact Unit are semantic parts of a function contract.
type FunctionValueType struct {
	valueType      ValueType
	enumDefinition ID
	hasEnum        bool
	unit           Unit
	hasUnit        bool
}

func NewFunctionValueType(valueType ValueType, enumDefinitionID ID, unitValue *Unit) (FunctionValueType, error) {
	if !valueType.IsValid() {
		return FunctionValueType{}, fmt.Errorf("%w: function value type is invalid", ErrInvalidArgument)
	}
	result := FunctionValueType{valueType: valueType}
	if valueType == EnumType {
		if enumDefinitionID.IsZero() {
			return FunctionValueType{}, fmt.Errorf("%w: Enum function type requires an EnumDefinition ID", ErrInvalidArgument)
		}
		result.enumDefinition, result.hasEnum = enumDefinitionID, true
	} else if !enumDefinitionID.IsZero() {
		return FunctionValueType{}, fmt.Errorf("%w: non-Enum function type must not have an EnumDefinition ID", ErrInvalidArgument)
	}
	if unitValue != nil {
		if !unitValue.IsValid() {
			return FunctionValueType{}, fmt.Errorf("%w: function type unit is invalid", ErrInvalidArgument)
		}
		result.unit, result.hasUnit = *unitValue, true
	}
	return result, nil
}
func simpleFunctionValueType(valueType ValueType) FunctionValueType {
	value, _ := NewFunctionValueType(valueType, ID{}, nil)
	return value
}
func (t FunctionValueType) ValueType() ValueType         { return t.valueType }
func (t FunctionValueType) EnumDefinitionID() (ID, bool) { return t.enumDefinition, t.hasEnum }
func (t FunctionValueType) Unit() (Unit, bool)           { return t.unit, t.hasUnit }
func (t FunctionValueType) IsValid() bool                { _, err := t.internal(); return err == nil }
func (t FunctionValueType) internal() (typing.Type, error) {
	kind, err := t.valueType.internal()
	if err != nil {
		return typing.Type{}, err
	}
	if kind == enginevalue.Enum {
		if !t.hasEnum {
			return typing.Type{}, fmt.Errorf("%w: Enum function type requires an EnumDefinition ID", ErrInvalidArgument)
		}
		id, err := t.enumDefinition.internal()
		if err != nil {
			return typing.Type{}, err
		}
		return typing.NewQualifiedEnum(id, t.unit.value, t.hasUnit, typing.TemporalScalar, false), nil
	}
	if t.hasEnum {
		return typing.Type{}, fmt.Errorf("%w: non-Enum function type has an EnumDefinition ID", ErrInvalidArgument)
	}
	return typing.NewQualified(kind, t.unit.value, t.hasUnit, typing.TemporalScalar, false), nil
}
func functionValueTypeFromInternal(value typing.Type) FunctionValueType {
	result := FunctionValueType{valueType: valueTypeFromInternal(value.ValueType())}
	if id, ok := value.EnumDefinitionID(); ok {
		result.enumDefinition, result.hasEnum = idFromInternal(id), true
	}
	if unitValue, ok := value.Unit(); ok {
		result.unit, result.hasUnit = Unit{value: unitValue}, true
	}
	return result
}

// FunctionSignature describes parameter and result type contracts. The
// original constructor remains for unitless non-Enum signatures; use
// NewQualifiedFunctionSignature where identity or unit contracts matter.
type FunctionSignature struct{ value internalfunction.Signature }

func NewFunctionSignature(parameters []ValueType, result ValueType, nullable bool) (FunctionSignature, error) {
	if len(parameters) > MaxFunctionArguments {
		return FunctionSignature{}, fmt.Errorf("%w: function parameter count exceeds %d", ErrInvalidArgument, MaxFunctionArguments)
	}
	parameterContracts := make([]FunctionValueType, len(parameters))
	for index, parameter := range parameters {
		parameterContracts[index] = simpleFunctionValueType(parameter)
		if !parameterContracts[index].IsValid() {
			return FunctionSignature{}, fmt.Errorf("%w: invalid function parameter type", ErrInvalidArgument)
		}
	}
	resultContract := simpleFunctionValueType(result)
	if !resultContract.IsValid() {
		return FunctionSignature{}, fmt.Errorf("%w: invalid function result type", ErrInvalidArgument)
	}
	return NewQualifiedFunctionSignature(parameterContracts, resultContract, nullable)
}
func NewQualifiedFunctionSignature(parameters []FunctionValueType, result FunctionValueType, nullable bool) (FunctionSignature, error) {
	if len(parameters) > MaxFunctionArguments {
		return FunctionSignature{}, fmt.Errorf("%w: function parameter count exceeds %d", ErrInvalidArgument, MaxFunctionArguments)
	}
	internalParameters := make([]typing.Type, len(parameters))
	for index, parameter := range parameters {
		value, err := parameter.internal()
		if err != nil {
			return FunctionSignature{}, fmt.Errorf("%w: invalid function parameter type", ErrInvalidArgument)
		}
		internalParameters[index] = value
	}
	internalResult, err := result.internal()
	if err != nil {
		return FunctionSignature{}, fmt.Errorf("%w: invalid function result type", ErrInvalidArgument)
	}
	signature, err := internalfunction.NewSignature(internalParameters, internalResult, nullable)
	if err != nil {
		return FunctionSignature{}, fmt.Errorf("%w: invalid function signature", ErrInvalidArgument)
	}
	return FunctionSignature{value: signature}, nil
}
func (s FunctionSignature) ParameterTypes() []ValueType {
	contracts := s.ParameterContracts()
	result := make([]ValueType, len(contracts))
	for index, contract := range contracts {
		result[index] = contract.ValueType()
	}
	return result
}
func (s FunctionSignature) ParameterContracts() []FunctionValueType {
	values := s.value.Parameters()
	result := make([]FunctionValueType, len(values))
	for index, value := range values {
		result[index] = functionValueTypeFromInternal(value)
	}
	return result
}
func (s FunctionSignature) ResultType() ValueType {
	return functionValueTypeFromInternal(s.value.Result()).ValueType()
}
func (s FunctionSignature) ResultContract() FunctionValueType {
	return functionValueTypeFromInternal(s.value.Result())
}
func (s FunctionSignature) NullableResult() bool { return s.value.NullableResult() }
func (s FunctionSignature) IsValid() bool        { return s.value.IsValid() }

type FunctionRequirementSpec struct {
	Ref       FunctionRef
	Signature FunctionSignature
}
type FunctionRequirement struct{ value internalfunction.Requirement }

func NewFunctionRequirement(spec FunctionRequirementSpec) (FunctionRequirement, error) {
	ref, err := spec.Ref.internal()
	if err != nil || !spec.Signature.IsValid() {
		return FunctionRequirement{}, fmt.Errorf("%w: invalid function requirement", ErrInvalidArgument)
	}
	requirement, err := internalfunction.NewRequirement(ref, spec.Signature.value)
	if err != nil {
		return FunctionRequirement{}, fmt.Errorf("%w: invalid function requirement", ErrInvalidArgument)
	}
	return FunctionRequirement{value: requirement}, nil
}
func (r FunctionRequirement) Ref() FunctionRef { return functionRefFromInternal(r.value.Ref()) }
func (r FunctionRequirement) Signature() FunctionSignature {
	return FunctionSignature{value: r.value.Signature()}
}
func (r FunctionRequirement) IsValid() bool { return r.value.IsValid() }
func functionRequirementFromInternal(value internalfunction.Requirement) FunctionRequirement {
	return FunctionRequirement{value: value}
}

// AddFunctionRequirement declares a portable semantic contract used by one or
// more FunctionCall expressions. Commit canonicalizes the model's exposed
// requirements to the exact subset reachable from persistent expressions.
func (d *Draft) AddFunctionRequirement(requirement FunctionRequirement) error {
	if d == nil || !d.IsValid() || !requirement.IsValid() {
		return fmt.Errorf("%w: valid Draft and FunctionRequirement are required", ErrInvalidArgument)
	}
	d.value.AddFunctionRequirement(requirement.value)
	return nil
}

// FunctionRequirements returns the Draft declarations in owned form.
func (d *Draft) FunctionRequirements() []FunctionRequirement {
	if d == nil || !d.IsValid() {
		return nil
	}
	values := d.value.FunctionRequirements()
	result := make([]FunctionRequirement, len(values))
	for index, value := range values {
		result[index] = functionRequirementFromInternal(value)
	}
	return result
}

// FunctionRequirements returns the deterministic, unique requirements
// actually implied by this committed model's expression graph.
func (m Model) FunctionRequirements() []FunctionRequirement {
	if !m.IsValid() {
		return nil
	}
	values := m.version.FunctionRequirements()
	result := make([]FunctionRequirement, len(values))
	for index, value := range values {
		result[index] = functionRequirementFromInternal(value)
	}
	return result
}

// FunctionResult is the explicit outcome of an extension callback. Implement
// BoundFunctionResult to return Null; legacy BoundFunction implementations
// remain valid and return Defined values only. Undefined remains an Engine
// propagation state (for example from an Undefined argument), not a callback
// business result.
type FunctionResult struct {
	state     ResultState
	value     Value
	undefined UndefinedKind
}

func DefinedFunctionResult(value Value) (FunctionResult, error) {
	if !value.IsValid() {
		return FunctionResult{}, fmt.Errorf("%w: function result Value is invalid", ErrInvalidArgument)
	}
	return FunctionResult{state: Defined, value: value}, nil
}
func NullFunctionResult() FunctionResult { return FunctionResult{state: Null} }
func UndefinedFunctionResult(kind UndefinedKind) (FunctionResult, error) {
	if !undefinedKindValid(kind) {
		return FunctionResult{}, fmt.Errorf("%w: function Undefined kind is invalid", ErrInvalidArgument)
	}
	return FunctionResult{state: Undefined, undefined: kind}, nil
}
func (r FunctionResult) State() ResultState   { return r.state }
func (r FunctionResult) Value() (Value, bool) { return r.value, r.state == Defined }
func (r FunctionResult) Undefined() (UndefinedInfo, bool) {
	return UndefinedInfo{kind: r.undefined}, r.state == Undefined
}
func (r FunctionResult) IsValid() bool {
	if r.state == Defined {
		return r.value.IsValid()
	}
	if r.state == Null {
		return !r.value.IsValid() && r.undefined == ""
	}
	return r.state == Undefined && undefinedKindValid(r.undefined) && !r.value.IsValid()
}
func undefinedKindValid(kind UndefinedKind) bool {
	switch kind {
	case MissingValue, AmbiguousValue, DependencyUndefined, EmptyAggregation, IndeterminateCondition, InvalidOperation, ExternalUnavailable, DivisionByZero:
		return true
	default:
		return false
	}
}

// BoundFunction receives only defined Values and must be pure and concurrent-safe.
type BoundFunction interface {
	Call(args []Value) (Value, error)
}

// BoundFunctionResult is the optional extended callback result contract.
type BoundFunctionResult interface {
	CallResult(args []Value) (FunctionResult, error)
}
type FunctionImplementation interface {
	Ref() FunctionRef
	Bind(requirement FunctionRequirement) (BoundFunction, error)
}
type FunctionRegistry struct {
	implementations map[string]FunctionImplementation
	fingerprint     [32]byte
}

func NewFunctionRegistry(implementations ...FunctionImplementation) (FunctionRegistry, error) {
	m := make(map[string]FunctionImplementation, len(implementations))
	keys := make([]string, 0, len(implementations))
	for _, impl := range implementations {
		if impl == nil || !impl.Ref().IsValid() {
			return FunctionRegistry{}, fmt.Errorf("%w: invalid function implementation", ErrInvalidArgument)
		}
		key := impl.Ref().key()
		if _, ok := m[key]; ok {
			return FunctionRegistry{}, fmt.Errorf("%w: duplicate function %s", ErrInvalidArgument, impl.Ref())
		}
		m[key], keys = impl, append(keys, key)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, key := range keys {
		h.Write([]byte(key))
		h.Write([]byte{0})
	}
	var fingerprint [32]byte
	copy(fingerprint[:], h.Sum(nil))
	return FunctionRegistry{implementations: m, fingerprint: fingerprint}, nil
}
func (r FunctionRegistry) Lookup(ref FunctionRef) (FunctionImplementation, bool) {
	value, ok := r.implementations[ref.key()]
	return value, ok
}
func (r FunctionRegistry) Fingerprint() []byte { return append([]byte(nil), r.fingerprint[:]...) }
func (r FunctionRegistry) IsValid() bool       { return r.implementations != nil }

// Runtime is immutable and supplies host-bound implementations. Its registry
// fingerprint is runtime cache identity, never model semantic identity.
type Runtime struct{ registry FunctionRegistry }

func NewRuntime(registry FunctionRegistry) (Runtime, error) {
	if !registry.IsValid() {
		return Runtime{}, fmt.Errorf("%w: invalid function registry", ErrInvalidArgument)
	}
	return Runtime{registry: registry}, nil
}
func DefaultRuntime() Runtime {
	registry, _ := NewFunctionRegistry()
	return Runtime{registry: registry}
}
func (r Runtime) Registry() FunctionRegistry { return r.registry }
func (r Runtime) IsValid() bool              { return r.registry.IsValid() }
func (r Runtime) Fingerprint() []byte        { return r.registry.Fingerprint() }

// ValidateModel verifies that this Runtime can bind every exact semantic
// FunctionRequirement in model. It does not execute a function.
func (r Runtime) ValidateModel(model Model) error {
	if !r.IsValid() || !model.IsValid() {
		return fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	for _, requirement := range model.FunctionRequirements() {
		if _, err := r.Bind(requirement); err != nil {
			return err
		}
	}
	return nil
}

// Evaluate evaluates through this explicit immutable Runtime. A FunctionCall
// is never resolved through package-global registration.
func (r Runtime) Evaluate(model Model, target PropertyAddress, context CalculationContext) (Result, error) {
	if !r.IsValid() {
		return Result{}, fmt.Errorf("%w: runtime is invalid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return Result{}, err
	}
	version, addressValue, contextValue, err := model.request(target, context)
	if err != nil {
		return Result{}, err
	}
	result, err := evaluator.Evaluate(evaluator.BindRuntime(version, r), addressValue, contextValue)
	if err != nil {
		return Result{}, publicError(err)
	}
	return resultFromInternal(result), nil
}

// Explain evaluates through this Runtime and returns ordinary Engine
// provenance. FunctionCall contributes only its argument dependencies.
func (r Runtime) Explain(model Model, target PropertyAddress, context CalculationContext) (Explanation, error) {
	if !r.IsValid() {
		return Explanation{}, fmt.Errorf("%w: runtime is invalid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return Explanation{}, err
	}
	version, addressValue, contextValue, err := model.request(target, context)
	if err != nil {
		return Explanation{}, err
	}
	result, err := evaluator.Explain(evaluator.BindRuntime(version, r), addressValue, contextValue)
	if err != nil {
		return Explanation{}, publicError(err)
	}
	return explanationFromInternal(result), nil
}

// AnalyzeImpact uses this Runtime for observed evaluation. Structural and
// effective modes share the same runtime-bound source for one API contract.
func (r Runtime) AnalyzeImpact(model Model, mode ImpactMode, source PropertyAddress, context CalculationContext) (ImpactResult, error) {
	if !r.IsValid() || !model.IsValid() {
		return ImpactResult{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return ImpactResult{}, err
	}
	addressValue, err := source.internal()
	if err != nil {
		return ImpactResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return ImpactResult{}, err
	}
	if err := model.validateContext(contextValue); err != nil {
		return ImpactResult{}, err
	}
	runtimeSource := evaluator.BindRuntime(model.version, r)
	var result impact.Result
	switch mode {
	case StructuralImpact:
		result, err = impact.AnalyzeStructuralSource(runtimeSource, addressValue)
	case EffectiveImpact:
		result, err = impact.AnalyzeEffectiveSource(runtimeSource, addressValue, contextValue)
	case ObservedImpact:
		result, err = impact.AnalyzeObservedSource(runtimeSource, addressValue, contextValue)
	default:
		return ImpactResult{}, fmt.Errorf("%w: impact mode is invalid", ErrInvalidArgument)
	}
	if err != nil {
		return ImpactResult{}, publicError(err)
	}
	return impactResultFromInternal(result), nil
}

// WithDataset binds this Runtime to an immutable Dataset snapshot. The
// returned snapshot owns that binding for all later evaluations and Sessions.
func (r Runtime) WithDataset(model Model, dataset Dataset) (EvaluationSnapshot, error) {
	if !r.IsValid() {
		return EvaluationSnapshot{}, fmt.Errorf("%w: runtime is invalid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return EvaluationSnapshot{}, err
	}
	snapshot, err := model.WithDataset(dataset)
	if err != nil {
		return EvaluationSnapshot{}, err
	}
	snapshot.runtime = r
	return snapshot, nil
}

func (r Runtime) CheckConstraint(model Model, id ID, context CalculationContext) (ConstraintResult, error) {
	if !r.IsValid() || !model.IsValid() {
		return ConstraintResult{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return ConstraintResult{}, err
	}
	constraintID, err := id.internal()
	if err != nil {
		return ConstraintResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return ConstraintResult{}, err
	}
	if err := model.validateContext(contextValue); err != nil {
		return ConstraintResult{}, err
	}
	result, err := constraintcheck.One(evaluator.BindRuntime(model.version, r), constraintID, contextValue)
	if err != nil {
		return ConstraintResult{}, publicError(err)
	}
	return constraintResultFromInternal(result), nil
}
func (r Runtime) CheckConstraints(model Model, context CalculationContext) (ConstraintReport, error) {
	if !r.IsValid() || !model.IsValid() {
		return ConstraintReport{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return ConstraintReport{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return ConstraintReport{}, err
	}
	if err := model.validateContext(contextValue); err != nil {
		return ConstraintReport{}, err
	}
	report, err := constraintcheck.All(evaluator.BindRuntime(model.version, r), contextValue)
	if err != nil {
		return ConstraintReport{}, publicError(err)
	}
	return constraintReportFromInternal(report), nil
}
func (r Runtime) CheckFeasibility(model Model, context CalculationContext) (FeasibilityResult, error) {
	if !r.IsValid() || !model.IsValid() {
		return FeasibilityResult{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return FeasibilityResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return FeasibilityResult{}, err
	}
	if err := model.validateContext(contextValue); err != nil {
		return FeasibilityResult{}, err
	}
	result, err := feasibility.Evaluate(evaluator.BindRuntime(model.version, r), contextValue)
	if err != nil {
		return FeasibilityResult{}, publicError(err)
	}
	return feasibilityResultFromInternal(result), nil
}
func (r Runtime) EvaluateRule(model Model, id ID, context CalculationContext) (RuleResult, error) {
	if !r.IsValid() || !model.IsValid() {
		return RuleResult{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return RuleResult{}, err
	}
	ruleID, err := id.internal()
	if err != nil {
		return RuleResult{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return RuleResult{}, err
	}
	if err := model.validateContext(contextValue); err != nil {
		return RuleResult{}, err
	}
	result, err := ruleeval.One(evaluator.BindRuntime(model.version, r), ruleID, contextValue)
	if err != nil {
		return RuleResult{}, publicError(err)
	}
	return ruleResultFromInternal(result), nil
}
func (r Runtime) EvaluateRules(model Model, context CalculationContext) (RuleReport, error) {
	if !r.IsValid() || !model.IsValid() {
		return RuleReport{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return RuleReport{}, err
	}
	contextValue, err := context.internal()
	if err != nil {
		return RuleReport{}, err
	}
	if err := model.validateContext(contextValue); err != nil {
		return RuleReport{}, err
	}
	report, err := ruleeval.All(evaluator.BindRuntime(model.version, r), contextValue)
	if err != nil {
		return RuleReport{}, publicError(err)
	}
	return ruleReportFromInternal(report), nil
}

// CheckInvariant evaluates an Entity-subject invariant through this Runtime.
func (r Runtime) CheckInvariant(model Model, id ID, subject Entity, context CalculationContext) (InvariantResult, error) {
	if !r.IsValid() || !model.IsValid() {
		return InvariantResult{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return InvariantResult{}, err
	}
	ctx, err := context.internal()
	if err != nil {
		return InvariantResult{}, err
	}
	return checkInvariantObjectSource(evaluator.BindRuntime(model.version, r), id.value, address.EntityRef(subject.value.ID()), ctx)
}

// CheckInvariantForRelation evaluates a Relation-subject invariant through
// this Runtime.
func (r Runtime) CheckInvariantForRelation(model Model, id ID, subject Relation, context CalculationContext) (InvariantResult, error) {
	if !r.IsValid() || !model.IsValid() {
		return InvariantResult{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return InvariantResult{}, err
	}
	ctx, err := context.internal()
	if err != nil {
		return InvariantResult{}, err
	}
	return checkInvariantObjectSource(evaluator.BindRuntime(model.version, r), id.value, address.RelationRef(subject.value.ID()), ctx)
}

// CheckInvariants evaluates all invariants applicable to one Entity through
// this Runtime.
func (r Runtime) CheckInvariantsForEntity(model Model, subject Entity, context CalculationContext) (InvariantReport, error) {
	if !r.IsValid() || !model.IsValid() {
		return InvariantReport{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return InvariantReport{}, err
	}
	ctx, err := context.internal()
	if err != nil {
		return InvariantReport{}, err
	}
	return checkInvariantsForObjectSource(evaluator.BindRuntime(model.version, r), address.EntityRef(subject.value.ID()), ctx)
}

// CheckInvariantsForRelation evaluates all invariants applicable to one
// Relation through this Runtime.
func (r Runtime) CheckInvariantsForRelation(model Model, subject Relation, context CalculationContext) (InvariantReport, error) {
	if !r.IsValid() || !model.IsValid() {
		return InvariantReport{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return InvariantReport{}, err
	}
	ctx, err := context.internal()
	if err != nil {
		return InvariantReport{}, err
	}
	return checkInvariantsForObjectSource(evaluator.BindRuntime(model.version, r), address.RelationRef(subject.value.ID()), ctx)
}

// CheckInvariants evaluates every invariant in deterministic subject and ID
// order through this Runtime.
func (r Runtime) CheckInvariants(model Model, context CalculationContext) (InvariantReport, error) {
	if !r.IsValid() || !model.IsValid() {
		return InvariantReport{}, fmt.Errorf("%w: runtime and model must be valid", ErrInvalidArgument)
	}
	if err := r.ValidateModel(model); err != nil {
		return InvariantReport{}, err
	}
	ctx, err := context.internal()
	if err != nil {
		return InvariantReport{}, err
	}
	return checkAllInvariantsSource(evaluator.BindRuntime(model.version, r), ctx)
}

// NewSession creates a cache whose source is permanently decorated with this
// exact Runtime. A session cannot be retargeted to another registry.
func (r Runtime) NewSession(snapshot EvaluationSnapshot, context CalculationContext) (*Session, error) {
	if !r.IsValid() || !snapshot.IsValid() {
		return nil, fmt.Errorf("%w: runtime and evaluation snapshot must be valid", ErrInvalidArgument)
	}
	if string(r.Fingerprint()) != string(snapshot.runtime.Fingerprint()) {
		return nil, fmt.Errorf("%w: snapshot is bound to a different runtime", ErrIncompatibleRuntime)
	}
	if err := r.ValidateModel(snapshot.model); err != nil {
		return nil, err
	}
	return snapshot.NewSession(context)
}
func (r Runtime) Bind(requirement FunctionRequirement) (bound BoundFunction, err error) {
	if !r.IsValid() {
		return nil, fmt.Errorf("%w: invalid runtime", ErrInvalidArgument)
	}
	if !requirement.IsValid() {
		return nil, fmt.Errorf("%w: invalid function requirement", ErrInvalidArgument)
	}
	implementation, ok := r.registry.Lookup(requirement.Ref())
	if !ok {
		return nil, fmt.Errorf("%w: required function %s is unavailable", ErrIncompatibleRuntime, requirement.Ref())
	}
	defer func() {
		if recover() != nil {
			bound, err = nil, fmt.Errorf("%w: function %s", ErrFunctionPanic, requirement.Ref())
		}
	}()
	bound, err = implementation.Bind(requirement)
	if err != nil || bound == nil {
		return nil, fmt.Errorf("%w: function %s is incompatible: %v", ErrIncompatibleRuntime, requirement.Ref(), err)
	}
	return bound, nil
}

// Invoke is the engine-bound callback gateway. It contains callback panics,
// validates all explicit result states, and never lets a callback manufacture
// an unchecked graph value.
func (r Runtime) Invoke(requirement internalfunction.Requirement, arguments []enginevalue.Typed) (resolved enginevalue.Resolved, err error) {
	publicRequirement := functionRequirementFromInternal(requirement)
	bound, err := r.Bind(publicRequirement)
	if err != nil {
		return enginevalue.Resolved{}, err
	}
	args := make([]Value, len(arguments))
	for index, argument := range arguments {
		args[index] = valueFromInternal(argument)
	}
	defer func() {
		if recover() != nil {
			resolved, err = enginevalue.Resolved{}, fmt.Errorf("%w: function %s", ErrFunctionPanic, publicRequirement.Ref())
		}
	}()
	var result FunctionResult
	if extended, ok := bound.(BoundFunctionResult); ok {
		result, err = extended.CallResult(args)
	} else {
		value, callErr := bound.Call(args)
		if callErr != nil {
			err = callErr
		} else {
			result, err = DefinedFunctionResult(value)
		}
	}
	if err != nil {
		return enginevalue.Resolved{}, fmt.Errorf("%w: function %s: %v", ErrFunctionExecution, publicRequirement.Ref(), err)
	}
	if !result.IsValid() {
		return enginevalue.Resolved{}, fmt.Errorf("%w: function %s returned an invalid result", ErrFunctionResultContract, publicRequirement.Ref())
	}
	switch result.State() {
	case Defined:
		value, _ := result.Value()
		internal, valueErr := value.internal()
		if valueErr != nil || !typedMatchesFunctionType(internal, requirement.Signature().Result()) {
			return enginevalue.Resolved{}, fmt.Errorf("%w: function %s returned a value outside its declared result contract", ErrFunctionResultContract, publicRequirement.Ref())
		}
		return enginevalue.Defined(internal), nil
	case Null:
		if !requirement.Signature().NullableResult() {
			return enginevalue.Resolved{}, fmt.Errorf("%w: function %s returned Null for a non-nullable result", ErrFunctionResultContract, publicRequirement.Ref())
		}
		return enginevalue.Null(), nil
	case Undefined:
		return enginevalue.Resolved{}, fmt.Errorf("%w: function %s returned Undefined", ErrFunctionResultContract, publicRequirement.Ref())
	default:
		return enginevalue.Resolved{}, fmt.Errorf("%w: function %s returned an invalid state", ErrFunctionResultContract, publicRequirement.Ref())
	}
}
func typedMatchesFunctionType(value enginevalue.Typed, expected typing.Type) bool {
	if !value.IsValid() || value.Type() != expected.ValueType() || value.HasUnit() != expected.HasUnit() {
		return false
	}
	if value.HasUnit() {
		unitValue, _ := expected.Unit()
		if !value.Unit().Equal(unitValue) {
			return false
		}
	}
	actualEnum, actualHasEnum := value.EnumDefinitionID()
	expectedEnum, expectedHasEnum := expected.EnumDefinitionID()
	return actualHasEnum == expectedHasEnum && (!actualHasEnum || actualEnum == expectedEnum)
}
