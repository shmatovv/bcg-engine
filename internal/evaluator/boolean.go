package evaluator

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// InterpretScalarBool applies the common decision boundary for a scalar
// Boolean evaluator outcome. Null and Undefined are both indeterminate; they
// are deliberately not interpreted as false. Callers retain the original
// Result to expose the precise Null/Undefined outcome and provenance.
//
// A malformed evaluator outcome is an invariant failure, not an
// indeterminate business decision.
func InterpretScalarBool(resolved value.Resolved) (decision bool, indeterminate bool, err error) {
	if !resolved.IsValid() {
		return false, false, fmt.Errorf("invalid resolved value")
	}
	if resolved.IsNull() || resolved.IsUndefined() {
		return false, true, nil
	}
	typed := resolved.Value()
	if typed.Type() != value.Bool || typed.HasUnit() {
		return false, false, fmt.Errorf("resolved value is not a unitless Bool")
	}
	return typed.Raw().Bool(), false, nil
}
