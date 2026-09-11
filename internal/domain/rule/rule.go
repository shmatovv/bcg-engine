package rule

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
)

// Rule is an immutable declarative side-effect-free business rule.
//
// Rule condition is a standalone Boolean expression. It does not implicitly
// own an Entity or Relation, so concrete model values are referenced through
// AbsolutePropertyReference.
//
// Rule evaluation determines Active / Inactive / Undefined and exposes the
// declared Effect. Evaluation never mutates model data.
type Rule struct {
	id identity.UUID

	name string

	condition ast.Expression
	effect    Effect
}

func New(
	id identity.UUID,
	name string,
	condition ast.Expression,
	effect Effect,
) Rule {
	if id.IsZero() {
		panic("rule: id must not be zero")
	}

	if name == "" {
		panic("rule: name must not be empty")
	}

	if condition == nil {
		panic("rule: condition must not be nil")
	}

	if !effect.IsValid() {
		panic("rule: effect is invalid")
	}

	return Rule{
		id:        id,
		name:      name,
		condition: condition,
		effect:    effect,
	}
}

func (r Rule) ID() identity.UUID {
	return r.id
}

func (r Rule) Name() string {
	return r.name
}

func (r Rule) Condition() ast.Expression {
	return r.condition
}

func (r Rule) Effect() Effect {
	return r.effect
}

// IsValid reports structural validity only. Expression typing/reference and
// effect-target existence are ModelVersion-dependent validation concerns.
func (r Rule) IsValid() bool {
	return !r.id.IsZero() &&
		r.name != "" &&
		r.condition != nil &&
		r.effect.IsValid()
}

func (r Rule) String() string {
	return fmt.Sprintf(
		"Rule(%s, %s, %s)",
		r.id.String(),
		r.name,
		r.effect.Kind().String(),
	)
}
