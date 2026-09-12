package bcg

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
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
)

// IsValid reports whether o is a supported aggregate operation.
func (o AggregateOperation) IsValid() bool {
	return o >= AggregateSum && o <= AggregateMax
}

// RequiresMap reports whether o requires a mapped value expression.
func (o AggregateOperation) RequiresMap() bool {
	return o == AggregateSum || o == AggregateAverage || o == AggregateMin || o == AggregateMax
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
	default:
		return ast.AggregateOperationInvalid, invalidExpressionArgument("aggregate operation is invalid")
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
		return ast.AggregateOperationInvalid, invalidExpressionArgument("aggregate operation requires Count without a map")
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
