package runtime

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
)

// Override is a calculation-local value override for one concrete property.
//
// Calculation overrides are runtime input and are not stored in ModelVersion.
// They participate in resolution above Scenario and Base ValueBindings.
//
// Whether the target PropertyDefinition is eligible for a calculation
// override is checked by the resolver against the committed ModelVersion.
type Override struct {
	target address.PropertyAddress
	value  semantic.BindingValue
}

// NewOverride constructs a calculation-local value override.
func NewOverride(
	target address.PropertyAddress,
	bindingValue semantic.BindingValue,
) Override {
	if !target.IsValid() {
		panic("runtime: calculation override target is invalid")
	}

	if !bindingValue.IsValid() {
		panic("runtime: calculation override value is invalid")
	}

	return Override{
		target: target,
		value:  bindingValue,
	}
}

// Target returns the concrete property replaced by the override.
func (o Override) Target() address.PropertyAddress {
	return o.target
}

// Value returns the override value.
func (o Override) Value() semantic.BindingValue {
	return o.value
}

// IsValid reports whether o is structurally valid.
func (o Override) IsValid() bool {
	return o.target.IsValid() &&
		o.value.IsValid()
}

// Equal reports semantic equality of immutable calculation overrides.
func (o Override) Equal(other Override) bool {
	return o.IsValid() && other.IsValid() &&
		o.target == other.target && o.value.Equal(other.value)
}
