package scope

import domainscope "github.com/shmatovv/bcg-engine/v2/internal/domain/scope"

// Matches reports whether every selector required by a ValueBinding is
// satisfied by the calculation scope values.
//
// An empty selector set matches every calculation scope.
//
// Scope dimensions are independent. Extra calculation scope values do not
// make a binding less applicable.
//
// Matching is strict Typed equality. Core does not perform implicit scope
// unit conversion or coercion.
func Matches(
	selectors []domainscope.Selector,
	values domainscope.Values,
) bool {
	if !values.IsValid() {
		return false
	}

	for _, selector := range selectors {
		if !selector.IsValid() {
			return false
		}

		actual, exists := values.Get(
			selector.DefinitionID(),
		)

		if !exists ||
			!actual.Equal(selector.Value()) {
			return false
		}
	}

	return true
}

// Specificity returns the scope specificity of one binding.
//
// v1 defines scope specificity as the number of explicitly constrained
// independent scope dimensions.
func Specificity(
	selectors []domainscope.Selector,
) int {
	return len(selectors)
}
