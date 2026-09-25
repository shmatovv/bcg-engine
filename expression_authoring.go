package bcg

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
)

// MaxExpressionDepth is the maximum nesting accepted by BCG v1. Builders may
// assemble a deeper Draft expression, but Validate and Commit reject it using
// the same authoritative limit as decoding and evaluation.
const MaxExpressionDepth = ast.MaxExpressionDepth

// Expression is an immutable, opaque declarative expression. Construct it
// with the typed builders in this package. Its zero value is invalid.
//
// Expression has value semantics and can safely be copied into a
// PropertyDefinitionSpec or retained while editing a Draft. It never exposes
// internal AST nodes or mutable children.
type Expression struct{ value ast.Expression }

// ExpressionKind identifies the concrete immutable AST node held by an
// Expression. The zero value is invalid.
type ExpressionKind uint16

const (
	ExpressionConstant ExpressionKind = iota + 1
	ExpressionSelfPropertyReference
	ExpressionAbsolutePropertyReference
	ExpressionBoundPropertyReference
	ExpressionBoundContractPropertyReference
	ExpressionArithmetic
	ExpressionNegate
	ExpressionComparison
	ExpressionLogical
	ExpressionNot
	ExpressionIf
	ExpressionIsNull
	ExpressionCoalesce
	ExpressionAggregate
	ExpressionUnaryV2
	ExpressionBinaryV2
	ExpressionVariadicV2
	ExpressionFunctionCall
)

// IsValid reports whether k identifies a supported v1 expression node.
func (k ExpressionKind) IsValid() bool {
	return k >= ExpressionConstant && k <= ExpressionFunctionCall
}

// String returns the stable node name, or "Invalid".
func (k ExpressionKind) String() string {
	switch k {
	case ExpressionConstant:
		return "Constant"
	case ExpressionSelfPropertyReference:
		return "SelfPropertyReference"
	case ExpressionAbsolutePropertyReference:
		return "AbsolutePropertyReference"
	case ExpressionBoundPropertyReference:
		return "BoundPropertyReference"
	case ExpressionBoundContractPropertyReference:
		return "BoundContractPropertyReference"
	case ExpressionArithmetic:
		return "Arithmetic"
	case ExpressionNegate:
		return "Negate"
	case ExpressionComparison:
		return "Comparison"
	case ExpressionLogical:
		return "Logical"
	case ExpressionNot:
		return "Not"
	case ExpressionIf:
		return "If"
	case ExpressionIsNull:
		return "IsNull"
	case ExpressionCoalesce:
		return "Coalesce"
	case ExpressionAggregate:
		return "Aggregate"
	case ExpressionUnaryV2:
		return "Unary"
	case ExpressionBinaryV2:
		return "Binary"
	case ExpressionVariadicV2:
		return "Variadic"
	case ExpressionFunctionCall:
		return "FunctionCall"
	default:
		return "Invalid"
	}
}

// Kind reports e's concrete AST node kind, or an invalid value for a zero or
// malformed expression.
func (e Expression) Kind() ExpressionKind {
	if !e.IsValid() {
		return 0
	}
	switch e.value.(type) {
	case ast.Constant:
		return ExpressionConstant
	case ast.SelfPropertyReference:
		return ExpressionSelfPropertyReference
	case ast.AbsolutePropertyReference:
		return ExpressionAbsolutePropertyReference
	case ast.BoundPropertyDefinitionReference:
		return ExpressionBoundPropertyReference
	case ast.BoundContractPropertyReference:
		return ExpressionBoundContractPropertyReference
	case ast.Arithmetic:
		return ExpressionArithmetic
	case ast.Negate:
		return ExpressionNegate
	case ast.Comparison:
		return ExpressionComparison
	case ast.Logical:
		return ExpressionLogical
	case ast.Not:
		return ExpressionNot
	case ast.If:
		return ExpressionIf
	case ast.IsNull:
		return ExpressionIsNull
	case ast.Coalesce:
		return ExpressionCoalesce
	case ast.Aggregate:
		return ExpressionAggregate
	case ast.Unary:
		return ExpressionUnaryV2
	case ast.Binary:
		return ExpressionBinaryV2
	case ast.Variadic:
		return ExpressionVariadicV2
	case ast.FunctionCall:
		return ExpressionFunctionCall
	default:
		return 0
	}
}

// Children returns e's direct expression children in deterministic semantic
// order. Aggregate children are filter (when present), then map (when
// present). The returned slice is owned by the caller.
func (e Expression) Children() []Expression {
	if !e.IsValid() {
		return nil
	}
	var nodes []ast.Expression
	switch node := e.value.(type) {
	case ast.Arithmetic:
		nodes = []ast.Expression{node.Left(), node.Right()}
	case ast.Negate:
		nodes = []ast.Expression{node.Operand()}
	case ast.Comparison:
		nodes = []ast.Expression{node.Left(), node.Right()}
	case ast.Logical:
		nodes = []ast.Expression{node.Left(), node.Right()}
	case ast.Not:
		nodes = []ast.Expression{node.Operand()}
	case ast.If:
		nodes = []ast.Expression{node.Condition(), node.Then(), node.Else()}
	case ast.IsNull:
		nodes = []ast.Expression{node.Operand()}
	case ast.Coalesce:
		nodes = []ast.Expression{node.Primary(), node.Fallback()}
	case ast.Aggregate:
		if filter, ok := node.Filter(); ok {
			nodes = append(nodes, filter)
		}
		if mapped, ok := node.Map(); ok {
			nodes = append(nodes, mapped)
		}
	case ast.Unary:
		nodes = []ast.Expression{node.Operand()}
	case ast.Binary:
		nodes = []ast.Expression{node.Left(), node.Right()}
	case ast.Variadic:
		nodes = node.Operands()
	case ast.FunctionCall:
		nodes = node.Arguments()
	}
	result := make([]Expression, len(nodes))
	for i, node := range nodes {
		result[i] = expressionFromInternal(node)
	}
	return result
}

// ConstantValue returns the literal value for a constant expression.
func (e Expression) ConstantValue() (Value, bool) {
	node, ok := e.value.(ast.Constant)
	if !ok || !e.IsValid() {
		return Value{}, false
	}
	return valueFromInternal(node.Value()), true
}

