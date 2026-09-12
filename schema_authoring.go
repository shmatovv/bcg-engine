package bcg

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
)

// EntityTypeSpec describes one EntityType. Trait and PropertyDefinitions are
// references by explicit ID. Their existence and compatibility are checked by
// Draft.Validate and Draft.Commit, not while the object is assembled.
type EntityTypeSpec struct {
	ID                  ID
	Name                string
	Traits              []ID
	PropertyDefinitions []ID
}

// EntityType is an immutable public schema declaration for a class of
// Entities. It contains no Entity instances and does not model inheritance.
type EntityType struct{ value schema.EntityType }

// NewEntityType constructs a structurally valid EntityType declaration.
func NewEntityType(spec EntityTypeSpec) (EntityType, error) {
	id, err := requiredSchemaID(spec.ID, "entity type ID")
	if err != nil {
		return EntityType{}, err
	}
	if spec.Name == "" {
		return EntityType{}, invalidSchemaArgument("entity type name must not be empty")
	}
	traits, err := schemaIDs(spec.Traits, "entity type trait")
	if err != nil {
		return EntityType{}, err
	}
	definitions, err := schemaIDs(spec.PropertyDefinitions, "entity type property definition")
	if err != nil {
		return EntityType{}, err
	}
	return EntityType{value: schema.NewEntityType(id, spec.Name, traits, definitions)}, nil
}

// ID returns the stable EntityType identity.
func (t EntityType) ID() ID { return idFromInternal(t.value.ID()) }

// Name returns the EntityType presentation name.
func (t EntityType) Name() string { return t.value.Name() }

// Traits returns an owned list of required Trait IDs.
func (t EntityType) Traits() []ID { return publicSchemaIDs(t.value.Traits()) }

// PropertyDefinitions returns an owned list of available PropertyDefinition IDs.
func (t EntityType) PropertyDefinitions() []ID {
	return publicSchemaIDs(t.value.PropertyDefinitions())
}

// IsValid reports structural validity. Referential validity is checked by a Draft.
func (t EntityType) IsValid() bool { return t.value.IsValid() }

// EndpointConstraintKind identifies a RelationType endpoint restriction.
// The zero value is invalid and is used by RelationTypeSpec to mean that an
// endpoint restriction is omitted.
type EndpointConstraintKind uint8

const (
	// EndpointEntityType restricts an endpoint to one EntityType.
	EndpointEntityType EndpointConstraintKind = iota + 1
	// EndpointTrait restricts an endpoint to Entity instances whose type
	// implements one Trait.
	EndpointTrait
	// EndpointAnyEntity accepts every Entity.
	EndpointAnyEntity
)

// IsValid reports whether k is a supported endpoint restriction kind.
func (k EndpointConstraintKind) IsValid() bool {
	return k >= EndpointEntityType && k <= EndpointAnyEntity
}

// String returns the stable endpoint restriction name.
func (k EndpointConstraintKind) String() string {
	switch k {
	case EndpointEntityType:
		return "EntityType"
	case EndpointTrait:
		return "Trait"
	case EndpointAnyEntity:
		return "AnyEntity"
	default:
		return "Invalid"
	}
}

// EndpointConstraint restricts an optional RelationType endpoint. Its zero
// value means that no restriction is present; use IsPresent to distinguish
// that from EndpointAnyEntity.
type EndpointConstraint struct {
	kind EndpointConstraintKind
	id   ID
}

// EntityTypeEndpoint constructs an endpoint restriction for an EntityType.
func EntityTypeEndpoint(id ID) (EndpointConstraint, error) {
	if _, err := requiredSchemaID(id, "entity type endpoint ID"); err != nil {
		return EndpointConstraint{}, err
	}
	return EndpointConstraint{kind: EndpointEntityType, id: id}, nil
}

// TraitEndpoint constructs an endpoint restriction for a Trait.
func TraitEndpoint(id ID) (EndpointConstraint, error) {
	if _, err := requiredSchemaID(id, "trait endpoint ID"); err != nil {
		return EndpointConstraint{}, err
	}
	return EndpointConstraint{kind: EndpointTrait, id: id}, nil
}

