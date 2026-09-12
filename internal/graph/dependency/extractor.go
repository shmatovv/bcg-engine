package dependency

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
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

	depth, valid := ast.Depth(expression)
	if !valid {
		return nil, newBuildError(
			BuildErrorInvalidExpression,
			"expression is structurally invalid",
		)
	}
	if depth > ast.MaxExpressionDepth {
		return nil, newBuildError(
			BuildErrorInvalidExpression,
			"expression nesting exceeds the maximum supported depth",
		)
	}

	dependencies := make(
		map[address.PropertyAddress]struct{},
	)

	if err := e.walk(
		expression,
		context,
		dependencies,
	); err != nil {
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

func (e expressionExtractor) walk(
	expression ast.Expression,
	context extractionContext,
	dependencies map[address.PropertyAddress]struct{},
) error {
	if expression == nil {
		return newBuildError(
			BuildErrorInvalidExpression,
			"expression must not be nil",
		)
	}

	switch expression :=
		expression.(type) {

	case ast.Constant:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		return nil

	case ast.SelfPropertyReference:
		if !expression.IsValid() ||
			!context.self.IsValid() {

			return invalidReferenceError()
		}

		propertyAddress :=
			address.NewPropertyAddress(
				context.self,
				expression.DefinitionID(),
			)

		if err :=
			e.validatePropertyAddress(
				propertyAddress,
			); err != nil {

			return err
		}

		dependencies[propertyAddress] =
			struct{}{}

		return nil

	case ast.AbsolutePropertyReference:
		if !expression.IsValid() {
			return invalidReferenceError()
		}

		propertyAddress :=
			expression.Address()

		if err :=
			e.validatePropertyAddress(
				propertyAddress,
			); err != nil {

			return err
		}

		dependencies[propertyAddress] =
			struct{}{}

		return nil

	case ast.BoundPropertyDefinitionReference:
		if !expression.IsValid() {
			return invalidReferenceError()
		}

		boundObject, err :=
			boundObjectForTarget(
				context,
				expression.Target(),
			)

		if err != nil {
			return err
		}

		propertyAddress :=
			address.NewPropertyAddress(
				boundObject,
				expression.DefinitionID(),
			)

		if err :=
			e.validatePropertyAddress(
				propertyAddress,
			); err != nil {

			return err
		}

		dependencies[propertyAddress] =
			struct{}{}

		return nil

	case ast.BoundContractPropertyReference:
		if !expression.IsValid() {
			return invalidReferenceError()
		}

		boundObject, err :=
			boundObjectForTarget(
				context,
				expression.Target(),
			)

		if err != nil {
			return err
		}

		owner, err :=
			e.objectOwner(
				boundObject,
			)

		if err != nil {
			return err
		}

		definition, ok :=
			e.source.ContractImplementation(
				owner,
				expression.ContractID(),
			)

		if !ok ||
			!definition.IsValid() ||
			definition.Owner() != owner {

			return newBuildError(
				BuildErrorContractImplementationNotFound,
				"bound contract implementation could not be resolved for concrete object type",
			)
		}

		contractID, hasContract :=
			definition.ContractID()

		if !hasContract ||
			contractID !=
				expression.ContractID() {

			return newBuildError(
				BuildErrorContractImplementationNotFound,
				"resolved property definition does not implement the referenced contract",
			)
		}

		propertyAddress :=
			address.NewPropertyAddress(
				boundObject,
				definition.ID(),
			)

		if err :=
			e.validatePropertyAddress(
				propertyAddress,
			); err != nil {

			return err
		}

		dependencies[propertyAddress] =
			struct{}{}

		return nil

	case ast.Arithmetic:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		if err := e.walk(
			expression.Left(),
			context,
			dependencies,
		); err != nil {
			return err
		}

		return e.walk(
			expression.Right(),
			context,
			dependencies,
		)

	case ast.Negate:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		return e.walk(
			expression.Operand(),
			context,
			dependencies,
		)

	case ast.Comparison:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		if err := e.walk(
			expression.Left(),
			context,
			dependencies,
		); err != nil {
			return err
		}

		return e.walk(
			expression.Right(),
			context,
			dependencies,
		)

	case ast.Logical:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		if err := e.walk(
			expression.Left(),
			context,
			dependencies,
		); err != nil {
			return err
		}

		return e.walk(
			expression.Right(),
			context,
			dependencies,
		)

	case ast.Not:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		return e.walk(
			expression.Operand(),
			context,
			dependencies,
		)

	case ast.If:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		if err := e.walk(
			expression.Condition(),
			context,
			dependencies,
		); err != nil {
			return err
		}

		if err := e.walk(
			expression.Then(),
			context,
			dependencies,
		); err != nil {
			return err
		}

		return e.walk(
			expression.Else(),
			context,
			dependencies,
		)

	case ast.IsNull:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		return e.walk(
			expression.Operand(),
			context,
			dependencies,
		)

	case ast.Coalesce:
		if !expression.IsValid() {
			return invalidExpressionError()
		}

		if err := e.walk(
			expression.Primary(),
			context,
			dependencies,
		); err != nil {
			return err
		}

		return e.walk(
			expression.Fallback(),
			context,
			dependencies,
		)

	case ast.Aggregate:
		return e.walkAggregate(
			expression,
			context,
			dependencies,
		)

	default:
		return newBuildError(
			BuildErrorInvalidExpression,
			"unsupported expression node",
		)
	}
}

func (e expressionExtractor) walkAggregate(
	expression ast.Aggregate,
	context extractionContext,
	dependencies map[address.PropertyAddress]struct{},
) error {
	if !expression.IsValid() {
		return invalidExpressionError()
	}

	origin, err := traversalOrigin(context, expression.Source())
	if err != nil {
		return err
	}

	_, ok := e.relationType(expression.Source().RelationTypeID())
	if !ok {

		return newBuildError(
			BuildErrorInvalidTraversal,
			"aggregate traversal relation type could not be resolved",
		)
	}

	items, err :=
		e.traversalItems(
			origin.ID(),
			expression.Source(),
		)

	if err != nil {
		return err
	}

	for _, item := range items {
		relation, ok :=
			e.source.Relation(
				item.relation.ID(),
			)

		if !ok ||
			!relation.IsValid() {

			return newBuildError(
				BuildErrorInvalidTraversal,
				"traversed relation could not be resolved",
			)
		}

		if applicability, hasApplicability :=
			relation.Applicability(); hasApplicability {

			if err := e.walk(
				applicability,
				extractionContext{
					self: address.RelationRef(
						relation.ID(),
					),
				},
				dependencies,
			); err != nil {
				return err
			}
		}

		childContext :=
			context.withBinding(
				item,
			)

		if filter, hasFilter :=
			expression.Filter(); hasFilter {

			if err := e.walk(
				filter,
				childContext,
				dependencies,
			); err != nil {
				return err
			}
		}

		if mapExpression, hasMap :=
			expression.Map(); hasMap {

			if err := e.walk(
				mapExpression,
				childContext,
				dependencies,
			); err != nil {
				return err
			}
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
	relations := e.source.Relations()

	sort.Slice(
		relations,
		func(i, j int) bool {
			return compareUUID(
				relations[i].ID(),
				relations[j].ID(),
			) < 0
		},
	)

	result := make(
		[]concreteBinding,
		0,
	)

	for _, relation := range relations {

		if !relation.IsValid() ||
			relation.TypeID() !=
				source.RelationTypeID() {

			continue
		}

		switch source.Direction() {
		case ast.TraversalDirectionOutgoing:
			if relation.From() !=
				selfEntityID {

				continue
			}

			result = append(
				result,
				newConcreteBinding(
					relation,
					relation.To(),
				),
			)

		case ast.TraversalDirectionIncoming:
			if relation.To() !=
				selfEntityID {

				continue
			}

			result = append(
				result,
				newConcreteBinding(
					relation,
					relation.From(),
				),
			)

		case ast.TraversalDirectionBoth:
			if relation.From() ==
				selfEntityID {

				result = append(
					result,
					newConcreteBinding(
						relation,
						relation.To(),
					),
				)
			}

			if relation.To() ==
				selfEntityID &&
				relation.From() !=
					selfEntityID {

				result = append(
					result,
					newConcreteBinding(
						relation,
						relation.From(),
					),
				)
			}

		default:
			return nil, newBuildError(
				BuildErrorInvalidTraversal,
				"aggregate traversal direction is invalid",
			)
		}
	}

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
