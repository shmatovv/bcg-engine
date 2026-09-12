package evaluator

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/internal/provenance"
	enginevalue "github.com/shmatovv/bcg-engine/internal/types/value"
)

type concreteExpressionBinding struct {
	relation address.ObjectRef
	entity   address.ObjectRef
}

func (b concreteExpressionBinding) IsValid() bool {
	return b.relation.IsValid() &&
		b.relation.Kind() == address.ObjectRefRelation &&
		b.entity.IsValid() &&
		b.entity.Kind() == address.ObjectRefEntity
}

type expressionContext struct {
	self address.ObjectRef

	static    inference.Context
	hasStatic bool

	binding    concreteExpressionBinding
	hasBinding bool
}

func newExpressionContext(
	self address.ObjectRef,
	owner schema.PropertyOwner,
) expressionContext {
	if !self.IsValid() {
		panic("evaluator: expression self object is invalid")
	}

	if !owner.IsValid() {
		panic("evaluator: expression self owner is invalid")
	}

	return expressionContext{
		self:      self,
		static:    inference.NewContext(owner),
		hasStatic: true,
	}
}

func (c expressionContext) inferenceContext(
	state *evaluationState,
) (inference.Context, error) {
	if state == nil {
		panic("evaluator: nil evaluation state")
	}

	if c.hasStatic {
		if !c.static.IsValid() {
			return inference.Context{},
				newError(
					ErrorInvariantViolation,
					"runtime expression has invalid static context",
					nil,
				)
		}

		return c.static, nil
	}

	if !c.self.IsValid() {
		return inference.EmptyContext(), nil
	}

	owner, ok := state.version.ObjectType(c.self)
	if !ok || !owner.IsValid() {
		return inference.Context{},
			newError(
				ErrorInvariantViolation,
				"expression self owner could not be resolved from committed ModelVersion",
				nil,
			)
	}

	return inference.NewContext(owner), nil
}

func (c expressionContext) withBinding(
	binding concreteExpressionBinding,
	staticBinding inference.Binding,
	parentStatic inference.Context,
) expressionContext {
	if !binding.IsValid() {
		panic("evaluator: aggregate concrete binding is invalid")
	}

	if !staticBinding.IsValid() {
		panic("evaluator: aggregate static binding is invalid")
	}

	if !parentStatic.IsValid() {
		panic("evaluator: aggregate parent static context is invalid")
	}

	c.binding = binding
	c.hasBinding = true
	c.static = parentStatic.WithBinding(staticBinding)
	c.hasStatic = true

	return c
}

func (c expressionContext) boundObject(
	target ast.BoundTarget,
) (address.ObjectRef, bool) {
	if !c.hasBinding || !c.binding.IsValid() {
		return address.ObjectRef{}, false
	}

	switch target {
	case ast.BoundTargetEntity:
		return c.binding.entity, true

	case ast.BoundTargetRelation:
		return c.binding.relation, true

	default:
		return address.ObjectRef{}, false
	}
}

func (c expressionContext) IsValid() bool {
	if c.hasStatic {
		if !c.static.IsValid() {
			return false
		}
	} else if !c.self.IsValid() {
		// A self-less expression is valid only when its static environment is
		// explicitly provided (normally inference.EmptyContext()).
		return false
	}

	if c.hasBinding {
		return c.binding.IsValid() && c.hasStatic
	}

	return !c.binding.IsValid()
}

