package rule

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
)

// Rule is an immutable, named declarative Boolean decision.
//
// Rule condition is a standalone Boolean expression. It has no implicit
// Entity or Relation owner, so direct concrete property reads use
// AbsolutePropertyReference. Normal evaluator semantics still govern any
// other expression form that is valid without an implicit self context.
//
// Rule evaluation determines Active / Inactive / Undefined. A Rule has no
// assignments, effects, priority, or execution semantics; evaluation never
// mutates model data.
type Rule struct {
	id identity.UUID

	name string

	condition ast.Expression

	metadata map[string]string
}

func New(
	id identity.UUID,
	name string,
	condition ast.Expression,
) Rule {
	return NewWithMetadata(id, name, condition, nil)
}

// NewWithMetadata constructs a Rule with serializable string metadata.
// Metadata is copied on input and output, so it cannot provide a mutable
// backdoor into a committed ModelVersion.
func NewWithMetadata(
	id identity.UUID,
	name string,
	condition ast.Expression,
	metadata map[string]string,
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

	for key := range metadata {
		if key == "" {
			panic("rule: metadata key must not be empty")
		}
	}

	return Rule{
		id:        id,
		name:      name,
		condition: condition,
		metadata:  cloneMetadata(metadata),
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

// Metadata returns an owned copy of Rule metadata.
func (r Rule) Metadata() map[string]string {
	return cloneMetadata(r.metadata)
}

// IsValid reports structural validity only. Expression typing/reference and
// reference validation are ModelVersion-dependent validation concerns.
func (r Rule) IsValid() bool {
	if r.id.IsZero() || r.name == "" || r.condition == nil {
		return false
	}
	for key := range r.metadata {
		if key == "" {
			return false
		}
	}
	return true
}

func (r Rule) String() string {
	return fmt.Sprintf(
		"Rule(%s, %s)",
		r.id.String(),
		r.name,
	)
}

func cloneMetadata(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, item := range source {
		result[key] = item
	}
	return result
}
