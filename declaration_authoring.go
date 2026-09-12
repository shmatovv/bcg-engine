package bcg

import (
	"fmt"
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
)

// IsValid reports whether s is a BCG v1 Constraint severity.
func (s ConstraintSeverity) IsValid() bool {
	return s == SeverityInfo || s == SeverityWarning || s == SeverityError || s == SeverityHard
}

// String returns the stable Constraint severity name.
func (s ConstraintSeverity) String() string {
	if !s.IsValid() {
		return "Invalid"
	}
	return string(s)
}

func (s ConstraintSeverity) internal() (constraint.Severity, error) {
	switch s {
	case SeverityInfo:
		return constraint.SeverityInfo, nil
	case SeverityWarning:
		return constraint.SeverityWarning, nil
	case SeverityError:
		return constraint.SeverityError, nil
	case SeverityHard:
		return constraint.SeverityHard, nil
	default:
		return constraint.SeverityInvalid, fmt.Errorf("%w: constraint severity is invalid", ErrInvalidArgument)
	}
}

// ConstraintSpec describes one immutable declarative Constraint. A zero
// Applicability means always applicable. An empty Message means absent; the
// v1 model language has no meaningful present-but-empty message form.
type ConstraintSpec struct {
	ID            ID
	Name          string
	Applicability Expression
	Assertion     Expression
	Severity      ConstraintSeverity
	Message       string
}

// Constraint is an immutable declarative Boolean assertion.
type Constraint struct{ value constraint.Constraint }

// NewConstraint constructs a Constraint. Boolean typing, standalone reference
// rules, dependencies, and depth are checked authoritatively by Draft.Validate
// and Draft.Commit.
func NewConstraint(spec ConstraintSpec) (Constraint, error) {
	id, err := spec.ID.internal()
	if err != nil {
		return Constraint{}, fmt.Errorf("%w: constraint ID: %v", ErrInvalidArgument, err)
	}
	if spec.Name == "" {
		return Constraint{}, fmt.Errorf("%w: constraint name must not be empty", ErrInvalidArgument)
	}
	if !spec.Assertion.IsValid() {
		return Constraint{}, fmt.Errorf("%w: constraint assertion is invalid", ErrInvalidArgument)
	}
	severity, err := spec.Severity.internal()
	if err != nil {
		return Constraint{}, err
	}
	if spec.Applicability.value == nil {
		return Constraint{value: constraint.New(id, spec.Name, nil, false, spec.Assertion.value, severity, spec.Message, spec.Message != "")}, nil
	}
	if !spec.Applicability.IsValid() {
		return Constraint{}, fmt.Errorf("%w: constraint applicability is invalid", ErrInvalidArgument)
	}
	return Constraint{value: constraint.New(id, spec.Name, spec.Applicability.value, true, spec.Assertion.value, severity, spec.Message, spec.Message != "")}, nil
}

func constraintFromInternal(value constraint.Constraint) Constraint { return Constraint{value: value} }

// ID returns the Constraint identity.
func (c Constraint) ID() ID { return idFromInternal(c.value.ID()) }

// Name returns the presentation name.
func (c Constraint) Name() string { return c.value.Name() }

// Applicability returns the optional Boolean applicability expression.
func (c Constraint) Applicability() (Expression, bool) {
	expression, ok := c.value.Applicability()
	if !ok {
		return Expression{}, false
	}
	return expressionFromInternal(expression), true
}

// Assertion returns the required Boolean assertion expression.
func (c Constraint) Assertion() Expression { return expressionFromInternal(c.value.Assertion()) }

// Severity returns the declared business severity.
func (c Constraint) Severity() ConstraintSeverity { return severityFromInternal(c.value.Severity()) }

// Message returns the optional diagnostic message.
func (c Constraint) Message() (string, bool) { return c.value.Message() }

// IsValid reports structural Constraint validity.
func (c Constraint) IsValid() bool { return c.value.IsValid() }

// AddConstraint adds a Constraint without evaluating cross-model semantics.
func (d *Draft) AddConstraint(value Constraint) error {
	if !d.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft and Constraint are required", ErrInvalidArgument)
	}
	d.value.AddConstraint(value.value)
	return nil
}

// ReplaceConstraint replaces the Constraint with id while retaining identity.
func (d *Draft) ReplaceConstraint(id ID, value Constraint) error {
	if !d.IsValid() || id.IsZero() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft, Constraint ID, and Constraint are required", ErrInvalidArgument)
	}
	if !value.ID().Equal(id) {
		return fmt.Errorf("%w: replacement Constraint must retain its ID", ErrInvalidArgument)
	}
	if !d.value.ReplaceConstraint(id.value, value.value) {
		return fmt.Errorf("%w: Constraint %s was not found", ErrInvalidArgument, id)
	}
	return nil
}