func (s *evaluationState) evaluateExpression(
	expression ast.Expression,
	context expressionContext,
) (Result, error) {
	if err := s.enterExpression(); err != nil {
		return Result{}, err
	}
	defer s.leaveExpression()

	if expression == nil {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"committed expression is nil",
				nil,
			)
	}

	if !context.IsValid() {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"runtime expression context is invalid",
				nil,
			)
	}

	switch expression :=
		expression.(type) {

	case ast.Constant:
		if !expression.IsValid() {
			return Result{},
				invalidCommittedExpressionError()
		}

		return newResult(
			enginevalue.Defined(
				expression.Value(),
			),
			provenance.NewConstant(),
		), nil

	case ast.SelfPropertyReference:
		if !expression.IsValid() {
			return Result{},
				invalidCommittedExpressionError()
		}

		if !context.self.IsValid() {
			return Result{},
				newError(
					ErrorInvariantViolation,
					"self property reference is used in a standalone expression",
					nil,
				)
		}

		return s.evaluateReference(
			address.NewPropertyAddress(
				context.self,
				expression.DefinitionID(),
			),
		)

	case ast.AbsolutePropertyReference:
		if !expression.IsValid() {
			return Result{},
				invalidCommittedExpressionError()
		}

		return s.evaluateReference(
			expression.Address(),
		)

	case ast.BoundPropertyDefinitionReference:
		return s.evaluateBoundDefinitionReference(
			expression,
			context,
		)

	case ast.BoundContractPropertyReference:
		return s.evaluateBoundContractReference(
			expression,
			context,
		)

	case ast.Arithmetic:
		if !expression.IsValid() {
			return Result{},
				invalidCommittedExpressionError()
		}

		left, err :=
			s.evaluateExpression(
				expression.Left(),
				context,
			)

		if err != nil {
			return Result{}, err
		}

		right, err :=
			s.evaluateExpression(
				expression.Right(),
				context,
			)

		if err != nil {
			return Result{}, err
		}

		resolved, err :=
			s.evaluateArithmetic(
				expression.Operator(),
				left.value,
				right.value,
			)

		if err != nil {
			return Result{}, err
		}

		return newResult(
			resolved,
			provenance.NewOperation(
				expression.
					Operator().
					String(),
				left.provenance,
				right.provenance,
			),
		), nil

	case ast.Negate:
		if !expression.IsValid() {
			return Result{},
				invalidCommittedExpressionError()
		}

		operand, err :=
			s.evaluateExpression(
				expression.Operand(),
				context,
			)

		if err != nil {
			return Result{}, err
		}

		resolved, err :=
			evaluateNegate(
				operand.value,
			)

		if err != nil {
			return Result{}, err
		}

		return newResult(
			resolved,
			provenance.NewOperation(
				"Negate",
				operand.provenance,
			),
		), nil

	case ast.Comparison:
		if !expression.IsValid() {
			return Result{},
				invalidCommittedExpressionError()
		}

		left, err :=
			s.evaluateExpression(
				expression.Left(),
				context,
			)

		if err != nil {
			return Result{}, err
		}

		right, err :=
			s.evaluateExpression(
				expression.Right(),
				context,
			)

		if err != nil {
			return Result{}, err
		}

		resolved, err :=
			evaluateComparison(
				expression.Operator(),
				left.value,
				right.value,
			)

		if err != nil {
			return Result{}, err
		}

		return newResult(
			resolved,
			provenance.NewOperation(
				expression.
					Operator().
					String(),
				left.provenance,
				right.provenance,
			),
		), nil

	case ast.Logical:
		return s.evaluateLogical(
			expression,
			context,
		)

	case ast.Not:
		if !expression.IsValid() {
			return Result{},
				invalidCommittedExpressionError()
		}

		operand, err :=
			s.evaluateExpression(
				expression.Operand(),
				context,
			)

		if err != nil {
			return Result{}, err
		}

		resolved, err :=
			evaluateNot(
				operand.value,
			)

		if err != nil {
			return Result{}, err
		}

		return newResult(
			resolved,
			provenance.NewOperation(
				"Not",
				operand.provenance,
			),
		), nil

	case ast.If:
		return s.evaluateIf(
			expression,
			context,
		)

	case ast.IsNull:
		if !expression.IsValid() {
			return Result{},
				invalidCommittedExpressionError()
		}

		operand, err :=
			s.evaluateExpression(
				expression.Operand(),
				context,
			)

		if err != nil {
			return Result{}, err
		}

		resolved :=
			evaluateIsNull(
				operand.value,
			)

		return newResult(
			resolved,
			provenance.NewOperation(
				"IsNull",
				operand.provenance,
			),
		), nil

	case ast.Coalesce:
		return s.evaluateCoalesce(
			expression,
			context,
		)

	case ast.Aggregate:
		return s.evaluateAggregate(
			expression,
			context,
		)

	default:
		return Result{},
			newError(
				ErrorUnsupportedExpression,
				"unsupported expression node",
				nil,
			)
	}
}