// PropertyReferenceKind identifies a property-reference node shape.
type PropertyReferenceKind uint8

const (
	PropertyReferenceSelf PropertyReferenceKind = iota + 1
	PropertyReferenceAbsolute
	PropertyReferenceBoundDefinition
	PropertyReferenceBoundContract
)

// PropertyReference is the typed, immutable description of a property
// reference. Exactly the accessor documented for its Kind returns data.
type PropertyReference struct {
	kind       PropertyReferenceKind
	definition ID
	contract   ID
	address    PropertyAddress
	binding    AggregateBinding
}

func (r PropertyReference) Kind() PropertyReferenceKind { return r.kind }
func (r PropertyReference) DefinitionID() (ID, bool) {
	return r.definition, r.kind == PropertyReferenceSelf || r.kind == PropertyReferenceBoundDefinition
}
func (r PropertyReference) ContractID() (ID, bool) {
	return r.contract, r.kind == PropertyReferenceBoundContract
}
func (r PropertyReference) Address() (PropertyAddress, bool) {
	return r.address, r.kind == PropertyReferenceAbsolute
}
func (r PropertyReference) Binding() (AggregateBinding, bool) {
	return r.binding, r.kind == PropertyReferenceBoundDefinition || r.kind == PropertyReferenceBoundContract
}

// PropertyReference returns e's typed property-reference data.
func (e Expression) PropertyReference() (PropertyReference, bool) {
	if !e.IsValid() {
		return PropertyReference{}, false
	}
	switch node := e.value.(type) {
	case ast.SelfPropertyReference:
		return PropertyReference{kind: PropertyReferenceSelf, definition: idFromInternal(node.DefinitionID())}, true
	case ast.AbsolutePropertyReference:
		return PropertyReference{kind: PropertyReferenceAbsolute, address: addressFromInternal(node.Address())}, true
	case ast.BoundPropertyDefinitionReference:
		return PropertyReference{kind: PropertyReferenceBoundDefinition, definition: idFromInternal(node.DefinitionID()), binding: aggregateBindingFromInternal(node.Target())}, true
	case ast.BoundContractPropertyReference:
		return PropertyReference{kind: PropertyReferenceBoundContract, contract: idFromInternal(node.ContractID()), binding: aggregateBindingFromInternal(node.Target())}, true
	default:
		return PropertyReference{}, false
	}
}

// UnaryOperationKind identifies a v1 unary operation.
type UnaryOperationKind uint8

const (
	UnaryNegate UnaryOperationKind = iota + 1
	UnaryNot
	UnaryIsNull
)

// UnaryExpression describes one immutable unary operation.
type UnaryExpression struct {
	kind    UnaryOperationKind
	operand Expression
}

func (o UnaryExpression) Kind() UnaryOperationKind { return o.kind }
func (o UnaryExpression) Operand() Expression      { return o.operand }

// UnaryOperation returns e's typed unary operation data.
func (e Expression) UnaryOperation() (UnaryExpression, bool) {
	if !e.IsValid() {
		return UnaryExpression{}, false
	}
	switch node := e.value.(type) {
	case ast.Negate:
		return UnaryExpression{kind: UnaryNegate, operand: expressionFromInternal(node.Operand())}, true
	case ast.Not:
		return UnaryExpression{kind: UnaryNot, operand: expressionFromInternal(node.Operand())}, true
	case ast.IsNull:
		return UnaryExpression{kind: UnaryIsNull, operand: expressionFromInternal(node.Operand())}, true
	default:
		return UnaryExpression{}, false
	}
}

// BinaryOperationKind identifies a v1 arithmetic, comparison, or logical
// binary operation.
type BinaryOperationKind uint8

const (
	BinaryAdd BinaryOperationKind = iota + 1
	BinarySubtract
	BinaryMultiply
	BinaryDivide
	BinaryEqual
	BinaryNotEqual
	BinaryLess
	BinaryLessOrEqual
	BinaryGreater
	BinaryGreaterOrEqual
	BinaryAnd
	BinaryOr
)

// BinaryExpression describes one immutable binary operation.
type BinaryExpression struct {
	kind        BinaryOperationKind
	left, right Expression
}

func (o BinaryExpression) Kind() BinaryOperationKind { return o.kind }
func (o BinaryExpression) Left() Expression          { return o.left }
func (o BinaryExpression) Right() Expression         { return o.right }

// BinaryOperation returns e's typed arithmetic, comparison, or logical
// binary operation data. Coalesce is returned by ConditionalOperation.
func (e Expression) BinaryOperation() (BinaryExpression, bool) {
	if !e.IsValid() {
		return BinaryExpression{}, false
	}
	var kind BinaryOperationKind
	var left, right ast.Expression
	switch node := e.value.(type) {
	case ast.Arithmetic:
		left, right = node.Left(), node.Right()
		switch node.Operator() {
		case ast.ArithmeticAdd:
			kind = BinaryAdd
		case ast.ArithmeticSubtract:
			kind = BinarySubtract
		case ast.ArithmeticMultiply:
			kind = BinaryMultiply
		case ast.ArithmeticDivide:
			kind = BinaryDivide
		}
	case ast.Comparison:
		left, right = node.Left(), node.Right()
		switch node.Operator() {
		case ast.ComparisonEqual:
			kind = BinaryEqual
		case ast.ComparisonNotEqual:
			kind = BinaryNotEqual
		case ast.ComparisonLess:
			kind = BinaryLess
		case ast.ComparisonLessOrEqual:
			kind = BinaryLessOrEqual
		case ast.ComparisonGreater:
			kind = BinaryGreater
		case ast.ComparisonGreaterOrEqual:
			kind = BinaryGreaterOrEqual
		}
	case ast.Logical:
		left, right = node.Left(), node.Right()
		switch node.Operator() {
		case ast.LogicalAnd:
			kind = BinaryAnd
		case ast.LogicalOr:
			kind = BinaryOr
		}
	default:
		return BinaryExpression{}, false
	}
	return BinaryExpression{kind: kind, left: expressionFromInternal(left), right: expressionFromInternal(right)}, kind != 0
}

