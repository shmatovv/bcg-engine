package check

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
)

// Source is the small declaration-plus-evaluation capability used by
// Constraint execution.
type Source interface {
	evaluator.Source
	Constraint(identity.UUID) (constraint.Constraint, bool)
	Constraints() []constraint.Constraint
}
