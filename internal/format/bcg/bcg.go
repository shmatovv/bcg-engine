// Package bcg implements the versioned portable BCG model artifact format.
//
// BCG v1 is canonical UTF-8 JSON for immutable logical model data. Runtime
// execution state (indexes, sessions, caches, provenance and reports) is
// intentionally outside this package and is rebuilt after decoding.
package bcg

import (
	"errors"
	"fmt"
	"io"

	"github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/model"
	modelscenario "github.com/shmatovv/bcg-engine/internal/scenario"
)

const (
	// FormatVersion is the current version of the portable BCG wire schema.
	FormatVersion = 1

	// EngineCompatibilityV1 identifies the semantic engine generation that
	// understands BCG format v1. It is intentionally not a git revision.
	EngineCompatibilityV1 = "bcg-engine-v1"
)

var (
	ErrMalformedDocument              = errors.New("bcg: malformed document")
	ErrUnsupportedFormatVersion       = errors.New("bcg: unsupported format version")
	ErrUnsupportedEngineCompatibility = errors.New("bcg: unsupported engine compatibility")
	ErrWireSemantic                   = errors.New("bcg: wire semantic error")
	ErrModelValidation                = errors.New("bcg: model validation failed")
	ErrScenarioValidation             = errors.New("bcg: scenario validation failed")
	ErrArtifactInvalid                = errors.New("bcg: invalid artifact")
)

// Artifact is one committed immutable ModelVersion and its optional Scenario
// overlays. ModelName belongs to the long-lived Model identity rather than
// ModelVersion, so it is carried here explicitly.
//
// Artifact owns its Scenario slice. ModelVersion and Scenario values are
// immutable domain values.
type Artifact struct {
	modelName string
	version   model.ModelVersion
	scenarios []scenario.Scenario
}

// NewArtifact creates a portable logical artifact. The supplied version must
// already be committed. Every Scenario must be a valid overlay of it.
func NewArtifact(
	modelName string,
	version model.ModelVersion,
	scenarios []scenario.Scenario,
) (Artifact, error) {
	if modelName == "" || !version.IsCommitted() {
		return Artifact{}, fmt.Errorf("%w: committed model version and non-empty model name are required", ErrArtifactInvalid)
	}
	copyScenarios := append([]scenario.Scenario(nil), scenarios...)
	seenScenarioIDs := make(map[string]struct{}, len(copyScenarios))
	for index, candidate := range copyScenarios {
		if _, exists := seenScenarioIDs[candidate.ID().String()]; exists {
			return Artifact{}, fmt.Errorf("%w: scenarios[%d] duplicates Scenario ID", ErrArtifactInvalid, index)
		}
		seenScenarioIDs[candidate.ID().String()] = struct{}{}
		report := modelscenario.Validate(version, candidate)
		if !report.OK() {
			return Artifact{}, fmt.Errorf("%w: scenarios[%d]: %w", ErrScenarioValidation, index, report)
		}
	}
	return Artifact{modelName: modelName, version: version, scenarios: copyScenarios}, nil
}

func (a Artifact) ModelName() string { return a.modelName }

func (a Artifact) ModelVersion() model.ModelVersion { return a.version }

func (a Artifact) Scenarios() []scenario.Scenario {
	return append([]scenario.Scenario(nil), a.scenarios...)
}

func (a Artifact) IsValid() bool {
	if a.modelName == "" || !a.version.IsCommitted() {
		return false
	}
	for _, candidate := range a.scenarios {
		if !modelscenario.Validate(a.version, candidate).OK() {
			return false
		}
	}
	return true
}

// Marshal returns canonical UTF-8 JSON for a valid artifact.
func Marshal(artifact Artifact) ([]byte, error) {
	if !artifact.IsValid() {
		return nil, fmt.Errorf("%w: cannot marshal invalid artifact", ErrArtifactInvalid)
	}
	document, err := newWireDocument(artifact)
	if err != nil {
		return nil, err
	}
	return marshalCanonical(document)
}

// Encode writes canonical UTF-8 JSON to writer.
func Encode(writer io.Writer, artifact Artifact) error {
	if writer == nil {
		return fmt.Errorf("%w: nil writer", ErrArtifactInvalid)
	}
	data, err := Marshal(artifact)
	if err != nil {
		return err
	}
	n, err := writer.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

// Unmarshal parses one complete BCG JSON document.
func Unmarshal(data []byte) (Artifact, error) {
	document, err := parseDocument(data)
	if err != nil {
		return Artifact{}, err
	}
	return document.toArtifact()
}

// Decode reads and parses one complete BCG JSON document. It intentionally
// owns no files or storage concerns.
func Decode(reader io.Reader) (Artifact, error) {
	data, err := readDocument(reader)
	if err != nil {
		return Artifact{}, err
	}
	return Unmarshal(data)
}
