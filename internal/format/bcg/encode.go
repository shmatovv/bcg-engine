package bcg

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/function"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
	"github.com/shmatovv/bcg-engine/v2/internal/types/rational"
	"github.com/shmatovv/bcg-engine/v2/internal/types/series"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// SemanticChange is an internal-format neutral description of one changed
// persistent declaration.  Values are compared as typed wire DTOs, never by
// their JSON representation.  Key is used only for declarations without a
// UUID (bindings and function requirements).
type SemanticChange struct {
	Kind        string
	ID          string
	Key         string
	Operation   string
	Fields      []string
	OldFeatures []string
	NewFeatures []string
}

func marshalCanonical(document wireDocument) ([]byte, error) {
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal canonical JSON: %w", ErrWireSemantic, err)
	}
	return data, nil
}

// SemanticProjection returns canonical model semantics without lifecycle and
// diagnostic metadata. It is deliberately separate from Artifact wire bytes.
func SemanticProjection(version model.ModelVersion) ([]byte, error) {
	w, err := wireVersionFromDomain(version)
	if err != nil {
		return nil, err
	}
	w.ID, w.ModelID, w.Label, w.ParentVersionID = "", "", "", nil
	for i := range w.EntityTypes {
		w.EntityTypes[i].Name = ""
	}
	for i := range w.RelationTypes {
		w.RelationTypes[i].Name = ""
	}
	for i := range w.Traits {
		w.Traits[i].Name = ""
	}
	for i := range w.Contracts {
		w.Contracts[i].Name = ""
	}
	for i := range w.Properties {
		w.Properties[i].Name = ""
	}
	for i := range w.Scopes {
		w.Scopes[i].Name = ""
	}
	for i := range w.Entities {
		w.Entities[i].Name = nil
	}
	for i := range w.EnumDefinitions {
		w.EnumDefinitions[i].Name = ""
		for j := range w.EnumDefinitions[i].Options {
			w.EnumDefinitions[i].Options[j].Name = ""
		}
	}
	for i := range w.EntityKeys {
		w.EntityKeys[i].Name = ""
	}
	for i := range w.Invariants {
		w.Invariants[i].Name = ""
	}
	for i := range w.Constraints {
		w.Constraints[i].Name = ""
	}
	for i := range w.Rules {
		w.Rules[i].Name = ""
		w.Rules[i].Metadata = nil
	}
	return json.Marshal(w)
}

// CompareSemanticVersions compares every persistent declaration in two
// committed model snapshots. It deliberately reuses the canonical wire DTOs:
// they are lossless, typed semantic values, and already normalize unordered
// collections. JSON is used only to derive stable keys for UUID-less bindings.
func CompareSemanticVersions(oldVersion, newVersion model.ModelVersion) ([]SemanticChange, error) {
	oldWire, err := wireVersionFromDomain(oldVersion)
	if err != nil {
		return nil, err
	}
	newWire, err := wireVersionFromDomain(newVersion)
	if err != nil {
		return nil, err
	}
	oldRecords, err := semanticRecords(oldWire)
	if err != nil {
		return nil, err
	}
	newRecords, err := semanticRecords(newWire)
	if err != nil {
		return nil, err
	}
	oldByKey := make(map[string]semanticRecord, len(oldRecords))
	newByKey := make(map[string]semanticRecord, len(newRecords))
	keys := make([]string, 0, len(oldRecords)+len(newRecords))
	for _, r := range oldRecords {
		oldByKey[r.mapKey()] = r
		keys = append(keys, r.mapKey())
	}
	for _, r := range newRecords {
		if _, ok := oldByKey[r.mapKey()]; !ok {
			keys = append(keys, r.mapKey())
		}
		newByKey[r.mapKey()] = r
	}
	sort.Strings(keys)
	changes := make([]SemanticChange, 0)
	for _, key := range keys {
		o, oldOK := oldByKey[key]
		n, newOK := newByKey[key]
		if !oldOK {
			changes = append(changes, SemanticChange{Kind: n.kind, ID: n.id, Key: n.key, Operation: "Added", NewFeatures: semanticFeatures(n.value)})
			continue
		}
		if !newOK {
			changes = append(changes, SemanticChange{Kind: o.kind, ID: o.id, Key: o.key, Operation: "Removed", OldFeatures: semanticFeatures(o.value)})
			continue
		}
		if reflect.DeepEqual(o.value, n.value) {
			continue
		}
		changes = append(changes, SemanticChange{Kind: o.kind, ID: o.id, Key: o.key, Operation: "Modified", Fields: semanticFields(o.value, n.value), OldFeatures: semanticFeatures(o.value), NewFeatures: semanticFeatures(n.value)})
	}
	return changes, nil
}

type semanticRecord struct {
	kind, id, key string
	value         any
}

func (r semanticRecord) mapKey() string { return r.kind + "\x00" + r.id + "\x00" + r.key }

