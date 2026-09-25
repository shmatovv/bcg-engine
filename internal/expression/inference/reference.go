package inference

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// ReferenceErrorKind identifies why static reference inference failed.
type ReferenceErrorKind uint8

const (
	ReferenceErrorInvalid ReferenceErrorKind = iota
	ReferenceErrorInvalidReference
	ReferenceErrorMissingSelf
	ReferenceErrorMissingBinding
	ReferenceErrorDefinitionNotFound
	ReferenceErrorObjectNotFound
	ReferenceErrorOwnerMismatch
	ReferenceErrorTargetNotConcrete
	ReferenceErrorContractNotFound
	ReferenceErrorContractMismatch
	ReferenceErrorContractImplementationNotFound
	ReferenceErrorContractTypeNotGuaranteed
	ReferenceErrorIncompatibleContractTypes
	ReferenceErrorInvalidDefinition
)

// IsValid reports whether k is a valid reference error kind.
func (k ReferenceErrorKind) IsValid() bool {
	switch k {
	case ReferenceErrorInvalidReference,
		ReferenceErrorMissingSelf,
		ReferenceErrorMissingBinding,
		ReferenceErrorDefinitionNotFound,
		ReferenceErrorObjectNotFound,
		ReferenceErrorOwnerMismatch,
		ReferenceErrorTargetNotConcrete,
		ReferenceErrorContractNotFound,
		ReferenceErrorContractMismatch,
		ReferenceErrorContractImplementationNotFound,
		ReferenceErrorContractTypeNotGuaranteed,
		ReferenceErrorIncompatibleContractTypes,
		ReferenceErrorInvalidDefinition:
		return true
	default:
		return false
	}
}

// String returns the canonical name of k.
func (k ReferenceErrorKind) String() string {
	switch k {
	case ReferenceErrorInvalidReference:
		return "InvalidReference"
	case ReferenceErrorMissingSelf:
		return "MissingSelf"
	case ReferenceErrorMissingBinding:
		return "MissingBinding"
	case ReferenceErrorDefinitionNotFound:
		return "DefinitionNotFound"
	case ReferenceErrorObjectNotFound:
		return "ObjectNotFound"
	case ReferenceErrorOwnerMismatch:
		return "OwnerMismatch"
	case ReferenceErrorTargetNotConcrete:
		return "TargetNotConcrete"
	case ReferenceErrorContractNotFound:
		return "ContractNotFound"
	case ReferenceErrorContractMismatch:
		return "ContractMismatch"
	case ReferenceErrorContractImplementationNotFound:
		return "ContractImplementationNotFound"
	case ReferenceErrorContractTypeNotGuaranteed:
		return "ContractTypeNotGuaranteed"
	case ReferenceErrorIncompatibleContractTypes:
		return "IncompatibleContractTypes"
	case ReferenceErrorInvalidDefinition:
		return "InvalidDefinition"
	default:
		return "Invalid"
	}
}

// ReferenceError describes a static reference inference failure.
type ReferenceError struct {
	kind ReferenceErrorKind
}

// Kind returns the reference error kind.
func (e ReferenceError) Kind() ReferenceErrorKind {
	return e.kind
}

// Error implements error.
func (e ReferenceError) Error() string {
	switch e.kind {
	case ReferenceErrorInvalidReference:
		return "inference: invalid property reference"
	case ReferenceErrorMissingSelf:
		return "inference: self reference used without self context"
	case ReferenceErrorMissingBinding:
		return "inference: bound reference used outside aggregate binding"
	case ReferenceErrorDefinitionNotFound:
		return "inference: property definition not found"
	case ReferenceErrorObjectNotFound:
		return "inference: referenced object type not found"
	case ReferenceErrorOwnerMismatch:
		return "inference: property definition owner does not match reference target"
	case ReferenceErrorTargetNotConcrete:
		return "inference: bound target does not have one concrete schema owner"
	case ReferenceErrorContractNotFound:
		return "inference: property contract not found"
	case ReferenceErrorContractMismatch:
		return "inference: property definition does not satisfy its property contract"
	case ReferenceErrorContractImplementationNotFound:
		return "inference: property contract implementation not found"
	case ReferenceErrorContractTypeNotGuaranteed:
		return "inference: property contract type is not guaranteed by bound entity constraint"
	case ReferenceErrorIncompatibleContractTypes:
		return "inference: bound contract has incompatible types across possible entity constraints"
	case ReferenceErrorInvalidDefinition:
		return "inference: resolved property definition is invalid"
	default:
		return "inference: invalid reference error"
	}
}

