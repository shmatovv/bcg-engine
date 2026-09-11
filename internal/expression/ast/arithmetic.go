package ast

// ArithmeticOperator identifies a binary arithmetic operation.
type ArithmeticOperator uint8

const (
	ArithmeticInvalid ArithmeticOperator = iota
	ArithmeticAdd
	ArithmeticSubtract
	ArithmeticMultiply
	ArithmeticDivide
)

// IsValid reports whether o is a valid arithmetic operator.
func (o ArithmeticOperator) IsValid() bool {
	switch o {
	case ArithmeticAdd,
		ArithmeticSubtract,
		ArithmeticMultiply,
		ArithmeticDivide:
		return true
	default:
		return false
	}
}

// String returns the canonical name of o.
func (o ArithmeticOperator) String() string {
	switch o {
	case ArithmeticAdd:
		return "Add"
	case ArithmeticSubtract:
		return "Subtract"
	case ArithmeticMultiply:
		return "Multiply"
	case ArithmeticDivide:
		return "Divide"
	default:
		return "Invalid"
	}
}

// Arithmetic represents a binary arithmetic expression.
//
// Examples:
//
//	self.price + self.tax
//	self.length * self.width
//	self.total / self.quantity
//
// Type and unit compatibility are not checked by the AST itself.
// They are responsibilities of static expression validation.
type Arithmetic struct {
	operator ArithmeticOperator
	left     Expression
	right    Expression
}

// NewArithmetic constructs a binary arithmetic expression.
func NewArithmetic(
	operator ArithmeticOperator,
	left Expression,
	right Expression,
) Arithmetic {
	if !operator.IsValid() {
		panic("ast: arithmetic operator must be valid")
	}

	if left == nil {
		panic("ast: arithmetic left operand must not be nil")
	}

	if right == nil {
		panic("ast: arithmetic right operand must not be nil")
	}

	return Arithmetic{
		operator: operator,
		left:     left,
		right:    right,
	}
}

// Add constructs an addition expression.
func Add(left, right Expression) Arithmetic {
	return NewArithmetic(
		ArithmeticAdd,
		left,
		right,
	)
}

// Subtract constructs a subtraction expression.
func Subtract(left, right Expression) Arithmetic {
	return NewArithmetic(
		ArithmeticSubtract,
		left,
		right,
	)
}

// Multiply constructs a multiplication expression.
func Multiply(left, right Expression) Arithmetic {
	return NewArithmetic(
		ArithmeticMultiply,
		left,
		right,
	)
}

// Divide constructs a division expression.
func Divide(left, right Expression) Arithmetic {
	return NewArithmetic(
		ArithmeticDivide,
		left,
		right,
	)
}

// Operator returns the arithmetic operation.
func (e Arithmetic) Operator() ArithmeticOperator {
	return e.operator
}

// Left returns the left operand.
func (e Arithmetic) Left() Expression {
	return e.left
}

// Right returns the right operand.
func (e Arithmetic) Right() Expression {
	return e.right
}

// IsValid reports whether e is structurally valid.
func (e Arithmetic) IsValid() bool {
	return e.operator.IsValid() &&
		e.left != nil &&
		e.right != nil
}

func (Arithmetic) expressionNode() {}

// Negate represents unary arithmetic negation.
//
// Example:
//
//	-self.balance
type Negate struct {
	operand Expression
}

// NewNegate constructs an arithmetic negation expression.
func NewNegate(operand Expression) Negate {
	if operand == nil {
		panic("ast: negate operand must not be nil")
	}

	return Negate{
		operand: operand,
	}
}

// Operand returns the expression being negated.
func (e Negate) Operand() Expression {
	return e.operand
}

// IsValid reports whether e is structurally valid.
func (e Negate) IsValid() bool {
	return e.operand != nil
}

func (Negate) expressionNode() {}
