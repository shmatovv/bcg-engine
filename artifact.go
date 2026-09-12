package bcg

import (
	"fmt"
	"io"
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/scenario"
	formatbcg "github.com/shmatovv/bcg-engine/internal/format/bcg"
)

// FormatVersion is the current portable .bcg wire-format version.
const FormatVersion = formatbcg.FormatVersion

// EngineCompatibility identifies the semantic engine generation supported by
// this public facade. It is not a source-control revision.
const EngineCompatibility = formatbcg.EngineCompatibilityV1

// Artifact is one portable committed Model and its optional immutable Scenario overlays.
type Artifact struct{ value formatbcg.Artifact }

// NewArtifact constructs a portable Artifact from a committed Model and zero
// or more immutable Scenarios. Every Scenario must belong to exactly that
// model version and pass the established Scenario validation rules.
func NewArtifact(model Model, scenarios ...Scenario) (Artifact, error) {
	if !model.IsValid() {
		return Artifact{}, fmt.Errorf("%w: committed model is required", ErrInvalidArgument)
	}
	values := make([]scenario.Scenario, len(scenarios))
	for index, candidate := range scenarios {
		values[index] = candidate.value
	}
	artifact, err := formatbcg.NewArtifact(model.name, model.version, values)
	if err != nil {
		return Artifact{}, publicError(err)
	}
	return Artifact{value: artifact}, nil
}

// Decode reads one .bcg artifact from r.
func Decode(r io.Reader) (Artifact, error) {
	decoded, err := formatbcg.Decode(r)
	if err != nil {
		return Artifact{}, publicError(err)
	}
	return Artifact{value: decoded}, nil
}

// Unmarshal decodes one complete .bcg document.
func Unmarshal(data []byte) (Artifact, error) {
	decoded, err := formatbcg.Unmarshal(data)
	if err != nil {
		return Artifact{}, publicError(err)
	}
	return Artifact{value: decoded}, nil
}

// Encode writes canonical deterministic .bcg JSON.
func Encode(w io.Writer, artifact Artifact) error {
	if !artifact.IsValid() {
		return fmt.Errorf("%w: artifact is invalid", ErrInvalidArgument)
	}
	return publicError(formatbcg.Encode(w, artifact.value))
}

// Marshal returns canonical deterministic .bcg JSON.
func Marshal(artifact Artifact) ([]byte, error) {
	if !artifact.IsValid() {
		return nil, fmt.Errorf("%w: artifact is invalid", ErrInvalidArgument)
	}
	encoded, err := formatbcg.Marshal(artifact.value)
	if err != nil {
		return nil, publicError(err)
	}
	return encoded, nil
}

// IsValid reports whether a is a decoded or otherwise valid portable artifact.
func (a Artifact) IsValid() bool { return a.value.IsValid() }

// ModelName returns the presentation name of the artifact's long-lived Model identity.
func (a Artifact) ModelName() string { return a.value.ModelName() }

// Model returns the committed immutable calculation model.
func (a Artifact) Model() Model {
	if !a.IsValid() {
		return Model{}
	}
	return newModel(a.value.ModelName(), a.value.ModelVersion())
}

// Scenario returns one Scenario by UUID.
func (a Artifact) Scenario(id ID) (Scenario, bool) {
	if !a.IsValid() || id.IsZero() {
		return Scenario{}, false
	}
	for _, candidate := range a.value.Scenarios() {
		if candidate.ID() == id.value {
			return scenarioFromInternal(candidate), true
		}
	}
	return Scenario{}, false
}

// Scenarios returns a deterministic owned list of Scenario descriptors.
func (a Artifact) Scenarios() []Scenario {
	if !a.IsValid() {
		return nil
	}
	items := a.value.Scenarios()
	sort.Slice(items, func(i, j int) bool { return items[i].ID().String() < items[j].ID().String() })
	result := make([]Scenario, len(items))
	for index, item := range items {
		result[index] = scenarioFromInternal(item)
	}
	return result
}

// Scenario is an immutable overlay tied to exactly one Model version.
type Scenario struct{ value scenario.Scenario }

func scenarioFromInternal(value scenario.Scenario) Scenario { return Scenario{value: value} }

// IsValid reports whether s is a structurally valid immutable Scenario.
func (s Scenario) IsValid() bool { return s.value.IsValid() }

// ID returns the Scenario UUID.
func (s Scenario) ID() ID { return idFromInternal(s.value.ID()) }

// Name returns the presentation name.
func (s Scenario) Name() string { return s.value.Name() }

// BaseVersionID returns the Model version that s overlays.
func (s Scenario) BaseVersionID() ID { return idFromInternal(s.value.BaseModelVersionID()) }
