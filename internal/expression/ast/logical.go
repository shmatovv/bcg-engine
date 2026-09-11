package ast

// LogicalOperator identifies a binary logical operation.
type LogicalOperator uint8

const (
	LogicalInvalid LogicalOperator = iota
	LogicalAnd
	LogicalOr
)

// IsValid reports whether o is a valid logical operator.
func (o LogicalOperator) IsValid() bool {
	switch o {
	case LogicalAnd, LogicalOr:
		return true
	default:
		return false
	}
}

// String returns the canonical name of o.
func (o LogicalOperator) String() string {
	switch o {
	case LogicalAnd:
		return "And"
	case LogicalOr:
		return "Or"
	default:
		return "Invalid"
	}
}

// Logical represents a binary logical expression.
//
// Both operands must statically resolve to Boolean.
// This requirement is checked by static expression validation.
type Logical struct {
	operator LogicalOperator
	left     Expression
	right    Expression
}

// NewLogical constructs a binary logical expression.
func NewLogical(
	operator LogicalOperator,
	left Expression,
	right Expression,
) Logical {
	if !operator.IsValid() {
		panic("ast: logical operator must be valid")
	}

	if left == nil {
		panic("ast: logical left operand must not be nil")
	}

	if right == nil {
		panic("ast: logical right operand must not be nil")
	}

	return Logical{
		operator: operator,
		left:     left,
		right:    right,
	}
}

// And constructs a logical AND expression.
func And(left, right Expression) Logical {
	return NewLogical(
		LogicalAnd,
		left,
		right,
	)
}

// Or constructs a logical OR expression.
func Or(left, right Expression) Logical {
	return NewLogical(
		LogicalOr,
		left,
		right,
	)
}

// Operator returns the logical operation.
func (e Logical) Operator() LogicalOperator {
	return e.operator
}

// Left returns the left operand.
func (e Logical) Left() Expression {
	return e.left
}

// Right returns the right operand.
func (e Logical) Right() Expression {
	return e.right
}

// IsValid reports whether e is structurally valid.
func (e Logical) IsValid() bool {
	return e.operator.IsValid() &&
		e.left != nil &&
		e.right != nil
}

func (Logical) expressionNode() {}

// Not represents logical negation.
//
// The operand must statically resolve to Boolean.
type Not struct {
	operand Expression
}

// NewNot constructs a logical NOT expression.
func NewNot(operand Expression) Not {
	if operand == nil {
		panic("ast: not operand must not be nil")
	}

	return Not{
		operand: operand,
	}
}

// Operand returns the expression being negated.
func (e Not) Operand() Expression {
	return e.operand
}

// IsValid reports whether e is structurally valid.
func (e Not) IsValid() bool {
	return e.operand != nil
}

func (Not) expressionNode() {}
