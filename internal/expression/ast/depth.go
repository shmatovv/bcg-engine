package ast

// MaxExpressionDepth is the maximum structural nesting accepted by v1. The
// limit is shared by model validation, evaluation and the portable decoder so
// an accepted expression can never exhaust the host goroutine stack merely by
// being traversed through a recursive implementation detail.
const MaxExpressionDepth = 512

// Depth reports the maximum structural nesting of expression without
// recursively walking the AST. Its second result is false for nil, malformed
// or unsupported nodes. Aggregate filter and map expressions are part of the
// same expression tree for this limit.
func Depth(expression Expression) (int, bool) {
	if expression == nil {
		return 0, false
	}

	type pending struct {
		expression Expression
		depth      int
	}

	stack := []pending{{expression: expression, depth: 1}}
	maximum := 0

	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]

		if current.expression == nil {
			return 0, false
		}
		if current.depth > maximum {
			maximum = current.depth
		}

		var children []Expression
		switch node := current.expression.(type) {
		case Constant:
			if !node.IsValid() {
				return 0, false
			}

		case SelfPropertyReference:
			if !node.IsValid() {
				return 0, false
			}

		case AbsolutePropertyReference:
			if !node.IsValid() {
				return 0, false
			}

		case BoundPropertyDefinitionReference:
			if !node.IsValid() {
				return 0, false
			}

		case BoundContractPropertyReference:
			if !node.IsValid() {
				return 0, false
			}

		case Arithmetic:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Left(), node.Right())

		case Negate:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Operand())

		case Comparison:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Left(), node.Right())

		case Logical:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Left(), node.Right())

		case Not:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Operand())

		case If:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Condition(), node.Then(), node.Else())

		case IsNull:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Operand())

		case Coalesce:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Primary(), node.Fallback())

		case Aggregate:
			if !node.IsValid() {
				return 0, false
			}
			if filter, hasFilter := node.Filter(); hasFilter {
				children = append(children, filter)
			}
			if mapExpression, hasMap := node.Map(); hasMap {
				children = append(children, mapExpression)
			}
		case Unary:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Operand())
		case Binary:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Left(), node.Right())
		case Variadic:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Operands()...)

		case FunctionCall:
			if !node.IsValid() {
				return 0, false
			}
			children = append(children, node.Arguments()...)

		default:
			return 0, false
		}

		for _, child := range children {
			stack = append(stack, pending{expression: child, depth: current.depth + 1})
		}
	}

	return maximum, true
}
