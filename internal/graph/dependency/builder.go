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

		entityType, ok :=
			b.source.EntityType(
				entity.TypeID(),
			)

		if !ok ||
			!entityType.IsValid() ||
			entityType.ID() !=
				entity.TypeID() {

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

		relationType, ok :=
			b.source.RelationType(
				relation.TypeID(),
			)

		if !ok ||
			!relationType.IsValid() ||
			relationType.ID() !=
				relation.TypeID() {

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
				source: b.source,
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