// AnyEntityEndpoint constructs an endpoint restriction that accepts every Entity.
func AnyEntityEndpoint() EndpointConstraint {
	return EndpointConstraint{kind: EndpointAnyEntity}
}

// Kind returns the endpoint restriction kind, or an invalid value when the
// endpoint is omitted.
func (c EndpointConstraint) Kind() EndpointConstraintKind { return c.kind }

// ID returns the referenced EntityType or Trait ID. It returns false for an
// omitted restriction and for EndpointAnyEntity.
func (c EndpointConstraint) ID() (ID, bool) {
	if c.kind != EndpointEntityType && c.kind != EndpointTrait {
		return ID{}, false
	}
	return c.id, true
}

// IsPresent reports whether c represents an explicitly supplied restriction.
func (c EndpointConstraint) IsPresent() bool { return c.kind.IsValid() }

// IsValid reports whether c is a valid present endpoint restriction. An
// omitted endpoint is not a valid constraint, but is valid in RelationTypeSpec.
func (c EndpointConstraint) IsValid() bool {
	switch c.kind {
	case EndpointEntityType, EndpointTrait:
		return !c.id.IsZero()
	case EndpointAnyEntity:
		return c.id.IsZero()
	default:
		return false
	}
}

func (c EndpointConstraint) internal() (schema.TypeConstraint, bool, error) {
	if !c.IsPresent() {
		return schema.TypeConstraint{}, false, nil
	}
	if !c.IsValid() {
		return schema.TypeConstraint{}, false, invalidSchemaArgument("relation endpoint constraint is invalid")
	}
	switch c.kind {
	case EndpointEntityType:
		return schema.EntityTypeConstraint(c.id.value), true, nil
	case EndpointTrait:
		return schema.TraitConstraint(c.id.value), true, nil
	case EndpointAnyEntity:
		return schema.AnyEntityConstraint(), true, nil
	default:
		return schema.TypeConstraint{}, false, invalidSchemaArgument("relation endpoint constraint is invalid")
	}
}

func endpointConstraintFromInternal(value schema.TypeConstraint) EndpointConstraint {
	switch value.Kind() {
	case schema.TypeConstraintEntityType:
		id, _ := value.ID()
		return EndpointConstraint{kind: EndpointEntityType, id: idFromInternal(id)}
	case schema.TypeConstraintTrait:
		id, _ := value.ID()
		return EndpointConstraint{kind: EndpointTrait, id: idFromInternal(id)}
	case schema.TypeConstraintAnyEntity:
		return AnyEntityEndpoint()
	default:
		return EndpointConstraint{}
	}
}

// RelationTypeSpec describes one RelationType. A zero From or To omits that
// endpoint restriction; EndpointAnyEntity is an explicit unrestricted endpoint.
type RelationTypeSpec struct {
	ID                  ID
	Name                string
	From                EndpointConstraint
	To                  EndpointConstraint
	PropertyDefinitions []ID
}

// RelationType is an immutable public schema declaration for a relation.
type RelationType struct{ value schema.RelationType }

// NewRelationType constructs a structurally valid RelationType declaration.
func NewRelationType(spec RelationTypeSpec) (RelationType, error) {
	id, err := requiredSchemaID(spec.ID, "relation type ID")
	if err != nil {
		return RelationType{}, err
	}
	if spec.Name == "" {
		return RelationType{}, invalidSchemaArgument("relation type name must not be empty")
	}
	from, hasFrom, err := spec.From.internal()
	if err != nil {
		return RelationType{}, err
	}
	to, hasTo, err := spec.To.internal()
	if err != nil {
		return RelationType{}, err
	}
	definitions, err := schemaIDs(spec.PropertyDefinitions, "relation type property definition")
	if err != nil {
		return RelationType{}, err
	}
	return RelationType{value: schema.NewRelationType(id, spec.Name, from, hasFrom, to, hasTo, definitions)}, nil
}

