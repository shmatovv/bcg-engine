package model

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/internal/graph/dependency"
)

// draftDependencySource adapts a Draft to dependency.Source.
//
// Dependency extraction is needed before Commit because cycle detection is
// itself a blocking validation step.
//
// Draft lookup semantics remain tolerant:
//
//   - validationIndexes use first occurrence wins for duplicate UUIDs;
//   - ContractImplementation refuses ambiguous implementations;
//   - malformed model objects are returned as-is and dependency.Build decides
//     whether they are usable.
//
// This adapter does not define committed model semantics.
type draftDependencySource struct {
	draft   *Draft
	indexes validationIndexes
}

func newDraftDependencySource(
	draft *Draft,
	indexes validationIndexes,
) draftDependencySource {
	if draft == nil {
		panic("model: nil draft dependency source")
	}

	return draftDependencySource{
		draft:   draft,
		indexes: indexes,
	}
}

func (s draftDependencySource) Entities() []semantic.Entity {
	return cloneSlice(
		s.draft.entities,
	)
}

func (s draftDependencySource) Relations() []semantic.Relation {
	return cloneSlice(
		s.draft.relations,
	)
}

func (s draftDependencySource) OutgoingRelations(entityID, relationTypeID identity.UUID) []semantic.Relation {
	result := make([]semantic.Relation, 0)
	for _, relation := range s.draft.relations {
		if relation.TypeID() == relationTypeID && relation.From() == entityID {
			result = append(result, relation)
		}
	}
	return result
}

func (s draftDependencySource) IncomingRelations(entityID, relationTypeID identity.UUID) []semantic.Relation {
	result := make([]semantic.Relation, 0)
	for _, relation := range s.draft.relations {
		if relation.TypeID() == relationTypeID && relation.To() == entityID {
			result = append(result, relation)
		}
	}
	return result
}

func (s draftDependencySource) EntityType(
	id identity.UUID,
) (schema.EntityType, bool) {
	result, ok :=
		s.indexes.entityTypeByID[id]

	return result, ok
}

func (s draftDependencySource) RelationType(
	id identity.UUID,
) (schema.RelationType, bool) {
	result, ok :=
		s.indexes.relationTypeByID[id]

	return result, ok
}

func (s draftDependencySource) PropertyDefinition(
	id identity.UUID,
) (schema.PropertyDefinition, bool) {
	result, ok :=
		s.indexes.propertyDefinitionByID[id]

	return result, ok
}

func (s draftDependencySource) Entity(
	id identity.UUID,
) (semantic.Entity, bool) {
	result, ok :=
		s.indexes.entityByID[id]

	return result, ok
}

func (s draftDependencySource) Relation(
	id identity.UUID,
) (semantic.Relation, bool) {
	result, ok :=
		s.indexes.relationByID[id]

	return result, ok
}

// ContractImplementation resolves exactly one PropertyDefinition implementing
// contractID for owner.
//
// Draft may temporarily contain duplicate implementations. Dependency
// extraction must not silently pick one of them because doing so could hide a
// dependency or produce a graph dependent on insertion order.
//
// Therefore:
//
//	0 implementations -> false
//	1 implementation  -> definition, true
//	2+ implementations -> false
func (s draftDependencySource) ContractImplementation(
	owner schema.PropertyOwner,
	contractID identity.UUID,
) (schema.PropertyDefinition, bool) {
	if !owner.IsValid() ||
		contractID.IsZero() {

		return schema.PropertyDefinition{},
			false
	}

	var result schema.PropertyDefinition
	found := false

	for _, definition := range s.draft.propertyDefinitions {

		if !definition.IsValid() ||
			definition.Owner() != owner {

			continue
		}

		candidateContractID, hasContract :=
			definition.ContractID()

		if !hasContract ||
			candidateContractID !=
				contractID {

			continue
		}

		if found {
			return schema.PropertyDefinition{},
				false
		}

		result = definition
		found = true
	}

	if !found {
		return schema.PropertyDefinition{},
			false
	}

	return result, true
}

var _ dependency.Source = draftDependencySource{}
