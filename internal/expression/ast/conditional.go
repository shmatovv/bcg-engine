package ast

// If represents a conditional expression.
//
// Semantically:
//
//	if condition then thenExpr else elseExpr
//
// condition must statically resolve to Boolean.
// thenExpr and elseExpr must have compatible types and units.
// These requirements are checked by static expression validation.
type If struct {
	condition Expression
	thenExpr  Expression
	elseExpr  Expression
}

// NewIf constructs a conditional expression.
func NewIf(
	condition Expression,
	thenExpr Expression,
	elseExpr Expression,
) If {
	if condition == nil {
		panic("ast: if condition must not be nil")
	}

	if thenExpr == nil {
		panic("ast: if then expression must not be nil")
	}

	if elseExpr == nil {
		panic("ast: if else expression must not be nil")
	}

	return If{
		condition: condition,
		thenExpr:  thenExpr,
		elseExpr:  elseExpr,
	}
}

// Condition returns the condition expression.
func (e If) Condition() Expression {
	return e.condition
}

// Then returns the expression evaluated when the condition is true.
func (e If) Then() Expression {
	return e.thenExpr
}

// Else returns the expression evaluated when the condition is false.
func (e If) Else() Expression {
	return e.elseExpr
}

// IsValid reports whether e is structurally valid.
func (e If) IsValid() bool {
	return e.condition != nil &&
		e.thenExpr != nil &&
		e.elseExpr != nil
}

func (If) expressionNode() {}

// IsNull tests whether an expression resolves to explicit Null.
//
// The result of IsNull has Boolean ValueType.
//
// Null and Undefined are distinct states. IsNull tests only for Null;
// Undefined propagation is handled by evaluation semantics.
type IsNull struct {
	operand Expression
}

// NewIsNull constructs an explicit-null test.
func NewIsNull(operand Expression) IsNull {
	if operand == nil {
		panic("ast: is-null operand must not be nil")
	}

	return IsNull{
		operand: operand,
	}
}

// Operand returns the expression being tested.
func (e IsNull) Operand() Expression {
	return e.operand
}

// IsValid reports whether e is structurally valid.
func (e IsNull) IsValid() bool {
	return e.operand != nil
}

func (IsNull) expressionNode() {}

// Coalesce returns its fallback expression when the primary expression
// resolves to explicit Null.
//
// The primary and fallback expressions must have compatible types and
// units. Compatibility is checked by static expression validation.
//
// Undefined is distinct from Null and is not silently converted to Null.
type Coalesce struct {
	primary  Expression
	fallback Expression
}

// NewCoalesce constructs a coalesce expression.
func NewCoalesce(
	primary Expression,
	fallback Expression,
) Coalesce {
	if primary == nil {
		panic("ast: coalesce primary expression must not be nil")
	}

	if fallback == nil {
		panic("ast: coalesce fallback expression must not be nil")
	}

	return Coalesce{
		primary:  primary,
		fallback: fallback,
	}
}

// Primary returns the primary expression.
func (e Coalesce) Primary() Expression {
	return e.primary
}

// Fallback returns the fallback expression.
func (e Coalesce) Fallback() Expression {
	return e.fallback
}

// IsValid reports whether e is structurally valid.
func (e Coalesce) IsValid() bool {
	return e.primary != nil &&
		e.fallback != nil
}

func (Coalesce) expressionNode() {}
