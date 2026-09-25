package bcg

// Runtime-only canonical dataset payload helpers. They deliberately reuse the
// established BCG value/series wire codecs, while the public .bcgd envelope
// remains an independent format contract.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"unicode/utf8"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
)

type datasetContentWire struct {
	Entities  []wireEntity   `json:"entities"`
	Relations []wireRelation `json:"relations"`
	Bindings  []wireBinding  `json:"bindings"`
}

// ValidateDatasetEnvelope applies the same hostile-input JSON checks used by
// .bcg before the public .bcgd facade interprets identity fields.
func ValidateDatasetEnvelope(data []byte) error {
	if len(data) == 0 || len(data) > maxDocumentBytes || !utf8.Valid(data) || bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return fmt.Errorf("%w: invalid dataset document", ErrMalformedDocument)
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	var envelope struct {
		FormatVersion        *int             `json:"formatVersion"`
		EngineCompatibility  *string          `json:"engineCompatibility"`
		DatasetID            *string          `json:"datasetId"`
		ModelVersionID       *string          `json:"modelVersionId"`
		ModelSemanticDigest  *string          `json:"modelSemanticDigest"`
		DatasetContentDigest *string          `json:"datasetContentDigest"`
		Content              *json.RawMessage `json:"content"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("%w: dataset document JSON: %v", ErrMalformedDocument, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: trailing dataset document data", ErrMalformedDocument)
	}
	if envelope.FormatVersion == nil || envelope.EngineCompatibility == nil || envelope.DatasetID == nil || envelope.ModelVersionID == nil || envelope.ModelSemanticDigest == nil || envelope.DatasetContentDigest == nil || envelope.Content == nil {
		return fmt.Errorf("%w: missing dataset document field", ErrMalformedDocument)
	}
	return nil
}

func MarshalDatasetContent(d model.Dataset) ([]byte, error) {
	w := datasetContentWire{Entities: make([]wireEntity, 0), Relations: make([]wireRelation, 0), Bindings: make([]wireBinding, 0)}
	for _, v := range d.Entities() {
		w.Entities = append(w.Entities, wireEntityFromDomain(v))
	}
	for _, v := range d.Relations() {
		x, e := wireRelationFromDomain(v)
		if e != nil {
			return nil, e
		}
		w.Relations = append(w.Relations, x)
	}
	for _, v := range d.ValueBindings() {
		x, e := wireBindingFromDomain(v)
		if e != nil {
			return nil, e
		}
		w.Bindings = append(w.Bindings, x)
	}
	sortWireByJSON(w.Entities)
	sortWireByJSON(w.Relations)
	sortWireByJSON(w.Bindings)
	return json.Marshal(w)
}

func UnmarshalDatasetContent(version model.ModelVersion, id identity.UUID, data []byte) (model.Dataset, error) {
	if len(data) == 0 || len(data) > maxDocumentBytes || !utf8.Valid(data) {
		return model.Dataset{}, fmt.Errorf("%w: invalid dataset content", ErrMalformedDocument)
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return model.Dataset{}, err
	}
	if err := validateRequiredWireFields(data, reflect.TypeFor[datasetContentWire](), ""); err != nil {
		return model.Dataset{}, err
	}
	var w datasetContentWire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&w); err != nil {
		return model.Dataset{}, fmt.Errorf("%w: dataset content JSON: %v", ErrMalformedDocument, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return model.Dataset{}, fmt.Errorf("%w: trailing dataset content", ErrMalformedDocument)
	}
	b, err := model.NewDatasetBuilder(version, id)
	if err != nil {
		return model.Dataset{}, err
	}
	for i, x := range w.Entities {
		v, e := x.toDomain(fmt.Sprintf("entities[%d]", i))
		if e != nil {
			return model.Dataset{}, e
		}
		if e = b.AddEntity(v); e != nil {
			return model.Dataset{}, e
		}
	}
	for i, x := range w.Relations {
		v, e := x.toDomain(fmt.Sprintf("relations[%d]", i), 0)
		if e != nil {
			return model.Dataset{}, e
		}
		if e = b.AddRelation(v); e != nil {
			return model.Dataset{}, e
		}
	}
	for i, x := range w.Bindings {
		v, e := x.toDomain(fmt.Sprintf("bindings[%d]", i))
		if e != nil {
			return model.Dataset{}, e
		}
		if e = b.AddValueBinding(v); e != nil {
			return model.Dataset{}, e
		}
	}
	return b.Build()
}