// TypeFromPropertyDefinition resolves the complete static expression type
// represented by definition.
//
// PropertyDefinition supplies:
//
//   - ValueType;
//   - Unit;
//   - TemporalMode.
//
// Nullability belongs to PropertyContract.
//
// Therefore:
//
//   - a definition without a contract is non-nullable;
//   - a definition implementing a contract inherits the contract's
//     nullability.
//
// For contracted definitions this function also defensively verifies the
// locally checkable contract invariants:
//
//   - ContractID;
//   - ValueType;
//   - Dimension;
//   - TemporalMode.
//
// Full model validation remains responsible for proving the complete schema
// and semantic consistency of a committed model.
func TypeFromPropertyDefinition(
	definition schema.PropertyDefinition,
	resolver Resolver,
) (typing.Type, error) {
	if !definition.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidDefinition,
		}
	}

	temporalMode, ok := typingTemporalMode(
		definition.TemporalMode(),
	)
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidDefinition,
		}
	}

	nullable := false

	contractID, hasContract := definition.ContractID()
	if hasContract {
		contract, ok := resolver.PropertyContract(
			contractID,
		)
		if !ok || !contract.IsValid() {
			return typing.Type{}, ReferenceError{
				kind: ReferenceErrorContractNotFound,
			}
		}

		if !definitionSatisfiesContract(
			definition,
			contract,
		) {
			return typing.Type{}, ReferenceError{
				kind: ReferenceErrorContractMismatch,
			}
		}

		nullable = contract.Nullable()
	}

	propertyUnit, hasUnit := definition.Unit()
	if definition.ValueType() == value.Enum {
		id, ok := definition.EnumDefinitionID()
		if !ok {
			return typing.Type{}, ReferenceError{kind: ReferenceErrorDefinitionNotFound}
		}
		return typing.NewQualifiedEnum(id, propertyUnit, hasUnit, temporalMode, nullable), nil
	}

	return typing.NewQualified(
		definition.ValueType(),
		propertyUnit,
		hasUnit,
		temporalMode,
		nullable,
	), nil
}

// InferSelfPropertyReference determines the static type of a self property
// reference.
//
// The referenced PropertyDefinition must:
//
//   - exist;
//   - be valid;
//   - belong to the current self owner.
func InferSelfPropertyReference(
	reference ast.SelfPropertyReference,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !reference.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidReference,
		}
	}

	self, ok := context.Self()
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorMissingSelf,
		}
	}

	definition, ok := resolver.PropertyDefinition(
		reference.DefinitionID(),
	)
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorDefinitionNotFound,
		}
	}

	if !definition.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidDefinition,
		}
	}

	if !propertyOwnersEqual(
		definition.Owner(),
		self,
	) {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorOwnerMismatch,
		}
	}

	return TypeFromPropertyDefinition(
		definition,
		resolver,
	)
}

// InferAbsolutePropertyReference determines the static type of an absolute
// property reference.
//
// An absolute reference fixes both:
//
//   - one concrete Entity or Relation;
//   - one PropertyDefinition.
//
// Static inference resolves the concrete object's schema owner and verifies
// that the referenced definition belongs to that owner.
func InferAbsolutePropertyReference(
	reference ast.AbsolutePropertyReference,
	resolver Resolver,
) (typing.Type, error) {
	if !reference.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidReference,
		}
	}

	propertyAddress := reference.Address()

	owner, ok := resolver.ObjectType(
		propertyAddress.Object(),
	)
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorObjectNotFound,
		}
	}

	if !owner.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorObjectNotFound,
		}
	}

	definition, ok := resolver.PropertyDefinition(
		propertyAddress.DefinitionID(),
	)
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorDefinitionNotFound,
		}
	}

	if !definition.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidDefinition,
		}
	}

	if !propertyOwnersEqual(
		definition.Owner(),
		owner,
	) {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorOwnerMismatch,
		}
	}

	return TypeFromPropertyDefinition(
		definition,
		resolver,
	)
}

// InferBoundPropertyDefinitionReference determines the static type of a
// concrete PropertyDefinition reference on the current aggregate binding.
//
// A concrete definition reference is valid only when the selected target has
// one statically known concrete schema owner.
//
// Therefore:
//
//   - bound.Relation is always eligible;
//   - bound.Entity is eligible only when every possible entity binding
//     identifies the same concrete EntityType.
//
// Trait, AnyEntity, or heterogeneous entity bindings must use semantic
// PropertyContract references instead.
func InferBoundPropertyDefinitionReference(
	reference ast.BoundPropertyDefinitionReference,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !reference.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidReference,
		}
	}

	binding, ok := context.Binding()
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorMissingBinding,
		}
	}

	owner, ok := binding.TargetOwner(
		reference.Target(),
	)
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorTargetNotConcrete,
		}
	}

	definition, ok := resolver.PropertyDefinition(
		reference.DefinitionID(),
	)
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorDefinitionNotFound,
		}
	}

	if !definition.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidDefinition,
		}
	}

	if !propertyOwnersEqual(
		definition.Owner(),
		owner,
	) {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorOwnerMismatch,
		}
	}

	return TypeFromPropertyDefinition(
		definition,
		resolver,
	)
}

