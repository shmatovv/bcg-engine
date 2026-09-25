package dependency

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
)

type concreteBinding struct {
	relation address.ObjectRef
	entity   address.ObjectRef
}

type extractionContext struct {
	self address.ObjectRef

	binding    concreteBinding
	hasBinding bool
}

func (c extractionContext) withBinding(
	binding concreteBinding,
) extractionContext {
	c.binding = binding
	c.hasBinding = true

	return c
}

type expressionExtractor struct {
	source Source

	// The Builder shares these caches across all expressions in one graph
	// construction. Standalone extraction lazily creates its own bounded cache.
	entityTypes   map[identity.UUID]schema.EntityType
	relationTypes map[identity.UUID]schema.RelationType
	objectOwners  map[address.ObjectRef]schema.PropertyOwner
}

func (e expressionExtractor) extract(
	expression ast.Expression,
	context extractionContext,
) ([]address.PropertyAddress, error) {
	if e.entityTypes == nil {
		e.entityTypes = make(map[identity.UUID]schema.EntityType)
	}
	if e.relationTypes == nil {
		e.relationTypes = make(map[identity.UUID]schema.RelationType)
	}
	if e.objectOwners == nil {
		e.objectOwners = make(map[address.ObjectRef]schema.PropertyOwner)
	}

	requirements, err := AnalyzeStatic(expression)
	if err != nil {
		return nil, newBuildError(BuildErrorInvalidExpression, err.Error())
	}

	dependencies := make(
		map[address.PropertyAddress]struct{},
	)

	if err := e.resolveRequirements(requirements, 0, context, dependencies); err != nil {
		return nil, err
	}

	result := make(
		[]address.PropertyAddress,
		0,
		len(dependencies),
	)

	for dependencyAddress := range dependencies {

		result = append(
			result,
			dependencyAddress,
		)
	}

	sortPropertyAddresses(
		result,
	)

	return result, nil
}

// resolveRequirements performs only source-specific address resolution. The
// AST shape and dependency semantics come solely from AnalyzeStatic.
func (e expressionExtractor) resolveRequirements(requirements []StaticRequirement, parent uint32, context extractionContext, dependencies map[address.PropertyAddress]struct{}) error {
	for _, requirement := range requirements {
		if requirement.ParentTraversalID() != parent {
			continue
		}
		switch requirement.Kind() {
		case StaticSelfProperty:
			if !context.self.IsValid() {
				return invalidReferenceError()
			}
			if err := e.addProperty(address.NewPropertyAddress(context.self, requirement.DefinitionID()), dependencies); err != nil {
				return err
			}
		case StaticAbsoluteProperty:
			property, _ := requirement.Address()
			if err := e.addProperty(property, dependencies); err != nil {
				return err
			}
		case StaticBoundProperty:
			bound, err := boundObjectForTarget(context, requirement.BoundTarget())
			if err != nil {
				return err
			}
			if err := e.addProperty(address.NewPropertyAddress(bound, requirement.DefinitionID()), dependencies); err != nil {
				return err
			}
		case StaticBoundContract:
			if err := e.addContractProperty(requirement, context, dependencies); err != nil {
				return err
			}
		case StaticTraversal:
			if err := e.resolveTraversal(requirements, requirement, context, dependencies); err != nil {
				return err
			}
		case StaticFunction:
			// Function implementations have no hidden data dependencies.
		default:
			return invalidExpressionError()
		}
	}
	return nil
}

func (e expressionExtractor) addProperty(property address.PropertyAddress, dependencies map[address.PropertyAddress]struct{}) error {
	if err := e.validatePropertyAddress(property); err != nil {
		return err
	}
	dependencies[property] = struct{}{}
	return nil
}

func (e expressionExtractor) addContractProperty(requirement StaticRequirement, context extractionContext, dependencies map[address.PropertyAddress]struct{}) error {
	bound, err := boundObjectForTarget(context, requirement.BoundTarget())
	if err != nil {
		return err
	}
	owner, err := e.objectOwner(bound)
	if err != nil {
		return err
	}
	definition, ok := e.source.ContractImplementation(owner, requirement.ContractID())
	if !ok || !definition.IsValid() || definition.Owner() != owner {
		return newBuildError(BuildErrorContractImplementationNotFound, "bound contract implementation could not be resolved for concrete object type")
	}
	contract, hasContract := definition.ContractID()
	if !hasContract || contract != requirement.ContractID() {
		return newBuildError(BuildErrorContractImplementationNotFound, "resolved property definition does not implement the referenced contract")
	}
	return e.addProperty(address.NewPropertyAddress(bound, definition.ID()), dependencies)
}

