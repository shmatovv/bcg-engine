package inference

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
)

// ErrorKind identifies a recursive static inference failure.
type ErrorKind uint8

const (
	ErrorInvalid ErrorKind = iota
	ErrorNilExpression
	ErrorUnsupportedExpression
	ErrorInvalidExpression
	ErrorChildInference
	ErrorTypecheck
)

// IsValid reports whether k is a valid inference error kind.
func (k ErrorKind) IsValid() bool {
	switch k {
	case ErrorNilExpression,
		ErrorUnsupportedExpression,
		ErrorInvalidExpression,
		ErrorChildInference,
		ErrorTypecheck:
		return true
	default:
		return false
	}
}

// String returns the canonical name of k.
func (k ErrorKind) String() string {
	switch k {
	case ErrorNilExpression:
		return "NilExpression"
	case ErrorUnsupportedExpression:
		return "UnsupportedExpression"
	case ErrorInvalidExpression:
		return "InvalidExpression"
	case ErrorChildInference:
		return "ChildInference"
	case ErrorTypecheck:
		return "Typecheck"
	default:
		return "Invalid"
	}
}

// Error describes a recursive static expression inference failure.
//
// cause preserves the lower-level error so callers may inspect it with
// errors.Is/errors.As.
type Error struct {
	kind  ErrorKind
	cause error
}

// Kind returns the inference error kind.
func (e Error) Kind() ErrorKind {
	return e.kind
}

// Unwrap returns the underlying inference/typecheck/reference error.
func (e Error) Unwrap() error {
	return e.cause
}

// Error implements error.
func (e Error) Error() string {
	switch e.kind {
	case ErrorNilExpression:
		return "inference: expression must not be nil"
	case ErrorUnsupportedExpression:
		return "inference: unsupported expression"
	case ErrorInvalidExpression:
		return "inference: invalid expression"
	case ErrorChildInference:
		return "inference: child expression inference failed"
	case ErrorTypecheck:
		return "inference: expression typecheck failed"
	default:
		return "inference: invalid inference error"
	}
}

// Infer determines the static type of expression in its lexical context.
// Aggregate inference introduces traversal bindings for filter and map;
// standalone contexts never acquire an implicit self.
func Infer(
	expression ast.Expression,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if expression == nil {
		return typing.Type{}, Error{
			kind: ErrorNilExpression,
		}
	}

	if resolver == nil {
		panic("inference: resolver must not be nil")
	}

	if !context.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
			cause: fmt.Errorf(
				"inference: invalid inference context",
			),
		}
	}

	depth, valid := ast.Depth(expression)
	if !valid {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}
	if depth > ast.MaxExpressionDepth {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
			cause: fmt.Errorf(
				"inference: expression nesting exceeds %d",
				ast.MaxExpressionDepth,
			),
		}
	}

	switch expression := expression.(type) {
	case ast.Constant:
		return inferConstant(expression)

	case ast.SelfPropertyReference:
		return InferSelfPropertyReference(
			expression,
			context,
			resolver,
		)

	case ast.AbsolutePropertyReference:
		return InferAbsolutePropertyReference(
			expression,
			resolver,
		)

	case ast.BoundPropertyDefinitionReference:
		return InferBoundPropertyDefinitionReference(
			expression,
			context,
			resolver,
		)

	case ast.BoundContractPropertyReference:
		return InferBoundContractPropertyReference(
			expression,
			context,
			resolver,
		)

	case ast.Arithmetic:
		return inferArithmetic(
			expression,
			context,
			resolver,
		)

	case ast.Negate:
		return inferNegate(
			expression,
			context,
			resolver,
		)

	case ast.Comparison:
		return inferComparison(
			expression,
			context,
			resolver,
		)

	case ast.Logical:
		return inferLogical(
			expression,
			context,
			resolver,
		)

	case ast.Not:
		return inferNot(
			expression,
			context,
			resolver,
		)

	case ast.If:
		return inferIf(
			expression,
			context,
			resolver,
		)

	case ast.IsNull:
		return inferIsNull(
			expression,
			context,
			resolver,
		)

	case ast.Coalesce:
		return inferCoalesce(
			expression,
			context,
			resolver,
		)

	case ast.Aggregate:
		return inferAggregate(
			expression,
			context,
			resolver,
		)

	default:
		return typing.Type{}, Error{
			kind: ErrorUnsupportedExpression,
			cause: fmt.Errorf(
				"inference: unsupported expression type %T",
				expression,
			),
		}
	}
}

func inferConstant(
	expression ast.Constant,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	typedValue := expression.Value()

	if typedValue.HasUnit() {
		return typing.NewWithUnit(
			typedValue.Type(),
			typedValue.Unit(),
		), nil
	}

	return typing.New(
		typedValue.Type(),
	), nil
}

func inferArithmetic(
	expression ast.Arithmetic,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	left, err := inferChild(
		expression.Left(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	right, err := inferChild(
		expression.Right(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	result, err := typecheck.InferArithmetic(
		expression.Operator(),
		left,
		right,
	)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

func inferNegate(
	expression ast.Negate,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	operand, err := inferChild(
		expression.Operand(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	result, err := typecheck.InferNegate(operand)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

func inferComparison(
	expression ast.Comparison,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	left, err := inferChild(
		expression.Left(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	right, err := inferChild(
		expression.Right(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	result, err := typecheck.InferComparison(
		expression.Operator(),
		left,
		right,
	)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

func inferLogical(
	expression ast.Logical,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	left, err := inferChild(
		expression.Left(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	right, err := inferChild(
		expression.Right(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	result, err := typecheck.InferLogical(
		left,
		right,
	)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

func inferNot(
	expression ast.Not,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	operand, err := inferChild(
		expression.Operand(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	result, err := typecheck.InferNot(operand)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

func inferIf(
	expression ast.If,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	condition, err := inferChild(
		expression.Condition(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	thenType, err := inferChild(
		expression.Then(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	elseType, err := inferChild(
		expression.Else(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	result, err := typecheck.InferIf(
		condition,
		thenType,
		elseType,
	)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

func inferIsNull(
	expression ast.IsNull,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	operand, err := inferChild(
		expression.Operand(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	result, err := typecheck.InferIsNull(operand)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

func inferCoalesce(
	expression ast.Coalesce,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, Error{
			kind: ErrorInvalidExpression,
		}
	}

	primary, err := inferChild(
		expression.Primary(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	fallback, err := inferChild(
		expression.Fallback(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, err
	}

	result, err := typecheck.InferCoalesce(
		primary,
		fallback,
	)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

func inferChild(
	expression ast.Expression,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	result, err := Infer(
		expression,
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, Error{
			kind:  ErrorChildInference,
			cause: err,
		}
	}

	return result, nil
}

var _ error = Error{}