// ID returns the stable RelationType identity.
func (t RelationType) ID() ID { return idFromInternal(t.value.ID()) }

// Name returns the RelationType presentation name.
func (t RelationType) Name() string { return t.value.Name() }

// From returns the optional source endpoint restriction.
func (t RelationType) From() (EndpointConstraint, bool) {
	value, ok := t.value.From()
	if !ok {
		return EndpointConstraint{}, false
	}
	return endpointConstraintFromInternal(value), true
}

// To returns the optional target endpoint restriction.
func (t RelationType) To() (EndpointConstraint, bool) {
	value, ok := t.value.To()
	if !ok {
		return EndpointConstraint{}, false
	}
	return endpointConstraintFromInternal(value), true
}

// PropertyDefinitions returns an owned list of available PropertyDefinition IDs.
func (t RelationType) PropertyDefinitions() []ID {
	return publicSchemaIDs(t.value.PropertyDefinitions())
}

// IsValid reports structural validity. Referential validity is checked by a Draft.
func (t RelationType) IsValid() bool { return t.value.IsValid() }

// PropertyOwnerKind identifies the schema type that owns a PropertyDefinition.
type PropertyOwnerKind uint8

const (
	// PropertyOwnerEntityType means an EntityType owns the property.
	PropertyOwnerEntityType PropertyOwnerKind = iota + 1
	// PropertyOwnerRelationType means a RelationType owns the property.
	PropertyOwnerRelationType
)

// IsValid reports whether k is a supported property owner kind.
func (k PropertyOwnerKind) IsValid() bool {
	return k == PropertyOwnerEntityType || k == PropertyOwnerRelationType
}

// String returns the stable property owner kind name.
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

// PropertyOwner identifies the schema owner of a PropertyDefinition. Its zero
// value is invalid.
type PropertyOwner struct {
	kind PropertyOwnerKind
	id   ID
}

// EntityTypeOwner constructs an EntityType property owner.
func EntityTypeOwner(id ID) (PropertyOwner, error) {
	if _, err := requiredSchemaID(id, "entity type owner ID"); err != nil {
		return PropertyOwner{}, err
	}
	return PropertyOwner{kind: PropertyOwnerEntityType, id: id}, nil
}

// RelationTypeOwner constructs a RelationType property owner.
func RelationTypeOwner(id ID) (PropertyOwner, error) {
	if _, err := requiredSchemaID(id, "relation type owner ID"); err != nil {
		return PropertyOwner{}, err
	}
	return PropertyOwner{kind: PropertyOwnerRelationType, id: id}, nil
}

// Kind returns the schema owner kind.
func (o PropertyOwner) Kind() PropertyOwnerKind { return o.kind }

// ID returns the owning EntityType or RelationType identity.
func (o PropertyOwner) ID() ID { return o.id }

// IsValid reports whether o has a supported kind and non-zero identity.
func (o PropertyOwner) IsValid() bool { return o.kind.IsValid() && !o.id.IsZero() }

func (o PropertyOwner) internal() (schema.PropertyOwner, error) {
	if !o.IsValid() {
		return schema.PropertyOwner{}, invalidSchemaArgument("property owner is invalid")
	}
	switch o.kind {
	case PropertyOwnerEntityType:
		return schema.EntityTypeOwner(o.id.value), nil
	case PropertyOwnerRelationType:
		return schema.RelationTypeOwner(o.id.value), nil
	default:
		return schema.PropertyOwner{}, invalidSchemaArgument("property owner is invalid")
	}
}

func propertyOwnerFromInternal(value schema.PropertyOwner) PropertyOwner {
	switch value.Kind() {
	case schema.PropertyOwnerEntityType:
		return PropertyOwner{kind: PropertyOwnerEntityType, id: idFromInternal(value.ID())}
	case schema.PropertyOwnerRelationType:
		return PropertyOwner{kind: PropertyOwnerRelationType, id: idFromInternal(value.ID())}
	default:
		return PropertyOwner{}
	}
}

