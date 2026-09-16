package eval

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/evaluator"
)

// Source is the small declaration-plus-evaluation capability used by Rule
// execution.
type Source interface {
	evaluator.Source
	Rule(identity.UUID) (domainrule.Rule, bool)
	Rules() []domainrule.Rule
}
