package rule

import "github.com/shmatovv/bcg-engine/internal/domain/identity"

// EffectKind identifies one declarative action requested by an active Rule.
//
// Rule effects are descriptions, not mutations. Evaluating a Rule never
// changes ModelVersion or CalculationContext.
type EffectKind uint8

const (
	EffectInvalid EffectKind = iota
	EffectConstraintActivation
)

func (k EffectKind) IsValid() bool {
	return k == EffectConstraintActivation
}

func (k EffectKind) String() string {
	switch k {
	case EffectConstraintActivation:
		return "ConstraintActivation"
	default:
		return "Invalid"
	}
}

// Effect is one immutable declarative RuleEffect.
//
// Core v1 currently has one concrete effect whose target already exists in the
// domain model: activation of a Constraint. The effect itself does not run or
// mutate that Constraint; orchestration may consume active effects explicitly.
//
// Alternative activation can be added as another EffectKind when Alternative
// becomes a first-class model object.
type Effect struct {
	kind EffectKind

	constraintID identity.UUID
}

// ActivateConstraint constructs a declarative Constraint activation effect.
func ActivateConstraint(
	constraintID identity.UUID,
) Effect {
	if constraintID.IsZero() {
		panic("rule: constraint activation target must not be zero")
	}

	return Effect{
		kind:         EffectConstraintActivation,
		constraintID: constraintID,
	}
}

func (e Effect) Kind() EffectKind {
	return e.kind
}

// ConstraintID returns the target Constraint for a ConstraintActivation
// effect.
func (e Effect) ConstraintID() (
	identity.UUID,
	bool,
) {
	if e.kind != EffectConstraintActivation ||
		e.constraintID.IsZero() {
		return identity.UUID{}, false
	}

	return e.constraintID, true
}

func (e Effect) IsValid() bool {
	switch e.kind {
	case EffectConstraintActivation:
		return !e.constraintID.IsZero()
	default:
		return false
	}
}
