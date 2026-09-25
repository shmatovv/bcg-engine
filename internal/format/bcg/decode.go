package bcg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

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
	modelscenario "github.com/shmatovv/bcg-engine/v2/internal/scenario"
	"github.com/shmatovv/bcg-engine/v2/internal/types/date"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/v2/internal/types/duration"
	"github.com/shmatovv/bcg-engine/v2/internal/types/precision"
	"github.com/shmatovv/bcg-engine/v2/internal/types/rational"
	"github.com/shmatovv/bcg-engine/v2/internal/types/series"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

const (
	maxDocumentBytes = 64 << 20
	maxJSONDepth     = 2048
)

func readDocument(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: nil reader", ErrMalformedDocument)
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxDocumentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read document: %w", ErrMalformedDocument, err)
	}
	if len(data) > maxDocumentBytes {
		return nil, fmt.Errorf("%w: document exceeds %d byte limit", ErrMalformedDocument, maxDocumentBytes)
	}
	return data, nil
}

func parseDocument(data []byte) (wireDocument, error) {
	if len(data) > maxDocumentBytes {
		return wireDocument{}, fmt.Errorf("%w: document exceeds %d byte limit", ErrMalformedDocument, maxDocumentBytes)
	}
	if len(data) == 0 {
		return wireDocument{}, fmt.Errorf("%w: empty input", ErrMalformedDocument)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return wireDocument{}, fmt.Errorf("%w: UTF-8 BOM is not accepted", ErrMalformedDocument)
	}
	if !utf8.Valid(data) {
		return wireDocument{}, fmt.Errorf("%w: invalid UTF-8", ErrMalformedDocument)
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return wireDocument{}, err
	}
	if err := checkEnvelope(data); err != nil {
		return wireDocument{}, err
	}
	if err := validateRequiredWireFields(data, reflect.TypeFor[wireDocument](), ""); err != nil {
		return wireDocument{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document wireDocument
	if err := decoder.Decode(&document); err != nil {
		return wireDocument{}, fmt.Errorf("%w: %w", ErrMalformedDocument, err)
	}
	if err := requireDocumentEnvelope(document); err != nil {
		return wireDocument{}, err
	}
	return document, nil
}

// checkEnvelope deliberately precedes full payload validation: a future
// format version must be rejected as unsupported rather than diagnosed using
// v1 payload rules.
func checkEnvelope(data []byte) error {
	var envelope struct {
		FormatVersion       *int    `json:"formatVersion"`
		EngineCompatibility *string `json:"engineCompatibility"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("%w: envelope: %w", ErrMalformedDocument, err)
	}
	if envelope.FormatVersion == nil {
		return wireError("formatVersion", "is required")
	}
	if *envelope.FormatVersion != 1 && *envelope.FormatVersion != FormatVersion {
		return fmt.Errorf("%w: got %d, supported %d", ErrUnsupportedFormatVersion, *envelope.FormatVersion, FormatVersion)
	}
	if envelope.EngineCompatibility == nil {
		return wireError("engineCompatibility", "is required")
	}
	expected := EngineCompatibilityV2
	if *envelope.FormatVersion == 1 {
		expected = EngineCompatibilityV1
	}
	if *envelope.EngineCompatibility != expected {
		return fmt.Errorf("%w: got %q, supported %q", ErrUnsupportedEngineCompatibility, *envelope.EngineCompatibility, expected)
	}
	return nil
}

// validateRequiredWireFields rejects omitted and JSON-null required fields
// before Go zero values could accidentally turn omission into a semantic
// default. DTO fields with omitempty are the only variant-controlled fields;
// nullable pointers remain required on the wire and are encoded explicitly as
// JSON null when absent.
func validateRequiredWireFields(data []byte, typ reflect.Type, path string) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil {
			return wireError(pathOrDocument(path), "must be a JSON object")
		}
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			if field.PkgPath != "" {
				continue
			}
			name, options := jsonField(field)
			if name == "" || name == "-" {
				continue
			}
			raw, exists := object[name]
			fieldPath := joinPath(path, name)
			if !exists {
				if !options["omitempty"] {
					return wireError(fieldPath, "is required")
				}
				continue
			}
			if bytes.Equal(raw, []byte("null")) {
				if field.Type.Kind() != reflect.Pointer {
					return wireError(fieldPath, "must not be JSON null")
				}
				continue
			}
			if err := validateRequiredWireFields(raw, field.Type, fieldPath); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if typ.Kind() == reflect.Slice && bytes.Equal(data, []byte("null")) {
			return wireError(pathOrDocument(path), "must not be JSON null")
		}
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return wireError(pathOrDocument(path), "must be a JSON array")
		}
		for index, raw := range items {
			if err := validateRequiredWireFields(raw, typ.Elem(), fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	default:
		// json.Unmarshal below checks primitive JSON type compatibility. This
		// layer only needs to reject null for non-optional semantic fields.
		if bytes.Equal(data, []byte("null")) {
			return wireError(pathOrDocument(path), "must not be JSON null")
		}
	}
	return nil
}

func jsonField(field reflect.StructField) (string, map[string]bool) {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name, nil
	}
	parts := strings.Split(tag, ",")
	options := make(map[string]bool, len(parts)-1)
	for _, option := range parts[1:] {
		options[option] = true
	}
	return parts[0], options
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func pathOrDocument(path string) string {
	if path == "" {
		return "document"
	}
	return path
}

func requireDocumentEnvelope(document wireDocument) error {
	if document.FormatVersion != 1 && document.FormatVersion != FormatVersion {
		return fmt.Errorf("%w: got %d, supported %d", ErrUnsupportedFormatVersion, document.FormatVersion, FormatVersion)
	}
	expected := EngineCompatibilityV2
	if document.FormatVersion == 1 {
		expected = EngineCompatibilityV1
	}
	if document.EngineCompatibility != expected {
		return fmt.Errorf("%w: got %q, supported %q", ErrUnsupportedEngineCompatibility, document.EngineCompatibility, expected)
	}
	return nil
}

// rejectDuplicateJSONKeys makes v1 strict where encoding/json otherwise uses
// last-wins semantics. It also proves that exactly one JSON document exists.
func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, 0); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedDocument, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%w: trailing JSON document", ErrMalformedDocument)
		}
		return fmt.Errorf("%w: trailing data: %w", ErrMalformedDocument, err)
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return fmt.Errorf("JSON nesting exceeds %d", maxJSONDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("unterminated object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("unterminated array")
		}
	default:
		return fmt.Errorf("unexpected delimiter %q", delim)
	}
	return nil
}

func (document wireDocument) toArtifact() (artifact Artifact, err error) {
	// Domain constructors intentionally panic on programmer-invalid inputs.
	// Decode handles untrusted input, so conversion turns such a panic into a
	// wire error instead of leaking it across the boundary.
	defer func() {
		if recovered := recover(); recovered != nil {
			artifact = Artifact{}
			err = fmt.Errorf("%w: invalid domain construction: %v", ErrWireSemantic, recovered)
		}
	}()
	if err := requireDocumentEnvelope(document); err != nil {
		return Artifact{}, err
	}
	if document.FormatVersion == FormatVersion {
		for i, r := range document.Artifact.Version.RelationTypes {
			if r.OutgoingCardinality == nil || r.IncomingCardinality == nil {
				return Artifact{}, wireError(fmt.Sprintf("artifact.version.relationTypes[%d]", i), "v2 cardinalities are required")
			}
		}
		for i, p := range document.Artifact.Version.Properties {
			if p.Computation.Kind == "input" && p.Computation.Presence == "" {
				return Artifact{}, wireError(fmt.Sprintf("artifact.version.propertyDefinitions[%d].computation.presence", i), "is required in v2")
			}
		}
	}
	modelID, err := parseUUID(document.Artifact.Model.ID, "artifact.model.id")
	if err != nil {
		return Artifact{}, err
	}
	if document.Artifact.Model.Name == "" {
		return Artifact{}, wireError("artifact.model.name", "must not be empty")
	}
	version, err := document.Artifact.Version.toModelVersion(modelID, document.Artifact.Model.Name)
	if err != nil {
		return Artifact{}, err
	}
	scenarios := make([]scenario.Scenario, len(document.Artifact.Scenarios))
	for index, item := range document.Artifact.Scenarios {
		candidate, err := item.toScenario(fmt.Sprintf("artifact.scenarios[%d]", index))
		if err != nil {
			return Artifact{}, err
		}
		report := modelscenario.Validate(version, candidate)
		if !report.OK() {
			return Artifact{}, fmt.Errorf("%w: scenarios[%d]: %w", ErrScenarioValidation, index, report)
		}
		scenarios[index] = candidate
	}
	return NewArtifact(document.Artifact.Model.Name, version, scenarios)
}

func (item wireVersion) toModelVersion(modelID identity.UUID, modelName string) (model.ModelVersion, error) {
	versionID, err := parseUUID(item.ID, "artifact.version.id")
	if err != nil {
		return model.ModelVersion{}, err
	}
	encodedModelID, err := parseUUID(item.ModelID, "artifact.version.modelId")
	if err != nil {
		return model.ModelVersion{}, err
	}
	if encodedModelID != modelID {
		return model.ModelVersion{}, wireError("artifact.version.modelId", "does not match artifact.model.id")
	}
	policy, err := item.Precision.toDomain("artifact.version.precision")
	if err != nil {
		return model.ModelVersion{}, err
	}
	draft := model.NewDraft()
	for index, encoded := range item.EntityTypes {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.entityTypes[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddEntityType(value)
	}
	for index, encoded := range item.EnumDefinitions {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.enumDefinitions[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddEnumDefinition(value)
	}
	for index, encoded := range item.RelationTypes {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.relationTypes[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddRelationType(value)
	}
	for index, encoded := range item.Traits {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.traits[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddTrait(value)
	}
	for index, encoded := range item.Contracts {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.propertyContracts[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddPropertyContract(value)
	}
	for index, encoded := range item.Properties {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.propertyDefinitions[%d]", index), 0)
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddPropertyDefinition(value)
	}
	for index, encoded := range item.EntityKeys {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.entityKeyDefinitions[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddEntityKeyDefinition(value)
	}
	for index, encoded := range item.Scopes {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.scopeDefinitions[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddScopeDefinition(value)
	}
	for index, encoded := range item.Entities {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.entities[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddEntity(value)
	}
	for index, encoded := range item.Relations {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.relations[%d]", index), 0)
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddRelation(value)
	}
	for index, encoded := range item.Bindings {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.valueBindings[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddValueBinding(value)
	}
	for index, encoded := range item.Constraints {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.constraints[%d]", index), 0)
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddConstraint(value)
	}
	for index, encoded := range item.Rules {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.rules[%d]", index), 0)
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddRule(value)
	}
	for index, encoded := range item.Invariants {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.invariants[%d]", index), 0)
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddInvariant(value)
	}
	for index, encoded := range item.FunctionRequirements {
		value, err := encoded.toDomain(fmt.Sprintf("artifact.version.functionRequirements[%d]", index))
		if err != nil {
			return model.ModelVersion{}, err
		}
		draft.AddFunctionRequirement(value)
	}
	options := model.NewCommitOptions(versionID, item.Label, policy)
	if item.ParentVersionID != nil {
		parent, err := parseUUID(*item.ParentVersionID, "artifact.version.parentVersionId")
		if err != nil {
			return model.ModelVersion{}, err
		}
		options = options.WithParent(parent)
	}
	version, err := model.Commit(model.NewModel(modelID, modelName), draft, options)
	if err != nil {
		return model.ModelVersion{}, fmt.Errorf("%w: %w", ErrModelValidation, err)
	}
	if len(item.FunctionRequirements) != len(version.FunctionRequirements()) {
		return model.ModelVersion{}, wireError("artifact.version.functionRequirements", "must exactly match function calls")
	}
	for i, requirement := range version.FunctionRequirements() {
		encoded, e := wireFunctionRequirementFromDomain(requirement)
		if e != nil {
			return model.ModelVersion{}, e
		}
		if !reflect.DeepEqual(encoded, item.FunctionRequirements[i]) {
			return model.ModelVersion{}, wireError("artifact.version.functionRequirements", "must exactly match derived requirements")
		}
	}
	return version, nil
}

func (w wireEnumDefinition) toDomain(path string) (schema.EnumDefinition, error) {
	id, e := parseUUID(w.ID, path+".id")
	if e != nil {
		return schema.EnumDefinition{}, e
	}
	opts := make([]schema.EnumOption, len(w.Options))
	for i, o := range w.Options {
		x, e := parseUUID(o.ID, fmt.Sprintf("%s.options[%d].id", path, i))
		if e != nil {
			return schema.EnumDefinition{}, e
		}
		opts[i] = schema.NewEnumOption(x, o.Name)
	}
	return schema.NewEnumDefinition(id, w.Name, opts), nil
}
func (w wireEntityKey) toDomain(path string) (schema.EntityKeyDefinition, error) {
	id, e := parseUUID(w.ID, path+".id")
	if e != nil {
		return schema.EntityKeyDefinition{}, e
	}
	et, e := parseUUID(w.EntityTypeID, path+".entityTypeId")
	if e != nil {
		return schema.EntityKeyDefinition{}, e
	}
	ps, e := parseIDs(w.PropertyDefinitionIDs, path+".propertyDefinitionIds")
	if e != nil {
		return schema.EntityKeyDefinition{}, e
	}
	return schema.NewEntityKeyDefinition(id, w.Name, et, ps), nil
}
func (w wireCardinality) toDomain(path string) (schema.Cardinality, error) {
	min, e := strconv.ParseUint(w.Min, 10, 64)
	if e != nil || w.Min == "" || strconv.FormatUint(min, 10) != w.Min {
		return schema.Cardinality{}, wireError(path+".min", "must be canonical unsigned integer")
	}
	if w.Max == nil {
		return schema.UnboundedCardinality(min), nil
	}
	max, e := strconv.ParseUint(*w.Max, 10, 64)
	if e != nil || strconv.FormatUint(max, 10) != *w.Max || min > max {
		return schema.Cardinality{}, wireError(path, "invalid cardinality")
	}
	return schema.NewCardinality(min, max, true), nil
}
func (w wireInvariant) toDomain(path string, depth int) (invariant.Invariant, error) {
	id, e := parseUUID(w.ID, path+".id")
	if e != nil {
		return invariant.Invariant{}, e
	}
	tid, e := parseUUID(w.Subject.TypeID, path+".subject.typeId")
	if e != nil {
		return invariant.Invariant{}, e
	}
	var s invariant.Subject
	if w.Subject.Kind == "entity_type" {
		s = invariant.EntityTypeSubject(tid)
	} else if w.Subject.Kind == "relation_type" {
		s = invariant.RelationTypeSubject(tid)
	} else {
		return invariant.Invariant{}, wireError(path+".subject.kind", "unknown invariant subject")
	}
	a, e := w.Assertion.toDomain(path+".assertion", depth)
	if e != nil {
		return invariant.Invariant{}, e
	}
	var app ast.Expression
	if w.Applicability != nil {
		app, e = w.Applicability.toDomain(path+".applicability", depth)
		if e != nil {
			return invariant.Invariant{}, e
		}
	}
	return invariant.New(id, w.Name, s, app, w.Applicability != nil, a), nil
}
func (w wireFunctionRequirement) toDomain(path string) (function.Requirement, error) {
	ref, e := function.NewRef(w.Ref.Namespace, w.Ref.Name, function.Version{Major: w.Ref.Major, Minor: w.Ref.Minor, Patch: w.Ref.Patch})
	if e != nil {
		return function.Requirement{}, wireError(path+".ref", "invalid function ref")
	}
	ps := make([]typing.Type, len(w.Parameters))
	for i, x := range w.Parameters {
		ps[i], e = x.toDomain(fmt.Sprintf("%s.parameters[%d]", path, i))
		if e != nil {
			return function.Requirement{}, e
		}
	}
	r, e := w.Result.toDomain(path + ".result")
	if e != nil {
		return function.Requirement{}, e
	}
	sig, e := function.NewSignature(ps, r, w.NullableResult)
	if e != nil {
		return function.Requirement{}, wireError(path, "invalid signature")
	}
	return function.NewRequirement(ref, sig)
}
func (w wireFunctionType) toDomain(path string) (typing.Type, error) {
	vt, ok := parseValueType(w.ValueType)
	if !ok {
		return typing.Type{}, wireError(path+".valueType", "unknown value type")
	}
	tm := typing.TemporalScalar
	if w.TemporalMode != "scalar" {
		return typing.Type{}, wireError(path+".temporalMode", "must be scalar")
	}
	var u unit.Unit
	var hu bool
	var e error
	if w.Unit != nil {
		u, e = w.Unit.toDomain(path + ".unit")
		if e != nil {
			return typing.Type{}, e
		}
		hu = true
	}
	if vt == value.Enum {
		if w.EnumDefinitionID == nil {
			return typing.Type{}, wireError(path+".enumDefinitionId", "required")
		}
		id, e := parseUUID(*w.EnumDefinitionID, path+".enumDefinitionId")
		if e != nil {
			return typing.Type{}, e
		}
		return typing.NewQualifiedEnum(id, u, hu, tm, w.Nullable), nil
	}
	if w.EnumDefinitionID != nil {
		return typing.Type{}, wireError(path+".enumDefinitionId", "only valid for enum")
	}
	return typing.NewQualified(vt, u, hu, tm, w.Nullable), nil
}

func (item wirePrecision) toDomain(path string) (precision.Policy, error) {
	scale, err := strconv.ParseUint(item.Scale, 10, 64)
	if err != nil || item.Scale == "" || strconv.FormatUint(scale, 10) != item.Scale {
		return precision.Policy{}, wireError(path+".scale", "must be a canonical unsigned integer")
	}
	if scale > decimal.MaxScale {
		return precision.Policy{}, wireError(path+".scale", "exceeds decimal maximum scale")
	}
	mode, ok := parseRounding(item.Rounding)
	if !ok {
		return precision.Policy{}, wireError(path+".rounding", "unknown rounding mode")
	}
	return precision.New(scale, mode), nil
}

func (item wireEntityType) toDomain(path string) (schema.EntityType, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return schema.EntityType{}, err
	}
	traits, err := parseIDs(item.Traits, path+".traits")
	if err != nil {
		return schema.EntityType{}, err
	}
	definitions, err := parseIDs(item.PropertyDefinitions, path+".propertyDefinitions")
	if err != nil {
		return schema.EntityType{}, err
	}
	return schema.NewEntityType(id, item.Name, traits, definitions), nil
}

func (item wireRelationType) toDomain(path string) (schema.RelationType, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return schema.RelationType{}, err
	}
	definitions, err := parseIDs(item.PropertyDefinitions, path+".propertyDefinitions")
	if err != nil {
		return schema.RelationType{}, err
	}
	var from, to schema.TypeConstraint
	var hasFrom, hasTo bool
	if item.From != nil {
		from, err = item.From.toDomain(path + ".from")
		if err != nil {
			return schema.RelationType{}, err
		}
		hasFrom = true
	}
	if item.To != nil {
		to, err = item.To.toDomain(path + ".to")
		if err != nil {
			return schema.RelationType{}, err
		}
		hasTo = true
	}
	outgoing, incoming := schema.UnboundedCardinality(0), schema.UnboundedCardinality(0)
	if item.OutgoingCardinality != nil {
		outgoing, err = item.OutgoingCardinality.toDomain(path + ".outgoingCardinality")
		if err != nil {
			return schema.RelationType{}, err
		}
	}
	if item.IncomingCardinality != nil {
		incoming, err = item.IncomingCardinality.toDomain(path + ".incomingCardinality")
		if err != nil {
			return schema.RelationType{}, err
		}
	}
	return schema.NewRelationTypeWithCardinality(id, item.Name, from, hasFrom, to, hasTo, definitions, outgoing, incoming), nil
}

func (item wireTypeConstraint) toDomain(path string) (schema.TypeConstraint, error) {
	switch item.Kind {
	case "entity_type":
		if item.ID == nil {
			return schema.TypeConstraint{}, wireError(path+".id", "is required")
		}
		id, err := parseUUID(*item.ID, path+".id")
		if err != nil {
			return schema.TypeConstraint{}, err
		}
		return schema.EntityTypeConstraint(id), nil
	case "trait":
		if item.ID == nil {
			return schema.TypeConstraint{}, wireError(path+".id", "is required")
		}
		id, err := parseUUID(*item.ID, path+".id")
		if err != nil {
			return schema.TypeConstraint{}, err
		}
		return schema.TraitConstraint(id), nil
	case "any_entity":
		if item.ID != nil {
			return schema.TypeConstraint{}, wireError(path+".id", "must be absent for any_entity")
		}
		return schema.AnyEntityConstraint(), nil
	default:
		return schema.TypeConstraint{}, wireError(path+".kind", "unknown endpoint constraint kind")
	}
}

func (item wireTrait) toDomain(path string) (schema.Trait, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return schema.Trait{}, err
	}
	contracts, err := parseIDs(item.RequiredContracts, path+".requiredPropertyContracts")
	if err != nil {
		return schema.Trait{}, err
	}
	return schema.NewTrait(id, item.Name, contracts), nil
}

func (item wireContract) toDomain(path string) (schema.PropertyContract, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return schema.PropertyContract{}, err
	}
	typ, ok := parseValueType(item.ValueType)
	if !ok {
		return schema.PropertyContract{}, wireError(path+".valueType", "unknown value type")
	}
	temporal, ok := parseTemporalMode(item.TemporalMode)
	if !ok {
		return schema.PropertyContract{}, wireError(path+".temporalMode", "unknown temporal mode")
	}
	var dimension unit.Dimension
	hasDimension := item.Dimension != nil
	if hasDimension {
		dimension, err = item.Dimension.toDomain(path + ".dimension")
		if err != nil {
			return schema.PropertyContract{}, err
		}
	}
	if item.Nullable == nil {
		return schema.PropertyContract{}, wireError(path+".nullable", "is required")
	}
	if typ == value.Enum {
		if item.EnumDefinitionID == nil {
			return schema.PropertyContract{}, wireError(path+".enumDefinitionId", "required for enum")
		}
		eid, e := parseUUID(*item.EnumDefinitionID, path+".enumDefinitionId")
		if e != nil {
			return schema.PropertyContract{}, e
		}
		return schema.NewEnumPropertyContract(id, item.Name, eid, dimension, hasDimension, temporal, *item.Nullable), nil
	}
	if item.EnumDefinitionID != nil {
		return schema.PropertyContract{}, wireError(path+".enumDefinitionId", "only valid for enum")
	}
	return schema.NewPropertyContract(id, item.Name, typ, dimension, hasDimension, temporal, *item.Nullable), nil
}

func (item wireProperty) toDomain(path string, depth int) (schema.PropertyDefinition, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return schema.PropertyDefinition{}, err
	}
	owner, err := item.Owner.toDomain(path + ".owner")
	if err != nil {
		return schema.PropertyDefinition{}, err
	}
	typ, ok := parseValueType(item.ValueType)
	if !ok {
		return schema.PropertyDefinition{}, wireError(path+".valueType", "unknown value type")
	}
	temporal, ok := parseTemporalMode(item.TemporalMode)
	if !ok {
		return schema.PropertyDefinition{}, wireError(path+".temporalMode", "unknown temporal mode")
	}
	var contractID identity.UUID
	hasContract := item.ContractID != nil
	if hasContract {
		contractID, err = parseUUID(*item.ContractID, path+".contractId")
		if err != nil {
			return schema.PropertyDefinition{}, err
		}
	}
	var propertyUnit unit.Unit
	hasUnit := item.Unit != nil
	if hasUnit {
		propertyUnit, err = item.Unit.toDomain(path + ".unit")
		if err != nil {
			return schema.PropertyDefinition{}, err
		}
	}
	computation, err := item.Computation.toDomain(path+".computation", depth)
	if err != nil {
		return schema.PropertyDefinition{}, err
	}
	if typ == value.Enum {
		if item.EnumDefinitionID == nil {
			return schema.PropertyDefinition{}, wireError(path+".enumDefinitionId", "required for enum")
		}
		eid, e := parseUUID(*item.EnumDefinitionID, path+".enumDefinitionId")
		if e != nil {
			return schema.PropertyDefinition{}, e
		}
		return schema.NewEnumPropertyDefinition(id, owner, contractID, hasContract, item.Name, eid, propertyUnit, hasUnit, temporal, computation), nil
	}
	if item.EnumDefinitionID != nil {
		return schema.PropertyDefinition{}, wireError(path+".enumDefinitionId", "only valid for enum")
	}
	return schema.NewPropertyDefinition(id, owner, contractID, hasContract, item.Name, typ, propertyUnit, hasUnit, temporal, computation), nil
}

func (item wireOwner) toDomain(path string) (schema.PropertyOwner, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return schema.PropertyOwner{}, err
	}
	switch item.Kind {
	case "entity_type":
		return schema.EntityTypeOwner(id), nil
	case "relation_type":
		return schema.RelationTypeOwner(id), nil
	default:
		return schema.PropertyOwner{}, wireError(path+".kind", "unknown property owner kind")
	}
}

func (item wireComputation) toDomain(path string, depth int) (schema.Computation, error) {
	switch item.Kind {
	case "input":
		if item.Expression != nil {
			return schema.Computation{}, wireError(path+".expression", "must be absent for input computation")
		}
		if item.Presence == "" || item.Presence == "optional" {
			return schema.InputComputation(), nil
		}
		if item.Presence == "required" {
			return schema.InputComputationWithPresence(schema.PresenceRequired), nil
		}
		return schema.Computation{}, wireError(path+".presence", "unknown presence")
	case "derived":
		if item.Expression == nil {
			return schema.Computation{}, wireError(path+".expression", "is required for derived computation")
		}
		expression, err := item.Expression.toDomain(path+".expression", depth)
		if err != nil {
			return schema.Computation{}, err
		}
		return schema.DerivedComputation(expression), nil
	default:
		return schema.Computation{}, wireError(path+".kind", "unknown computation kind")
	}
}

func (item wireScope) toDomain(path string) (scope.Definition, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return scope.Definition{}, err
	}
	typ, ok := parseValueType(item.ValueType)
	if !ok {
		return scope.Definition{}, wireError(path+".valueType", "unknown value type")
	}
	return scope.NewDefinition(id, item.Name, typ), nil
}

func (item wireEntity) toDomain(path string) (semantic.Entity, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return semantic.Entity{}, err
	}
	typeID, err := parseUUID(item.TypeID, path+".typeId")
	if err != nil {
		return semantic.Entity{}, err
	}
	if item.Name == nil {
		return semantic.NewEntity(id, typeID, "", false), nil
	}
	return semantic.NewEntity(id, typeID, *item.Name, true), nil
}

func (item wireRelation) toDomain(path string, depth int) (semantic.Relation, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return semantic.Relation{}, err
	}
	typeID, err := parseUUID(item.TypeID, path+".typeId")
	if err != nil {
		return semantic.Relation{}, err
	}
	from, err := parseUUID(item.From, path+".from")
	if err != nil {
		return semantic.Relation{}, err
	}
	to, err := parseUUID(item.To, path+".to")
	if err != nil {
		return semantic.Relation{}, err
	}
	if item.Applicability == nil {
		return semantic.NewRelation(id, typeID, from, to, nil, false), nil
	}
	expression, err := item.Applicability.toDomain(path+".applicability", depth)
	if err != nil {
		return semantic.Relation{}, err
	}
	return semantic.NewRelation(id, typeID, from, to, expression, true), nil
}

func (item wireBinding) toDomain(path string) (semantic.ValueBinding, error) {
	target, err := item.Target.toDomain(path + ".target")
	if err != nil {
		return semantic.ValueBinding{}, err
	}
	bindingValue, err := item.Value.toDomain(path + ".value")
	if err != nil {
		return semantic.ValueBinding{}, err
	}
	selectors, err := selectorsToDomain(item.ScopeSelectors, path+".scopeSelectors")
	if err != nil {
		return semantic.ValueBinding{}, err
	}
	period, hasPeriod, err := effectivePeriodToDomain(item.EffectivePeriod, path+".effectivePeriod")
	if err != nil {
		return semantic.ValueBinding{}, err
	}
	return semantic.NewValueBinding(target, bindingValue, selectors, period, hasPeriod), nil
}

func (item wireTarget) toDomain(path string) (semantic.ValueTarget, error) {
	switch item.Kind {
	case "object_property":
		if item.Address == nil || item.Owner != nil || item.DefinitionID != nil {
			return semantic.ValueTarget{}, wireError(path, "object_property must contain only address")
		}
		addressValue, err := item.Address.toDomain(path + ".address")
		if err != nil {
			return semantic.ValueTarget{}, err
		}
		return semantic.ObjectPropertyTarget(addressValue), nil
	case "type_property":
		if item.Address != nil || item.Owner == nil || item.DefinitionID == nil {
			return semantic.ValueTarget{}, wireError(path, "type_property must contain owner and definitionId")
		}
		owner, err := item.Owner.toDomain(path + ".owner")
		if err != nil {
			return semantic.ValueTarget{}, err
		}
		definitionID, err := parseUUID(*item.DefinitionID, path+".definitionId")
		if err != nil {
			return semantic.ValueTarget{}, err
		}
		return semantic.TypePropertyTarget(owner, definitionID), nil
	default:
		return semantic.ValueTarget{}, wireError(path+".kind", "unknown target kind")
	}
}

func (item wireAddress) toDomain(path string) (address.PropertyAddress, error) {
	object, err := item.Object.toDomain(path + ".object")
	if err != nil {
		return address.PropertyAddress{}, err
	}
	definitionID, err := parseUUID(item.DefinitionID, path+".definitionId")
	if err != nil {
		return address.PropertyAddress{}, err
	}
	return address.NewPropertyAddress(object, definitionID), nil
}
func (item wireObjectRef) toDomain(path string) (address.ObjectRef, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return address.ObjectRef{}, err
	}
	switch item.Kind {
	case "entity":
		return address.EntityRef(id), nil
	case "relation":
		return address.RelationRef(id), nil
	default:
		return address.ObjectRef{}, wireError(path+".kind", "unknown object reference kind")
	}
}

func (item wireBindingValue) toDomain(path string) (semantic.BindingValue, error) {
	switch item.Kind {
	case "defined":
		if item.Value == nil || item.Series != nil {
			return semantic.BindingValue{}, wireError(path, "defined binding must contain only value")
		}
		v, err := item.Value.toDomain(path + ".value")
		if err != nil {
			return semantic.BindingValue{}, err
		}
		return semantic.DefinedBindingValue(v), nil
	case "null":
		if item.Value != nil || item.Series != nil {
			return semantic.BindingValue{}, wireError(path, "null binding has unexpected payload")
		}
		return semantic.NullBindingValue(), nil
	case "series":
		if item.Value != nil || item.Series == nil {
			return semantic.BindingValue{}, wireError(path, "series binding must contain only series")
		}
		v, err := item.Series.toDomain(path + ".series")
		if err != nil {
			return semantic.BindingValue{}, err
		}
		return semantic.SeriesBindingValue(v), nil
	default:
		return semantic.BindingValue{}, wireError(path+".kind", "unknown binding value kind")
	}
}

func selectorsToDomain(items []wireSelector, path string) ([]scope.Selector, error) {
	result := make([]scope.Selector, len(items))
	for index, item := range items {
		definitionID, err := parseUUID(item.DefinitionID, fmt.Sprintf("%s[%d].definitionId", path, index))
		if err != nil {
			return nil, err
		}
		v, err := item.Value.toDomain(fmt.Sprintf("%s[%d].value", path, index))
		if err != nil {
			return nil, err
		}
		result[index] = scope.NewSelector(definitionID, v)
	}
	return result, nil
}

func effectivePeriodToDomain(item *wireEffectivePeriod, path string) (domaintime.EffectivePeriod, bool, error) {
	if item == nil {
		return domaintime.EffectivePeriod{}, false, nil
	}
	var from, to datetime.DateTime
	var err error
	hasFrom, hasTo := item.From != nil, item.To != nil
	if hasFrom {
		from, err = parseDateTime(*item.From, path+".from")
		if err != nil {
			return domaintime.EffectivePeriod{}, false, err
		}
	}
	if hasTo {
		to, err = parseDateTime(*item.To, path+".to")
		if err != nil {
			return domaintime.EffectivePeriod{}, false, err
		}
	}
	return domaintime.NewEffectivePeriod(from, hasFrom, to, hasTo), true, nil
}

func (item wireTyped) toDomain(path string) (value.Typed, error) {
	raw, err := (wireRaw{Type: item.Type, Data: item.Data}).toDomain(path)
	if err != nil {
		return value.Typed{}, err
	}
	if item.Unit == nil {
		return value.NewTyped(raw), nil
	}
	u, err := item.Unit.toDomain(path + ".unit")
	if err != nil {
		return value.Typed{}, err
	}
	return value.NewTypedWithUnit(raw, u), nil
}

func (item wireRaw) toDomain(path string) (value.Raw, error) {
	switch item.Type {
	case "bool":
		if item.Data == "true" {
			return value.NewBool(true), nil
		}
		if item.Data == "false" {
			return value.NewBool(false), nil
		}
		return value.Raw{}, wireError(path+".data", "must be true or false")
	case "integer":
		integer, ok := new(big.Int).SetString(item.Data, 10)
		if !ok || integer.String() != item.Data {
			return value.Raw{}, wireError(path+".data", "must be a canonical integer")
		}
		return value.NewInteger(integer), nil
	case "decimal":
		parsed, err := decimal.Parse(item.Data)
		if err != nil || parsed.String() != item.Data {
			return value.Raw{}, wireError(path+".data", "must be a canonical decimal")
		}
		return value.NewDecimal(parsed), nil
	case "string":
		return value.NewString(item.Data), nil
	case "date":
		parsed, err := date.Parse(item.Data)
		if err != nil || parsed.String() != item.Data {
			return value.Raw{}, wireError(path+".data", "must be a canonical date")
		}
		return value.NewDate(parsed), nil
	case "datetime":
		parsed, err := parseDateTime(item.Data, path+".data")
		if err != nil {
			return value.Raw{}, err
		}
		return value.NewDateTime(parsed), nil
	case "duration":
		parsed, err := duration.Parse(item.Data)
		if err != nil || parsed.String() != item.Data {
			return value.Raw{}, wireError(path+".data", "must be a canonical duration")
		}
		return value.NewDuration(parsed), nil
	case "enum":
		id, e := parseUUID(item.EnumDefinitionID, path+".enumDefinitionId")
		if e != nil {
			return value.Raw{}, e
		}
		opt, e := parseUUID(item.EnumOptionID, path+".enumOptionId")
		if e != nil {
			return value.Raw{}, e
		}
		if item.Data != "" {
			return value.Raw{}, wireError(path+".data", "must be empty for enum")
		}
		return value.NewEnum(id, opt), nil
	default:
		return value.Raw{}, wireError(path+".type", "unknown raw value type")
	}
}

func (item wireSeries) toDomain(path string) (series.Series, error) {
	elementType, ok := parseValueType(item.ElementType)
	if !ok {
		return series.Series{}, wireError(path+".elementType", "unknown value type")
	}
	var u unit.Unit
	hasUnit := item.Unit != nil
	var err error
	if hasUnit {
		u, err = item.Unit.toDomain(path + ".unit")
		if err != nil {
			return series.Series{}, err
		}
	}
	switch item.Axis {
	case "point":
		if len(item.Intervals) != 0 {
			return series.Series{}, wireError(path+".intervals", "must be empty for point series")
		}
		points := make([]series.Point, len(item.Points))
		for index, point := range item.Points {
			at, err := parseDateTime(point.At, fmt.Sprintf("%s.points[%d].at", path, index))
			if err != nil {
				return series.Series{}, err
			}
			sample, err := point.Sample.toDomain(fmt.Sprintf("%s.points[%d].sample", path, index))
			if err != nil {
				return series.Series{}, err
			}
			points[index] = series.NewPoint(at, sample)
		}
		return series.NewPointSeries(elementType, u, hasUnit, points), nil
	case "interval":
		if len(item.Points) != 0 {
			return series.Series{}, wireError(path+".points", "must be empty for interval series")
		}
		intervals := make([]series.Interval, len(item.Intervals))
		for index, interval := range item.Intervals {
			from, err := parseDateTime(interval.From, fmt.Sprintf("%s.intervals[%d].from", path, index))
			if err != nil {
				return series.Series{}, err
			}
			to, err := parseDateTime(interval.To, fmt.Sprintf("%s.intervals[%d].to", path, index))
			if err != nil {
				return series.Series{}, err
			}
			sample, err := interval.Sample.toDomain(fmt.Sprintf("%s.intervals[%d].sample", path, index))
			if err != nil {
				return series.Series{}, err
			}
			intervals[index] = series.NewInterval(from, to, sample)
		}
		return series.NewIntervalSeries(elementType, u, hasUnit, intervals), nil
	default:
		return series.Series{}, wireError(path+".axis", "unknown series axis")
	}
}

func (item wireSeriesSample) toDomain(path string) (series.Sample, error) {
	if item.Null == nil {
		return series.Sample{}, wireError(path+".null", "is required")
	}
	if *item.Null {
		if item.Value != nil {
			return series.Sample{}, wireError(path, "null sample has value")
		}
		return series.Null(), nil
	}
	if item.Value == nil {
		return series.Sample{}, wireError(path+".value", "is required for defined sample")
	}
	raw, err := item.Value.toDomain(path + ".value")
	if err != nil {
		return series.Sample{}, err
	}
	return series.Defined(raw), nil
}

func (item wireDimension) toDomain(path string) (unit.Dimension, error) {
	factors := make([]unit.DimensionFactor, len(item.Factors))
	for index, factor := range item.Factors {
		factors[index] = unit.DimensionFactor{ID: unit.ID(factor.ID), Exponent: factor.Exponent}
	}
	dimension := unit.NewDimension(factors)
	canonical := wireDimensionFromDomain(dimension)
	if !equalDimensionWire(canonical, item) {
		return unit.Dimension{}, wireError(path, "dimension factors must be canonical")
	}
	return dimension, nil
}

func (item wireUnit) toDomain(path string) (unit.Unit, error) {
	dimension, err := item.Dimension.toDomain(path + ".dimension")
	if err != nil {
		return unit.Unit{}, err
	}
	numerator, ok := new(big.Int).SetString(item.Scale.Numerator, 10)
	if !ok || numerator.String() != item.Scale.Numerator {
		return unit.Unit{}, wireError(path+".scale.numerator", "must be a canonical integer")
	}
	denominator, ok := new(big.Int).SetString(item.Scale.Denominator, 10)
	if !ok || denominator.Sign() <= 0 || denominator.String() != item.Scale.Denominator {
		return unit.Unit{}, wireError(path+".scale.denominator", "must be a positive canonical integer")
	}
	factors := make([]unit.SymbolicFactor, len(item.Factors))
	for index, factor := range item.Factors {
		factors[index] = unit.SymbolicFactor{ID: unit.UnitID(factor.ID), Exponent: factor.Exponent}
	}
	conversions := make([]unit.ConversionFactorSpec, len(item.ConversionFactors))
	for index, factor := range item.ConversionFactors {
		conversions[index] = unit.ConversionFactorSpec{ID: unit.ConversionSpaceID(factor.ID), Exponent: factor.Exponent}
	}
	result := unit.NewComposedUnit(dimension, factors, conversions, rational.New(numerator, denominator))
	canonical, err := wireUnitFromDomain(result)
	if err != nil {
		return unit.Unit{}, err
	}
	if !equalUnitWire(canonical, item) {
		return unit.Unit{}, wireError(path, "unit components must be canonical")
	}
	return result, nil
}

func (item wireConstraint) toDomain(path string, depth int) (constraint.Constraint, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return constraint.Constraint{}, err
	}
	assertion, err := item.Assertion.toDomain(path+".assertion", depth)
	if err != nil {
		return constraint.Constraint{}, err
	}
	severity, ok := parseSeverity(item.Severity)
	if !ok {
		return constraint.Constraint{}, wireError(path+".severity", "unknown severity")
	}
	var applicability ast.Expression
	hasApplicability := item.Applicability != nil
	if hasApplicability {
		applicability, err = item.Applicability.toDomain(path+".applicability", depth)
		if err != nil {
			return constraint.Constraint{}, err
		}
	}
	if item.Message == nil {
		return constraint.New(id, item.Name, applicability, hasApplicability, assertion, severity, "", false), nil
	}
	return constraint.New(id, item.Name, applicability, hasApplicability, assertion, severity, *item.Message, true), nil
}

func (item wireRule) toDomain(path string, depth int) (rule.Rule, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return rule.Rule{}, err
	}
	condition, err := item.Condition.toDomain(path+".condition", depth)
	if err != nil {
		return rule.Rule{}, err
	}
	metadata := make(map[string]string, len(item.Metadata))
	for index, pair := range item.Metadata {
		if pair.Key == "" {
			return rule.Rule{}, wireError(fmt.Sprintf("%s.metadata[%d].key", path, index), "must not be empty")
		}
		if _, exists := metadata[pair.Key]; exists {
			return rule.Rule{}, wireError(fmt.Sprintf("%s.metadata[%d].key", path, index), "is duplicated")
		}
		metadata[pair.Key] = pair.Value
	}
	return rule.NewWithMetadata(id, item.Name, condition, metadata), nil
}

func (item wireScenario) toScenario(path string) (scenario.Scenario, error) {
	id, err := parseUUID(item.ID, path+".id")
	if err != nil {
		return scenario.Scenario{}, err
	}
	baseID, err := parseUUID(item.BaseModelVersionID, path+".baseModelVersionId")
	if err != nil {
		return scenario.Scenario{}, err
	}
	overrides := make([]scenario.Override, len(item.Overrides))
	for index, encoded := range item.Overrides {
		value, err := encoded.toDomain(fmt.Sprintf("%s.overrides[%d]", path, index))
		if err != nil {
			return scenario.Scenario{}, err
		}
		overrides[index] = value
	}
	return scenario.New(id, item.Name, baseID, overrides), nil
}

func (item wireOverride) toDomain(path string) (scenario.Override, error) {
	target, err := item.Target.toDomain(path + ".target")
	if err != nil {
		return scenario.Override{}, err
	}
	bindingValue, err := item.Value.toDomain(path + ".value")
	if err != nil {
		return scenario.Override{}, err
	}
	selectors, err := selectorsToDomain(item.ScopeSelectors, path+".scopeSelectors")
	if err != nil {
		return scenario.Override{}, err
	}
	period, hasPeriod, err := effectivePeriodToDomain(item.EffectivePeriod, path+".effectivePeriod")
	if err != nil {
		return scenario.Override{}, err
	}
	return scenario.NewOverride(target, bindingValue, selectors, period, hasPeriod), nil
}

func (item wireExpression) toDomain(path string, depth int) (ast.Expression, error) {
	// depth is zero-based while ast.Depth counts nodes. Rejecting at the
	// maximum keeps the decoder's accepted tree height exactly aligned with
	// the shared AST/model-validation limit.
	if depth >= ast.MaxExpressionDepth {
		return nil, wireError(path, fmt.Sprintf("expression nesting exceeds %d", ast.MaxExpressionDepth))
	}
	switch item.Kind {
	case "constant":
		if item.Value == nil || !item.only("value") {
			return nil, wireError(path, "constant must contain only value")
		}
		v, err := item.Value.toDomain(path + ".value")
		if err != nil {
			return nil, err
		}
		return ast.NewConstant(v), nil
	case "arithmetic":
		if !item.only("operator", "left", "right") || item.Left == nil || item.Right == nil {
			return nil, wireError(path, "arithmetic requires operator, left and right")
		}
		operator, ok := parseArithmetic(item.Operator)
		if !ok {
			return nil, wireError(path+".operator", "unknown arithmetic operator")
		}
		left, err := item.Left.toDomain(path+".left", depth+1)
		if err != nil {
			return nil, err
		}
		right, err := item.Right.toDomain(path+".right", depth+1)
		if err != nil {
			return nil, err
		}
		return ast.NewArithmetic(operator, left, right), nil
	case "negate":
		return item.unary(path, depth, "negate", func(operand ast.Expression) ast.Expression { return ast.NewNegate(operand) })
	case "comparison":
		if !item.only("operator", "left", "right") || item.Left == nil || item.Right == nil {
			return nil, wireError(path, "comparison requires operator, left and right")
		}
		operator, ok := parseComparison(item.Operator)
		if !ok {
			return nil, wireError(path+".operator", "unknown comparison operator")
		}
		left, err := item.Left.toDomain(path+".left", depth+1)
		if err != nil {
			return nil, err
		}
		right, err := item.Right.toDomain(path+".right", depth+1)
		if err != nil {
			return nil, err
		}
		return ast.NewComparison(operator, left, right), nil
	case "logical":
		if !item.only("operator", "left", "right") || item.Left == nil || item.Right == nil {
			return nil, wireError(path, "logical requires operator, left and right")
		}
		operator, ok := parseLogical(item.Operator)
		if !ok {
			return nil, wireError(path+".operator", "unknown logical operator")
		}
		left, err := item.Left.toDomain(path+".left", depth+1)
		if err != nil {
			return nil, err
		}
		right, err := item.Right.toDomain(path+".right", depth+1)
		if err != nil {
			return nil, err
		}
		return ast.NewLogical(operator, left, right), nil
	case "not":
		return item.unary(path, depth, "not", func(operand ast.Expression) ast.Expression { return ast.NewNot(operand) })
	case "if":
		if !item.only("condition", "then", "else") || item.Condition == nil || item.Then == nil || item.Else == nil {
			return nil, wireError(path, "if requires condition, then and else")
		}
		condition, err := item.Condition.toDomain(path+".condition", depth+1)
		if err != nil {
			return nil, err
		}
		thenExpr, err := item.Then.toDomain(path+".then", depth+1)
		if err != nil {
			return nil, err
		}
		elseExpr, err := item.Else.toDomain(path+".else", depth+1)
		if err != nil {
			return nil, err
		}
		return ast.NewIf(condition, thenExpr, elseExpr), nil
	case "is_null":
		return item.unary(path, depth, "is_null", func(operand ast.Expression) ast.Expression { return ast.NewIsNull(operand) })
	case "coalesce":
		if !item.only("primary", "fallback") || item.Primary == nil || item.Fallback == nil {
			return nil, wireError(path, "coalesce requires primary and fallback")
		}
		primary, err := item.Primary.toDomain(path+".primary", depth+1)
		if err != nil {
			return nil, err
		}
		fallback, err := item.Fallback.toDomain(path+".fallback", depth+1)
		if err != nil {
			return nil, err
		}
		return ast.NewCoalesce(primary, fallback), nil
	case "self_property":
		if !item.only("definitionId") {
			return nil, wireError(path, "self_property must contain only definitionId")
		}
		id, err := parseUUID(item.DefinitionID, path+".definitionId")
		if err != nil {
			return nil, err
		}
		return ast.NewSelfPropertyReference(id), nil
	case "absolute_property":
		if !item.only("address") || item.Address == nil {
			return nil, wireError(path, "absolute_property requires address")
		}
		addressValue, err := item.Address.toDomain(path + ".address")
		if err != nil {
			return nil, err
		}
		return ast.NewAbsolutePropertyReference(addressValue), nil
	case "bound_property":
		if !item.only("target", "definitionId") {
			return nil, wireError(path, "bound_property requires target and definitionId")
		}
		target, ok := parseBoundTarget(item.Target)
		if !ok {
			return nil, wireError(path+".target", "unknown bound target")
		}
		id, err := parseUUID(item.DefinitionID, path+".definitionId")
		if err != nil {
			return nil, err
		}
		return ast.NewBoundPropertyDefinitionReference(target, id), nil
	case "bound_contract_property":
		if !item.only("target", "contractId") {
			return nil, wireError(path, "bound_contract_property requires target and contractId")
		}
		target, ok := parseBoundTarget(item.Target)
		if !ok {
			return nil, wireError(path+".target", "unknown bound target")
		}
		id, err := parseUUID(item.ContractID, path+".contractId")
		if err != nil {
			return nil, err
		}
		return ast.NewBoundContractPropertyReference(target, id), nil
	case "aggregate":
		return item.aggregateToDomain(path, depth)
	case "unary":
		if !item.only("operator", "operand") || item.Operand == nil {
			return nil, wireError(path, "unary requires operator and operand")
		}
		op, ok := parseUnary(item.Operator)
		if !ok {
			return nil, wireError(path+".operator", "unknown unary operator")
		}
		x, e := item.Operand.toDomain(path+".operand", depth+1)
		if e != nil {
			return nil, e
		}
		return ast.NewUnary(op, x), nil
	case "binary":
		if !item.only("operator", "left", "right") || item.Left == nil || item.Right == nil {
			return nil, wireError(path, "binary requires operator, left and right")
		}
		op, ok := parseBinary(item.Operator)
		if !ok {
			return nil, wireError(path+".operator", "unknown binary operator")
		}
		l, e := item.Left.toDomain(path+".left", depth+1)
		if e != nil {
			return nil, e
		}
		r, e := item.Right.toDomain(path+".right", depth+1)
		if e != nil {
			return nil, e
		}
		return ast.NewBinary(op, l, r), nil
	case "variadic":
		if !item.only("operator", "operands") || len(item.Operands) == 0 {
			return nil, wireError(path, "variadic requires operator and operands")
		}
		op, ok := parseVariadic(item.Operator)
		if !ok {
			return nil, wireError(path+".operator", "unknown variadic operator")
		}
		xs := make([]ast.Expression, len(item.Operands))
		for i, x := range item.Operands {
			var e error
			xs[i], e = x.toDomain(fmt.Sprintf("%s.operands[%d]", path, i), depth+1)
			if e != nil {
				return nil, e
			}
		}
		return ast.NewVariadic(op, xs), nil
	case "function_call":
		if !item.only("function", "arguments") || item.Function == nil {
			return nil, wireError(path, "function_call requires function and arguments")
		}
		ref, e := function.NewRef(item.Function.Namespace, item.Function.Name, function.Version{Major: item.Function.Major, Minor: item.Function.Minor, Patch: item.Function.Patch})
		if e != nil {
			return nil, wireError(path+".function", "invalid function ref")
		}
		xs := make([]ast.Expression, len(item.Arguments))
		for i, x := range item.Arguments {
			xs[i], e = x.toDomain(fmt.Sprintf("%s.arguments[%d]", path, i), depth+1)
			if e != nil {
				return nil, e
			}
		}
		return ast.NewFunctionCall(ref, xs), nil
	default:
		return nil, wireError(path+".kind", "unknown expression kind")
	}
}

func (item wireExpression) unary(path string, depth int, kind string, construct func(ast.Expression) ast.Expression) (ast.Expression, error) {
	if !item.only("operand") || item.Operand == nil {
		return nil, wireError(path, kind+" requires operand")
	}
	operand, err := item.Operand.toDomain(path+".operand", depth+1)
	if err != nil {
		return nil, err
	}
	return construct(operand), nil
}

func (item wireExpression) aggregateToDomain(path string, depth int) (ast.Expression, error) {
	if !item.only("source", "filter", "map", "operation") || item.Source == nil {
		return nil, wireError(path, "aggregate requires source and operation")
	}
	source, err := item.Source.toDomain(path + ".source")
	if err != nil {
		return nil, err
	}
	operation, ok := parseAggregate(item.Operation)
	if !ok {
		return nil, wireError(path+".operation", "unknown aggregate operation")
	}
	var filter, mapExpr ast.Expression
	if item.Filter != nil {
		filter, err = item.Filter.toDomain(path+".filter", depth+1)
		if err != nil {
			return nil, err
		}
	}
	if item.Map != nil {
		mapExpr, err = item.Map.toDomain(path+".map", depth+1)
		if err != nil {
			return nil, err
		}
	}
	switch operation {
	case ast.AggregateOperationCount:
		if mapExpr != nil {
			return nil, wireError(path+".map", "count must not have map")
		}
		if filter == nil {
			return ast.NewCountAggregate(source), nil
		}
		return ast.NewFilteredCountAggregate(source, filter), nil
	default:
		if mapExpr == nil {
			return nil, wireError(path+".map", "mapped aggregate requires map")
		}
		if filter == nil {
			return ast.NewMappedAggregate(source, mapExpr, operation), nil
		}
		return ast.NewFilteredMappedAggregate(source, filter, mapExpr, operation), nil
	}
}

func (item wireTraversalSource) toDomain(path string) (ast.TraversalSource, error) {
	direction, ok := parseTraversalDirection(item.Direction)
	if !ok {
		return ast.TraversalSource{}, wireError(path+".direction", "unknown traversal direction")
	}
	relationTypeID, err := parseUUID(item.RelationTypeID, path+".relationTypeId")
	if err != nil {
		return ast.TraversalSource{}, err
	}
	switch item.Origin {
	case "self":
		if item.AbsoluteOrigin != nil {
			return ast.TraversalSource{}, wireError(path+".absoluteOrigin", "must be absent for self origin")
		}
		return ast.NewTraversalSource(direction, relationTypeID), nil
	case "absolute_entity":
		if item.AbsoluteOrigin == nil {
			return ast.TraversalSource{}, wireError(path+".absoluteOrigin", "is required for absolute origin")
		}
		origin, err := item.AbsoluteOrigin.toDomain(path + ".absoluteOrigin")
		if err != nil {
			return ast.TraversalSource{}, err
		}
		return ast.NewTraversalSourceFromEntity(origin, direction, relationTypeID), nil
	case "bound_entity":
		if item.AbsoluteOrigin != nil {
			return ast.TraversalSource{}, wireError(path+".absoluteOrigin", "must be absent for bound origin")
		}
		return ast.NewTraversalSourceFromBoundEntity(direction, relationTypeID), nil
	default:
		return ast.TraversalSource{}, wireError(path+".origin", "unknown traversal origin")
	}
}

func (item wireExpression) only(allowed ...string) bool {
	allowedSet := make(map[string]bool, len(allowed))
	for _, field := range allowed {
		allowedSet[field] = true
	}
	present := item.presentFields()
	for _, field := range present {
		if !allowedSet[field] {
			return false
		}
	}
	return true
}
func (item wireExpression) presentFields() []string {
	fields := make([]string, 0, 16)
	if item.Value != nil {
		fields = append(fields, "value")
	}
	if item.Operator != "" {
		fields = append(fields, "operator")
	}
	if item.Left != nil {
		fields = append(fields, "left")
	}
	if item.Right != nil {
		fields = append(fields, "right")
	}
	if item.Operand != nil {
		fields = append(fields, "operand")
	}
	if item.Condition != nil {
		fields = append(fields, "condition")
	}
	if item.Then != nil {
		fields = append(fields, "then")
	}
	if item.Else != nil {
		fields = append(fields, "else")
	}
	if item.Primary != nil {
		fields = append(fields, "primary")
	}
	if item.Fallback != nil {
		fields = append(fields, "fallback")
	}
	if item.DefinitionID != "" {
		fields = append(fields, "definitionId")
	}
	if item.ContractID != "" {
		fields = append(fields, "contractId")
	}
	if item.Address != nil {
		fields = append(fields, "address")
	}
	if item.Target != "" {
		fields = append(fields, "target")
	}
	if item.Source != nil {
		fields = append(fields, "source")
	}
	if item.Filter != nil {
		fields = append(fields, "filter")
	}
	if item.Map != nil {
		fields = append(fields, "map")
	}
	if item.Operation != "" {
		fields = append(fields, "operation")
	}
	if item.Operands != nil {
		fields = append(fields, "operands")
	}
	if item.Arguments != nil {
		fields = append(fields, "arguments")
	}
	if item.Function != nil {
		fields = append(fields, "function")
	}
	return fields
}

func parseUUID(text, path string) (identity.UUID, error) {
	id, err := identity.Parse(text)
	if err != nil || id.IsZero() || id.String() != text {
		return identity.UUID{}, wireError(path, "must be a non-zero canonical UUID")
	}
	return id, nil
}
func parseIDs(items []string, path string) ([]identity.UUID, error) {
	result := make([]identity.UUID, len(items))
	for index, item := range items {
		id, err := parseUUID(item, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return nil, err
		}
		result[index] = id
	}
	return result, nil
}
func parseDateTime(text, path string) (datetime.DateTime, error) {
	parsed, err := datetime.Parse(text)
	if err != nil || parsed.String() != text {
		return datetime.DateTime{}, wireError(path, "must be a canonical UTC datetime")
	}
	return parsed, nil
}
func wireError(path, message string) error {
	return fmt.Errorf("%w: %s: %s", ErrWireSemantic, path, message)
}

func parseValueType(text string) (value.Type, bool) {
	switch text {
	case "bool":
		return value.Bool, true
	case "integer":
		return value.Integer, true
	case "decimal":
		return value.Decimal, true
	case "string":
		return value.String, true
	case "date":
		return value.Date, true
	case "date_time":
		return value.DateTime, true
	case "duration":
		return value.Duration, true
	case "enum":
		return value.Enum, true
	default:
		return value.Invalid, false
	}
}
func parseTemporalMode(text string) (schema.TemporalMode, bool) {
	switch text {
	case "scalar":
		return schema.TemporalScalar, true
	case "series":
		return schema.TemporalSeries, true
	default:
		return schema.TemporalInvalid, false
	}
}
func parseRounding(text string) (precision.Mode, bool) {
	switch text {
	case "truncate":
		return precision.Truncate, true
	case "floor":
		return precision.Floor, true
	case "ceil":
		return precision.Ceil, true
	case "half_up":
		return precision.HalfUp, true
	case "half_even":
		return precision.HalfEven, true
	default:
		return precision.Invalid, false
	}
}
func parseSeverity(text string) (constraint.Severity, bool) {
	switch text {
	case "info":
		return constraint.SeverityInfo, true
	case "warning":
		return constraint.SeverityWarning, true
	case "error":
		return constraint.SeverityError, true
	case "hard":
		return constraint.SeverityHard, true
	default:
		return constraint.SeverityInvalid, false
	}
}
func parseArithmetic(text string) (ast.ArithmeticOperator, bool) {
	switch text {
	case "add":
		return ast.ArithmeticAdd, true
	case "subtract":
		return ast.ArithmeticSubtract, true
	case "multiply":
		return ast.ArithmeticMultiply, true
	case "divide":
		return ast.ArithmeticDivide, true
	default:
		return ast.ArithmeticInvalid, false
	}
}
func parseComparison(text string) (ast.ComparisonOperator, bool) {
	switch text {
	case "equal":
		return ast.ComparisonEqual, true
	case "not_equal":
		return ast.ComparisonNotEqual, true
	case "less":
		return ast.ComparisonLess, true
	case "less_or_equal":
		return ast.ComparisonLessOrEqual, true
	case "greater":
		return ast.ComparisonGreater, true
	case "greater_or_equal":
		return ast.ComparisonGreaterOrEqual, true
	default:
		return ast.ComparisonInvalid, false
	}
}
func parseLogical(text string) (ast.LogicalOperator, bool) {
	switch text {
	case "and":
		return ast.LogicalAnd, true
	case "or":
		return ast.LogicalOr, true
	default:
		return ast.LogicalInvalid, false
	}
}
func parseUnary(s string) (ast.UnaryOperator, bool) {
	for x := ast.UnaryOperatorIsDefined; x <= ast.UnaryOperatorMinute; x++ {
		if lowerEnum(x.String()) == s {
			return x, true
		}
	}
	return ast.UnaryOperatorInvalid, false
}
func parseBinary(s string) (ast.BinaryOperator, bool) {
	for x := ast.BinaryOperatorModulo; x <= ast.BinaryOperatorDifference; x++ {
		if lowerEnum(x.String()) == s {
			return x, true
		}
	}
	return ast.BinaryOperatorInvalid, false
}
func parseVariadic(s string) (ast.VariadicOperator, bool) {
	for x := ast.VariadicOperatorConcat; x <= ast.VariadicOperatorScalarMax; x++ {
		if lowerEnum(x.String()) == s {
			return x, true
		}
	}
	return ast.VariadicOperatorInvalid, false
}
func parseBoundTarget(text string) (ast.BoundTarget, bool) {
	switch text {
	case "entity":
		return ast.BoundTargetEntity, true
	case "relation":
		return ast.BoundTargetRelation, true
	default:
		return ast.BoundTargetInvalid, false
	}
}
func parseAggregate(text string) (ast.AggregateOperation, bool) {
	switch text {
	case "sum":
		return ast.AggregateOperationSum, true
	case "average":
		return ast.AggregateOperationAverage, true
	case "count":
		return ast.AggregateOperationCount, true
	case "min":
		return ast.AggregateOperationMin, true
	case "max":
		return ast.AggregateOperationMax, true
	default:
		return ast.AggregateOperationInvalid, false
	}
}
func parseTraversalDirection(text string) (ast.TraversalDirection, bool) {
	switch text {
	case "outgoing":
		return ast.TraversalDirectionOutgoing, true
	case "incoming":
		return ast.TraversalDirectionIncoming, true
	case "both":
		return ast.TraversalDirectionBoth, true
	default:
		return ast.TraversalDirectionInvalid, false
	}
}

func equalDimensionWire(left, right wireDimension) bool {
	if len(left.Factors) != len(right.Factors) {
		return false
	}
	for index := range left.Factors {
		if left.Factors[index] != right.Factors[index] {
			return false
		}
	}
	return true
}
func equalUnitWire(left, right wireUnit) bool {
	return equalDimensionWire(left.Dimension, right.Dimension) && equalWireFactors(left.Factors, right.Factors) && equalWireFactors(left.ConversionFactors, right.ConversionFactors) && left.Scale == right.Scale
}
func equalWireFactors(left, right []wireUnitFactor) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
