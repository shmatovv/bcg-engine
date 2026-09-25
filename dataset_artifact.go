package bcg

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	formatbcg "github.com/shmatovv/bcg-engine/v2/internal/format/bcg"
)

// DatasetFormatVersion is independent of the .bcg model artifact version.
const DatasetFormatVersion = 1

// DatasetEngineCompatibility identifies the BCG Dataset Format v1 contract.
const DatasetEngineCompatibility = "bcg-engine-v2"

// DatasetArtifact is an immutable portable runtime snapshot. Schema and
// executable function implementations are intentionally absent.
type DatasetArtifact struct {
	dataset                    Dataset
	modelVersionID             ID
	modelDigest, contentDigest Digest
}

func (a DatasetArtifact) IsValid() bool {
	return a.dataset.IsValid() && !a.modelVersionID.IsZero() && a.modelDigest != (Digest{}) && a.contentDigest != (Digest{})
}
func (a DatasetArtifact) Dataset() Dataset             { return a.dataset }
func (a DatasetArtifact) DatasetID() ID                { return a.dataset.ID() }
func (a DatasetArtifact) ModelVersionID() ID           { return a.modelVersionID }
func (a DatasetArtifact) ModelSemanticDigest() Digest  { return a.modelDigest }
func (a DatasetArtifact) DatasetContentDigest() Digest { return a.contentDigest }

func NewDatasetArtifact(dataset Dataset) (DatasetArtifact, error) {
	if !dataset.IsValid() || dataset.modelDigest == (Digest{}) {
		return DatasetArtifact{}, fmt.Errorf("%w: valid Dataset built from a Model is required", ErrInvalidArgument)
	}
	return DatasetArtifact{dataset: dataset, modelVersionID: dataset.ModelVersionID(), modelDigest: dataset.modelDigest, contentDigest: dataset.ContentDigest()}, nil
}

type datasetDocument struct {
	FormatVersion        int             `json:"formatVersion"`
	EngineCompatibility  string          `json:"engineCompatibility"`
	DatasetID            string          `json:"datasetId"`
	ModelVersionID       string          `json:"modelVersionId"`
	ModelSemanticDigest  string          `json:"modelSemanticDigest"`
	DatasetContentDigest string          `json:"datasetContentDigest"`
	Content              json.RawMessage `json:"content"`
}

func MarshalDataset(a DatasetArtifact) ([]byte, error) {
	if !a.IsValid() || a.dataset.ID().IsZero() {
		return nil, fmt.Errorf("%w: dataset artifact is invalid", ErrInvalidArgument)
	}
	c, err := formatbcg.MarshalDatasetContent(a.dataset.value)
	if err != nil {
		return nil, publicError(err)
	}
	d := datasetDocument{DatasetFormatVersion, DatasetEngineCompatibility, a.DatasetID().String(), a.modelVersionID.String(), a.modelDigest.String(), a.contentDigest.String(), c}
	return json.Marshal(d)
}
func EncodeDataset(w io.Writer, a DatasetArtifact) error {
	if isNilInterface(w) {
		return fmt.Errorf("%w: writer is nil", ErrInvalidArgument)
	}
	b, e := MarshalDataset(a)
	if e != nil {
		return e
	}
	n, e := w.Write(b)
	if e != nil {
		return e
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}
func UnmarshalDataset(model Model, data []byte) (DatasetArtifact, error) {
	return decodeDataset(model, data)
}
func DecodeDataset(model Model, r io.Reader) (DatasetArtifact, error) {
	if isNilInterface(r) {
		return DatasetArtifact{}, fmt.Errorf("%w: reader is nil", ErrInvalidArgument)
	}
	b, e := io.ReadAll(io.LimitReader(r, (64<<20)+1))
	if e != nil {
		return DatasetArtifact{}, e
	}
	if len(b) > 64<<20 {
		return DatasetArtifact{}, fmt.Errorf("%w: .bcgd exceeds 64MiB limit", ErrMalformedArtifact)
	}
	return decodeDataset(model, b)
}
func decodeDataset(m Model, b []byte) (DatasetArtifact, error) {
	if !m.IsValid() {
		return DatasetArtifact{}, fmt.Errorf("%w: committed model required", ErrInvalidArgument)
	}
	if err := formatbcg.ValidateDatasetEnvelope(b); err != nil {
		return DatasetArtifact{}, publicError(err)
	}
	var d datasetDocument
	if err := json.Unmarshal(b, &d); err != nil {
		return DatasetArtifact{}, fmt.Errorf("%w: malformed .bcgd: %v", ErrMalformedArtifact, err)
	}
	if d.FormatVersion != DatasetFormatVersion || d.EngineCompatibility != DatasetEngineCompatibility {
		return DatasetArtifact{}, fmt.Errorf("%w: unsupported .bcgd format", ErrMalformedArtifact)
	}
	id, e := ParseID(d.DatasetID)
	if e != nil {
		return DatasetArtifact{}, e
	}
	vid, e := ParseID(d.ModelVersionID)
	if e != nil {
		return DatasetArtifact{}, e
	}
	if !vid.Equal(m.VersionID()) {
		return DatasetArtifact{}, fmt.Errorf("%w: Dataset ModelVersionID mismatch", ErrInvalidArgument)
	}
	md, e := parseDigest(d.ModelSemanticDigest)
	if e != nil {
		return DatasetArtifact{}, e
	}
	if md != m.SemanticDigest() {
		return DatasetArtifact{}, fmt.Errorf("%w: Dataset ModelSemanticDigest mismatch", ErrInvalidArgument)
	}
	cd, e := parseDigest(d.DatasetContentDigest)
	if e != nil {
		return DatasetArtifact{}, e
	}
	v, e := formatbcg.UnmarshalDatasetContent(m.version, id.value, d.Content)
	if e != nil {
		return DatasetArtifact{}, publicError(e)
	}
	ds := Dataset{value: v, modelDigest: md}
	if ds.ContentDigest() != cd {
		return DatasetArtifact{}, fmt.Errorf("%w: DatasetContentDigest mismatch", ErrInvalidArgument)
	}
	return DatasetArtifact{dataset: ds, modelVersionID: vid, modelDigest: md, contentDigest: cd}, nil
}
func parseDigest(s string) (Digest, error) {
	var d Digest
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != len(d) {
		return d, fmt.Errorf("%w: invalid digest", ErrInvalidArgument)
	}
	copy(d[:], b)
	return d, nil
}