// ConditionalOperationKind identifies the v1 control-expression forms.
type ConditionalOperationKind uint8

const (
	ConditionalIf ConditionalOperationKind = iota + 1
	ConditionalCoalesce
)

// ConditionalExpression describes one immutable If or Coalesce expression.
// Children returns condition/then/else for If and primary/fallback for
// Coalesce.
type ConditionalExpression struct {
	kind     ConditionalOperationKind
	children []Expression
}

func (o ConditionalExpression) Kind() ConditionalOperationKind { return o.kind }
func (o ConditionalExpression) Children() []Expression {
	return append([]Expression(nil), o.children...)
}
func (o ConditionalExpression) Condition() (Expression, bool) {
	if o.kind != ConditionalIf || len(o.children) != 3 {
		return Expression{}, false
	}
	return o.children[0], true
}
func (o ConditionalExpression) Then() (Expression, bool) {
	if o.kind != ConditionalIf || len(o.children) != 3 {
		return Expression{}, false
	}
	return o.children[1], true
}
func (o ConditionalExpression) Else() (Expression, bool) {
	if o.kind != ConditionalIf || len(o.children) != 3 {
		return Expression{}, false
	}
	return o.children[2], true
}
func (o ConditionalExpression) Primary() (Expression, bool) {
	if o.kind != ConditionalCoalesce || len(o.children) != 2 {
		return Expression{}, false
	}
	return o.children[0], true
}
func (o ConditionalExpression) Fallback() (Expression, bool) {
	if o.kind != ConditionalCoalesce || len(o.children) != 2 {
		return Expression{}, false
	}
	return o.children[1], true
}

// ConditionalOperation returns e's typed If or Coalesce data.
func (e Expression) ConditionalOperation() (ConditionalExpression, bool) {
	if !e.IsValid() {
		return ConditionalExpression{}, false
	}
	switch node := e.value.(type) {
	case ast.If:
		return ConditionalExpression{kind: ConditionalIf, children: []Expression{expressionFromInternal(node.Condition()), expressionFromInternal(node.Then()), expressionFromInternal(node.Else())}}, true
	case ast.Coalesce:
		return ConditionalExpression{kind: ConditionalCoalesce, children: []Expression{expressionFromInternal(node.Primary()), expressionFromInternal(node.Fallback())}}, true
	default:
		return ConditionalExpression{}, false
	}
}

// AggregateExpression describes the traversal and optional filter/map data of
// an aggregate expression.
type AggregateExpression struct {
	source    TraversalSource
	operation AggregateOperation
	filter    Expression
	hasFilter bool
	mapped    Expression
	hasMap    bool
}

func (o AggregateExpression) Source() TraversalSource       { return o.source }
func (o AggregateExpression) Operation() AggregateOperation { return o.operation }
func (o AggregateExpression) Filter() (Expression, bool)    { return o.filter, o.hasFilter }
func (o AggregateExpression) Map() (Expression, bool)       { return o.mapped, o.hasMap }

// AggregateOperation returns e's typed aggregate data, including its
// traversal source and optional filter/map expressions.
func (e Expression) AggregateOperation() (AggregateExpression, bool) {
	node, ok := e.value.(ast.Aggregate)
	if !ok || !e.IsValid() {
		return AggregateExpression{}, false
	}
	result := AggregateExpression{source: TraversalSource{value: node.Source()}, operation: aggregateOperationFromInternal(node.Operation())}
	if filter, ok := node.Filter(); ok {
		result.filter, result.hasFilter = expressionFromInternal(filter), true
	}
	if mapped, ok := node.Map(); ok {
		result.mapped, result.hasMap = expressionFromInternal(mapped), true
	}
	return result, true
}

// ExpressionVisitor receives each node during WalkExpression's deterministic
// pre-order traversal.
type ExpressionVisitor interface{ VisitExpression(Expression) error }

// ExpressionVisitorFunc adapts a function to ExpressionVisitor.
type ExpressionVisitorFunc func(Expression) error

func (f ExpressionVisitorFunc) VisitExpression(expression Expression) error { return f(expression) }

// WalkExpression visits expr and every descendant in deterministic pre-order.
// A valid expression is required; no Engine-owned AST state is exposed.
func WalkExpression(expr Expression, visitor ExpressionVisitor) error {
	if !expr.IsValid() {
		return invalidExpressionArgument("expression is invalid")
	}
	if visitor == nil {
		return invalidExpressionArgument("expression visitor is nil")
	}
	stack := []Expression{expr}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if err := visitor.VisitExpression(current); err != nil {
			return err
		}
		children := current.Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
	return nil
}

// IsValid reports whether e contains a structurally valid supported AST.
// Expressions deeper than MaxExpressionDepth remain structurally valid here
// so that Draft can report the existing authoritative depth validation issue.
func (e Expression) IsValid() bool {
	_, valid := ast.Depth(e.value)
	return valid
}

// Depth returns e's structural nesting depth. It is iterative and does not
// recurse through the expression tree. The boolean is false for a zero or
// malformed expression.
func (e Expression) Depth() (int, bool) { return ast.Depth(e.value) }

// Constant constructs a literal expression from a defined public Value.
// BCG v1 has no Null literal AST node: Null is a resolution state supplied by
// bindings and tested with IsNull rather than encoded as a magic Value.
func Constant(value Value) (Expression, error) {
	internal, err := value.internal()
	if err != nil {
		return Expression{}, fmt.Errorf("%w: expression constant: %v", ErrInvalidArgument, err)
	}
	return expressionFromInternal(ast.NewConstant(internal)), nil
}

// SelfProperty references a PropertyDefinition on the EntityType or
// RelationType that owns the current derived property. Its contextual
// availability is checked by Draft.Validate and Draft.Commit.
func SelfProperty(definitionID ID) (Expression, error) {
	id, err := expressionID(definitionID, "self property definition ID")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewSelfPropertyReference(id)), nil
}

