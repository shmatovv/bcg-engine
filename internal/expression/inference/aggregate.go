package inference

import (
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/typecheck"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
)

// AggregateInferenceErrorKind identifies why recursive aggregate inference
// failed.
type AggregateInferenceErrorKind uint8

const (
	AggregateInferenceErrorInvalid AggregateInferenceErrorKind = iota
	AggregateInferenceErrorInvalidExpression
	AggregateInferenceErrorTraversal
	AggregateInferenceErrorFilter
	AggregateInferenceErrorMap
	AggregateInferenceErrorTypecheck
)

// IsValid reports whether k is a valid aggregate inference error kind.
func (k AggregateInferenceErrorKind) IsValid() bool {
	switch k {
	case AggregateInferenceErrorInvalidExpression,
		AggregateInferenceErrorTraversal,
		AggregateInferenceErrorFilter,
		AggregateInferenceErrorMap,
		AggregateInferenceErrorTypecheck:
		return true
	default:
		return false
	}
}

// String returns the canonical name of k.
func (k AggregateInferenceErrorKind) String() string {
	switch k {
	case AggregateInferenceErrorInvalidExpression:
		return "InvalidExpression"
	case AggregateInferenceErrorTraversal:
		return "Traversal"
	case AggregateInferenceErrorFilter:
		return "Filter"
	case AggregateInferenceErrorMap:
		return "Map"
	case AggregateInferenceErrorTypecheck:
		return "Typecheck"
	default:
		return "Invalid"
	}
}

// AggregateInferenceError describes a recursive aggregate inference failure.
//
// cause preserves the exact lower-level error for errors.Is/errors.As.
type AggregateInferenceError struct {
	kind  AggregateInferenceErrorKind
	cause error
}

// Kind returns the aggregate inference error kind.
func (e AggregateInferenceError) Kind() AggregateInferenceErrorKind {
	return e.kind
}

// Unwrap returns the underlying failure.
func (e AggregateInferenceError) Unwrap() error {
	return e.cause
}

// Error implements error.
func (e AggregateInferenceError) Error() string {
	switch e.kind {
	case AggregateInferenceErrorInvalidExpression:
		return "inference: invalid aggregate expression"
	case AggregateInferenceErrorTraversal:
		return "inference: aggregate traversal inference failed"
	case AggregateInferenceErrorFilter:
		return "inference: aggregate filter inference failed"
	case AggregateInferenceErrorMap:
		return "inference: aggregate map inference failed"
	case AggregateInferenceErrorTypecheck:
		return "inference: aggregate typecheck failed"
	default:
		return "inference: invalid aggregate inference error"
	}
}

// inferAggregate recursively determines the static result type of aggregate.
//
// Aggregate inference is special because filter and map expressions are
// evaluated inside a traversal binding environment.
//
// The parent context is therefore extended with:
//
//   - bound.Relation;
//   - bound.Entity.
//
// Existing self remains unchanged.
//
// Evaluation/type-inference order mirrors aggregate semantics:
//
//  1. derive traversal binding;
//  2. infer and validate optional filter;
//  3. infer optional map;
//  4. infer aggregate result type.
func inferAggregate(
	expression ast.Aggregate,
	context Context,
	resolver Resolver,
) (typing.Type, error) {
	if !expression.IsValid() {
		return typing.Type{}, AggregateInferenceError{
			kind: AggregateInferenceErrorInvalidExpression,
		}
	}

	binding, err := BindingFromTraversal(
		expression.Source(),
		context,
		resolver,
	)
	if err != nil {
		return typing.Type{}, AggregateInferenceError{
			kind:  AggregateInferenceErrorTraversal,
			cause: err,
		}
	}

	childContext := context.WithBinding(binding)

	filterExpression, hasFilter := expression.Filter()

	if hasFilter {
		filterType, err := Infer(
			filterExpression,
			childContext,
			resolver,
		)
		if err != nil {
			return typing.Type{}, AggregateInferenceError{
				kind:  AggregateInferenceErrorFilter,
				cause: err,
			}
		}

		if err := typecheck.CheckAggregateFilter(
			filterType,
			true,
		); err != nil {
			return typing.Type{}, AggregateInferenceError{
				kind:  AggregateInferenceErrorTypecheck,
				cause: err,
			}
		}
	}

	mapExpression, hasMap := expression.Map()

	var mapType typing.Type

	if hasMap {
		inferredMapType, err := Infer(
			mapExpression,
			childContext,
			resolver,
		)
		if err != nil {
			return typing.Type{}, AggregateInferenceError{
				kind:  AggregateInferenceErrorMap,
				cause: err,
			}
		}

		mapType = inferredMapType
	}

	result, err := typecheck.InferAggregate(
		expression.Operation(),
		mapType,
		hasMap,
	)
	if err != nil {
		return typing.Type{}, AggregateInferenceError{
			kind:  AggregateInferenceErrorTypecheck,
			cause: err,
		}
	}

	return result, nil
}

var _ error = AggregateInferenceError{}