// ComputationKind identifies whether a property is supplied as Input or
// calculated by a Derived expression. The zero value is invalid.
type ComputationKind uint8

const (
	// ComputationInput represents a runtime-supplied property value.
	ComputationInput ComputationKind = iota + 1
	// ComputationDerived represents a value calculated from an Expression.
	ComputationDerived
)

// IsValid reports whether k is a supported computation kind.
func (k ComputationKind) IsValid() bool {
	return k == ComputationInput || k == ComputationDerived
}

// String returns the stable computation kind name.
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

// PropertyComputation describes how a PropertyDefinition obtains its value.
type PropertyComputation struct{ value schema.Computation }

// InputComputation constructs a valid Input computation.
func InputComputation() PropertyComputation {
	return PropertyComputation{value: schema.InputComputation()}
}

// DerivedComputation constructs a Derived computation from an immutable
// public Expression. Its value and schema compatibility are checked by
// Draft.Validate and Draft.Commit.
func DerivedComputation(expression Expression) (PropertyComputation, error) {
	if !expression.IsValid() {
		return PropertyComputation{}, invalidSchemaArgument("derived computation expression is invalid")
	}
	return PropertyComputation{value: schema.DerivedComputation(expression.value)}, nil
}

// Kind returns the computation kind.
func (c PropertyComputation) Kind() ComputationKind {
	switch c.value.Kind() {
	case schema.ComputationInput:
		return ComputationInput
	case schema.ComputationDerived:
		return ComputationDerived
	default:
		return 0
	}
}

// Expression returns the Derived expression when c is derived.
func (c PropertyComputation) Expression() (Expression, bool) {
	expression, ok := c.value.Expression()
	if !ok {
		return Expression{}, false
	}
	return Expression{value: expression}, true
}

// IsValid reports structural validity.
func (c PropertyComputation) IsValid() bool { return c.value.IsValid() }

func propertyComputationFromInternal(value schema.Computation) PropertyComputation {
	return PropertyComputation{value: value}
}

// PropertyDefinitionSpec describes one concrete property implementation. A
// zero ContractID means no PropertyContract. A zero Unit means no concrete
// unit; DimensionlessUnit is a distinct explicit unit. Property nullable
// semantics belong to an implemented PropertyContract when present.
type PropertyDefinitionSpec struct {
	ID           ID
	Owner        PropertyOwner
	ContractID   ID
	Name         string
	ValueType    ValueType
	Unit         Unit
	TemporalMode TemporalMode
	Computation  PropertyComputation
}

// PropertyDefinition is an immutable public schema declaration for one
// concrete EntityType or RelationType property.
type PropertyDefinition struct{ value schema.PropertyDefinition }

// NewPropertyDefinition constructs a structurally valid property declaration.
// Cross-schema references and type/unit compatibility are validated by Draft.
func NewPropertyDefinition(spec PropertyDefinitionSpec) (PropertyDefinition, error) {
	id, err := requiredSchemaID(spec.ID, "property definition ID")
	if err != nil {
		return PropertyDefinition{}, err
	}
	owner, err := spec.Owner.internal()
	if err != nil {
		return PropertyDefinition{}, err
	}
	if spec.Name == "" {
		return PropertyDefinition{}, invalidSchemaArgument("property definition name must not be empty")
	}
	valueType, err := spec.ValueType.internal()
	if err != nil {
		return PropertyDefinition{}, err
	}
	temporalMode, err := spec.TemporalMode.internal()
	if err != nil {
		return PropertyDefinition{}, err
	}
	if !spec.Computation.IsValid() {
		return PropertyDefinition{}, invalidSchemaArgument("property definition computation is invalid")
	}
	contractID, hasContract := identity.UUID{}, !spec.ContractID.IsZero()
	if hasContract {
		contractID = spec.ContractID.value
	}
	propertyUnit, hasUnit := spec.Unit.value, spec.Unit.IsValid()
	return PropertyDefinition{value: schema.NewPropertyDefinition(
		id,
		owner,
		contractID,
		hasContract,
		spec.Name,
		valueType,
		propertyUnit,
		hasUnit,
		temporalMode,
		spec.Computation.value,
	)}, nil
}