// AbsoluteProperty references one concrete Entity or Relation property.
func AbsoluteProperty(property PropertyAddress) (Expression, error) {
	addressValue, err := property.internal()
	if err != nil {
		return Expression{}, fmt.Errorf("%w: absolute property: %v", ErrInvalidArgument, err)
	}
	return expressionFromInternal(ast.NewAbsolutePropertyReference(addressValue)), nil
}

// AggregateBinding identifies one object bound for each item in an aggregate
// traversal. The binding names are semantic, not implementation stack slots.
type AggregateBinding uint8

const (
	// BoundEntity is the Entity at the opposite endpoint of a traversed Relation.
	BoundEntity AggregateBinding = iota + 1
	// BoundRelation is the Relation currently being traversed.
	BoundRelation
)

// IsValid reports whether b is a supported aggregate binding.
func (b AggregateBinding) IsValid() bool {
	return b == BoundEntity || b == BoundRelation
}

// String returns the stable binding name.
func (b AggregateBinding) String() string {
	switch b {
	case BoundEntity:
		return "Entity"
	case BoundRelation:
		return "Relation"
	default:
		return "Invalid"
	}
}

func (b AggregateBinding) internal() (ast.BoundTarget, error) {
	switch b {
	case BoundEntity:
		return ast.BoundTargetEntity, nil
	case BoundRelation:
		return ast.BoundTargetRelation, nil
	default:
		return ast.BoundTargetInvalid, invalidExpressionArgument("aggregate binding is invalid")
	}
}

func aggregateBindingFromInternal(binding ast.BoundTarget) AggregateBinding {
	switch binding {
	case ast.BoundTargetEntity:
		return BoundEntity
	case ast.BoundTargetRelation:
		return BoundRelation
	default:
		return 0
	}
}

// BoundProperty references a PropertyDefinition on the Entity or Relation
// bound by the nearest enclosing aggregate traversal.
func BoundProperty(binding AggregateBinding, definitionID ID) (Expression, error) {
	target, err := binding.internal()
	if err != nil {
		return Expression{}, err
	}
	id, err := expressionID(definitionID, "bound property definition ID")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewBoundPropertyDefinitionReference(target, id)), nil
}

// BoundContractProperty resolves a PropertyContract implementation on the
// Entity or Relation bound by the nearest enclosing aggregate traversal.
func BoundContractProperty(binding AggregateBinding, contractID ID) (Expression, error) {
	target, err := binding.internal()
	if err != nil {
		return Expression{}, err
	}
	id, err := expressionID(contractID, "bound property contract ID")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewBoundContractPropertyReference(target, id)), nil
}

// Add constructs an addition expression.
func Add(left, right Expression) (Expression, error) {
	return binaryExpression("add", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.Add(left, right)
	})
}

// Subtract constructs a subtraction expression.
func Subtract(left, right Expression) (Expression, error) {
	return binaryExpression("subtract", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.Subtract(left, right)
	})
}

// Multiply constructs a multiplication expression.
func Multiply(left, right Expression) (Expression, error) {
	return binaryExpression("multiply", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.Multiply(left, right)
	})
}

// Divide constructs a division expression.
func Divide(left, right Expression) (Expression, error) {
	return binaryExpression("divide", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.Divide(left, right)
	})
}

// Negate constructs an arithmetic negation expression.
func Negate(operand Expression) (Expression, error) {
	value, err := requiredExpression(operand, "negate operand")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewNegate(value)), nil
}

// Equal constructs an equality comparison expression.
func Equal(left, right Expression) (Expression, error) {
	return binaryExpression("equal", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.Equal(left, right)
	})
}

// NotEqual constructs an inequality comparison expression.
func NotEqual(left, right Expression) (Expression, error) {
	return binaryExpression("not-equal", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.NotEqual(left, right)
	})
}

// Less constructs a less-than comparison expression.
func Less(left, right Expression) (Expression, error) {
	return binaryExpression("less", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.Less(left, right)
	})
}

// LessOrEqual constructs a less-than-or-equal comparison expression.
func LessOrEqual(left, right Expression) (Expression, error) {
	return binaryExpression("less-or-equal", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.LessOrEqual(left, right)
	})
}

// Greater constructs a greater-than comparison expression.
func Greater(left, right Expression) (Expression, error) {
	return binaryExpression("greater", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.Greater(left, right)
	})
}

// GreaterOrEqual constructs a greater-than-or-equal comparison expression.
func GreaterOrEqual(left, right Expression) (Expression, error) {
	return binaryExpression("greater-or-equal", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.GreaterOrEqual(left, right)
	})
}

// And constructs a Boolean conjunction expression.
func And(left, right Expression) (Expression, error) {
	return binaryExpression("and", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.And(left, right)
	})
}

// Or constructs a Boolean disjunction expression.
func Or(left, right Expression) (Expression, error) {
	return binaryExpression("or", left, right, func(left, right ast.Expression) ast.Expression {
		return ast.Or(left, right)
	})
}

// Not constructs a Boolean negation expression.
func Not(operand Expression) (Expression, error) {
	value, err := requiredExpression(operand, "not operand")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewNot(value)), nil
}

// If constructs a conditional expression. The validator determines whether
// condition is Boolean and whether the two result branches are compatible.
func If(condition, thenExpression, elseExpression Expression) (Expression, error) {
	conditionValue, err := requiredExpression(condition, "if condition")
	if err != nil {
		return Expression{}, err
	}
	thenValue, err := requiredExpression(thenExpression, "if then expression")
	if err != nil {
		return Expression{}, err
	}
	elseValue, err := requiredExpression(elseExpression, "if else expression")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewIf(conditionValue, thenValue, elseValue)), nil
}

// IsNull constructs an explicit-Null test. Undefined remains distinct from
// Null and keeps the evaluator's existing propagation semantics.
func IsNull(operand Expression) (Expression, error) {
	value, err := requiredExpression(operand, "is-null operand")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewIsNull(value)), nil
}

// Coalesce returns fallback when primary resolves to explicit Null. Undefined
// is not treated as Null.
func Coalesce(primary, fallback Expression) (Expression, error) {
	primaryValue, err := requiredExpression(primary, "coalesce primary")
	if err != nil {
		return Expression{}, err
	}
	fallbackValue, err := requiredExpression(fallback, "coalesce fallback")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewCoalesce(primaryValue, fallbackValue)), nil
}