// InferBoundContractPropertyReference determines the static type of a
// PropertyContract reference on the current aggregate binding.
//
// Relation target:
//
// The relation binding always has one concrete RelationType, so its concrete
// PropertyDefinition implementation can be resolved directly.
//
// Entity target:
//
// The entity binding may represent multiple endpoint constraints. The
// contract must therefore have one statically compatible type for every
// possible constraint.
//
// This is what makes expressions such as:
//
//	boundEntity.PRICE
//
// safe for Trait-based and bidirectional heterogeneous traversals.
func InferBoundContractPropertyReference(
	reference ast.BoundContractPropertyReference,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !reference.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidReference,
		}
	}

	binding, ok := context.Binding()
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorMissingBinding,
		}
	}

	switch reference.Target() {
	case ast.BoundTargetRelation:
		return inferBoundRelationContract(
			reference,
			binding,
			resolver,
		)

	case ast.BoundTargetEntity:
		return inferBoundEntityContract(
			reference,
			binding,
			resolver,
		)

	default:
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidReference,
		}
	}
}

func inferBoundRelationContract(
	reference ast.BoundContractPropertyReference,
	binding Binding,
	resolver Resolver,
) (typing.Type, error) {
	owner := binding.RelationOwner()

	if !owner.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorTargetNotConcrete,
		}
	}

	definition, ok := resolver.ContractImplementation(
		owner,
		reference.ContractID(),
	)
	if !ok {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorContractImplementationNotFound,
		}
	}

	if !definition.IsValid() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorInvalidDefinition,
		}
	}

	if !propertyOwnersEqual(
		definition.Owner(),
		owner,
	) {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorOwnerMismatch,
		}
	}

	contractID, hasContract := definition.ContractID()
	if !hasContract ||
		contractID != reference.ContractID() {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorContractImplementationNotFound,
		}
	}

	return TypeFromPropertyDefinition(
		definition,
		resolver,
	)
}

func inferBoundEntityContract(
	reference ast.BoundContractPropertyReference,
	binding Binding,
	resolver Resolver,
) (typing.Type, error) {
	constraints := binding.Entity().Constraints()
	if len(constraints) == 0 {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorContractTypeNotGuaranteed,
		}
	}

	var result typing.Type
	hasResult := false

	for _, constraint := range constraints {
		current, ok := resolver.ContractType(
			constraint,
			reference.ContractID(),
		)
		if !ok || !current.IsValid() {
			return typing.Type{}, ReferenceError{
				kind: ReferenceErrorContractTypeNotGuaranteed,
			}
		}

		if !hasResult {
			result = current
			hasResult = true
			continue
		}

		unified, err := typecheck.Unify(
			result,
			current,
		)
		if err != nil {
			return typing.Type{}, ReferenceError{
				kind: ReferenceErrorIncompatibleContractTypes,
			}
		}

		result = unified
	}

	if !hasResult {
		return typing.Type{}, ReferenceError{
			kind: ReferenceErrorContractTypeNotGuaranteed,
		}
	}

	return result, nil
}

// typingTemporalMode translates the schema-layer TemporalMode into the
// expression typing-layer TemporalMode.
//
// Keeping the two types separate prevents expression typing from depending on
// the schema package and therefore avoids introducing an import cycle.
func typingTemporalMode(
	mode schema.TemporalMode,
) (typing.TemporalMode, bool) {
	switch mode {
	case schema.TemporalScalar:
		return typing.TemporalScalar, true

	case schema.TemporalSeries:
		return typing.TemporalSeries, true

	default:
		return typing.TemporalInvalid, false
	}
}

// definitionSatisfiesContract verifies the contract invariants that can be
// checked from one PropertyDefinition and its PropertyContract.
//
// A contract constrains:
//
//   - ValueType;
//   - Dimension presence and value;
//   - TemporalMode.
//
// Unit identity itself is intentionally not part of the contract. Different
// concrete implementations may use different units belonging to the required
// Dimension.
//
// Nullability is also not checked against PropertyDefinition because
// PropertyDefinition does not independently declare nullability. It inherits
// that semantic from the contract.
func definitionSatisfiesContract(
	definition schema.PropertyDefinition,
	contract schema.PropertyContract,
) bool {
	if !definition.IsValid() || !contract.IsValid() {
		return false
	}

	contractID, hasContract := definition.ContractID()
	if !hasContract || contractID != contract.ID() {
		return false
	}

	if definition.ValueType() != contract.ValueType() {
		return false
	}
	if definition.ValueType() == value.Enum {
		left, leftOK := definition.EnumDefinitionID()
		right, rightOK := contract.EnumDefinitionID()
		if !leftOK || !rightOK || left != right {
			return false
		}
	}

	if definition.TemporalMode() != contract.TemporalMode() {
		return false
	}

	contractDimension, hasDimension := contract.Dimension()
	if !hasDimension {
		// Absence of Dimension means that the contract does not constrain
		// dimensional semantics. It does not require the implementation to
		// be unitless.
		return true
	}

	propertyUnit, hasUnit := definition.Unit()
	if !hasUnit {
		return false
	}

	return propertyUnit.Dimension().Equal(contractDimension)
}

func propertyOwnersEqual(
	left schema.PropertyOwner,
	right schema.PropertyOwner,
) bool {
	if !left.IsValid() || !right.IsValid() {
		return false
	}

	return left.Kind() == right.Kind() &&
		left.ID() == right.ID()
}

var _ error = ReferenceError{}
