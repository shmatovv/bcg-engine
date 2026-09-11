package dependency

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
)

// ExtractStandaloneExpression returns every concrete PropertyAddress directly
// referenced by expression after structural Aggregate expansion.
//
// The expression is evaluated without an implicit self object. Therefore
// SelfPropertyReference and Aggregate traversal that requires self are
// rejected. AbsolutePropertyReference remains fully supported.
//
// The result is deterministic and sorted by PropertyAddress.
func ExtractStandaloneExpression(
	source Source,
	expression ast.Expression,
) ([]address.PropertyAddress, error) {
	if source == nil {
		panic("dependency: source must not be nil")
	}

	return expressionExtractor{
		source: source,
	}.extract(
		expression,
		extractionContext{},
	)
}