func semanticRecords(w wireVersion) ([]semanticRecord, error) {
	r := make([]semanticRecord, 0)
	add := func(kind, id string, value any) { r = append(r, semanticRecord{kind: kind, id: id, value: value}) }
	r = append(r, semanticRecord{kind: "PrecisionPolicy", key: "model", value: w.Precision})
	for _, x := range w.EntityTypes {
		add("EntityType", x.ID, x)
	}
	for _, x := range w.EnumDefinitions {
		y := x
		y.Options = nil
		add("EnumDefinition", x.ID, y)
		for _, option := range x.Options {
			add("EnumOption", option.ID, struct {
				DefinitionID string
				Option       wireEnumOption
			}{x.ID, option})
		}
	}
	for _, x := range w.EntityKeys {
		add("EntityKeyDefinition", x.ID, x)
	}
	for _, x := range w.RelationTypes {
		add("RelationType", x.ID, x)
	}
	for _, x := range w.Traits {
		add("Trait", x.ID, x)
	}
	for _, x := range w.Contracts {
		add("PropertyContract", x.ID, x)
	}
	for _, x := range w.Properties {
		add("PropertyDefinition", x.ID, x)
	}
	for _, x := range w.Scopes {
		add("ScopeDefinition", x.ID, x)
	}
	for _, x := range w.Entities {
		add("Entity", x.ID, x)
	}
	for _, x := range w.Relations {
		add("Relation", x.ID, x)
	}
	for _, x := range w.Constraints {
		add("Constraint", x.ID, x)
	}
	for _, x := range w.Rules {
		add("Rule", x.ID, x)
	}
	for _, x := range w.Invariants {
		add("Invariant", x.ID, x)
	}
	for _, x := range w.Bindings {
		identity := x
		identity.Value = wireBindingValue{}
		b, err := json.Marshal(identity)
		if err != nil {
			return nil, err
		}
		r = append(r, semanticRecord{kind: "ValueBinding", key: string(b), value: x})
	}
	for _, x := range w.FunctionRequirements {
		// FunctionRef identity includes its exact version. Length prefixes keep
		// dotted namespaces and names unambiguous, matching the runtime key.
		key := fmt.Sprintf("%d:%s%d:%s@%010d.%010d.%010d", len(x.Ref.Namespace), x.Ref.Namespace, len(x.Ref.Name), x.Ref.Name, x.Ref.Major, x.Ref.Minor, x.Ref.Patch)
		r = append(r, semanticRecord{kind: "FunctionRequirement", key: key, value: x})
	}
	return r, nil
}

func semanticFields(oldValue, newValue any) []string {
	o, n := reflect.ValueOf(oldValue), reflect.ValueOf(newValue)
	if o.Kind() != reflect.Struct || n.Kind() != reflect.Struct || o.Type() != n.Type() {
		return []string{"Semantic"}
	}
	fields := make([]string, 0)
	semanticFieldValues(o, n, "", &fields)
	return fields
}

func semanticFieldValues(oldValue, newValue reflect.Value, prefix string, fields *[]string) {
	for i := 0; i < oldValue.NumField(); i++ {
		o, n := oldValue.Field(i), newValue.Field(i)
		if reflect.DeepEqual(o.Interface(), n.Interface()) {
			continue
		}
		name := prefix + oldValue.Type().Field(i).Name
		if o.Kind() == reflect.Struct && n.Kind() == reflect.Struct {
			semanticFieldValues(o, n, name+".", fields)
			continue
		}
		*fields = append(*fields, name)
	}
}

func semanticFeatures(value any) []string {
	features := []string{}
	switch v := value.(type) {
	case wireProperty:
		if v.Computation.Kind == "input" {
			features = append(features, "input", "presence:"+v.Computation.Presence)
		}
		if v.Computation.Kind == "derived" {
			features = append(features, "derived")
		}
	case wireRelationType:
		features = append(features, "outgoing:"+v.OutgoingCardinality.Min, "incoming:"+v.IncomingCardinality.Min)
		if v.OutgoingCardinality.Max != nil {
			features = append(features, "outgoing-max:"+*v.OutgoingCardinality.Max)
		}
		if v.IncomingCardinality.Max != nil {
			features = append(features, "incoming-max:"+*v.IncomingCardinality.Max)
		}
	}
	return features
}

func newWireDocument(artifact Artifact) (wireDocument, error) {
	version, err := wireVersionFromDomain(artifact.version)
	if err != nil {
		return wireDocument{}, err
	}
	scenarios := artifact.scenarios
	resultScenarios := make([]wireScenario, len(scenarios))
	for index, candidate := range scenarios {
		encoded, err := wireScenarioFromDomain(candidate)
		if err != nil {
			return wireDocument{}, fmt.Errorf("scenarios[%d]: %w", index, err)
		}
		resultScenarios[index] = encoded
	}
	sort.Slice(resultScenarios, func(i, j int) bool { return resultScenarios[i].ID < resultScenarios[j].ID })
	return wireDocument{
		FormatVersion:       FormatVersion,
		EngineCompatibility: EngineCompatibilityV2,
		Artifact: wireArtifact{
			Model:     wireModel{ID: artifact.version.ModelID().String(), Name: artifact.modelName},
			Version:   version,
			Scenarios: resultScenarios,
		},
	}, nil
}