func (s *evaluationState) evaluateBoundDefinitionReference(
	reference ast.BoundPropertyDefinitionReference,
	context expressionContext,
) (Result, error) {
	if !reference.IsValid() {
		return Result{},
			invalidCommittedExpressionError()
	}

	object, ok :=
		context.boundObject(
			reference.Target(),
		)

	if !ok {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"bound PropertyDefinition reference is used outside an aggregate binding",
				nil,
			)
	}

	return s.evaluateReference(
		address.NewPropertyAddress(
			object,
			reference.DefinitionID(),
		),
	)
}

func (s *evaluationState) evaluateBoundContractReference(
	reference ast.BoundContractPropertyReference,
	context expressionContext,
) (Result, error) {
	if !reference.IsValid() {
		return Result{},
			invalidCommittedExpressionError()
	}

	object, ok :=
		context.boundObject(
			reference.Target(),
		)

	if !ok {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"bound PropertyContract reference is used outside an aggregate binding",
				nil,
			)
	}

	owner, ok :=
		s.version.ObjectType(object)

	if !ok || !owner.IsValid() {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"bound object owner could not be resolved from committed ModelVersion",
				nil,
			)
	}

	definition, ok :=
		s.version.ContractImplementation(
			owner,
			reference.ContractID(),
		)

	if !ok ||
		!definition.IsValid() ||
		definition.Owner() != owner {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"bound PropertyContract implementation could not be resolved",
				nil,
			)
	}

	result, err :=
		s.evaluateReference(
			address.NewPropertyAddress(
				object,
				definition.ID(),
			),
		)

	if err != nil {
		return Result{}, err
	}

	staticContext, err := context.inferenceContext(s)
	if err != nil {
		return Result{}, err
	}

	expectedType, err :=
		inference.Infer(
			reference,
			staticContext,
			s.version,
		)

	if err != nil ||
		!expectedType.IsValid() {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"committed bound PropertyContract reference could not be statically inferred at runtime",
				err,
			)
	}

	normalized, err :=
		s.normalizeResolvedToStaticType(
			result.value,
			expectedType,
		)

	if err != nil {
		return Result{}, err
	}

	return newResult(
		normalized,
		provenance.NewOperation(
			"BoundContract",
			result.provenance,
		),
	), nil
}

func (s *evaluationState) evaluateReference(
	target address.PropertyAddress,
) (Result, error) {
	dependency, err :=
		s.evaluateProperty(
			target,
		)

	if err != nil {
		return Result{}, err
	}

	resolved :=
		dependency.value

	if resolved.IsUndefined() {
		resolved =
			dependencyUndefined()
	}

	return newResult(
		resolved,
		provenance.NewProperty(
			provenance.KindReference,
			target,
			dependency.provenance,
		),
	), nil
}

func (s *evaluationState) evaluateLogical(
	expression ast.Logical,
	context expressionContext,
) (Result, error) {
	if !expression.IsValid() {
		return Result{},
			invalidCommittedExpressionError()
	}

	left, err :=
		s.evaluateExpression(
			expression.Left(),
			context,
		)

	if err != nil {
		return Result{}, err
	}

	short,
		determined,
		err :=
		logicalShortCircuit(
			expression.Operator(),
			left.value,
		)

	if err != nil {
		return Result{}, err
	}

	if determined {
		return newResult(
			short,
			provenance.NewOperationDecision(
				expression.
					Operator().
					String(),
				provenance.DecisionShortCircuited,
				left.provenance,
			),
		), nil
	}

	right, err :=
		s.evaluateExpression(
			expression.Right(),
			context,
		)

	if err != nil {
		return Result{}, err
	}

	resolved, err :=
		evaluateLogicalPair(
			expression.Operator(),
			left.value,
			right.value,
		)

	if err != nil {
		return Result{}, err
	}

	return newResult(
		resolved,
		provenance.NewOperation(
			expression.
				Operator().
				String(),
			left.provenance,
			right.provenance,
		),
	), nil
}

