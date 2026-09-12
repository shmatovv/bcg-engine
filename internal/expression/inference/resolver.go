package inference

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
)

// Resolver provides the schema and semantic information required for
// static expression inference.
//
// It supports both committed ModelVersions and draft-validation adapters.
type Resolver interface {
	// PropertyDefinition resolves one concrete PropertyDefinition.
	PropertyDefinition(
		id identity.UUID,
	) (schema.PropertyDefinition, bool)

	// PropertyContract resolves one PropertyContract.
	//
	// Contracts provide semantic guarantees shared by concrete property
	// implementations. In particular, nullability belongs to the contract
	// rather than to PropertyDefinition.
	PropertyContract(
		id identity.UUID,
	) (schema.PropertyContract, bool)

	// EntityType resolves one EntityType.
	EntityType(
		id identity.UUID,
	) (schema.EntityType, bool)

	// RelationType resolves one RelationType.
	RelationType(
		id identity.UUID,
	) (schema.RelationType, bool)

	// ObjectType resolves the schema owner of one concrete model object.
	//
	// For an Entity reference it returns:
	//
	//	EntityTypeOwner(entity.TypeID())
	//
	// For a Relation reference it returns:
	//
	//	RelationTypeOwner(relation.TypeID())
	//
	// This capability is required by AbsolutePropertyReference.
	ObjectType(
		object address.ObjectRef,
	) (schema.PropertyOwner, bool)

	// ContractImplementation resolves the concrete PropertyDefinition
	// implementing contractID for one concrete owner.
	//
	// This capability is used when the bound target is statically known to
	// have exactly one concrete EntityType or RelationType.
	ContractImplementation(
		owner schema.PropertyOwner,
		contractID identity.UUID,
	) (schema.PropertyDefinition, bool)

	// ContractType resolves the single statically guaranteed expression type
	// of contractID for every Entity allowed by constraint.
	//
	// The returned type must include the complete static semantics:
	//
	//   - ValueType;
	//   - Unit;
	//   - TemporalMode;
	//   - Nullability.
	//
	// The result is valid only when the resolver can prove that:
	//
	//   - the contract is guaranteed by the constraint;
	//   - every possible implementation has a compatible ValueType;
	//   - every possible implementation has compatible Unit semantics;
	//   - every possible implementation has compatible TemporalMode;
	//   - one common typing.Type can safely represent every runtime value.
	//
	// Examples:
	//
	//	EntityType(Board), PRICE
	//	    -> Scalar<Decimal<RUB/item>>
	//
	//	Trait(Priced), PRICE where every implementation belongs to the same
	//	automatic conversion space
	//	    -> one deterministic common type
	//
	//	Trait(Priced), PRICE where some implementations are RUB/item and
	//	others are EUR/item
	//	    -> false
	//
	//	AnyEntity, PRICE without a model-wide guarantee
	//	    -> false
	//
	// This method intentionally asks a semantic question instead of exposing
	// the resolver's internal indexes or requiring inference to enumerate
	// model types.
	//
	// IMPORTANT:
	//
	// For constraints covering multiple concrete EntityTypes, the selected
	// Unit representation must be deterministic and must not depend on map
	// iteration or insertion order.
	ContractType(
		constraint schema.TypeConstraint,
		contractID identity.UUID,
	) (typing.Type, bool)
}