func wireVersionFromDomain(version model.ModelVersion) (wireVersion, error) {
	parent, hasParent := version.ParentVersionID()
	result := wireVersion{
		ID:                   version.ID().String(),
		ModelID:              version.ModelID().String(),
		Label:                version.VersionLabel(),
		Precision:            wirePrecisionFromDomain(version.PrecisionPolicy()),
		EntityTypes:          make([]wireEntityType, len(version.EntityTypes())),
		EnumDefinitions:      make([]wireEnumDefinition, len(version.EnumDefinitions())),
		EntityKeys:           make([]wireEntityKey, len(version.EntityKeyDefinitions())),
		RelationTypes:        make([]wireRelationType, len(version.RelationTypes())),
		Traits:               make([]wireTrait, len(version.Traits())),
		Contracts:            make([]wireContract, len(version.PropertyContracts())),
		Properties:           make([]wireProperty, len(version.PropertyDefinitions())),
		Scopes:               make([]wireScope, len(version.ScopeDefinitions())),
		Entities:             make([]wireEntity, len(version.Entities())),
		Relations:            make([]wireRelation, len(version.Relations())),
		Bindings:             make([]wireBinding, len(version.ValueBindings())),
		Constraints:          make([]wireConstraint, len(version.Constraints())),
		Rules:                make([]wireRule, len(version.Rules())),
		Invariants:           make([]wireInvariant, len(version.Invariants())),
		FunctionRequirements: make([]wireFunctionRequirement, len(version.FunctionRequirements())),
	}
	if hasParent {
		value := parent.String()
		result.ParentVersionID = &value
	}

	for index, item := range version.EntityTypes() {
		result.EntityTypes[index] = wireEntityTypeFromDomain(item)
	}
	for index, item := range version.EnumDefinitions() {
		result.EnumDefinitions[index] = wireEnumDefinitionFromDomain(item)
	}
	for index, item := range version.EntityKeyDefinitions() {
		result.EntityKeys[index] = wireEntityKeyFromDomain(item)
	}
	for index, item := range version.RelationTypes() {
		encoded, err := wireRelationTypeFromDomain(item)
		if err != nil {
			return wireVersion{}, err
		}
		result.RelationTypes[index] = encoded
	}
	for index, item := range version.Traits() {
		result.Traits[index] = wireTraitFromDomain(item)
	}
	for index, item := range version.PropertyContracts() {
		result.Contracts[index] = wireContractFromDomain(item)
	}
	for index, item := range version.PropertyDefinitions() {
		encoded, err := wirePropertyFromDomain(item)
		if err != nil {
			return wireVersion{}, fmt.Errorf("propertyDefinitions[%d]: %w", index, err)
		}
		result.Properties[index] = encoded
	}
	for index, item := range version.ScopeDefinitions() {
		result.Scopes[index] = wireScopeFromDomain(item)
	}
	for index, item := range version.Entities() {
		result.Entities[index] = wireEntityFromDomain(item)
	}
	for index, item := range version.Relations() {
		encoded, err := wireRelationFromDomain(item)
		if err != nil {
			return wireVersion{}, fmt.Errorf("relations[%d]: %w", index, err)
		}
		result.Relations[index] = encoded
	}
	for index, item := range version.ValueBindings() {
		encoded, err := wireBindingFromDomain(item)
		if err != nil {
			return wireVersion{}, fmt.Errorf("valueBindings[%d]: %w", index, err)
		}
		result.Bindings[index] = encoded
	}
	for index, item := range version.Constraints() {
		encoded, err := wireConstraintFromDomain(item)
		if err != nil {
			return wireVersion{}, fmt.Errorf("constraints[%d]: %w", index, err)
		}
		result.Constraints[index] = encoded
	}
	for index, item := range version.Rules() {
		encoded, err := wireRuleFromDomain(item)
		if err != nil {
			return wireVersion{}, fmt.Errorf("rules[%d]: %w", index, err)
		}
		result.Rules[index] = encoded
	}
	for index, item := range version.Invariants() {
		encoded, err := wireInvariantFromDomain(item)
		if err != nil {
			return wireVersion{}, err
		}
		result.Invariants[index] = encoded
	}
	for index, item := range version.FunctionRequirements() {
		encoded, err := wireFunctionRequirementFromDomain(item)
		if err != nil {
			return wireVersion{}, err
		}
		result.FunctionRequirements[index] = encoded
	}

	sort.Slice(result.EntityTypes, func(i, j int) bool { return result.EntityTypes[i].ID < result.EntityTypes[j].ID })
	sort.Slice(result.EnumDefinitions, func(i, j int) bool { return result.EnumDefinitions[i].ID < result.EnumDefinitions[j].ID })
	sort.Slice(result.EntityKeys, func(i, j int) bool { return result.EntityKeys[i].ID < result.EntityKeys[j].ID })
	sort.Slice(result.RelationTypes, func(i, j int) bool { return result.RelationTypes[i].ID < result.RelationTypes[j].ID })
	sort.Slice(result.Traits, func(i, j int) bool { return result.Traits[i].ID < result.Traits[j].ID })
	sort.Slice(result.Contracts, func(i, j int) bool { return result.Contracts[i].ID < result.Contracts[j].ID })
	sort.Slice(result.Properties, func(i, j int) bool { return result.Properties[i].ID < result.Properties[j].ID })
	sort.Slice(result.Scopes, func(i, j int) bool { return result.Scopes[i].ID < result.Scopes[j].ID })
	sort.Slice(result.Entities, func(i, j int) bool { return result.Entities[i].ID < result.Entities[j].ID })
	sort.Slice(result.Relations, func(i, j int) bool { return result.Relations[i].ID < result.Relations[j].ID })
	sortWireByJSON(result.Bindings)
	sort.Slice(result.Constraints, func(i, j int) bool { return result.Constraints[i].ID < result.Constraints[j].ID })
	sort.Slice(result.Rules, func(i, j int) bool { return result.Rules[i].ID < result.Rules[j].ID })
	sort.Slice(result.Invariants, func(i, j int) bool { return result.Invariants[i].ID < result.Invariants[j].ID })
	sort.Slice(result.FunctionRequirements, func(i, j int) bool {
		a, b := result.FunctionRequirements[i].Ref, result.FunctionRequirements[j].Ref
		return fmt.Sprintf("%d:%s%d:%s@%010d.%010d.%010d", len(a.Namespace), a.Namespace, len(a.Name), a.Name, a.Major, a.Minor, a.Patch) < fmt.Sprintf("%d:%s%d:%s@%010d.%010d.%010d", len(b.Namespace), b.Namespace, len(b.Name), b.Name, b.Major, b.Minor, b.Patch)
	})
	return result, nil
}

