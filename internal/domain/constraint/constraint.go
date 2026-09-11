package constraint

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
)

// Constraint is an immutable declarative business assertion.
//
// A Constraint never mutates ModelVersion or CalculationContext. It answers
// whether one already-computable model state is acceptable.
//
// applicability is optional. assertion is required.
//
// Constraint expressions are standalone expressions in Core v1. They do not
// implicitly own an Entity/Relation self object. Concrete model values are
// therefore referenced through AbsolutePropertyReference.
type Constraint struct {
	id identity.UUID

	name string

	applicability    ast.Expression
	hasApplicability bool

	assertion ast.Expression

	severity Severity

	message    string
	hasMessage bool
}

// New constructs an immutable Constraint.
//
// Expression reference/type validity is model-dependent and belongs to Draft
// validation rather than this constructor.
func New(
	id identity.UUID,
	name string,
	applicability ast.Expression,
	hasApplicability bool,
	assertion ast.Expression,
	severity Severity,
	message string,
	hasMessage bool,
) Constraint {
	if id.IsZero() {
		panic("constraint: id must not be zero")
	}

	if name == "" {
		panic("constraint: name must not be empty")
	}

	if assertion == nil {
		panic("constraint: assertion must not be nil")
	}

	if !severity.IsValid() {
		panic("constraint: severity is invalid")
	}

	if hasApplicability && applicability == nil {
		panic("constraint: applicability must not be nil when present")
	}

	if !hasApplicability {
		applicability = nil
	}

	if hasMessage {
		if message == "" {
			panic("constraint: message must not be empty when present")
		}
	} else {
		message = ""
	}

	return Constraint{
		id:               id,
		name:             name,
		applicability:    applicability,
		hasApplicability: hasApplicability,
		assertion:        assertion,
		severity:         severity,
		message:          message,
		hasMessage:       hasMessage,
	}
}

func (c Constraint) ID() identity.UUID {
	return c.id
}

func (c Constraint) Name() string {
	return c.name
}

func (c Constraint) Applicability() (ast.Expression, bool) {
	return c.applicability, c.hasApplicability
}

func (c Constraint) Assertion() ast.Expression {
	return c.assertion
}

func (c Constraint) Severity() Severity {
	return c.severity
}

func (c Constraint) Message() (string, bool) {
	return c.message, c.hasMessage
}

// IsValid reports structural validity only.
func (c Constraint) IsValid() bool {
	if c.id.IsZero() ||
		c.name == "" ||
		c.assertion == nil ||
		!c.severity.IsValid() {
		return false
	}

	if c.hasApplicability {
		if c.applicability == nil {
			return false
		}
	} else if c.applicability != nil {
		return false
	}

	if c.hasMessage {
		return c.message != ""
	}

	return c.message == ""
}

func (c Constraint) String() string {
	return fmt.Sprintf(
		"Constraint(%s, %s, %s)",
		c.id.String(),
		c.name,
		c.severity.String(),
	)
}