// ID returns the stable PropertyDefinition identity.
func (d PropertyDefinition) ID() ID { return idFromInternal(d.value.ID()) }

// Owner returns the EntityType or RelationType that owns d.
func (d PropertyDefinition) Owner() PropertyOwner { return propertyOwnerFromInternal(d.value.Owner()) }

// ContractID returns the optional implemented PropertyContract identity.
func (d PropertyDefinition) ContractID() (ID, bool) {
	id, ok := d.value.ContractID()
	if !ok {
		return ID{}, false
	}
	return idFromInternal(id), true
}

// Name returns the property presentation name.
func (d PropertyDefinition) Name() string { return d.value.Name() }

// ValueType returns the property's scalar value type.
func (d PropertyDefinition) ValueType() ValueType {
	return valueTypeFromInternal(d.value.ValueType())
}

// Unit returns the optional concrete property unit.
func (d PropertyDefinition) Unit() (Unit, bool) {
	value, ok := d.value.Unit()
	if !ok {
		return Unit{}, false
	}
	return Unit{value: value}, true
}

// TemporalMode returns the property's scalar or series temporal mode.
func (d PropertyDefinition) TemporalMode() TemporalMode {
	mode, err := temporalModeFromInternal(d.value.TemporalMode())
	if err != nil {
		return 0
	}
	return mode
}

// Computation returns the property's Input or Derived computation.
func (d PropertyDefinition) Computation() PropertyComputation {
	return propertyComputationFromInternal(d.value.Computation())
}

// IsValid reports structural validity. Model-wide validity is checked by a Draft.
func (d PropertyDefinition) IsValid() bool { return d.value.IsValid() }

// PropertyContractSpec describes one semantic PropertyContract. Dimension is
// optional: nil means no dimensional requirement, while a pointer to
// DimensionlessDimension means an explicit dimensionless requirement.
type PropertyContractSpec struct {
	ID           ID
	Name         string
	ValueType    ValueType
	Dimension    *Dimension
	TemporalMode TemporalMode
	Nullable     bool
}

// PropertyContract is an immutable type and nullability requirement. It has
// no computation or base value.
type PropertyContract struct{ value schema.PropertyContract }

// NewPropertyContract constructs a structurally valid PropertyContract.
func NewPropertyContract(spec PropertyContractSpec) (PropertyContract, error) {
	id, err := requiredSchemaID(spec.ID, "property contract ID")
	if err != nil {
		return PropertyContract{}, err
	}
	if spec.Name == "" {
		return PropertyContract{}, invalidSchemaArgument("property contract name must not be empty")
	}
	valueType, err := spec.ValueType.internal()
	if err != nil {
		return PropertyContract{}, err
	}
	temporalMode, err := spec.TemporalMode.internal()
	if err != nil {
		return PropertyContract{}, err
	}
	dimension, hasDimension := Dimension{}, spec.Dimension != nil
	if hasDimension {
		dimension = *spec.Dimension
		if !dimension.IsValid() {
			return PropertyContract{}, invalidSchemaArgument("property contract dimension is invalid")
		}
	}
	return PropertyContract{value: schema.NewPropertyContract(
		id,
		spec.Name,
		valueType,
		dimension.value,
		hasDimension,
		temporalMode,
		spec.Nullable,
	)}, nil
}

// ID returns the stable PropertyContract identity.
func (c PropertyContract) ID() ID { return idFromInternal(c.value.ID()) }

// Name returns the PropertyContract presentation name.
func (c PropertyContract) Name() string { return c.value.Name() }

// ValueType returns the required scalar value type.
func (c PropertyContract) ValueType() ValueType {
	return valueTypeFromInternal(c.value.ValueType())
}

// Dimension returns the optional required dimension.
func (c PropertyContract) Dimension() (Dimension, bool) {
	value, ok := c.value.Dimension()
	if !ok {
		return Dimension{}, false
	}
	return Dimension{value: value}, true
}

