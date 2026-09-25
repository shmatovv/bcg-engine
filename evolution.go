package bcg

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	formatbcg "github.com/shmatovv/bcg-engine/v2/internal/format/bcg"
)

// ChangeKind identifies the persistent semantic declaration family affected
// by a ModelChange.
type ChangeKind string

const (
	ChangeEntityType          ChangeKind = "EntityType"
	ChangeRelationType        ChangeKind = "RelationType"
	ChangePropertyDefinition  ChangeKind = "PropertyDefinition"
	ChangePropertyContract    ChangeKind = "PropertyContract"
	ChangeTrait               ChangeKind = "Trait"
	ChangeScopeDefinition     ChangeKind = "ScopeDefinition"
	ChangeEnumDefinition      ChangeKind = "EnumDefinition"
	ChangeEnumOption          ChangeKind = "EnumOption"
	ChangeEntityKeyDefinition ChangeKind = "EntityKeyDefinition"
	ChangeInvariant           ChangeKind = "Invariant"
	ChangeConstraint          ChangeKind = "Constraint"
	ChangeRule                ChangeKind = "Rule"
	ChangeEntity              ChangeKind = "Entity"
	ChangeRelation            ChangeKind = "Relation"
	ChangeValueBinding        ChangeKind = "ValueBinding"
	ChangeFunctionRequirement ChangeKind = "FunctionRequirement"
	ChangePrecisionPolicy     ChangeKind = "PrecisionPolicy"
)

// ChangeOperation says whether a stable declaration was introduced, removed,
// or changed in place.
type ChangeOperation string

const (
	ChangeAdded    ChangeOperation = "Added"
	ChangeRemoved  ChangeOperation = "Removed"
	ChangeModified ChangeOperation = "Modified"
)

// ModelChange is an immutable, typed description of one persistent semantic
// declaration change. ID is present for UUID-addressable declarations; Key is
// a canonical compound identity for value bindings and function requirements.
type ModelChange struct {
	kind                     ChangeKind
	id                       ID
	hasID                    bool
	key                      string
	operation                ChangeOperation
	fields                   []string
	oldFeatures, newFeatures []string
}

func (c ModelChange) Kind() ChangeKind           { return c.kind }
func (c ModelChange) ID() (ID, bool)             { return c.id, c.hasID }
func (c ModelChange) Key() string                { return c.key }
func (c ModelChange) Operation() ChangeOperation { return c.operation }
func (c ModelChange) ChangedFields() []string    { return append([]string(nil), c.fields...) }

// ModelDiff is the deterministic semantic comparison of two committed
// versions of one logical Model. Changes are sorted by declaration family and
// stable identity.
type ModelDiff struct{ changes []ModelChange }

func (d ModelDiff) Changes() []ModelChange { return append([]ModelChange(nil), d.changes...) }
func (d ModelDiff) Len() int               { return len(d.changes) }
func (d ModelDiff) Empty() bool            { return len(d.changes) == 0 }

// CompareModels compares committed versions of the same logical Model. Names
// are diagnostic metadata, so a same-ID rename is a modification, never a
// remove/add pair.
func CompareModels(oldModel, newModel Model) (ModelDiff, error) {
	if !oldModel.IsValid() || !newModel.IsValid() {
		return ModelDiff{}, fmt.Errorf("%w: committed models required", ErrInvalidModel)
	}
	if oldModel.modelID != newModel.modelID {
		return ModelDiff{}, fmt.Errorf("%w: %s != %s", ErrDifferentLogicalModel, oldModel.ModelID(), newModel.ModelID())
	}
	changes, err := formatbcg.CompareSemanticVersions(oldModel.version, newModel.version)
	if err != nil {
		return ModelDiff{}, fmt.Errorf("%w: semantic projection: %v", ErrInvalidModel, err)
	}
	result := ModelDiff{changes: make([]ModelChange, 0, len(changes))}
	for _, change := range changes {
		item := ModelChange{kind: ChangeKind(change.Kind), key: change.Key, operation: ChangeOperation(change.Operation), fields: append([]string(nil), change.Fields...)}
		item.oldFeatures, item.newFeatures = append([]string(nil), change.OldFeatures...), append([]string(nil), change.NewFeatures...)
		if change.ID != "" {
			id, err := ParseID(change.ID)
			if err != nil {
				return ModelDiff{}, err
			}
			item.id, item.hasID = id, true
		}
		result.changes = append(result.changes, item)
	}
	sort.Slice(result.changes, func(i, j int) bool {
		a, b := result.changes[i], result.changes[j]
		ai, bi := a.key, b.key
		if a.hasID {
			ai = a.id.String()
		}
		if b.hasID {
			bi = b.id.String()
		}
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		return ai < bi
	})
	return result, nil
}