func wireEnumDefinitionFromDomain(d schema.EnumDefinition) wireEnumDefinition {
	r := wireEnumDefinition{ID: d.ID().String(), Name: d.Name(), Options: make([]wireEnumOption, len(d.Options()))}
	for i, o := range d.Options() {
		r.Options[i] = wireEnumOption{ID: o.ID().String(), Name: o.Name()}
	}
	sort.Slice(r.Options, func(i, j int) bool { return r.Options[i].ID < r.Options[j].ID })
	return r
}
func wireEntityKeyFromDomain(k schema.EntityKeyDefinition) wireEntityKey {
	return wireEntityKey{ID: k.ID().String(), Name: k.Name(), EntityTypeID: k.EntityTypeID().String(), PropertyDefinitionIDs: func() []string {
		ids := k.PropertyDefinitionIDs()
		r := make([]string, len(ids))
		for i, id := range ids {
			r[i] = id.String()
		}
		return r
	}()}
}
func wireCardinalityFromDomain(c schema.Cardinality) wireCardinality {
	r := wireCardinality{Min: strconv.FormatUint(c.Min(), 10)}
	if m, ok := c.Max(); ok {
		s := strconv.FormatUint(m, 10)
		r.Max = &s
	}
	return r
}
func wireInvariantFromDomain(i invariant.Invariant) (wireInvariant, error) {
	a, e := wireExpressionFromDomain(i.Assertion())
	if e != nil {
		return wireInvariant{}, e
	}
	r := wireInvariant{ID: i.ID().String(), Name: i.Name(), Subject: wireInvariantSubject{TypeID: i.Subject().TypeID().String()}, Assertion: a}
	if i.Subject().Kind() == invariant.SubjectEntityType {
		r.Subject.Kind = "entity_type"
	} else {
		r.Subject.Kind = "relation_type"
	}
	if x, ok := i.Applicability(); ok {
		q, e := wireExpressionFromDomain(x)
		if e != nil {
			return wireInvariant{}, e
		}
		r.Applicability = &q
	}
	return r, nil
}
func wireFunctionRequirementFromDomain(r function.Requirement) (wireFunctionRequirement, error) {
	x := wireFunctionRequirement{Ref: wireFunctionRef{Namespace: r.Ref().Namespace, Name: r.Ref().Name, Major: r.Ref().Version.Major, Minor: r.Ref().Version.Minor, Patch: r.Ref().Version.Patch}, Parameters: []wireFunctionType{}, NullableResult: r.Signature().NullableResult()}
	for _, p := range r.Signature().Parameters() {
		q, e := wireFunctionTypeFromDomain(p)
		if e != nil {
			return x, e
		}
		x.Parameters = append(x.Parameters, q)
	}
	q, e := wireFunctionTypeFromDomain(r.Signature().Result())
	x.Result = q
	return x, e
}
func wireFunctionTypeFromDomain(t typing.Type) (wireFunctionType, error) {
	r := wireFunctionType{ValueType: lowerEnum(t.ValueType().String()), TemporalMode: lowerEnum(t.TemporalMode().String()), Nullable: t.Nullable()}
	if id, ok := t.EnumDefinitionID(); ok {
		s := id.String()
		r.EnumDefinitionID = &s
	}
	if u, ok := t.Unit(); ok {
		q, e := wireUnitFromDomain(u)
		if e != nil {
			return r, e
		}
		r.Unit = &q
	}
	return r, nil
}

func wirePrecisionFromDomain(policy precision.Policy) wirePrecision {
	return wirePrecision{Scale: strconv.FormatUint(policy.Scale(), 10), Rounding: lowerEnum(policy.Rounding().String())}
}

func wireEntityTypeFromDomain(item schema.EntityType) wireEntityType {
	return wireEntityType{ID: item.ID().String(), Name: item.Name(), Traits: sortedIDs(item.Traits()), PropertyDefinitions: sortedIDs(item.PropertyDefinitions())}
}

func wireRelationTypeFromDomain(item schema.RelationType) (wireRelationType, error) {
	from, hasFrom := item.From()
	to, hasTo := item.To()
	result := wireRelationType{ID: item.ID().String(), Name: item.Name(), PropertyDefinitions: sortedIDs(item.PropertyDefinitions())}
	o, in := wireCardinalityFromDomain(item.OutgoingCardinality()), wireCardinalityFromDomain(item.IncomingCardinality())
	result.OutgoingCardinality, result.IncomingCardinality = &o, &in
	if hasFrom {
		encoded, err := wireTypeConstraintFromDomain(from)
		if err != nil {
			return wireRelationType{}, err
		}
		result.From = &encoded
	}
	if hasTo {
		encoded, err := wireTypeConstraintFromDomain(to)
		if err != nil {
			return wireRelationType{}, err
		}
		result.To = &encoded
	}
	return result, nil
}

func wireTypeConstraintFromDomain(item schema.TypeConstraint) (wireTypeConstraint, error) {
	result := wireTypeConstraint{Kind: lowerEnum(item.Kind().String())}
	if id, ok := item.ID(); ok {
		value := id.String()
		result.ID = &value
	}
	if result.Kind == "invalid" {
		return wireTypeConstraint{}, fmt.Errorf("%w: invalid relation endpoint constraint", ErrWireSemantic)
	}
	return result, nil
}

func wireTraitFromDomain(item schema.Trait) wireTrait {
	return wireTrait{ID: item.ID().String(), Name: item.Name(), RequiredContracts: sortedIDs(item.RequiredPropertyContracts())}
}

