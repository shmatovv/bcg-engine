package ast

// UnaryOperator identifies a pure v2 unary expression operation.
type UnaryOperator uint8

const (
	UnaryOperatorInvalid UnaryOperator = iota
	UnaryOperatorIsDefined
	UnaryOperatorIsUndefined
	UnaryOperatorAbs
	UnaryOperatorRound
	UnaryOperatorFloor
	UnaryOperatorCeil
	UnaryOperatorLength
	UnaryOperatorLower
	UnaryOperatorUpper
	UnaryOperatorTrim
	UnaryOperatorYear
	UnaryOperatorMonth
	UnaryOperatorDay
	UnaryOperatorHour
	UnaryOperatorMinute
)

func (o UnaryOperator) IsValid() bool { return o > UnaryOperatorInvalid && o <= UnaryOperatorMinute }
func (o UnaryOperator) String() string {
	return [...]string{"Invalid", "IsDefined", "IsUndefined", "Abs", "Round", "Floor", "Ceil", "Length", "Lower", "Upper", "Trim", "Year", "Month", "Day", "Hour", "Minute"}[o]
}

type Unary struct {
	operator UnaryOperator
	operand  Expression
}

func NewUnary(operator UnaryOperator, operand Expression) Unary {
	if !operator.IsValid() || operand == nil {
		panic("ast: invalid unary operation")
	}
	return Unary{operator, operand}
}
func (u Unary) Operator() UnaryOperator { return u.operator }
func (u Unary) Operand() Expression     { return u.operand }
func (u Unary) IsValid() bool           { return u.operator.IsValid() && u.operand != nil }
func (Unary) expressionNode()           {}

type BinaryOperator uint8

const (
	BinaryOperatorInvalid BinaryOperator = iota
	BinaryOperatorModulo
	BinaryOperatorContains
	BinaryOperatorStartsWith
	BinaryOperatorEndsWith
	BinaryOperatorAddDuration
	BinaryOperatorSubtractDuration
	BinaryOperatorDifference
)

func (o BinaryOperator) IsValid() bool {
	return o > BinaryOperatorInvalid && o <= BinaryOperatorDifference
}
func (o BinaryOperator) String() string {
	return [...]string{"Invalid", "Modulo", "Contains", "StartsWith", "EndsWith", "AddDuration", "SubtractDuration", "Difference"}[o]
}

type Binary struct {
	operator    BinaryOperator
	left, right Expression
}

func NewBinary(operator BinaryOperator, left, right Expression) Binary {
	if !operator.IsValid() || left == nil || right == nil {
		panic("ast: invalid binary operation")
	}
	return Binary{operator, left, right}
}
func (b Binary) Operator() BinaryOperator { return b.operator }
func (b Binary) Left() Expression         { return b.left }
func (b Binary) Right() Expression        { return b.right }
func (b Binary) IsValid() bool            { return b.operator.IsValid() && b.left != nil && b.right != nil }
func (Binary) expressionNode()            {}

type VariadicOperator uint8

const (
	VariadicOperatorInvalid VariadicOperator = iota
	VariadicOperatorConcat
	VariadicOperatorScalarMin
	VariadicOperatorScalarMax
)

func (o VariadicOperator) IsValid() bool {
	return o > VariadicOperatorInvalid && o <= VariadicOperatorScalarMax
}
func (o VariadicOperator) String() string {
	return [...]string{"Invalid", "Concat", "ScalarMin", "ScalarMax"}[o]
}

type Variadic struct {
	operator VariadicOperator
	operands []Expression
}

func NewVariadic(operator VariadicOperator, operands []Expression) Variadic {
	if !operator.IsValid() || len(operands) == 0 {
		panic("ast: invalid variadic operation")
	}
	copy := append([]Expression(nil), operands...)
	for _, v := range copy {
		if v == nil {
			panic("ast: nil variadic operand")
		}
	}
	return Variadic{operator, copy}
}
func (v Variadic) Operator() VariadicOperator { return v.operator }
func (v Variadic) Operands() []Expression     { return append([]Expression(nil), v.operands...) }
func (v Variadic) IsValid() bool {
	if !v.operator.IsValid() || len(v.operands) == 0 {
		return false
	}
	for _, x := range v.operands {
		if x == nil {
			return false
		}
	}
	return true
}
func (Variadic) expressionNode() {}