// UnaryV2Operation identifies a v2 unary expression operation.
type UnaryV2Operation uint8

const (
	UnaryIsDefined UnaryV2Operation = iota + 1
	UnaryIsUndefined
	UnaryAbs
	UnaryRound
	UnaryFloor
	UnaryCeil
	UnaryLength
	UnaryLower
	UnaryUpper
	UnaryTrim
	UnaryYear
	UnaryMonth
	UnaryDay
	UnaryHour
	UnaryMinute
)

func (u UnaryV2Operation) IsValid() bool                       { return u >= UnaryIsDefined && u <= UnaryMinute }
func unaryV2Internal(u UnaryV2Operation) ast.UnaryOperator     { return ast.UnaryOperator(u) }
func unaryV2FromInternal(u ast.UnaryOperator) UnaryV2Operation { return UnaryV2Operation(u) }

type UnaryV2Expression struct {
	operation UnaryV2Operation
	operand   Expression
}

func (o UnaryV2Expression) Operation() UnaryV2Operation { return o.operation }
func (o UnaryV2Expression) Operand() Expression         { return o.operand }
func (e Expression) UnaryV2Operation() (UnaryV2Expression, bool) {
	n, ok := e.value.(ast.Unary)
	if !ok || !e.IsValid() {
		return UnaryV2Expression{}, false
	}
	return UnaryV2Expression{unaryV2FromInternal(n.Operator()), expressionFromInternal(n.Operand())}, true
}
func unaryV2(operation UnaryV2Operation, operand Expression) (Expression, error) {
	v, err := requiredExpression(operand, "unary operand")
	if err != nil {
		return Expression{}, err
	}
	if !operation.IsValid() {
		return Expression{}, invalidExpressionArgument("unary operation is invalid")
	}
	return expressionFromInternal(ast.NewUnary(unaryV2Internal(operation), v)), nil
}
func IsDefined(operand Expression) (Expression, error)   { return unaryV2(UnaryIsDefined, operand) }
func IsUndefined(operand Expression) (Expression, error) { return unaryV2(UnaryIsUndefined, operand) }
func Abs(operand Expression) (Expression, error)         { return unaryV2(UnaryAbs, operand) }
func Round(operand Expression) (Expression, error)       { return unaryV2(UnaryRound, operand) }
func Floor(operand Expression) (Expression, error)       { return unaryV2(UnaryFloor, operand) }
func Ceil(operand Expression) (Expression, error)        { return unaryV2(UnaryCeil, operand) }
func Length(operand Expression) (Expression, error)      { return unaryV2(UnaryLength, operand) }
func Lower(operand Expression) (Expression, error)       { return unaryV2(UnaryLower, operand) }
func Upper(operand Expression) (Expression, error)       { return unaryV2(UnaryUpper, operand) }
func Trim(operand Expression) (Expression, error)        { return unaryV2(UnaryTrim, operand) }
func Year(operand Expression) (Expression, error)        { return unaryV2(UnaryYear, operand) }
func Month(operand Expression) (Expression, error)       { return unaryV2(UnaryMonth, operand) }
func Day(operand Expression) (Expression, error)         { return unaryV2(UnaryDay, operand) }
func Hour(operand Expression) (Expression, error)        { return unaryV2(UnaryHour, operand) }
func Minute(operand Expression) (Expression, error)      { return unaryV2(UnaryMinute, operand) }

type BinaryV2Operation uint8

const (
	BinaryModulo BinaryV2Operation = iota + 1
	BinaryContains
	BinaryStartsWith
	BinaryEndsWith
	BinaryAddDuration
	BinarySubtractDuration
	BinaryDifference
)

func (b BinaryV2Operation) IsValid() bool { return b >= BinaryModulo && b <= BinaryDifference }

type BinaryV2Expression struct {
	operation   BinaryV2Operation
	left, right Expression
}

func (o BinaryV2Expression) Operation() BinaryV2Operation { return o.operation }
func (o BinaryV2Expression) Left() Expression             { return o.left }
func (o BinaryV2Expression) Right() Expression            { return o.right }
func (e Expression) BinaryV2Operation() (BinaryV2Expression, bool) {
	n, ok := e.value.(ast.Binary)
	if !ok || !e.IsValid() {
		return BinaryV2Expression{}, false
	}
	return BinaryV2Expression{BinaryV2Operation(n.Operator()), expressionFromInternal(n.Left()), expressionFromInternal(n.Right())}, true
}
func binaryV2(operation BinaryV2Operation, left, right Expression) (Expression, error) {
	l, e := requiredExpression(left, "binary left operand")
	if e != nil {
		return Expression{}, e
	}
	r, e := requiredExpression(right, "binary right operand")
	if e != nil {
		return Expression{}, e
	}
	if !operation.IsValid() {
		return Expression{}, invalidExpressionArgument("binary operation is invalid")
	}
	return expressionFromInternal(ast.NewBinary(ast.BinaryOperator(operation), l, r)), nil
}
func Modulo(left, right Expression) (Expression, error) { return binaryV2(BinaryModulo, left, right) }
func Contains(left, right Expression) (Expression, error) {
	return binaryV2(BinaryContains, left, right)
}
func StartsWith(left, right Expression) (Expression, error) {
	return binaryV2(BinaryStartsWith, left, right)
}
func EndsWith(left, right Expression) (Expression, error) {
	return binaryV2(BinaryEndsWith, left, right)
}
func AddDuration(left, right Expression) (Expression, error) {
	return binaryV2(BinaryAddDuration, left, right)
}
func SubtractDuration(left, right Expression) (Expression, error) {
	return binaryV2(BinarySubtractDuration, left, right)
}
func Difference(left, right Expression) (Expression, error) {
	return binaryV2(BinaryDifference, left, right)
}

type VariadicV2Operation uint8

const (
	VariadicConcat VariadicV2Operation = iota + 1
	VariadicScalarMin
	VariadicScalarMax
)