// TemporalMode returns the required scalar or series temporal mode.
func (c PropertyContract) TemporalMode() TemporalMode {
	mode, err := temporalModeFromInternal(c.value.TemporalMode())
	if err != nil {
		return 0
	}
	return mode
}

// Nullable reports whether Null is permitted for implementations of c.
func (c PropertyContract) Nullable() bool { return c.value.Nullable() }

// IsValid reports structural validity. Model-wide validity is checked by a Draft.
func (c PropertyContract) IsValid() bool { return c.value.IsValid() }

// TraitSpec describes a reusable PropertyContract requirement set. Traits are
// composition only: BCG v1 has no Trait or EntityType inheritance.
type TraitSpec struct {
	ID                        ID
	Name                      string
	RequiredPropertyContracts []ID
}

// Trait is an immutable reusable schema requirement set.
type Trait struct{ value schema.Trait }

// NewTrait constructs a structurally valid Trait declaration.
func NewTrait(spec TraitSpec) (Trait, error) {
	id, err := requiredSchemaID(spec.ID, "trait ID")
	if err != nil {
		return Trait{}, err
	}
	if spec.Name == "" {
		return Trait{}, invalidSchemaArgument("trait name must not be empty")
	}
	contracts, err := schemaIDs(spec.RequiredPropertyContracts, "trait property contract")
	if err != nil {
		return Trait{}, err
	}
	return Trait{value: schema.NewTrait(id, spec.Name, contracts)}, nil
}

// ID returns the stable Trait identity.
func (t Trait) ID() ID { return idFromInternal(t.value.ID()) }

// Name returns the Trait presentation name.
func (t Trait) Name() string { return t.value.Name() }

// RequiredPropertyContracts returns an owned list of required contract IDs.
func (t Trait) RequiredPropertyContracts() []ID {
	return publicSchemaIDs(t.value.RequiredPropertyContracts())
}

// IsValid reports structural validity. Referential validity is checked by a Draft.
func (t Trait) IsValid() bool { return t.value.IsValid() }

// ScopeDefinitionSpec describes one independent scope dimension. BCG v1
// scope definitions have an ID, name, and scalar value type only.
type ScopeDefinitionSpec struct {
	ID        ID
	Name      string
	ValueType ValueType
}

// ScopeDefinition is an immutable declaration for one scope selector dimension.
type ScopeDefinition struct{ value scope.Definition }

// NewScopeDefinition constructs a structurally valid ScopeDefinition.
func NewScopeDefinition(spec ScopeDefinitionSpec) (ScopeDefinition, error) {
	id, err := requiredSchemaID(spec.ID, "scope definition ID")
	if err != nil {
		return ScopeDefinition{}, err
	}
	if spec.Name == "" {
		return ScopeDefinition{}, invalidSchemaArgument("scope definition name must not be empty")
	}
	valueType, err := spec.ValueType.internal()
	if err != nil {
		return ScopeDefinition{}, err
	}
	return ScopeDefinition{value: scope.NewDefinition(id, spec.Name, valueType)}, nil
}

// ID returns the stable ScopeDefinition identity.
func (d ScopeDefinition) ID() ID { return idFromInternal(d.value.ID()) }

// Name returns the ScopeDefinition presentation name.
func (d ScopeDefinition) Name() string { return d.value.Name() }

// ValueType returns the required selector value type.
func (d ScopeDefinition) ValueType() ValueType {
	return valueTypeFromInternal(d.value.ValueType())
}

// IsValid reports structural validity.
func (d ScopeDefinition) IsValid() bool { return d.value.IsValid() }

// AddEntityType appends value to d. Duplicate IDs and dangling references are
// intentionally retained until Validate or Commit reports them.
func (d *Draft) AddEntityType(value EntityType) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !value.IsValid() {
		return invalidSchemaArgument("entity type is invalid")
	}
	d.value.AddEntityType(value.value)
	return nil
}