func wireContractFromDomain(item schema.PropertyContract) wireContract {
	dimension, hasDimension := item.Dimension()
	result := wireContract{ID: item.ID().String(), Name: item.Name(), ValueType: lowerEnum(item.ValueType().String()), TemporalMode: lowerEnum(item.TemporalMode().String()), Nullable: boolPointer(item.Nullable())}
	if hasDimension {
		encoded := wireDimensionFromDomain(dimension)
		result.Dimension = &encoded
	}
	if id, ok := item.EnumDefinitionID(); ok {
		x := id.String()
		result.EnumDefinitionID = &x
	}
	return result
}

func wirePropertyFromDomain(item schema.PropertyDefinition) (wireProperty, error) {
	result := wireProperty{ID: item.ID().String(), Owner: wireOwnerFromDomain(item.Owner()), Name: item.Name(), ValueType: lowerEnum(item.ValueType().String()), TemporalMode: lowerEnum(item.TemporalMode().String())}
	if contractID, ok := item.ContractID(); ok {
		value := contractID.String()
		result.ContractID = &value
	}
	if propertyUnit, ok := item.Unit(); ok {
		encoded, err := wireUnitFromDomain(propertyUnit)
		if err != nil {
			return wireProperty{}, err
		}
		result.Unit = &encoded
	}
	computation, err := wireComputationFromDomain(item.Computation())
	if err != nil {
		return wireProperty{}, err
	}
	result.Computation = computation
	if id, ok := item.EnumDefinitionID(); ok {
		x := id.String()
		result.EnumDefinitionID = &x
	}
	return result, nil
}

func wireOwnerFromDomain(item schema.PropertyOwner) wireOwner {
	return wireOwner{Kind: lowerEnum(item.Kind().String()), ID: item.ID().String()}
}
func wireScopeFromDomain(item scope.Definition) wireScope {
	return wireScope{ID: item.ID().String(), Name: item.Name(), ValueType: lowerEnum(item.ValueType().String())}
}

func wireEntityFromDomain(item semantic.Entity) wireEntity {
	result := wireEntity{ID: item.ID().String(), TypeID: item.TypeID().String()}
	if name, ok := item.Name(); ok {
		result.Name = &name
	}
	return result
}

func wireRelationFromDomain(item semantic.Relation) (wireRelation, error) {
	result := wireRelation{ID: item.ID().String(), TypeID: item.TypeID().String(), From: item.From().String(), To: item.To().String()}
	if expression, ok := item.Applicability(); ok {
		encoded, err := wireExpressionFromDomain(expression)
		if err != nil {
			return wireRelation{}, err
		}
		result.Applicability = &encoded
	}
	return result, nil
}

func wireBindingFromDomain(item semantic.ValueBinding) (wireBinding, error) {
	target, err := wireTargetFromDomain(item.Target())
	if err != nil {
		return wireBinding{}, err
	}
	bindingValue, err := wireBindingValueFromDomain(item.Value())
	if err != nil {
		return wireBinding{}, err
	}
	selectors, err := wireSelectorsFromDomain(item.ScopeSelectors())
	if err != nil {
		return wireBinding{}, err
	}
	result := wireBinding{Target: target, Value: bindingValue, ScopeSelectors: selectors}
	if period, ok := item.EffectivePeriod(); ok {
		encoded, err := wireEffectivePeriodFromDomain(period)
		if err != nil {
			return wireBinding{}, err
		}
		result.EffectivePeriod = &encoded
	}
	return result, nil
}

func wireTargetFromDomain(item semantic.ValueTarget) (wireTarget, error) {
	switch item.Kind() {
	case semantic.ValueTargetObjectProperty:
		addressValue, _ := item.ObjectProperty()
		encoded := wireAddressFromDomain(addressValue)
		return wireTarget{Kind: "object_property", Address: &encoded}, nil
	case semantic.ValueTargetTypeProperty:
		owner, definitionID, _ := item.TypeProperty()
		encodedOwner := wireOwnerFromDomain(owner)
		id := definitionID.String()
		return wireTarget{Kind: "type_property", Owner: &encodedOwner, DefinitionID: &id}, nil
	default:
		return wireTarget{}, fmt.Errorf("%w: invalid value target", ErrWireSemantic)
	}
}

func wireBindingValueFromDomain(item semantic.BindingValue) (wireBindingValue, error) {
	switch item.Kind() {
	case semantic.BindingValueDefined:
		v, _ := item.Value()
		encoded, err := wireTypedFromDomain(v)
		if err != nil {
			return wireBindingValue{}, err
		}
		return wireBindingValue{Kind: "defined", Value: &encoded}, nil
	case semantic.BindingValueNull:
		return wireBindingValue{Kind: "null"}, nil
	case semantic.BindingValueSeries:
		v, _ := item.Series()
		encoded, err := wireSeriesFromDomain(v)
		if err != nil {
			return wireBindingValue{}, err
		}
		return wireBindingValue{Kind: "series", Series: &encoded}, nil
	default:
		return wireBindingValue{}, fmt.Errorf("%w: invalid binding value", ErrWireSemantic)
	}
}

func wireSelectorsFromDomain(items []scope.Selector) ([]wireSelector, error) {
	result := make([]wireSelector, len(items))
	for index, item := range items {
		encoded, err := wireTypedFromDomain(item.Value())
		if err != nil {
			return nil, err
		}
		result[index] = wireSelector{DefinitionID: item.DefinitionID().String(), Value: encoded}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DefinitionID < result[j].DefinitionID })
	return result, nil
}

