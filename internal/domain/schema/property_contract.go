package schema

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// TemporalMode defines how values of a property are represented over time.
//
// TemporalMode is independent from ValueType.
// A property may either represent a scalar value or a time series.
type TemporalMode uint8

const (
	// TemporalInvalid represents an invalid or uninitialized temporal mode.
	TemporalInvalid TemporalMode = iota

	// TemporalScalar represents a scalar value.
	TemporalScalar

	// TemporalSeries represents a time series.
	TemporalSeries
)

// IsValid reports whether m is a valid TemporalMode.
func (m TemporalMode) IsValid() bool {
	return m == TemporalScalar || m == TemporalSeries
}

// String returns the canonical name of m.
//
// String returns "Invalid" for values that do not represent
// a valid TemporalMode.
func (m TemporalMode) String() string {
	switch m {
	case TemporalScalar:
		return "Scalar"
	case TemporalSeries:
		return "Series"
	default:
		return "Invalid"
	}
}

// PropertyContract describes the semantic and type contract of a property.
//
// A PropertyContract defines what kind of logical value a property represents,
// but does not define how that value is obtained.
//
// In particular, PropertyContract does not contain:
//
//   - Input/Derived computation semantics;
//   - Expression;
//   - ValueBinding.
//
// Those responsibilities belong to PropertyDefinition and runtime resolution.
//
// PropertyContract values are immutable after construction.
type PropertyContract struct {
	id                identity.UUID
	name              string
	valueType         value.Type
	enumDefinitionID  identity.UUID
	hasEnumDefinition bool
	dimension         unit.Dimension
	hasDimension      bool
	temporalMode      TemporalMode
	nullable          bool
}

func NewEnumPropertyContract(id identity.UUID, name string, enumDefinitionID identity.UUID, dimension unit.Dimension, hasDimension bool, temporalMode TemporalMode, nullable bool) PropertyContract {
	if enumDefinitionID.IsZero() {
		panic("schema: enum definition id must not be zero")
	}
	result := NewPropertyContract(id, name, value.Enum, dimension, hasDimension, temporalMode, nullable)
	result.enumDefinitionID, result.hasEnumDefinition = enumDefinitionID, true
	return result
}

// NewPropertyContract constructs a PropertyContract.
//
// NewPropertyContract panics if:
//
//   - id is the zero UUID;
//   - name is empty;
//   - valueType is invalid;
//   - dimension is present but invalid;
//   - temporalMode is invalid.
//
// These conditions represent invalid schema construction and therefore
// programming errors at this layer.
func NewPropertyContract(
	id identity.UUID,
	name string,
	valueType value.Type,
	dimension unit.Dimension,
	hasDimension bool,
	temporalMode TemporalMode,
	nullable bool,
) PropertyContract {
	if id.IsZero() {
		panic("schema: property contract id must not be zero")
	}

	if name == "" {
		panic("schema: property contract name must not be empty")
	}

	if !valueType.IsValid() {
		panic("schema: property contract value type is invalid")
	}

	if hasDimension && !dimension.IsValid() {
		panic("schema: property contract dimension is invalid")
	}

	if !temporalMode.IsValid() {
		panic("schema: property contract temporal mode is invalid")
	}

	if !hasDimension {
		dimension = unit.Dimension{}
	}

	return PropertyContract{
		id:           id,
		name:         name,
		valueType:    valueType,
		dimension:    dimension,
		hasDimension: hasDimension,
		temporalMode: temporalMode,
		nullable:     nullable,
	}
}

// ID returns the stable identity of the contract.
func (c PropertyContract) ID() identity.UUID {
	return c.id
}

// Name returns the presentation name of the contract.
//
// Name is metadata and is not part of computational identity.
func (c PropertyContract) Name() string {
	return c.name
}

// ValueType returns the value type required by the contract.
func (c PropertyContract) ValueType() value.Type {
	return c.valueType
}
func (c PropertyContract) EnumDefinitionID() (identity.UUID, bool) {
	return c.enumDefinitionID, c.hasEnumDefinition
}

// Dimension returns the dimension required by the contract.
//
// The boolean result reports whether the contract explicitly has
// dimensional semantics.
func (c PropertyContract) Dimension() (unit.Dimension, bool) {
	return c.dimension, c.hasDimension
}

// TemporalMode returns the temporal semantics of the contract.
func (c PropertyContract) TemporalMode() TemporalMode {
	return c.temporalMode
}

// Nullable reports whether Null is a valid resolved value for properties
// implementing this contract.
func (c PropertyContract) Nullable() bool {
	return c.nullable
}

// IsValid reports whether c represents a structurally valid PropertyContract.
//
// This method is primarily useful at serialization and validation boundaries.
// Objects created through NewPropertyContract are valid by construction.
func (c PropertyContract) IsValid() bool {
	if c.id.IsZero() {
		return false
	}

	if c.name == "" {
		return false
	}

	if !c.valueType.IsValid() {
		return false
	}
	if c.valueType == value.Enum {
		if !c.hasEnumDefinition || c.enumDefinitionID.IsZero() {
			return false
		}
	} else if c.hasEnumDefinition || !c.enumDefinitionID.IsZero() {
		return false
	}

	if c.hasDimension && !c.dimension.IsValid() {
		return false
	}

	if !c.temporalMode.IsValid() {
		return false
	}

	return true
}

// String returns a human-readable representation of the contract.
func (c PropertyContract) String() string {
	return fmt.Sprintf(
		"PropertyContract(%s, %s)",
		c.id.String(),
		c.name,
	)
}