// EvolutionEffect is a conservative consequence inferred solely from model
// semantics. It never asserts facts about records in any Dataset.
type EvolutionEffect uint16

const (
	MetadataOnly EvolutionEffect = 1 << iota
	EvaluationSemanticsChanged
	RequiresDatasetStructuralRevalidation
	RequiresIntegrityRecheck
	RequiresRecalculation
	RequiresTopologyRebuild
	PotentialDataMigration
	PotentialDataLoss
)

// EvolutionEffects is an immutable flag set.
type EvolutionEffects uint16

func (e EvolutionEffects) Has(effect EvolutionEffect) bool { return e&EvolutionEffects(effect) != 0 }
func (e EvolutionEffects) Values() []EvolutionEffect {
	all := []EvolutionEffect{MetadataOnly, EvaluationSemanticsChanged, RequiresDatasetStructuralRevalidation, RequiresIntegrityRecheck, RequiresRecalculation, RequiresTopologyRebuild, PotentialDataMigration, PotentialDataLoss}
	out := make([]EvolutionEffect, 0, len(all))
	for _, v := range all {
		if e.Has(v) {
			out = append(out, v)
		}
	}
	return out
}

// EvolutionChange associates conservative effect flags with a ModelChange.
type EvolutionChange struct {
	change  ModelChange
	effects EvolutionEffects
}

func (c EvolutionChange) Change() ModelChange       { return c.change }
func (c EvolutionChange) Effects() EvolutionEffects { return c.effects }

// EvolutionReport aggregates immutable per-change and report-wide effects.
type EvolutionReport struct {
	changes []EvolutionChange
	effects EvolutionEffects
}

func (r EvolutionReport) Changes() []EvolutionChange {
	return append([]EvolutionChange(nil), r.changes...)
}
func (r EvolutionReport) Effects() EvolutionEffects       { return r.effects }
func (r EvolutionReport) Has(effect EvolutionEffect) bool { return r.effects.Has(effect) }
func (r EvolutionReport) RequiresDataMigration() bool     { return r.Has(PotentialDataMigration) }
func (r EvolutionReport) RequiresRecalculation() bool     { return r.Has(RequiresRecalculation) }
func (r EvolutionReport) HasPotentialDataLoss() bool      { return r.Has(PotentialDataLoss) }

// AnalyzeEvolution conservatively classifies an evolution. It does not
// inspect, migrate, or otherwise make claims about a concrete Dataset.
func AnalyzeEvolution(oldModel, newModel Model) (EvolutionReport, error) {
	diff, err := CompareModels(oldModel, newModel)
	if err != nil {
		return EvolutionReport{}, err
	}
	r := EvolutionReport{changes: make([]EvolutionChange, 0, diff.Len())}
	for _, c := range diff.changes {
		effects := classifyEvolution(c)
		r.changes = append(r.changes, EvolutionChange{change: c, effects: effects})
		r.effects |= effects
	}
	return r, nil
}