type VariadicV2Expression struct {
	operation VariadicV2Operation
	operands  []Expression
}

func (o VariadicV2Expression) Operation() VariadicV2Operation { return o.operation }
func (o VariadicV2Expression) Operands() []Expression {
	return append([]Expression(nil), o.operands...)
}
func (e Expression) VariadicV2Operation() (VariadicV2Expression, bool) {
	n, ok := e.value.(ast.Variadic)
	if !ok || !e.IsValid() {
		return VariadicV2Expression{}, false
	}
	xs := n.Operands()
	out := make([]Expression, len(xs))
	for i, v := range xs {
		out[i] = expressionFromInternal(v)
	}
	return VariadicV2Expression{VariadicV2Operation(n.Operator()), out}, true
}
func variadicV2(operation VariadicV2Operation, operands ...Expression) (Expression, error) {
	if operation < VariadicConcat || operation > VariadicScalarMax || len(operands) == 0 {
		return Expression{}, invalidExpressionArgument("variadic operation is invalid")
	}
	xs := make([]ast.Expression, len(operands))
	for i, v := range operands {
		x, e := requiredExpression(v, "variadic operand")
		if e != nil {
			return Expression{}, e
		}
		xs[i] = x
	}
	return expressionFromInternal(ast.NewVariadic(ast.VariadicOperator(operation), xs)), nil
}
func Concat(operands ...Expression) (Expression, error) {
	return variadicV2(VariadicConcat, operands...)
}
func ScalarMin(operands ...Expression) (Expression, error) {
	return variadicV2(VariadicScalarMin, operands...)
}
func ScalarMax(operands ...Expression) (Expression, error) {
	return variadicV2(VariadicScalarMax, operands...)
}

// FunctionCallExpression is the immutable public view of a FunctionCall.
// Requirement signatures are model-level semantic declarations and can be
// inspected through Model.FunctionRequirements.
type FunctionCallExpression struct {
	ref       FunctionRef
	arguments []Expression
}

func (c FunctionCallExpression) Ref() FunctionRef { return c.ref }
func (c FunctionCallExpression) Arguments() []Expression {
	return append([]Expression(nil), c.arguments...)
}

// FunctionCall returns typed immutable FunctionCall data.
func (e Expression) FunctionCall() (FunctionCallExpression, bool) {
	node, ok := e.value.(ast.FunctionCall)
	if !ok || !e.IsValid() {
		return FunctionCallExpression{}, false
	}
	arguments := node.Arguments()
	result := make([]Expression, len(arguments))
	for index, argument := range arguments {
		result[index] = expressionFromInternal(argument)
	}
	return FunctionCallExpression{ref: functionRefFromInternal(node.Ref()), arguments: result}, true
}

// CallFunction constructs a first-class extension-function call. Function
// signature and availability are deliberately model/runtime concerns: model
// validation resolves ref against the committed FunctionRequirement.
func CallFunction(ref FunctionRef, args ...Expression) (Expression, error) {
	internalRef, err := ref.internal()
	if err != nil {
		return Expression{}, fmt.Errorf("%w: function ref is invalid", ErrInvalidArgument)
	}
	if len(args) > MaxFunctionArguments {
		return Expression{}, fmt.Errorf("%w: function argument count exceeds %d", ErrInvalidArgument, MaxFunctionArguments)
	}
	arguments := make([]ast.Expression, len(args))
	for index, argument := range args {
		value, argumentErr := requiredExpression(argument, "function argument")
		if argumentErr != nil {
			return Expression{}, argumentErr
		}
		arguments[index] = value
	}
	return expressionFromInternal(ast.NewFunctionCall(internalRef, arguments)), nil
}

// TraversalDirection selects outgoing, incoming, or both relation directions.
// Its zero value is invalid.
type TraversalDirection uint8

const (
	// TraverseOutgoing follows relations from the origin's From endpoint.
	TraverseOutgoing TraversalDirection = iota + 1
	// TraverseIncoming follows relations to the origin's To endpoint.
	TraverseIncoming
	// TraverseBoth follows both directions, subject to endpoint applicability.
	TraverseBoth
)

// IsValid reports whether d is a supported traversal direction.
func (d TraversalDirection) IsValid() bool {
	return d >= TraverseOutgoing && d <= TraverseBoth
}

// String returns the stable traversal direction name.
func (d TraversalDirection) String() string {
	switch d {
	case TraverseOutgoing:
		return "Outgoing"
	case TraverseIncoming:
		return "Incoming"
	case TraverseBoth:
		return "Both"
	default:
		return "Invalid"
	}
}

func (d TraversalDirection) internal() (ast.TraversalDirection, error) {
	switch d {
	case TraverseOutgoing:
		return ast.TraversalDirectionOutgoing, nil
	case TraverseIncoming:
		return ast.TraversalDirectionIncoming, nil
	case TraverseBoth:
		return ast.TraversalDirectionBoth, nil
	default:
		return ast.TraversalDirectionInvalid, invalidExpressionArgument("traversal direction is invalid")
	}
}

func traversalDirectionFromInternal(direction ast.TraversalDirection) TraversalDirection {
	switch direction {
	case ast.TraversalDirectionOutgoing:
		return TraverseOutgoing
	case ast.TraversalDirectionIncoming:
		return TraverseIncoming
	case ast.TraversalDirectionBoth:
		return TraverseBoth
	default:
		return 0
	}
}

// TraversalOrigin identifies how an aggregate traversal selects its origin Entity.
type TraversalOrigin uint8

const (
	// TraversalSelf starts at the current derived property's Entity self.
	TraversalSelf TraversalOrigin = iota + 1
	// TraversalAbsoluteEntity starts at one explicit concrete Entity.
	TraversalAbsoluteEntity
	// TraversalBoundEntity starts at the Entity bound by an enclosing aggregate.
	TraversalBoundEntity
)

// IsValid reports whether o is a supported traversal origin.
func (o TraversalOrigin) IsValid() bool {
	return o >= TraversalSelf && o <= TraversalBoundEntity
}