func (e expressionExtractor) resolveTraversal(requirements []StaticRequirement, requirement StaticRequirement, context extractionContext, dependencies map[address.PropertyAddress]struct{}) error {
	source, _ := requirement.Traversal()
	origin, err := traversalOrigin(context, source)
	if err != nil {
		return err
	}
	if _, ok := e.relationType(source.RelationTypeID()); !ok {
		return newBuildError(BuildErrorInvalidTraversal, "aggregate traversal relation type could not be resolved")
	}
	items, err := e.traversalItems(origin.ID(), source)
	if err != nil {
		return err
	}
	for _, item := range items {
		relation, ok := e.source.Relation(item.relation.ID())
		if !ok || !relation.IsValid() {
			return newBuildError(BuildErrorInvalidTraversal, "traversed relation could not be resolved")
		}
		if applicability, ok := relation.Applicability(); ok {
			applicabilityRequirements, err := AnalyzeStatic(applicability)
			if err != nil {
				return newBuildError(BuildErrorInvalidExpression, err.Error())
			}
			if err := e.resolveRequirements(applicabilityRequirements, 0, extractionContext{self: address.RelationRef(relation.ID())}, dependencies); err != nil {
				return err
			}
		}
		if err := e.resolveRequirements(requirements, requirement.TraversalID(), context.withBinding(item), dependencies); err != nil {
			return err
		}
	}
	return nil
}

func traversalOrigin(
	context extractionContext,
	source ast.TraversalSource,
) (address.ObjectRef, error) {
	if !source.IsValid() {
		return address.ObjectRef{}, newBuildError(
			BuildErrorInvalidTraversal,
			"aggregate traversal source is invalid",
		)
	}

	switch source.Origin() {
	case ast.TraversalOriginSelf:
		if !context.self.IsValid() ||
			context.self.Kind() != address.ObjectRefEntity {
			return address.ObjectRef{}, newBuildError(
				BuildErrorInvalidTraversal,
				"aggregate traversal requires an Entity self object",
			)
		}
		return context.self, nil

	case ast.TraversalOriginAbsoluteEntity:
		origin, ok := source.AbsoluteOrigin()
		if !ok || !origin.IsValid() ||
			origin.Kind() != address.ObjectRefEntity {
			return address.ObjectRef{}, newBuildError(
				BuildErrorInvalidTraversal,
				"aggregate traversal explicit origin must be an Entity object",
			)
		}
		return origin, nil

	case ast.TraversalOriginBoundEntity:
		if !context.hasBinding ||
			!context.binding.entity.IsValid() ||
			context.binding.entity.Kind() != address.ObjectRefEntity {
			return address.ObjectRef{}, newBuildError(
				BuildErrorMissingBinding,
				"aggregate traversal bound origin is used outside aggregate binding",
			)
		}
		return context.binding.entity, nil

	default:
		return address.ObjectRef{}, newBuildError(
			BuildErrorInvalidTraversal,
			"aggregate traversal origin is invalid",
		)
	}
}

func (e expressionExtractor) traversalItems(
	selfEntityID identity.UUID,
	source ast.TraversalSource,
) ([]concreteBinding, error) {
	result := make(
		[]concreteBinding,
		0,
	)

	appendRelations := func(relations []semantic.Relation, reverse, skipSelfLoop bool) {
		for _, relation := range relations {
			if !relation.IsValid() {
				continue
			}
			if skipSelfLoop && relation.From() == selfEntityID {
				continue
			} // one self-loop in Both
			other := relation.To()
			if reverse {
				other = relation.From()
			}
			result = append(result, newConcreteBinding(relation, other))
		}
	}

	switch source.Direction() {
	case ast.TraversalDirectionOutgoing:
		appendRelations(e.source.OutgoingRelations(selfEntityID, source.RelationTypeID()), false, false)
	case ast.TraversalDirectionIncoming:
		appendRelations(e.source.IncomingRelations(selfEntityID, source.RelationTypeID()), true, false)
	case ast.TraversalDirectionBoth:
		appendRelations(e.source.OutgoingRelations(selfEntityID, source.RelationTypeID()), false, false)
		appendRelations(e.source.IncomingRelations(selfEntityID, source.RelationTypeID()), true, true)
	default:
		return nil, newBuildError(BuildErrorInvalidTraversal, "aggregate traversal direction is invalid")
	}

	sort.Slice(result, func(i, j int) bool { return compareUUID(result[i].relation.ID(), result[j].relation.ID()) < 0 })

	return result, nil
}

