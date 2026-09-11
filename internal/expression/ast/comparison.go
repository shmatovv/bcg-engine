package ast

// ComparisonOperator identifies a binary comparison operation.
type ComparisonOperator uint8

const (
	ComparisonInvalid ComparisonOperator = iota
	ComparisonEqual
	ComparisonNotEqual
	ComparisonLess
	ComparisonLessOrEqual
	ComparisonGreater
	ComparisonGreaterOrEqual
)

// IsValid reports whether o is a valid comparison operator.
func (o ComparisonOperator) IsValid() bool {
	switch o {
	case ComparisonEqual,
		ComparisonNotEqual,
		ComparisonLess,
		ComparisonLessOrEqual,
		ComparisonGreater,
		ComparisonGreaterOrEqual:
		return true
	default:
		return false
	}
}

// String returns the canonical name of o.
func (o ComparisonOperator) String() string {
	switch o {
	case ComparisonEqual:
		return "Equal"
	case ComparisonNotEqual:
		return "NotEqual"
	case ComparisonLess:
		return "Less"
	case ComparisonLessOrEqual:
		return "LessOrEqual"
	case ComparisonGreater:
		return "Greater"
	case ComparisonGreaterOrEqual:
		return "GreaterOrEqual"
	default:
		return "Invalid"
	}
}

// Comparison represents a binary comparison expression.
//
// A valid comparison expression always has Boolean as its resulting
// ValueType. Operand compatibility is checked by static validation.
type Comparison struct {
	operator ComparisonOperator
	left     Expression
	right    Expression
}

// NewComparison constructs a binary comparison expression.
func NewComparison(
	operator ComparisonOperator,
	left Expression,
	right Expression,
) Comparison {
	if !operator.IsValid() {
		panic("ast: comparison operator must be valid")
	}

	if left == nil {
		panic("ast: comparison left operand must not be nil")
	}

	if right == nil {
		panic("ast: comparison right operand must not be nil")
	}

	return Comparison{
		operator: operator,
		left:     left,
		right:    right,
	}
}

// Equal constructs an equality comparison.
func Equal(left, right Expression) Comparison {
	return NewComparison(
		ComparisonEqual,
		left,
		right,
	)
}

// NotEqual constructs an inequality comparison.
func NotEqual(left, right Expression) Comparison {
	return NewComparison(
		ComparisonNotEqual,
		left,
		right,
	)
}

// Less constructs a less-than comparison.
func Less(left, right Expression) Comparison {
	return NewComparison(
		ComparisonLess,
		left,
		right,
	)
}

// LessOrEqual constructs a less-than-or-equal comparison.
func LessOrEqual(left, right Expression) Comparison {
	return NewComparison(
		ComparisonLessOrEqual,
		left,
		right,
	)
}

// Greater constructs a greater-than comparison.
func Greater(left, right Expression) Comparison {
	return NewComparison(
		ComparisonGreater,
		left,
		right,
	)
}

// GreaterOrEqual constructs a greater-than-or-equal comparison.
func GreaterOrEqual(left, right Expression) Comparison {
	return NewComparison(
		ComparisonGreaterOrEqual,
		left,
		right,
	)
}

// Operator returns the comparison operation.
func (e Comparison) Operator() ComparisonOperator {
	return e.operator
}

// Left returns the left operand.
func (e Comparison) Left() Expression {
	return e.left
}

// Right returns the right operand.
func (e Comparison) Right() Expression {
	return e.right
}

// IsValid reports whether e is structurally valid.
func (e Comparison) IsValid() bool {
	return e.operator.IsValid() &&
		e.left != nil &&
		e.right != nil
}

func (Comparison) expressionNode() {}