// String returns the stable traversal origin name.
func (o TraversalOrigin) String() string {
	switch o {
	case TraversalSelf:
		return "Self"
	case TraversalAbsoluteEntity:
		return "AbsoluteEntity"
	case TraversalBoundEntity:
		return "BoundEntity"
	default:
		return "Invalid"
	}
}

func traversalOriginFromInternal(origin ast.TraversalOrigin) TraversalOrigin {
	switch origin {
	case ast.TraversalOriginSelf:
		return TraversalSelf
	case ast.TraversalOriginAbsoluteEntity:
		return TraversalAbsoluteEntity
	case ast.TraversalOriginBoundEntity:
		return TraversalBoundEntity
	default:
		return 0
	}
}

// TraversalSource is an immutable one-hop relation traversal used by Aggregate.
// It has no mutable endpoint or registry state.
type TraversalSource struct{ value ast.TraversalSource }

// SelfTraversal starts from the current Entity self. Validation rejects it
// when the expression context has no Entity self, including standalone uses.
func SelfTraversal(direction TraversalDirection, relationTypeID ID) (TraversalSource, error) {
	directionValue, err := direction.internal()
	if err != nil {
		return TraversalSource{}, err
	}
	relationID, err := expressionID(relationTypeID, "traversal relation type ID")
	if err != nil {
		return TraversalSource{}, err
	}
	return TraversalSource{value: ast.NewTraversalSource(directionValue, relationID)}, nil
}

// AbsoluteEntityTraversal starts from one explicit concrete Entity. It is the
// standalone-safe traversal form because it does not invent an implicit self.
func AbsoluteEntityTraversal(entityID ID, direction TraversalDirection, relationTypeID ID) (TraversalSource, error) {
	entity, err := expressionID(entityID, "traversal absolute entity ID")
	if err != nil {
		return TraversalSource{}, err
	}
	directionValue, err := direction.internal()
	if err != nil {
		return TraversalSource{}, err
	}
	relationID, err := expressionID(relationTypeID, "traversal relation type ID")
	if err != nil {
		return TraversalSource{}, err
	}
	return TraversalSource{value: ast.NewTraversalSourceFromEntity(address.EntityRef(entity), directionValue, relationID)}, nil
}

// BoundEntityTraversal starts from the Entity bound by the nearest enclosing
// aggregate. Validation rejects it outside that lexical aggregate context.
func BoundEntityTraversal(direction TraversalDirection, relationTypeID ID) (TraversalSource, error) {
	directionValue, err := direction.internal()
	if err != nil {
		return TraversalSource{}, err
	}
	relationID, err := expressionID(relationTypeID, "traversal relation type ID")
	if err != nil {
		return TraversalSource{}, err
	}
	return TraversalSource{value: ast.NewTraversalSourceFromBoundEntity(directionValue, relationID)}, nil
}

// Origin returns the source origin form.
func (s TraversalSource) Origin() TraversalOrigin {
	return traversalOriginFromInternal(s.value.Origin())
}

// AbsoluteEntityID returns the explicit Entity origin, when present.
func (s TraversalSource) AbsoluteEntityID() (ID, bool) {
	origin, ok := s.value.AbsoluteOrigin()
	if !ok {
		return ID{}, false
	}
	return idFromInternal(origin.ID()), true
}

// Direction returns the traversal direction.
func (s TraversalSource) Direction() TraversalDirection {
	return traversalDirectionFromInternal(s.value.Direction())
}

// RelationTypeID returns the selected RelationType identity.
func (s TraversalSource) RelationTypeID() ID { return idFromInternal(s.value.RelationTypeID()) }

// IsValid reports structural traversal-source validity.
func (s TraversalSource) IsValid() bool { return s.value.IsValid() }

// AggregateOperation identifies a graph aggregate operation. Its zero value
// is invalid.
type AggregateOperation uint8

const (
	// AggregateSum sums numeric mapped values.
	AggregateSum AggregateOperation = iota + 1
	// AggregateAverage averages numeric mapped values.
	AggregateAverage
	// AggregateCount counts traversal items without evaluating a map expression.
	AggregateCount
	// AggregateMin selects the minimum mapped value.
	AggregateMin
	// AggregateMax selects the maximum mapped value.
	AggregateMax
	// AggregateExists reports whether at least one traversal item passes the optional filter.
	AggregateExists
	// AggregateAny applies strong three-valued OR to mapped Boolean values.
	AggregateAny
	// AggregateAll applies strong three-valued AND to mapped Boolean values.
	AggregateAll
	// AggregateDistinctCount counts semantically distinct mapped values.
	AggregateDistinctCount
)

// IsValid reports whether o is a supported aggregate operation.
func (o AggregateOperation) IsValid() bool { return o >= AggregateSum && o <= AggregateDistinctCount }

// RequiresMap reports whether o requires a mapped value expression.
func (o AggregateOperation) RequiresMap() bool {
	return o != AggregateCount && o != AggregateExists && o.IsValid()
}

// String returns the stable aggregate operation name.
func (o AggregateOperation) String() string {
	switch o {
	case AggregateSum:
		return "Sum"
	case AggregateAverage:
		return "Average"
	case AggregateCount:
		return "Count"
	case AggregateMin:
		return "Min"
	case AggregateMax:
		return "Max"
	case AggregateExists:
		return "Exists"
	case AggregateAny:
		return "Any"
	case AggregateAll:
		return "All"
	case AggregateDistinctCount:
		return "DistinctCount"
	default:
		return "Invalid"
	}
}

func (o AggregateOperation) internal() (ast.AggregateOperation, error) {
	switch o {
	case AggregateSum:
		return ast.AggregateOperationSum, nil
	case AggregateAverage:
		return ast.AggregateOperationAverage, nil
	case AggregateCount:
		return ast.AggregateOperationCount, nil
	case AggregateMin:
		return ast.AggregateOperationMin, nil
	case AggregateMax:
		return ast.AggregateOperationMax, nil
	case AggregateExists:
		return ast.AggregateOperationExists, nil
	case AggregateAny:
		return ast.AggregateOperationAny, nil
	case AggregateAll:
		return ast.AggregateOperationAll, nil
	case AggregateDistinctCount:
		return ast.AggregateOperationDistinctCount, nil
	default:
		return ast.AggregateOperationInvalid, invalidExpressionArgument("aggregate operation is invalid")
	}
}