// ReplaceEntityType updates the entity type identified by id. The replacement
// must retain that identity; use AddEntityType and RemoveEntityType for an
// intentional identity change.
func (d *Draft) ReplaceEntityType(id ID, value EntityType) error {
	if err := d.requireSchemaReplacement(id, value.ID(), value.IsValid(), "entity type"); err != nil {
		return err
	}
	if !d.value.ReplaceEntityType(id.value, value.value) {
		return schemaItemNotFound("entity type", id)
	}
	return nil
}

// RemoveEntityType removes one EntityType without cascading to dependents.
// Validate or Commit reports any dangling references left by the removal.
func (d *Draft) RemoveEntityType(id ID) error {
	if err := d.requireSchemaDraftID(id, "entity type ID"); err != nil {
		return err
	}
	if !d.value.RemoveEntityType(id.value) {
		return schemaItemNotFound("entity type", id)
	}
	return nil
}

// AddRelationType appends value to d.
func (d *Draft) AddRelationType(value RelationType) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !value.IsValid() {
		return invalidSchemaArgument("relation type is invalid")
	}
	d.value.AddRelationType(value.value)
	return nil
}

// ReplaceRelationType updates the relation type identified by id, preserving identity.
func (d *Draft) ReplaceRelationType(id ID, value RelationType) error {
	if err := d.requireSchemaReplacement(id, value.ID(), value.IsValid(), "relation type"); err != nil {
		return err
	}
	if !d.value.ReplaceRelationType(id.value, value.value) {
		return schemaItemNotFound("relation type", id)
	}
	return nil
}

// RemoveRelationType removes one RelationType without cascading to dependents.
func (d *Draft) RemoveRelationType(id ID) error {
	if err := d.requireSchemaDraftID(id, "relation type ID"); err != nil {
		return err
	}
	if !d.value.RemoveRelationType(id.value) {
		return schemaItemNotFound("relation type", id)
	}
	return nil
}

// AddPropertyDefinition appends value to d.
func (d *Draft) AddPropertyDefinition(value PropertyDefinition) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !value.IsValid() {
		return invalidSchemaArgument("property definition is invalid")
	}
	d.value.AddPropertyDefinition(value.value)
	return nil
}

// ReplacePropertyDefinition updates the property definition identified by id,
// preserving identity.
func (d *Draft) ReplacePropertyDefinition(id ID, value PropertyDefinition) error {
	if err := d.requireSchemaReplacement(id, value.ID(), value.IsValid(), "property definition"); err != nil {
		return err
	}
	if !d.value.ReplacePropertyDefinition(id.value, value.value) {
		return schemaItemNotFound("property definition", id)
	}
	return nil
}

// RemovePropertyDefinition removes one property definition without cascading.
func (d *Draft) RemovePropertyDefinition(id ID) error {
	if err := d.requireSchemaDraftID(id, "property definition ID"); err != nil {
		return err
	}
	if !d.value.RemovePropertyDefinition(id.value) {
		return schemaItemNotFound("property definition", id)
	}
	return nil
}

// AddPropertyContract appends value to d.
func (d *Draft) AddPropertyContract(value PropertyContract) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !value.IsValid() {
		return invalidSchemaArgument("property contract is invalid")
	}
	d.value.AddPropertyContract(value.value)
	return nil
}

// ReplacePropertyContract updates the property contract identified by id,
// preserving identity.
func (d *Draft) ReplacePropertyContract(id ID, value PropertyContract) error {
	if err := d.requireSchemaReplacement(id, value.ID(), value.IsValid(), "property contract"); err != nil {
		return err
	}
	if !d.value.ReplacePropertyContract(id.value, value.value) {
		return schemaItemNotFound("property contract", id)
	}
	return nil
}

// RemovePropertyContract removes one contract without cascading to Traits or properties.
func (d *Draft) RemovePropertyContract(id ID) error {
	if err := d.requireSchemaDraftID(id, "property contract ID"); err != nil {
		return err
	}
	if !d.value.RemovePropertyContract(id.value) {
		return schemaItemNotFound("property contract", id)
	}
	return nil
}