func newConcreteBinding(
	relation semantic.Relation,
	entityID identity.UUID,
) concreteBinding {
	return concreteBinding{
		relation: address.RelationRef(
			relation.ID(),
		),

		entity: address.EntityRef(
			entityID,
		),
	}
}

func boundObjectForTarget(
	context extractionContext,
	target ast.BoundTarget,
) (address.ObjectRef, error) {
	if !context.hasBinding {
		return address.ObjectRef{},
			newBuildError(
				BuildErrorMissingBinding,
				"bound property reference is used outside aggregate binding",
			)
	}

	switch target {
	case ast.BoundTargetEntity:
		return context.binding.entity,
			nil

	case ast.BoundTargetRelation:
		return context.binding.relation,
			nil

	default:
		return address.ObjectRef{},
			invalidReferenceError()
	}
}

func (e expressionExtractor) validatePropertyAddress(
	propertyAddress address.PropertyAddress,
) error {
	if !propertyAddress.IsValid() {
		return invalidReferenceError()
	}

	owner, err :=
		e.objectOwner(
			propertyAddress.Object(),
		)

	if err != nil {
		return err
	}

	definition, ok :=
		e.source.PropertyDefinition(
			propertyAddress.DefinitionID(),
		)

	if !ok ||
		!definition.IsValid() ||
		definition.ID() !=
			propertyAddress.DefinitionID() {

		return newBuildError(
			BuildErrorDefinitionNotFound,
			"referenced property definition could not be resolved",
		)
	}

	if definition.Owner() != owner {
		return newBuildError(
			BuildErrorDefinitionOwnerMismatch,
			"referenced property definition is not available on the concrete object type",
		)
	}

	return nil
}

func (e expressionExtractor) objectOwner(
	object address.ObjectRef,
) (schema.PropertyOwner, error) {
	if !object.IsValid() {
		return schema.PropertyOwner{},
			invalidReferenceError()
	}
	if owner, exists := e.objectOwners[object]; exists {
		return owner, nil
	}

	switch object.Kind() {
	case address.ObjectRefEntity:
		entity, ok :=
			e.source.Entity(
				object.ID(),
			)

		if !ok ||
			!entity.IsValid() ||
			entity.ID() !=
				object.ID() {

			return schema.PropertyOwner{},
				newBuildError(
					BuildErrorInvalidReference,
					"referenced entity could not be resolved",
				)
		}

		entityType, ok := e.entityType(entity.TypeID())
		if !ok {

			return schema.PropertyOwner{},
				newBuildError(
					BuildErrorTypeNotFound,
					"referenced entity type could not be resolved",
				)
		}

		owner := schema.EntityTypeOwner(entityType.ID())
		e.objectOwners[object] = owner
		return owner, nil

	case address.ObjectRefRelation:
		relation, ok :=
			e.source.Relation(
				object.ID(),
			)

		if !ok ||
			!relation.IsValid() ||
			relation.ID() !=
				object.ID() {

			return schema.PropertyOwner{},
				newBuildError(
					BuildErrorInvalidReference,
					"referenced relation could not be resolved",
				)
		}

		relationType, ok := e.relationType(relation.TypeID())
		if !ok {

			return schema.PropertyOwner{},
				newBuildError(
					BuildErrorTypeNotFound,
					"referenced relation type could not be resolved",
				)
		}

		owner := schema.RelationTypeOwner(relationType.ID())
		e.objectOwners[object] = owner
		return owner, nil

	default:
		return schema.PropertyOwner{},
			invalidReferenceError()
	}
}

func (e expressionExtractor) entityType(id identity.UUID) (schema.EntityType, bool) {
	if result, exists := e.entityTypes[id]; exists {
		return result, true
	}
	result, exists := e.source.EntityType(id)
	if !exists || !result.IsValid() || result.ID() != id {
		return schema.EntityType{}, false
	}
	e.entityTypes[id] = result
	return result, true
}

func (e expressionExtractor) relationType(id identity.UUID) (schema.RelationType, bool) {
	if result, exists := e.relationTypes[id]; exists {
		return result, true
	}
	result, exists := e.source.RelationType(id)
	if !exists || !result.IsValid() || result.ID() != id {
		return schema.RelationType{}, false
	}
	e.relationTypes[id] = result
	return result, true
}

func invalidExpressionError() error {
	return newBuildError(
		BuildErrorInvalidExpression,
		"expression node is structurally invalid",
	)
}

func invalidReferenceError() error {
	return newBuildError(
		BuildErrorInvalidReference,
		"property reference is structurally invalid",
	)
}
