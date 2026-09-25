package ast

import "github.com/shmatovv/bcg-engine/v2/internal/function"

// FunctionCall is a first-class semantic extension-function call. Its
// signature is model content looked up by Ref during inference/validation;
// the node itself intentionally contains only portable identity and ordered
// child expressions.
type FunctionCall struct {
	ref  function.Ref
	args []Expression
}

func NewFunctionCall(ref function.Ref, args []Expression) FunctionCall {
	if !ref.IsValid() {
		panic("ast: invalid function ref")
	}
	copy := append([]Expression(nil), args...)
	for _, arg := range copy {
		if arg == nil {
			panic("ast: nil function argument")
		}
	}
	return FunctionCall{ref: ref, args: copy}
}

func (f FunctionCall) Ref() function.Ref       { return f.ref }
func (f FunctionCall) Arguments() []Expression { return append([]Expression(nil), f.args...) }
func (f FunctionCall) IsValid() bool {
	if !f.ref.IsValid() {
		return false
	}
	for _, arg := range f.args {
		if arg == nil {
			return false
		}
	}
	return true
}
func (FunctionCall) expressionNode() {}