func aggregateOperationFromInternal(operation ast.AggregateOperation) AggregateOperation {
	switch operation {
	case ast.AggregateOperationSum:
		return AggregateSum
	case ast.AggregateOperationAverage:
		return AggregateAverage
	case ast.AggregateOperationCount:
		return AggregateCount
	case ast.AggregateOperationMin:
		return AggregateMin
	case ast.AggregateOperationMax:
		return AggregateMax
	case ast.AggregateOperationExists:
		return AggregateExists
	case ast.AggregateOperationAny:
		return AggregateAny
	case ast.AggregateOperationAll:
		return AggregateAll
	case ast.AggregateOperationDistinctCount:
		return AggregateDistinctCount
	default:
		return 0
	}
}

// Count constructs an unfiltered Count aggregate. Count deliberately has no
// map argument because the v1 evaluator never evaluates a map for Count.
func Count(source TraversalSource) (Expression, error) {
	if err := requiredTraversalSource(source); err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewCountAggregate(source.value)), nil
}

// FilteredCount constructs a Count aggregate with a Boolean per-item filter.
func FilteredCount(source TraversalSource, filter Expression) (Expression, error) {
	if err := requiredTraversalSource(source); err != nil {
		return Expression{}, err
	}
	filterValue, err := requiredExpression(filter, "aggregate filter")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewFilteredCountAggregate(source.value, filterValue)), nil
}

// Exists reports whether at least one traversal item remains after optional
// filtering. It does not evaluate a map expression.
func Exists(source TraversalSource) (Expression, error) {
	if err := requiredTraversalSource(source); err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewCountLikeAggregate(source.value, ast.AggregateOperationExists)), nil
}

// FilteredExists reports whether an item passes filter. A Null or Undefined
// filter is indeterminate rather than silently false.
func FilteredExists(source TraversalSource, filter Expression) (Expression, error) {
	if err := requiredTraversalSource(source); err != nil {
		return Expression{}, err
	}
	value, err := requiredExpression(filter, "aggregate filter")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewFilteredCountLikeAggregate(source.value, value, ast.AggregateOperationExists)), nil
}

func Any(source TraversalSource, mapExpression Expression) (Expression, error) {
	return Aggregate(source, AggregateAny, mapExpression)
}
func FilteredAny(source TraversalSource, filter, mapExpression Expression) (Expression, error) {
	return FilteredAggregate(source, filter, AggregateAny, mapExpression)
}
func All(source TraversalSource, mapExpression Expression) (Expression, error) {
	return Aggregate(source, AggregateAll, mapExpression)
}
func FilteredAll(source TraversalSource, filter, mapExpression Expression) (Expression, error) {
	return FilteredAggregate(source, filter, AggregateAll, mapExpression)
}
func DistinctCount(source TraversalSource, mapExpression Expression) (Expression, error) {
	return Aggregate(source, AggregateDistinctCount, mapExpression)
}
func FilteredDistinctCount(source TraversalSource, filter, mapExpression Expression) (Expression, error) {
	return FilteredAggregate(source, filter, AggregateDistinctCount, mapExpression)
}

// Aggregate constructs an unfiltered Sum, Average, Min, or Max aggregate.
// Count is constructed with Count or FilteredCount so it cannot accidentally
// acquire an unused map expression.
func Aggregate(source TraversalSource, operation AggregateOperation, mapExpression Expression) (Expression, error) {
	if err := requiredTraversalSource(source); err != nil {
		return Expression{}, err
	}
	operationValue, err := requiredMappedAggregateOperation(operation)
	if err != nil {
		return Expression{}, err
	}
	mapValue, err := requiredExpression(mapExpression, "aggregate map")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewMappedAggregate(source.value, mapValue, operationValue)), nil
}

// FilteredAggregate constructs a filtered Sum, Average, Min, or Max aggregate.
func FilteredAggregate(source TraversalSource, filter Expression, operation AggregateOperation, mapExpression Expression) (Expression, error) {
	if err := requiredTraversalSource(source); err != nil {
		return Expression{}, err
	}
	filterValue, err := requiredExpression(filter, "aggregate filter")
	if err != nil {
		return Expression{}, err
	}
	operationValue, err := requiredMappedAggregateOperation(operation)
	if err != nil {
		return Expression{}, err
	}
	mapValue, err := requiredExpression(mapExpression, "aggregate map")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(ast.NewFilteredMappedAggregate(source.value, filterValue, mapValue, operationValue)), nil
}

func expressionFromInternal(value ast.Expression) Expression { return Expression{value: value} }

func expressionID(id ID, name string) (identity.UUID, error) {
	value, err := id.internal()
	if err != nil {
		return identity.UUID{}, invalidExpressionArgument(name + " must not be zero")
	}
	return value, nil
}

func requiredExpression(expression Expression, name string) (ast.Expression, error) {
	if expression.value == nil || !expression.IsValid() {
		return nil, invalidExpressionArgument(name + " is invalid")
	}
	return expression.value, nil
}

func requiredTraversalSource(source TraversalSource) error {
	if !source.IsValid() {
		return invalidExpressionArgument("aggregate traversal source is invalid")
	}
	return nil
}

func requiredMappedAggregateOperation(operation AggregateOperation) (ast.AggregateOperation, error) {
	value, err := operation.internal()
	if err != nil {
		return ast.AggregateOperationInvalid, err
	}
	if !value.RequiresMap() {
		return ast.AggregateOperationInvalid, invalidExpressionArgument("aggregate operation does not accept a map")
	}
	return value, nil
}

func binaryExpression(name string, left, right Expression, construct func(ast.Expression, ast.Expression) ast.Expression) (Expression, error) {
	leftValue, err := requiredExpression(left, name+" left operand")
	if err != nil {
		return Expression{}, err
	}
	rightValue, err := requiredExpression(right, name+" right operand")
	if err != nil {
		return Expression{}, err
	}
	return expressionFromInternal(construct(leftValue, rightValue)), nil
}

func invalidExpressionArgument(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
}
