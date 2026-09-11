package schema

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

type PropertyOwnerKind uint8

const (
	PropertyOwnerInvalid PropertyOwnerKind = iota
	PropertyOwnerEntityType
	PropertyOwnerRelationType
)

func (k PropertyOwnerKind) IsValid() bool {
	return k == PropertyOwnerEntityType ||
		k == PropertyOwnerRelationType
}

func (k PropertyOwnerKind) String() string {
	switch k {
	case PropertyOwnerEntityType:
		return "EntityType"
	case PropertyOwnerRelationType:
		return "RelationType"
	default:
		return "Invalid"
	}
}

// PropertyOwner identifies the schema type that owns a PropertyDefinition.
type PropertyOwner struct {
	kind PropertyOwnerKind
	id   identity.UUID
}

func EntityTypeOwner(id identity.UUID) PropertyOwner {
	if id.IsZero() {
		panic("schema: entity type owner id must not be zero")
	}

	return PropertyOwner{
		kind: PropertyOwnerEntityType,
		id:   id,
	}
}

func RelationTypeOwner(id identity.UUID) PropertyOwner {
	if id.IsZero() {
		panic("schema: relation type owner id must not be zero")
	}

	return PropertyOwner{
		kind: PropertyOwnerRelationType,
		id:   id,
	}
}

func (o PropertyOwner) Kind() PropertyOwnerKind {
	return o.kind
}

func (o PropertyOwner) ID() identity.UUID {
	return o.id
}

func (o PropertyOwner) IsValid() bool {
	return o.kind.IsValid() && !o.id.IsZero()
}

// ComputationKind identifies how a PropertyDefinition obtains its value.
type ComputationKind uint8

const (
	ComputationInvalid ComputationKind = iota
	ComputationInput
	ComputationDerived
)

func (k ComputationKind) IsValid() bool {
	return k == ComputationInput || k == ComputationDerived
}

func (k ComputationKind) String() string {
	switch k {
	case ComputationInput:
		return "Input"
	case ComputationDerived:
		return "Derived"
	default:
		return "Invalid"
	}
}

// Computation describes how a property obtains its value.
//
// Input has no Expression.
// Derived always has an Expression.
type Computation struct {
	kind       ComputationKind
	expression ast.Expression
}

func InputComputation() Computation {
	return Computation{
		kind: ComputationInput,
	}
}

func DerivedComputation(expression ast.Expression) Computation {
	if expression == nil {
		panic("schema: derived computation expression must not be nil")
	}

	return Computation{
		kind:       ComputationDerived,
		expression: expression,
	}
}

func (c Computation) Kind() ComputationKind {
	return c.kind
}

func (c Computation) Expression() (ast.Expression, bool) {
	if c.kind != ComputationDerived {
		return nil, false
	}

	return c.expression, true
}

func (c Computation) IsValid() bool {
	switch c.kind {
	case ComputationInput:
		return c.expression == nil

	case ComputationDerived:
		return c.expression != nil

	default:
		return false
	}
}

// PropertyDefinition defines the concrete implementation of a property.
//
// ValueType, Unit, TemporalMode and computation semantics belong to the
// definition rather than to individual Entity or Relation instances.
type PropertyDefinition struct {
	id identity.UUID

	owner PropertyOwner

	contractID  identity.UUID
	hasContract bool

	name string

	valueType value.Type

	unit    unit.Unit
	hasUnit bool

	temporalMode TemporalMode
	computation  Computation
}

func NewPropertyDefinition(
	id identity.UUID,
	owner PropertyOwner,
	contractID identity.UUID,
	hasContract bool,
	name string,
	valueType value.Type,
	propertyUnit unit.Unit,
	hasUnit bool,
	temporalMode TemporalMode,
	computation Computation,
) PropertyDefinition {
	if id.IsZero() {
		panic("schema: property definition id must not be zero")
	}

	if !owner.IsValid() {
		panic("schema: property definition owner is invalid")
	}

	if hasContract && contractID.IsZero() {
		panic("schema: property definition contract id must not be zero")
	}

	if name == "" {
		panic("schema: property definition name must not be empty")
	}

	if !valueType.IsValid() {
		panic("schema: property definition value type is invalid")
	}

	if hasUnit && !propertyUnit.IsValid() {
		panic("schema: property definition unit is invalid")
	}

	if !temporalMode.IsValid() {
		panic("schema: property definition temporal mode is invalid")
	}

	if !computation.IsValid() {
		panic("schema: property definition computation is invalid")
	}

	if !hasContract {
		contractID = identity.UUID{}
	}

	if !hasUnit {
		propertyUnit = unit.Unit{}
	}

	return PropertyDefinition{
		id:           id,
		owner:        owner,
		contractID:   contractID,
		hasContract:  hasContract,
		name:         name,
		valueType:    valueType,
		unit:         propertyUnit,
		hasUnit:      hasUnit,
		temporalMode: temporalMode,
		computation:  computation,
	}
}

func (d PropertyDefinition) ID() identity.UUID {
	return d.id
}

func (d PropertyDefinition) Owner() PropertyOwner {
	return d.owner
}

func (d PropertyDefinition) ContractID() (identity.UUID, bool) {
	return d.contractID, d.hasContract
}

func (d PropertyDefinition) Name() string {
	return d.name
}

func (d PropertyDefinition) ValueType() value.Type {
	return d.valueType
}

func (d PropertyDefinition) Unit() (unit.Unit, bool) {
	return d.unit, d.hasUnit
}

func (d PropertyDefinition) TemporalMode() TemporalMode {
	return d.temporalMode
}

func (d PropertyDefinition) Computation() Computation {
	return d.computation
}

func (d PropertyDefinition) IsValid() bool {
	if d.id.IsZero() ||
		!d.owner.IsValid() ||
		d.name == "" ||
		!d.valueType.IsValid() ||
		!d.temporalMode.IsValid() ||
		!d.computation.IsValid() {
		return false
	}

	if d.hasContract {
		if d.contractID.IsZero() {
			return false
		}
	} else if !d.contractID.IsZero() {
		return false
	}

	if d.hasUnit {
		if !d.unit.IsValid() {
			return false
		}
	} else if d.unit.IsValid() {
		return false
	}

	return true
}

func (d PropertyDefinition) String() string {
	return fmt.Sprintf(
		"PropertyDefinition(%s, %s)",
		d.id.String(),
		d.name,
	)
}