// RemoveConstraint removes one Constraint without cascading to other model data.
func (d *Draft) RemoveConstraint(id ID) error {
	if !d.IsValid() || id.IsZero() {
		return fmt.Errorf("%w: valid Draft and Constraint ID are required", ErrInvalidArgument)
	}
	if !d.value.RemoveConstraint(id.value) {
		return fmt.Errorf("%w: Constraint %s was not found", ErrInvalidArgument, id)
	}
	return nil
}

// Constraints returns owned deterministic Constraint declarations in d.
func (d *Draft) Constraints() []Constraint {
	if !d.IsValid() {
		return nil
	}
	items := d.value.Constraints()
	result := make([]Constraint, len(items))
	for index, item := range items {
		result[index] = constraintFromInternal(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return result
}

// RuleSpec describes one immutable side-effect-free declarative Rule.
type RuleSpec struct {
	ID        ID
	Name      string
	Condition Expression
	Metadata  map[string]string
}

// Rule is an immutable declarative Boolean decision. Rules have no actions,
// effects, priority, salience, or rule chaining semantics in BCG v1.
type Rule struct{ value domainrule.Rule }

// NewRule constructs a Rule. Condition typing and standalone reference rules
// are checked by the existing Draft validation pipeline.
func NewRule(spec RuleSpec) (Rule, error) {
	id, err := spec.ID.internal()
	if err != nil {
		return Rule{}, fmt.Errorf("%w: rule ID: %v", ErrInvalidArgument, err)
	}
	if spec.Name == "" {
		return Rule{}, fmt.Errorf("%w: rule name must not be empty", ErrInvalidArgument)
	}
	if !spec.Condition.IsValid() {
		return Rule{}, fmt.Errorf("%w: rule condition is invalid", ErrInvalidArgument)
	}
	metadata := make(map[string]string, len(spec.Metadata))
	for key, value := range spec.Metadata {
		if key == "" {
			return Rule{}, fmt.Errorf("%w: rule metadata key must not be empty", ErrInvalidArgument)
		}
		metadata[key] = value
	}
	return Rule{value: domainrule.NewWithMetadata(id, spec.Name, spec.Condition.value, metadata)}, nil
}

func ruleFromInternal(value domainrule.Rule) Rule { return Rule{value: value} }

// ID returns the Rule identity.
func (r Rule) ID() ID { return idFromInternal(r.value.ID()) }

// Name returns the presentation name.
func (r Rule) Name() string { return r.value.Name() }

// Condition returns the required Boolean condition expression.
func (r Rule) Condition() Expression { return expressionFromInternal(r.value.Condition()) }

// Metadata returns an owned copy of the serializable string metadata.
func (r Rule) Metadata() map[string]string { return r.value.Metadata() }

// IsValid reports structural Rule validity.
func (r Rule) IsValid() bool { return r.value.IsValid() }

// AddRule adds a Rule without evaluating cross-model semantics.
func (d *Draft) AddRule(value Rule) error {
	if !d.IsValid() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft and Rule are required", ErrInvalidArgument)
	}
	d.value.AddRule(value.value)
	return nil
}

// ReplaceRule replaces the Rule with id while retaining identity.
func (d *Draft) ReplaceRule(id ID, value Rule) error {
	if !d.IsValid() || id.IsZero() || !value.IsValid() {
		return fmt.Errorf("%w: valid Draft, Rule ID, and Rule are required", ErrInvalidArgument)
	}
	if !value.ID().Equal(id) {
		return fmt.Errorf("%w: replacement Rule must retain its ID", ErrInvalidArgument)
	}
	if !d.value.ReplaceRule(id.value, value.value) {
		return fmt.Errorf("%w: Rule %s was not found", ErrInvalidArgument, id)
	}
	return nil
}

// RemoveRule removes one Rule without cascading to other model data.
func (d *Draft) RemoveRule(id ID) error {
	if !d.IsValid() || id.IsZero() {
		return fmt.Errorf("%w: valid Draft and Rule ID are required", ErrInvalidArgument)
	}
	if !d.value.RemoveRule(id.value) {
		return fmt.Errorf("%w: Rule %s was not found", ErrInvalidArgument, id)
	}
	return nil
}

// Rules returns owned deterministic Rule declarations in d.
func (d *Draft) Rules() []Rule {
	if !d.IsValid() {
		return nil
	}
	items := d.value.Rules()
	result := make([]Rule, len(items))
	for index, item := range items {
		result[index] = ruleFromInternal(item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return result
}
