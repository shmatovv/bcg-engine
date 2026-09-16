package model

import "github.com/shmatovv/bcg-engine/internal/domain/identity"

// EffectiveSourceIdentity identifies the immutable semantic graph used by a
// runtime evaluation. A model-only source has no DatasetID; a combined source
// is additionally bound to one exact Dataset snapshot.
//
// It is deliberately separate from ModelVersion identity: a Dataset can add
// concrete objects, relations and bindings without changing the model schema.
type EffectiveSourceIdentity struct {
	modelVersionID identity.UUID
	datasetID      identity.UUID
	hasDataset     bool
}

func NewModelSourceIdentity(modelVersionID identity.UUID) EffectiveSourceIdentity {
	if modelVersionID.IsZero() {
		panic("model: effective source ModelVersion id must not be zero")
	}
	return EffectiveSourceIdentity{modelVersionID: modelVersionID}
}

func NewDatasetSourceIdentity(modelVersionID, datasetID identity.UUID) EffectiveSourceIdentity {
	if modelVersionID.IsZero() || datasetID.IsZero() {
		panic("model: effective dataset source identities must not be zero")
	}
	return EffectiveSourceIdentity{modelVersionID: modelVersionID, datasetID: datasetID, hasDataset: true}
}

func (i EffectiveSourceIdentity) ModelVersionID() identity.UUID { return i.modelVersionID }

func (i EffectiveSourceIdentity) DatasetID() (identity.UUID, bool) { return i.datasetID, i.hasDataset }

func (i EffectiveSourceIdentity) IsValid() bool {
	if i.modelVersionID.IsZero() {
		return false
	}
	return (!i.hasDataset && i.datasetID.IsZero()) || (i.hasDataset && !i.datasetID.IsZero())
}

func (i EffectiveSourceIdentity) Equal(other EffectiveSourceIdentity) bool {
	return i.IsValid() && other.IsValid() &&
		i.modelVersionID == other.modelVersionID &&
		i.datasetID == other.datasetID &&
		i.hasDataset == other.hasDataset
}