func (s *evaluationState) evaluateIf(
	expression ast.If,
	context expressionContext,
) (Result, error) {
	if !expression.IsValid() {
		return Result{},
			invalidCommittedExpressionError()
	}

	condition, err :=
		s.evaluateExpression(
			expression.Condition(),
			context,
		)

	if err != nil {
		return Result{}, err
	}

	if condition.value.IsNull() ||
		condition.value.IsUndefined() {

		return newResult(
			undefined(
				enginevalue.
					UndefinedIndeterminateCondition,
			),
			provenance.NewOperationDecision(
				"If",
				provenance.DecisionIndeterminate,
				condition.provenance,
			),
		), nil
	}

	conditionValue :=
		condition.value.Value()

	if conditionValue.Type() !=
		enginevalue.Bool ||
		conditionValue.HasUnit() {

		return Result{},
			newError(
				ErrorInvariantViolation,
				"committed IF condition did not evaluate to unitless Bool",
				nil,
			)
	}

	selected := expression.Else()
	decision := provenance.DecisionElse

	if conditionValue.
		Raw().
		Bool() {

		selected = expression.Then()
		decision = provenance.DecisionThen
	}

	branch, err :=
		s.evaluateExpression(
			selected,
			context,
		)

	if err != nil {
		return Result{}, err
	}

	normalized, err := s.normalizeConditionalResult(expression, context, branch.value)
	if err != nil {
		return Result{}, err
	}
	return newResult(
		normalized,
		provenance.NewOperationDecision(
			"If",
			decision,
			condition.provenance,
			branch.provenance,
		),
	), nil
}

func (s *evaluationState) evaluateCoalesce(
	expression ast.Coalesce,
	context expressionContext,
) (Result, error) {
	if !expression.IsValid() {
		return Result{},
			invalidCommittedExpressionError()
	}

	primary, err :=
		s.evaluateExpression(
			expression.Primary(),
			context,
		)

	if err != nil {
		return Result{}, err
	}

	if primary.value.IsUndefined() {
		return newResult(
			dependencyUndefined(),
			provenance.NewOperationDecision(
				"Coalesce",
				provenance.DecisionPrimary,
				primary.provenance,
			),
		), nil
	}

	if !primary.value.IsNull() {
		normalized, err := s.normalizeConditionalResult(expression, context, primary.value)
		if err != nil {
			return Result{}, err
		}
		return newResult(
			normalized,
			provenance.NewOperationDecision(
				"Coalesce",
				provenance.DecisionPrimary,
				primary.provenance,
			),
		), nil
	}

	fallback, err :=
		s.evaluateExpression(
			expression.Fallback(),
			context,
		)

	if err != nil {
		return Result{}, err
	}

	resolved :=
		fallback.value

	if resolved.IsUndefined() {
		resolved =
			dependencyUndefined()
	}

	resolved, err = s.normalizeConditionalResult(expression, context, resolved)
	if err != nil {
		return Result{}, err
	}
	return newResult(
		resolved,
		provenance.NewOperationDecision(
			"Coalesce",
			provenance.DecisionFallback,
			primary.provenance,
			fallback.provenance,
		),
	), nil
}

// Normalize before parent operations consume a selected branch. Static inference
// inspects both branches without evaluating them or recording provenance.
func (s *evaluationState) normalizeConditionalResult(expression ast.Expression, context expressionContext, resolved enginevalue.Resolved) (enginevalue.Resolved, error) {
	staticContext, err := context.inferenceContext(s)
	if err != nil {
		return enginevalue.Resolved{}, err
	}
	expected, err := inference.Infer(expression, staticContext, s.version)
	if err != nil {
		return enginevalue.Resolved{}, newError(ErrorInvariantViolation, "conditional type inference failed", err)
	}
	return s.normalizeResolvedToStaticType(resolved, expected)
}

func invalidCommittedExpressionError() error {
	return newError(
		ErrorInvariantViolation,
		"committed model contains a structurally invalid expression",
		nil,
	)
}

func undefined(
	kind enginevalue.UndefinedKind,
) enginevalue.Resolved {
	return enginevalue.Undefined(
		enginevalue.NewUndefinedInfo(
			kind,
		),
	)
}

func dependencyUndefined() enginevalue.Resolved {
	return undefined(
		enginevalue.
			UndefinedDependencyUndefined,
	)
}
