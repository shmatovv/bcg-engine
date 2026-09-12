package dependency

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
)

// Build constructs the concrete value dependency graph for source.
//
// Every concrete property of every Entity and Relation becomes a graph
// vertex.
//
// Input properties have no expression-owned incoming dependencies.
//
// Derived properties are expanded from their AST into concrete
// PropertyAddress dependencies.
//
// Aggregations are expanded against the immutable Semantic Graph.
// Bound references therefore resolve to concrete Entity/Relation addresses.
func Build(
	source Source,
) (Graph, error) {
	if source == nil {
		panic(
			"dependency: source must not be nil",
		)
	}

	builder := graphBuilder{
		source: source,
		graph:  newGraph(),

		entityTypes:   make(map[identity.UUID]schema.EntityType),
		relationTypes: make(map[identity.UUID]schema.RelationType),
		objectOwners:  make(map[address.ObjectRef]schema.PropertyOwner),
	}

	if err := builder.addEntityProperties(); err != nil {
		return Graph{}, err
	}

	if err := builder.addRelationProperties(); err != nil {
		return Graph{}, err
	}

	return builder.graph, nil
}

type graphBuilder struct {
	source Source
	graph  Graph

	// Schema values validate their owned definition slices. Revalidating the
	// same type for every Entity, Relation or property reference turns a wide
	// model into O(objects × definitions). These maps retain only already
	// validated immutable source objects for this one Build invocation.
	entityTypes   map[identity.UUID]schema.EntityType
	relationTypes map[identity.UUID]schema.RelationType
	objectOwners  map[address.ObjectRef]schema.PropertyOwner
}

func (b *graphBuilder) addEntityProperties() error {
	entities := b.source.Entities()

	sort.Slice(
		entities,
		func(i, j int) bool {
			return compareUUID(
				entities[i].ID(),
				entities[j].ID(),
			) < 0
		},
	)

	for _, entity := range entities {
		if !entity.IsValid() {
			return newBuildError(
				BuildErrorInvalidSourceObject,
				"entity is structurally invalid",
			)
		}

		entityType, ok := b.entityType(entity.TypeID())
		if !ok {

			return newBuildError(
				BuildErrorTypeNotFound,
				"entity type could not be resolved",
			)
		}

		object :=
			address.EntityRef(
				entity.ID(),
			)

		owner :=
			schema.EntityTypeOwner(
				entityType.ID(),
			)
		b.objectOwners[object] = owner

		if err := b.addObjectProperties(
			object,
			owner,
			entityType.PropertyDefinitions(),
		); err != nil {
			return err
		}
	}

	return nil
}

func (b *graphBuilder) addRelationProperties() error {
	relations := b.source.Relations()

	sort.Slice(
		relations,
		func(i, j int) bool {
			return compareUUID(
				relations[i].ID(),
				relations[j].ID(),
			) < 0
		},
	)

	for _, relation := range relations {
		if !relation.IsValid() {
			return newBuildError(
				BuildErrorInvalidSourceObject,
				"relation is structurally invalid",
			)
		}

		relationType, ok := b.relationType(relation.TypeID())
		if !ok {

			return newBuildError(
				BuildErrorTypeNotFound,
				"relation type could not be resolved",
			)
		}

		object :=
			address.RelationRef(
				relation.ID(),
			)

		owner :=
			schema.RelationTypeOwner(
				relationType.ID(),
			)
		b.objectOwners[object] = owner

		if err := b.addObjectProperties(
			object,
			owner,
			relationType.PropertyDefinitions(),
		); err != nil {
			return err
		}
	}

	return nil
}

func (b *graphBuilder) addObjectProperties(
	object address.ObjectRef,
	owner schema.PropertyOwner,
	definitionIDs []identity.UUID,
) error {
	definitionIDs = append(
		[]identity.UUID(nil),
		definitionIDs...,
	)

	sort.Slice(
		definitionIDs,
		func(i, j int) bool {
			return compareUUID(
				definitionIDs[i],
				definitionIDs[j],
			) < 0
		},
	)

	for _, definitionID := range definitionIDs {

		definition, ok :=
			b.source.PropertyDefinition(
				definitionID,
			)

		if !ok ||
			!definition.IsValid() ||
			definition.ID() !=
				definitionID {

			return newBuildError(
				BuildErrorDefinitionNotFound,
				"property definition could not be resolved",
			)
		}

		if definition.Owner() != owner {
			return newBuildError(
				BuildErrorDefinitionOwnerMismatch,
				"property definition owner does not match concrete object type",
			)
		}

		target :=
			address.NewPropertyAddress(
				object,
				definition.ID(),
			)

		b.graph.addVertex(
			target,
		)

		if definition.
			Computation().
			Kind() !=
			schema.ComputationDerived {

			continue
		}

		expression, ok :=
			definition.
				Computation().
				Expression()

		if !ok ||
			expression == nil {

			return newBuildError(
				BuildErrorInvalidExpression,
				"derived property definition has no expression",
			)
		}

		extractor :=
			expressionExtractor{
				source:        b.source,
				entityTypes:   b.entityTypes,
				relationTypes: b.relationTypes,
				objectOwners:  b.objectOwners,
			}

		dependencies, err :=
			extractor.extract(
				expression,
				extractionContext{
					self: object,
				},
			)

		if err != nil {
			return err
		}

		for _, dependencyAddress := range dependencies {

			b.graph.addDependency(
				dependencyAddress,
				target,
			)
		}
	}

	return nil
}

func (b *graphBuilder) entityType(id identity.UUID) (schema.EntityType, bool) {
	if result, exists := b.entityTypes[id]; exists {
		return result, true
	}
	result, exists := b.source.EntityType(id)
	if !exists || !result.IsValid() || result.ID() != id {
		return schema.EntityType{}, false
	}
	b.entityTypes[id] = result
	return result, true
}

func (b *graphBuilder) relationType(id identity.UUID) (schema.RelationType, bool) {
	if result, exists := b.relationTypes[id]; exists {
		return result, true
	}
	result, exists := b.source.RelationType(id)
	if !exists || !result.IsValid() || result.ID() != id {
		return schema.RelationType{}, false
	}
	b.relationTypes[id] = result
	return result, true
}