func wireEffectivePeriodFromDomain(item domaintime.EffectivePeriod) (wireEffectivePeriod, error) {
	result := wireEffectivePeriod{}
	if from, ok := item.From(); ok {
		value := from.String()
		result.From = &value
	}
	if to, ok := item.To(); ok {
		value := to.String()
		result.To = &value
	}
	return result, nil
}

func wireConstraintFromDomain(item constraint.Constraint) (wireConstraint, error) {
	assertion, err := wireExpressionFromDomain(item.Assertion())
	if err != nil {
		return wireConstraint{}, err
	}
	result := wireConstraint{ID: item.ID().String(), Name: item.Name(), Assertion: assertion, Severity: lowerEnum(item.Severity().String())}
	if expression, ok := item.Applicability(); ok {
		encoded, err := wireExpressionFromDomain(expression)
		if err != nil {
			return wireConstraint{}, err
		}
		result.Applicability = &encoded
	}
	if message, ok := item.Message(); ok {
		result.Message = &message
	}
	return result, nil
}

func wireRuleFromDomain(item rule.Rule) (wireRule, error) {
	condition, err := wireExpressionFromDomain(item.Condition())
	if err != nil {
		return wireRule{}, err
	}
	metadata := item.Metadata()
	entries := make([]wireMetadata, 0, len(metadata))
	for key, value := range metadata {
		entries = append(entries, wireMetadata{Key: key, Value: value})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return wireRule{ID: item.ID().String(), Name: item.Name(), Condition: condition, Metadata: entries}, nil
}

func wireScenarioFromDomain(item scenario.Scenario) (wireScenario, error) {
	overrides := item.Overrides()
	result := wireScenario{ID: item.ID().String(), Name: item.Name(), BaseModelVersionID: item.BaseModelVersionID().String(), Overrides: make([]wireOverride, len(overrides))}
	for index, override := range overrides {
		encoded, err := wireOverrideFromDomain(override)
		if err != nil {
			return wireScenario{}, err
		}
		result.Overrides[index] = encoded
	}
	sortWireByJSON(result.Overrides)
	return result, nil
}

func wireOverrideFromDomain(item scenario.Override) (wireOverride, error) {
	bindingValue, err := wireBindingValueFromDomain(item.Value())
	if err != nil {
		return wireOverride{}, err
	}
	selectors, err := wireSelectorsFromDomain(item.ScopeSelectors())
	if err != nil {
		return wireOverride{}, err
	}
	result := wireOverride{Target: wireAddressFromDomain(item.Target()), Value: bindingValue, ScopeSelectors: selectors}
	if period, ok := item.EffectivePeriod(); ok {
		encoded, err := wireEffectivePeriodFromDomain(period)
		if err != nil {
			return wireOverride{}, err
		}
		result.EffectivePeriod = &encoded
	}
	return result, nil
}

func wireAddressFromDomain(item address.PropertyAddress) wireAddress {
	return wireAddress{Object: wireObjectRefFromDomain(item.Object()), DefinitionID: item.DefinitionID().String()}
}
func wireObjectRefFromDomain(item address.ObjectRef) wireObjectRef {
	return wireObjectRef{Kind: lowerEnum(item.Kind().String()), ID: item.ID().String()}
}

func wireTypedFromDomain(item value.Typed) (wireTyped, error) {
	raw, err := wireRawFromDomain(item.Raw())
	if err != nil {
		return wireTyped{}, err
	}
	result := wireTyped{Type: raw.Type, Data: raw.Data, EnumDefinitionID: raw.EnumDefinitionID, EnumOptionID: raw.EnumOptionID}
	if item.HasUnit() {
		encoded, err := wireUnitFromDomain(item.Unit())
		if err != nil {
			return wireTyped{}, err
		}
		result.Unit = &encoded
	}
	return result, nil
}

func wireRawFromDomain(item value.Raw) (wireRaw, error) {
	switch item.Type() {
	case value.Bool:
		return wireRaw{Type: "bool", Data: strconv.FormatBool(item.Bool())}, nil
	case value.Integer:
		return wireRaw{Type: "integer", Data: item.Integer().String()}, nil
	case value.Decimal:
		return wireRaw{Type: "decimal", Data: item.Decimal().String()}, nil
	case value.String:
		return wireRaw{Type: "string", Data: item.StringValue()}, nil
	case value.Date:
		return wireRaw{Type: "date", Data: item.Date().String()}, nil
	case value.DateTime:
		return wireRaw{Type: "datetime", Data: item.DateTime().String()}, nil
	case value.Duration:
		return wireRaw{Type: "duration", Data: item.Duration().String()}, nil
	case value.Enum:
		d, _ := item.EnumDefinitionID()
		o, _ := item.EnumOptionID()
		return wireRaw{Type: "enum", EnumDefinitionID: d.String(), EnumOptionID: o.String()}, nil
	default:
		return wireRaw{}, fmt.Errorf("%w: unsupported raw value type", ErrWireSemantic)
	}
}

func wireSeriesFromDomain(item series.Series) (wireSeries, error) {
	result := wireSeries{Axis: lowerEnum(item.AxisKind().String()), ElementType: lowerEnum(item.ElementType().String()), Points: []wirePoint{}, Intervals: []wireInterval{}}
	if item.HasUnit() {
		encoded, err := wireUnitFromDomain(item.Unit())
		if err != nil {
			return wireSeries{}, err
		}
		result.Unit = &encoded
	}
	for _, point := range item.Points() {
		sample, err := wireSampleFromDomain(point.Sample())
		if err != nil {
			return wireSeries{}, err
		}
		result.Points = append(result.Points, wirePoint{At: point.At().String(), Sample: sample})
	}
	for _, interval := range item.Intervals() {
		sample, err := wireSampleFromDomain(interval.Sample())
		if err != nil {
			return wireSeries{}, err
		}
		result.Intervals = append(result.Intervals, wireInterval{From: interval.From().String(), To: interval.To().String(), Sample: sample})
	}
	return result, nil
}

func wireSampleFromDomain(item series.Sample) (wireSeriesSample, error) {
	if item.IsNull() {
		return wireSeriesSample{Null: boolPointer(true)}, nil
	}
	v, ok := item.Value()
	if !ok {
		return wireSeriesSample{}, fmt.Errorf("%w: invalid series sample", ErrWireSemantic)
	}
	encoded, err := wireRawFromDomain(v)
	if err != nil {
		return wireSeriesSample{}, err
	}
	return wireSeriesSample{Null: boolPointer(false), Value: &encoded}, nil
}

func wireDimensionFromDomain(item unit.Dimension) wireDimension {
	factors := item.Factors()
	result := wireDimension{Factors: make([]wireDimensionFactor, len(factors))}
	for index, factor := range factors {
		result.Factors[index] = wireDimensionFactor{ID: string(factor.ID), Exponent: factor.Exponent}
	}
	return result
}

func wireUnitFromDomain(item unit.Unit) (wireUnit, error) {
	if !item.IsValid() {
		return wireUnit{}, fmt.Errorf("%w: invalid unit", ErrWireSemantic)
	}
	result := wireUnit{Dimension: wireDimensionFromDomain(item.Dimension()), Factors: []wireUnitFactor{}, ConversionFactors: []wireUnitFactor{}, Scale: wireRationalFromDomain(item.Scale())}
	for _, factor := range item.Factors() {
		result.Factors = append(result.Factors, wireUnitFactor{ID: string(factor.ID), Exponent: factor.Exponent})
	}
	for _, factor := range item.ConversionFactors() {
		result.ConversionFactors = append(result.ConversionFactors, wireUnitFactor{ID: string(factor.ID), Exponent: factor.Exponent})
	}
	return result, nil
}

func wireRationalFromDomain(item rational.Rational) wireRational {
	return wireRational{Numerator: item.Numerator().String(), Denominator: item.Denominator().String()}
}

func wireComputationFromDomain(item schema.Computation) (wireComputation, error) {
	switch item.Kind() {
	case schema.ComputationInput:
		p, _ := item.Presence()
		presence := "optional"
		if p == schema.PresenceRequired {
			presence = "required"
		}
		return wireComputation{Kind: "input", Presence: presence}, nil
	case schema.ComputationDerived:
		expression, _ := item.Expression()
		encoded, err := wireExpressionFromDomain(expression)
		if err != nil {
			return wireComputation{}, err
		}
		return wireComputation{Kind: "derived", Expression: &encoded}, nil
	default:
		return wireComputation{}, fmt.Errorf("%w: invalid computation", ErrWireSemantic)
	}
}

func wireExpressionFromDomain(expression ast.Expression) (wireExpression, error) {
	switch node := expression.(type) {
	case ast.Constant:
		value, err := wireTypedFromDomain(node.Value())
		if err != nil {
			return wireExpression{}, err
		}
		return wireExpression{Kind: "constant", Value: &value}, nil
	case *ast.Constant:
		return wireExpressionFromDomain(*node)
	case ast.Arithmetic:
		return wireBinaryExpression("arithmetic", lowerEnum(node.Operator().String()), node.Left(), node.Right())
	case *ast.Arithmetic:
		return wireExpressionFromDomain(*node)
	case ast.Negate:
		return wireUnaryExpression("negate", node.Operand())
	case *ast.Negate:
		return wireExpressionFromDomain(*node)
	case ast.Comparison:
		return wireBinaryExpression("comparison", lowerEnum(node.Operator().String()), node.Left(), node.Right())
	case *ast.Comparison:
		return wireExpressionFromDomain(*node)
	case ast.Logical:
		return wireBinaryExpression("logical", lowerEnum(node.Operator().String()), node.Left(), node.Right())
	case *ast.Logical:
		return wireExpressionFromDomain(*node)
	case ast.Not:
		return wireUnaryExpression("not", node.Operand())
	case *ast.Not:
		return wireExpressionFromDomain(*node)
	case ast.If:
		condition, err := wireExpressionFromDomain(node.Condition())
		if err != nil {
			return wireExpression{}, err
		}
		thenExpr, err := wireExpressionFromDomain(node.Then())
		if err != nil {
			return wireExpression{}, err
		}
		elseExpr, err := wireExpressionFromDomain(node.Else())
		if err != nil {
			return wireExpression{}, err
		}
		return wireExpression{Kind: "if", Condition: &condition, Then: &thenExpr, Else: &elseExpr}, nil
	case *ast.If:
		return wireExpressionFromDomain(*node)
	case ast.IsNull:
		return wireUnaryExpression("is_null", node.Operand())
	case *ast.IsNull:
		return wireExpressionFromDomain(*node)
	case ast.Coalesce:
		primary, err := wireExpressionFromDomain(node.Primary())
		if err != nil {
			return wireExpression{}, err
		}
		fallback, err := wireExpressionFromDomain(node.Fallback())
		if err != nil {
			return wireExpression{}, err
		}
		return wireExpression{Kind: "coalesce", Primary: &primary, Fallback: &fallback}, nil
	case *ast.Coalesce:
		return wireExpressionFromDomain(*node)
	case ast.SelfPropertyReference:
		return wireExpression{Kind: "self_property", DefinitionID: node.DefinitionID().String()}, nil
	case *ast.SelfPropertyReference:
		return wireExpressionFromDomain(*node)
	case ast.AbsolutePropertyReference:
		addressValue := wireAddressFromDomain(node.Address())
		return wireExpression{Kind: "absolute_property", Address: &addressValue}, nil
	case *ast.AbsolutePropertyReference:
		return wireExpressionFromDomain(*node)
	case ast.BoundPropertyDefinitionReference:
		return wireExpression{Kind: "bound_property", Target: lowerEnum(node.Target().String()), DefinitionID: node.DefinitionID().String()}, nil
	case *ast.BoundPropertyDefinitionReference:
		return wireExpressionFromDomain(*node)
	case ast.BoundContractPropertyReference:
		return wireExpression{Kind: "bound_contract_property", Target: lowerEnum(node.Target().String()), ContractID: node.ContractID().String()}, nil
	case *ast.BoundContractPropertyReference:
		return wireExpressionFromDomain(*node)
	case ast.Aggregate:
		return wireAggregateExpression(node)
	case *ast.Aggregate:
		return wireAggregateExpression(*node)
	case ast.Unary:
		q, e := wireUnaryExpression("unary", node.Operand())
		q.Operator = lowerEnum(node.Operator().String())
		return q, e
	case *ast.Unary:
		return wireExpressionFromDomain(*node)
	case ast.Binary:
		return wireBinaryExpression("binary", lowerEnum(node.Operator().String()), node.Left(), node.Right())
	case *ast.Binary:
		return wireExpressionFromDomain(*node)
	case ast.Variadic:
		r := wireExpression{Kind: "variadic", Operator: lowerEnum(node.Operator().String())}
		for _, v := range node.Operands() {
			q, e := wireExpressionFromDomain(v)
			if e != nil {
				return r, e
			}
			r.Operands = append(r.Operands, q)
		}
		return r, nil
	case *ast.Variadic:
		return wireExpressionFromDomain(*node)
	case ast.FunctionCall:
		r := wireExpression{Kind: "function_call", Function: &wireFunctionRef{Namespace: node.Ref().Namespace, Name: node.Ref().Name, Major: node.Ref().Version.Major, Minor: node.Ref().Version.Minor, Patch: node.Ref().Version.Patch}}
		for _, v := range node.Arguments() {
			q, e := wireExpressionFromDomain(v)
			if e != nil {
				return r, e
			}
			r.Arguments = append(r.Arguments, q)
		}
		return r, nil
	case *ast.FunctionCall:
		return wireExpressionFromDomain(*node)
	default:
		return wireExpression{}, fmt.Errorf("%w: unsupported AST node %T", ErrWireSemantic, expression)
	}
}

func wireUnaryExpression(kind string, operand ast.Expression) (wireExpression, error) {
	encoded, err := wireExpressionFromDomain(operand)
	if err != nil {
		return wireExpression{}, err
	}
	return wireExpression{Kind: kind, Operand: &encoded}, nil
}
func wireBinaryExpression(kind, operator string, left, right ast.Expression) (wireExpression, error) {
	encodedLeft, err := wireExpressionFromDomain(left)
	if err != nil {
		return wireExpression{}, err
	}
	encodedRight, err := wireExpressionFromDomain(right)
	if err != nil {
		return wireExpression{}, err
	}
	return wireExpression{Kind: kind, Operator: operator, Left: &encodedLeft, Right: &encodedRight}, nil
}

func wireAggregateExpression(node ast.Aggregate) (wireExpression, error) {
	source, err := wireTraversalSourceFromDomain(node.Source())
	if err != nil {
		return wireExpression{}, err
	}
	result := wireExpression{Kind: "aggregate", Source: &source, Operation: lowerEnum(node.Operation().String())}
	if filter, ok := node.Filter(); ok {
		encoded, err := wireExpressionFromDomain(filter)
		if err != nil {
			return wireExpression{}, err
		}
		result.Filter = &encoded
	}
	if mapExpr, ok := node.Map(); ok {
		encoded, err := wireExpressionFromDomain(mapExpr)
		if err != nil {
			return wireExpression{}, err
		}
		result.Map = &encoded
	}
	return result, nil
}

func wireTraversalSourceFromDomain(node ast.TraversalSource) (wireTraversalSource, error) {
	result := wireTraversalSource{Origin: lowerEnum(node.Origin().String()), Direction: lowerEnum(node.Direction().String()), RelationTypeID: node.RelationTypeID().String()}
	if origin, ok := node.AbsoluteOrigin(); ok {
		encoded := wireObjectRefFromDomain(origin)
		result.AbsoluteOrigin = &encoded
	}
	if result.Origin == "invalid" || result.Direction == "invalid" {
		return wireTraversalSource{}, fmt.Errorf("%w: invalid traversal source", ErrWireSemantic)
	}
	return result, nil
}

func sortedIDs(items []identity.UUID) []string {
	result := make([]string, len(items))
	for index, item := range items {
		result[index] = item.String()
	}
	sort.Strings(result)
	return result
}

func boolPointer(value bool) *bool { return &value }
func lowerEnum(input string) string {
	result := make([]byte, 0, len(input)+4)
	for index, byteValue := range input {
		if byteValue >= 'A' && byteValue <= 'Z' {
			if index != 0 {
				result = append(result, '_')
			}
			result = append(result, byte(byteValue+'a'-'A'))
			continue
		}
		result = append(result, byte(byteValue))
	}
	return string(result)
}

func sortWireByJSON[T any](items []T) {
	sort.SliceStable(items, func(i, j int) bool {
		left, _ := json.Marshal(items[i])
		right, _ := json.Marshal(items[j])
		return string(left) < string(right)
	})
}