// AddTrait appends value to d.
func (d *Draft) AddTrait(value Trait) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !value.IsValid() {
		return invalidSchemaArgument("trait is invalid")
	}
	d.value.AddTrait(value.value)
	return nil
}

// ReplaceTrait updates the trait identified by id, preserving identity.
func (d *Draft) ReplaceTrait(id ID, value Trait) error {
	if err := d.requireSchemaReplacement(id, value.ID(), value.IsValid(), "trait"); err != nil {
		return err
	}
	if !d.value.ReplaceTrait(id.value, value.value) {
		return schemaItemNotFound("trait", id)
	}
	return nil
}

// RemoveTrait removes one Trait without cascading to EntityTypes or relations.
func (d *Draft) RemoveTrait(id ID) error {
	if err := d.requireSchemaDraftID(id, "trait ID"); err != nil {
		return err
	}
	if !d.value.RemoveTrait(id.value) {
		return schemaItemNotFound("trait", id)
	}
	return nil
}

// AddScopeDefinition appends value to d.
func (d *Draft) AddScopeDefinition(value ScopeDefinition) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !value.IsValid() {
		return invalidSchemaArgument("scope definition is invalid")
	}
	d.value.AddScopeDefinition(value.value)
	return nil
}

// ReplaceScopeDefinition updates the scope definition identified by id,
// preserving identity.
func (d *Draft) ReplaceScopeDefinition(id ID, value ScopeDefinition) error {
	if err := d.requireSchemaReplacement(id, value.ID(), value.IsValid(), "scope definition"); err != nil {
		return err
	}
	if !d.value.ReplaceScopeDefinition(id.value, value.value) {
		return schemaItemNotFound("scope definition", id)
	}
	return nil
}

// RemoveScopeDefinition removes one scope definition without cascading.
func (d *Draft) RemoveScopeDefinition(id ID) error {
	if err := d.requireSchemaDraftID(id, "scope definition ID"); err != nil {
		return err
	}
	if !d.value.RemoveScopeDefinition(id.value) {
		return schemaItemNotFound("scope definition", id)
	}
	return nil
}

func (d *Draft) requireSchemaDraft() error {
	if !d.IsValid() {
		return invalidSchemaArgument("draft is invalid")
	}
	return nil
}

func (d *Draft) requireSchemaDraftID(id ID, name string) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if _, err := requiredSchemaID(id, name); err != nil {
		return err
	}
	return nil
}

func (d *Draft) requireSchemaReplacement(id, replacementID ID, replacementValid bool, name string) error {
	if err := d.requireSchemaDraftID(id, name+" ID"); err != nil {
		return err
	}
	if !replacementValid {
		return invalidSchemaArgument(name + " is invalid")
	}
	if !id.Equal(replacementID) {
		return invalidSchemaArgument("replacement " + name + " must retain its ID")
	}
	return nil
}

func requiredSchemaID(id ID, name string) (identity.UUID, error) {
	value, err := id.internal()
	if err != nil {
		return identity.UUID{}, fmt.Errorf("%w: %s must not be zero", ErrInvalidArgument, name)
	}
	return value, nil
}

func schemaIDs(ids []ID, name string) ([]identity.UUID, error) {
	result := make([]identity.UUID, len(ids))
	seen := make(map[identity.UUID]struct{}, len(ids))
	for index, id := range ids {
		value, err := requiredSchemaID(id, name+" ID")
		if err != nil {
			return nil, err
		}
		if _, exists := seen[value]; exists {
			return nil, invalidSchemaArgument(name + " IDs contain a duplicate")
		}
		seen[value] = struct{}{}
		result[index] = value
	}
	return result, nil
}

func publicSchemaIDs(ids []identity.UUID) []ID {
	result := make([]ID, len(ids))
	for index, id := range ids {
		result[index] = idFromInternal(id)
	}
	return result
}

func invalidSchemaArgument(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
}

func schemaItemNotFound(name string, id ID) error {
	return fmt.Errorf("%w: %s %s does not exist in draft", ErrInvalidArgument, name, id)
}
