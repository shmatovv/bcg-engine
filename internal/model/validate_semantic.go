package model

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
)

// validateSemanticData validates the structural and referential integrity
// of semantic model instances.
func (d *Draft) validateSemanticData(
	report *ValidationReport,
	indexes validationIndexes,
) {
	d.validateSemanticStructure(report)
	d.validateSemanticDuplicateIDs(report)
	d.validateEntityReferences(
		report,
		indexes.entityTypeByID,
	)
	d.validateRelationReferences(
		report,
		indexes.relationTypeByID,
		indexes.entityByID,
	)
}

func (d *Draft) validateSemanticStructure(
	report *ValidationReport,
) {
	for _, entity := range d.entities {
		if entity.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidEntity,
				entity.ID(),
				"entity is structurally invalid",
			),
		)
	}

	for _, relation := range d.relations {
		if relation.IsValid() {
			continue
		}

		report.append(
			newSubjectValidationIssue(
				ValidationIssueInvalidRelation,
				relation.ID(),
				"relation is structurally invalid",
			),
		)
	}
}

func (d *Draft) validateSemanticDuplicateIDs(
	report *ValidationReport,
) {
	entityIDs := make(
		map[identity.UUID]struct{},
		len(d.entities),
	)

	for _, entity := range d.entities {
		validateOneID(
			report,
			entityIDs,
			entity.ID(),
			ValidationIssueDuplicateEntityID,
			"entity",
		)
	}

	relationIDs := make(
		map[identity.UUID]struct{},
		len(d.relations),
	)

	for _, relation := range d.relations {
		validateOneID(
			report,
			relationIDs,
			relation.ID(),
			ValidationIssueDuplicateRelationID,
			"relation",
		)
	}
}

func (d *Draft) validateEntityReferences(
	report *ValidationReport,
	entityTypes map[identity.UUID]schema.EntityType,
) {
	for _, entity := range d.entities {
		if !entity.IsValid() {
			continue
		}

		if _, exists := entityTypes[entity.TypeID()]; exists {
			continue
		}

		report.append(
			newRelatedValidationIssue(
				ValidationIssueUnknownEntityType,
				entity.ID(),
				entity.TypeID(),
				"entity references unknown entity type",
			),
		)
	}
}

func (d *Draft) validateRelationReferences(
	report *ValidationReport,
	relationTypes map[identity.UUID]schema.RelationType,
	entities map[identity.UUID]semantic.Entity,
) {
	for _, relation := range d.relations {
		if !relation.IsValid() {
			continue
		}

		if _, exists := relationTypes[relation.TypeID()]; !exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueUnknownRelationType,
					relation.ID(),
					relation.TypeID(),
					"relation references unknown relation type",
				),
			)
		}

		if _, exists := entities[relation.From()]; !exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueUnknownEntity,
					relation.ID(),
					relation.From(),
					"relation references unknown from entity",
				),
			)
		}

		if _, exists := entities[relation.To()]; !exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueUnknownEntity,
					relation.ID(),
					relation.To(),
					"relation references unknown to entity",
				),
			)
		}
	}
}
