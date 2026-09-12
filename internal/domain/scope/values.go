package scope

import (
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// Values stores the scope dimension values selected for one calculation.
//
// At most one value may exist for each ScopeDefinition.
//
// Values is immutable and exposes entries in deterministic UUID order.
// The zero value represents an empty scope context and is valid.
type Values struct {
	selectors []Selector
}

// NewValues constructs an immutable set of calculation scope values.
//
// NewValues panics if a selector is invalid or if the same ScopeDefinition
// occurs more than once.
func NewValues(
	selectors []Selector,
) Values {
	if len(selectors) == 0 {
		return Values{}
	}

	copied := make(
		[]Selector,
		len(selectors),
	)

	seen := make(
		map[identity.UUID]struct{},
		len(selectors),
	)

	for index, selector := range selectors {
		if !selector.IsValid() {
			panic("scope: calculation scope selector is invalid")
		}

		definitionID := selector.DefinitionID()

		if _, exists := seen[definitionID]; exists {
			panic("scope: calculation scope contains duplicate definition")
		}

		seen[definitionID] = struct{}{}
		copied[index] = selector
	}

	sort.Slice(
		copied,
		func(i, j int) bool {
			return copied[i].DefinitionID().String() <
				copied[j].DefinitionID().String()
		},
	)

	return Values{
		selectors: copied,
	}
}

// Len returns the number of selected scope dimensions.
func (v Values) Len() int {
	return len(v.selectors)
}

// Get resolves the selected value for definitionID.
func (v Values) Get(
	definitionID identity.UUID,
) (value.Typed, bool) {
	if definitionID.IsZero() {
		return value.Typed{}, false
	}

	index := sort.Search(
		len(v.selectors),
		func(index int) bool {
			return v.selectors[index].DefinitionID().String() >=
				definitionID.String()
		},
	)

	if index >= len(v.selectors) ||
		v.selectors[index].DefinitionID() != definitionID {
		return value.Typed{}, false
	}

	return v.selectors[index].Value(), true
}

// Selectors returns all selected scope values in deterministic UUID order.
func (v Values) Selectors() []Selector {
	if len(v.selectors) == 0 {
		return nil
	}

	result := make(
		[]Selector,
		len(v.selectors),
	)

	copy(result, v.selectors)

	return result
}

// IsValid reports whether v contains valid unique scope dimensions in
// canonical order.
func (v Values) IsValid() bool {
	var previous identity.UUID

	for index, selector := range v.selectors {
		if !selector.IsValid() {
			return false
		}

		if index > 0 &&
			previous.String() >= selector.DefinitionID().String() {
			return false
		}

		previous = selector.DefinitionID()
	}

	return true
}

// Equal reports semantic equality of two immutable selected scope contexts.
func (v Values) Equal(other Values) bool {
	if !v.IsValid() || !other.IsValid() || len(v.selectors) != len(other.selectors) {
		return false
	}

	for index := range v.selectors {
		left, right := v.selectors[index], other.selectors[index]
		if left.DefinitionID() != right.DefinitionID() || !left.Value().Equal(right.Value()) {
			return false
		}
	}

	return true
}
