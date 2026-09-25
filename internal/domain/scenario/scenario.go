package scenario

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
)

// Scenario is an immutable overlay over exactly one ModelVersion.
//
// Scenario stores only value differences. It does not copy or mutate the base
// model and it does not contain schema/structure changes.
//
// Scenario inheritance and composition are intentionally absent from Core v1.
type Scenario struct {
	id identity.UUID

	name string

	baseModelVersionID identity.UUID
	baseDatasetID      identity.UUID
	hasBaseDataset     bool

	overrides []Override
}

// New constructs an immutable Scenario.
//
// New panics when:
//
//   - id is zero;
//   - name is empty;
//   - baseModelVersionID is zero;
//   - an Override is structurally invalid.
//
// Multiple overrides may target the same PropertyAddress because scope and
// effective time may intentionally partition the applicable contexts.
// Equal-specificity overlaps are handled by validation/resolution rather than
// by insertion order.
func New(
	id identity.UUID,
	name string,
	baseModelVersionID identity.UUID,
	overrides []Override,
) Scenario {
	return newScenario(id, name, baseModelVersionID, identity.UUID{}, false, overrides)
}

// NewForDataset constructs a runtime-only Scenario bound to one exact
// Dataset in addition to its ModelVersion.
func NewForDataset(id identity.UUID, name string, baseModelVersionID, datasetID identity.UUID, overrides []Override) Scenario {
	return newScenario(id, name, baseModelVersionID, datasetID, true, overrides)
}

func newScenario(
	id identity.UUID,
	name string,
	baseModelVersionID identity.UUID,
	datasetID identity.UUID,
	hasDataset bool,
	overrides []Override,
) Scenario {
	if id.IsZero() {
		panic("scenario: id must not be zero")
	}

	if name == "" {
		panic("scenario: name must not be empty")
	}

	if baseModelVersionID.IsZero() {
		panic("scenario: base ModelVersion id must not be zero")
	}
	if hasDataset && datasetID.IsZero() {
		panic("scenario: base Dataset id must not be zero")
	}
	if !hasDataset {
		datasetID = identity.UUID{}
	}

	copiedOverrides := cloneAndValidateOverrides(overrides)

	return Scenario{
		id:                 id,
		name:               name,
		baseModelVersionID: baseModelVersionID,
		baseDatasetID:      datasetID,
		hasBaseDataset:     hasDataset,
		overrides:          copiedOverrides,
	}
}

// ID returns the stable Scenario identity.
func (s Scenario) ID() identity.UUID {
	return s.id
}

// Name returns the presentation name of the Scenario.
func (s Scenario) Name() string {
	return s.name
}

// BaseModelVersionID returns the immutable ModelVersion this Scenario overlays.
func (s Scenario) BaseModelVersionID() identity.UUID {
	return s.baseModelVersionID
}

// BaseDatasetID returns the exact Dataset required by a Dataset-bound
// Scenario. Model-only Scenarios return false and remain portable.
func (s Scenario) BaseDatasetID() (identity.UUID, bool) { return s.baseDatasetID, s.hasBaseDataset }

// Overrides returns a defensive copy of all Scenario overrides.
func (s Scenario) Overrides() []Override {
	if len(s.overrides) == 0 {
		return nil
	}

	result := make(
		[]Override,
		len(s.overrides),
	)

	copy(result, s.overrides)

	return result
}

// IsValid reports structural Scenario validity.
//
// It intentionally does not prove compatibility with the base ModelVersion.
func (s Scenario) IsValid() bool {
	if s.id.IsZero() ||
		s.name == "" ||
		s.baseModelVersionID.IsZero() {
		return false
	}
	if s.hasBaseDataset != !s.baseDatasetID.IsZero() {
		return false
	}

	for _, override := range s.overrides {
		if !override.IsValid() {
			return false
		}
	}

	return true
}

// Equal reports semantic equality of two immutable Scenario overlays. Runtime
// cache keys use this complete comparison rather than only Scenario ID so a
// malformed caller-provided duplicate ID can never contaminate a cache entry.
func (s Scenario) Equal(other Scenario) bool {
	if !s.IsValid() || !other.IsValid() ||
		s.id != other.id ||
		s.name != other.name ||
		s.baseModelVersionID != other.baseModelVersionID ||
		s.baseDatasetID != other.baseDatasetID ||
		s.hasBaseDataset != other.hasBaseDataset ||
		len(s.overrides) != len(other.overrides) {
		return false
	}

	for index := range s.overrides {
		if !s.overrides[index].Equal(other.overrides[index]) {
			return false
		}
	}

	return true
}

// String returns a human-readable Scenario representation.
func (s Scenario) String() string {
	return fmt.Sprintf(
		"Scenario(%s, %s, base=%s)",
		s.id.String(),
		s.name,
		s.baseModelVersionID.String(),
	)
}

func cloneAndValidateOverrides(
	source []Override,
) []Override {
	if len(source) == 0 {
		return nil
	}

	result := make(
		[]Override,
		len(source),
	)

	for index, override := range source {
		if !override.IsValid() {
			panic("scenario: override is invalid")
		}

		result[index] = override
	}

	return result
}
