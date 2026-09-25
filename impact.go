package bcg

import "github.com/shmatovv/bcg-engine/v2/internal/impact"

// ImpactMode selects structural, effective, or actually observed dependency semantics.
type ImpactMode string

const (
	StructuralImpact ImpactMode = "Structural"
	EffectiveImpact  ImpactMode = "Effective"
	ObservedImpact   ImpactMode = "Observed"
)

// ImpactAffected is one downstream property and its minimum dependency distance.
type ImpactAffected struct {
	target       PropertyAddress
	minimumDepth int
}

// Target returns the affected property.
func (a ImpactAffected) Target() PropertyAddress { return a.target }

// MinimumDepth returns the shortest dependency path length from the source.
func (a ImpactAffected) MinimumDepth() int { return a.minimumDepth }

// IsDirect reports whether Target directly depends on the source.
func (a ImpactAffected) IsDirect() bool { return a.minimumDepth == 1 }

// ImpactEdge is one direct dependency-to-dependent edge in a result DAG.
type ImpactEdge struct{ from, to PropertyAddress }

// From returns the dependency property.
func (e ImpactEdge) From() PropertyAddress { return e.from }

// To returns the dependent property.
func (e ImpactEdge) To() PropertyAddress { return e.to }

// ImpactResult is a deterministic, immutable downstream consequence report.
type ImpactResult struct {
	modelVersionID ID
	datasetID      ID
	hasDataset     bool
	source         PropertyAddress
	mode           ImpactMode
	affected       []ImpactAffected
	edges          []ImpactEdge
}

// ModelVersionID returns the model version used for this analysis.
func (r ImpactResult) ModelVersionID() ID { return r.modelVersionID }

// DatasetID returns the optional exact Dataset used for this analysis.
// Model-only impact results return false.
func (r ImpactResult) DatasetID() (ID, bool) { return r.datasetID, r.hasDataset }

// Source returns the property from which downstream impact was analyzed.
func (r ImpactResult) Source() PropertyAddress { return r.source }

// Mode returns the semantics used for this impact result.
func (r ImpactResult) Mode() ImpactMode { return r.mode }

// Affected returns an owned deterministic copy of downstream properties.
func (r ImpactResult) Affected() []ImpactAffected {
	return append([]ImpactAffected(nil), r.affected...)
}

// Edges returns an owned deterministic copy of direct result-DAG edges.
func (r ImpactResult) Edges() []ImpactEdge { return append([]ImpactEdge(nil), r.edges...) }

// Len returns the number of affected properties.
func (r ImpactResult) Len() int { return len(r.affected) }

func impactResultFromInternal(value impact.Result) ImpactResult {
	result := ImpactResult{modelVersionID: idFromInternal(value.ModelVersionID()), source: addressFromInternal(value.Source()), mode: impactModeFromInternal(value.Mode())}
	if datasetID, ok := value.SourceIdentity().DatasetID(); ok {
		result.datasetID, result.hasDataset = idFromInternal(datasetID), true
	}
	affected := value.Affected()
	result.affected = make([]ImpactAffected, len(affected))
	for i, item := range affected {
		result.affected[i] = ImpactAffected{target: addressFromInternal(item.Target()), minimumDepth: item.MinimumDepth()}
	}
	edges := value.Edges()
	result.edges = make([]ImpactEdge, len(edges))
	for i, item := range edges {
		result.edges[i] = ImpactEdge{from: addressFromInternal(item.From()), to: addressFromInternal(item.To())}
	}
	return result
}
func impactModeFromInternal(mode impact.Mode) ImpactMode {
	switch mode {
	case impact.ModeStructural:
		return StructuralImpact
	case impact.ModeEffective:
		return EffectiveImpact
	case impact.ModeObserved:
		return ObservedImpact
	default:
		return ""
	}
}