func classifyEvolution(c ModelChange) EvolutionEffects {
	// Name is the sole diagnostic field. All other persistent changes are
	// evaluation semantics unless they are strictly schema metadata.
	if c.operation == ChangeModified && namesOnly(c.fields) {
		return EvolutionEffects(MetadataOnly)
	}
	e := EvolutionEffects(EvaluationSemanticsChanged)
	fields := func(name string) bool {
		for _, f := range c.fields {
			if f == name || strings.HasPrefix(f, name+".") {
				return true
			}
		}
		return false
	}
	switch c.kind {
	case ChangePropertyDefinition:
		e |= EvolutionEffects(RequiresDatasetStructuralRevalidation | RequiresRecalculation)
		if c.operation == ChangeRemoved {
			e |= EvolutionEffects(PotentialDataMigration | PotentialDataLoss)
		}
		if c.operation == ChangeAdded {
			if feature(c.newFeatures, "derived") {
				e |= EvolutionEffects(RequiresRecalculation)
			}
			if feature(c.newFeatures, "presence:required") {
				e |= EvolutionEffects(PotentialDataMigration | RequiresIntegrityRecheck)
			}
		}
		if c.operation == ChangeModified && (fields("ValueType") || fields("EnumDefinitionID") || fields("Unit")) {
			e |= EvolutionEffects(PotentialDataMigration)
		}
		if c.operation == ChangeModified && fields("Computation") && feature(c.oldFeatures, "input") != feature(c.newFeatures, "input") {
			e |= EvolutionEffects(PotentialDataMigration)
		}
		if feature(c.oldFeatures, "presence:optional") && feature(c.newFeatures, "presence:required") {
			e |= EvolutionEffects(PotentialDataMigration | RequiresIntegrityRecheck)
		}
	case ChangeEntityType, ChangeRelationType, ChangeEntity, ChangeRelation:
		e |= EvolutionEffects(RequiresDatasetStructuralRevalidation | RequiresTopologyRebuild)
		if c.operation == ChangeRemoved {
			e |= EvolutionEffects(PotentialDataMigration | PotentialDataLoss)
		}
		if c.kind == ChangeRelation || c.kind == ChangeRelationType {
			e |= EvolutionEffects(RequiresRecalculation)
		}
	case ChangeEnumDefinition, ChangeEnumOption:
		e |= EvolutionEffects(RequiresDatasetStructuralRevalidation)
		if c.operation == ChangeRemoved {
			e |= EvolutionEffects(PotentialDataMigration | PotentialDataLoss)
		}
	case ChangeEntityKeyDefinition, ChangeInvariant:
		e |= EvolutionEffects(RequiresIntegrityRecheck)
	case ChangeConstraint, ChangeRule:
		e |= EvolutionEffects(RequiresRecalculation)
	case ChangeValueBinding:
		e |= EvolutionEffects(RequiresRecalculation)
		if c.operation == ChangeRemoved {
			e |= EvolutionEffects(PotentialDataMigration | PotentialDataLoss)
		}
	case ChangeFunctionRequirement:
		e |= EvolutionEffects(RequiresRecalculation)
	case ChangePropertyContract, ChangeTrait, ChangeScopeDefinition:
		e |= EvolutionEffects(RequiresDatasetStructuralRevalidation | RequiresRecalculation)
	case ChangePrecisionPolicy:
		e |= EvolutionEffects(RequiresRecalculation)
	}
	// Cardinality and endpoint changes are topology/integrity concerns.
	if c.kind == ChangeRelationType && cardinalityTightened(c.oldFeatures, c.newFeatures) {
		e |= EvolutionEffects(RequiresIntegrityRecheck)
	}
	if c.kind == ChangeRelationType && (fields("From") || fields("To")) {
		e |= EvolutionEffects(RequiresTopologyRebuild | RequiresIntegrityRecheck)
	}
	if c.kind == ChangePropertyDefinition && fields("Computation") {
		e |= EvolutionEffects(RequiresRecalculation)
	}
	return e
}

func namesOnly(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		if f != "Name" && f != "Option.Name" {
			return false
		}
	}
	return true
}
func feature(features []string, want string) bool {
	for _, f := range features {
		if f == want {
			return true
		}
	}
	return false
}

func cardinalityTightened(oldFeatures, newFeatures []string) bool {
	old, next := cardinalityFeatures(oldFeatures), cardinalityFeatures(newFeatures)
	for _, direction := range []string{"outgoing", "incoming"} {
		omin, ook := old[direction+":min"]
		nmin, nok := next[direction+":min"]
		if ook && nok && nmin > omin {
			return true
		}
		omax, oldBounded := old[direction+":max"]
		nmax, newBounded := next[direction+":max"]
		if !oldBounded && newBounded {
			return true
		}
		if oldBounded && newBounded && nmax < omax {
			return true
		}
	}
	return false
}
func cardinalityFeatures(features []string) map[string]uint64 {
	r := map[string]uint64{}
	for _, f := range features {
		parts := strings.SplitN(f, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if strings.HasSuffix(key, "-max") {
			key = strings.TrimSuffix(key, "-max") + ":max"
		} else if key == "outgoing" || key == "incoming" {
			key += ":min"
		} else {
			continue
		}
		if n, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
			r[key] = n
		}
	}
	return r
}
