package evaluator

import (
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/function"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func (s *evaluationState) evaluateFunctionCall(expression ast.FunctionCall, context expressionContext) (Result, error) {
	if !expression.IsValid() {
		return Result{}, invalidCommittedExpressionError()
	}
	lookup, supported := s.version.(interface {
		FunctionRequirement(function.Ref) (function.Requirement, bool)
	})
	if !supported {
		return Result{}, newError(ErrorFunctionRuntime, "evaluation source cannot resolve function requirements", nil)
	}
	requirement, ok := lookup.FunctionRequirement(expression.Ref())
	if !ok || !requirement.IsValid() {
		return Result{}, newError(ErrorFunctionRuntime, "function call has no committed requirement", nil)
	}
	arguments := expression.Arguments()
	values := make([]enginevalue.Typed, 0, len(arguments))
	children := make([]provenance.Node, 0, len(arguments))
	for _, argument := range arguments {
		result, err := s.evaluateExpression(argument, context)
		if err != nil {
			return Result{}, err
		}
		children = append(children, result.provenance)
		if result.value.IsUndefined() {
			return newResult(dependencyUndefined(), provenance.NewOperation("FunctionCall", children...)), nil
		}
		if result.value.IsNull() {
			return newResult(enginevalue.Null(), provenance.NewOperation("FunctionCall", children...)), nil
		}
		values = append(values, result.value.Value())
	}
	runtime := functionRuntimeFromSource(s.version)
	if runtime == nil {
		return Result{}, newError(ErrorFunctionRuntime, "function call requires an explicit compatible runtime", nil)
	}
	resolved, err := runtime.Invoke(requirement, values)
	if err != nil {
		return Result{}, newError(ErrorFunctionRuntime, "extension function invocation failed", err)
	}
	if !resolved.IsValid() {
		return Result{}, newError(ErrorFunctionRuntime, "extension runtime returned an invalid resolved value", nil)
	}
	return newResult(resolved, provenance.NewOperation("FunctionCall", children...)), nil
}
