package typecheck

import (
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// AggregateErrorKind identifies why aggregate type inference failed.
type AggregateErrorKind uint8

const (
	AggregateErrorInvalid AggregateErrorKind = iota
	AggregateErrorInvalidOperation
	AggregateErrorInvalidMapType
	AggregateErrorInvalidFilterType
	AggregateErrorNonScalarFilter
)

// IsValid reports whether k is a valid aggregate error kind.
func (k AggregateErrorKind) IsValid() bool {
	switch k {
	case AggregateErrorInvalidOperation,
		AggregateErrorInvalidMapType,
		AggregateErrorInvalidFilterType,
		AggregateErrorNonScalarFilter:
		return true
	default:
		return false
	}
}

// String returns the canonical name of k.
func (k AggregateErrorKind) String() string {
	switch k {
	case AggregateErrorInvalidOperation:
		return "InvalidOperation"
	case AggregateErrorInvalidMapType:
		return "InvalidMapType"
	case AggregateErrorInvalidFilterType:
		return "InvalidFilterType"
	case AggregateErrorNonScalarFilter:
		return "NonScalarFilter"
	default:
		return "Invalid"
	}
}

// AggregateError describes a static aggregate typing failure.
type AggregateError struct {
	kind AggregateErrorKind
}

// Kind returns the aggregate error kind.
func (e AggregateError) Kind() AggregateErrorKind {
	return e.kind
}

// Error implements error.
func (e AggregateError) Error() string {
	switch e.kind {
	case AggregateErrorInvalidOperation:
		return "typecheck: invalid aggregate operation"
	case AggregateErrorInvalidMapType:
		return "typecheck: aggregate map type is invalid"
	case AggregateErrorInvalidFilterType:
		return "typecheck: aggregate filter must be boolean"
	case AggregateErrorNonScalarFilter:
		return "typecheck: aggregate filter must be scalar"
	default:
		return "typecheck: invalid aggregate error"
	}
}

// InferAggregate determines the static result type of a semantic-graph
// aggregate.
//
// Aggregate operates over related graph elements. It does not implicitly
// aggregate or resample the temporal axis.
//
// Therefore map TemporalMode is preserved:
//
//	SUM(Scalar<Decimal<RUB>>) -> Scalar<Decimal<RUB>>
//	SUM(Series<Decimal<RUB>>) -> Series<Decimal<RUB>>
//
// COUNT does not consume a map expression and always produces a non-nullable
// Scalar<Integer>.
//
// SUM requires a numeric map expression and preserves ValueType, Unit,
// TemporalMode and Nullability.
//
// AVERAGE requires a numeric map expression and promotes the ValueType to
// Decimal while preserving Unit, TemporalMode and Nullability.
//
// MIN and MAX preserve the complete map type.
//
// Empty-set Undefined semantics are runtime evaluation outcomes and do not
// make the static result Nullable:
//
//	SUM(empty)     -> Defined(0)
//	COUNT(empty)   -> Defined(0)
//	AVERAGE(empty) -> Undefined(EmptyAggregation)
//	MIN(empty)     -> Undefined(EmptyAggregation)
//	MAX(empty)     -> Undefined(EmptyAggregation)
//
// A nullable mapped value does, however, make SUM/AVERAGE/MIN/MAX statically
// nullable because mapped Null propagates according to Core v1 semantics.
func InferAggregate(
	operation ast.AggregateOperation,
	mapType typing.Type,
	hasMap bool,
) (typing.Type, error) {
	if !operation.IsValid() {
		return typing.Type{}, AggregateError{
			kind: AggregateErrorInvalidOperation,
		}
	}

	switch operation {
	case ast.AggregateOperationCount:
		if hasMap {
			return typing.Type{}, AggregateError{
				kind: AggregateErrorInvalidMapType,
			}
		}

		return typing.New(value.Integer), nil

	case ast.AggregateOperationSum:
		if !hasMap ||
			!isSummableAggregateType(mapType) {
			return typing.Type{}, AggregateError{
				kind: AggregateErrorInvalidMapType,
			}
		}

		return mapType, nil

	case ast.AggregateOperationAverage:
		if !hasMap ||
			!isAverageableAggregateType(mapType) {
			return typing.Type{}, AggregateError{
				kind: AggregateErrorInvalidMapType,
			}
		}

		return averageResultType(mapType), nil

	case ast.AggregateOperationMin,
		ast.AggregateOperationMax:
		if !hasMap ||
			!isOrderedAggregateType(mapType) {
			return typing.Type{}, AggregateError{
				kind: AggregateErrorInvalidMapType,
			}
		}

		return mapType, nil

	default:
		return typing.Type{}, AggregateError{
			kind: AggregateErrorInvalidOperation,
		}
	}
}

// CheckAggregateFilter validates the static type of an aggregate filter.
//
// An absent filter is valid.
//
// A present filter must be:
//
//   - unitless Bool;
//   - Scalar.
//
// Nullable<Bool> is allowed. Runtime three-valued predicate semantics decide
// what a Null filter result means for a concrete traversal item.
//
// Series<Bool> is rejected because graph filtering requires one predicate per
// traversed graph item. Implicit temporal reduction is forbidden.
func CheckAggregateFilter(
	filterType typing.Type,
	hasFilter bool,
) error {
	if !hasFilter {
		return nil
	}

	if !isBoolean(filterType) {
		return AggregateError{
			kind: AggregateErrorInvalidFilterType,
		}
	}

	if filterType.TemporalMode() != typing.TemporalScalar {
		return AggregateError{
			kind: AggregateErrorNonScalarFilter,
		}
	}

	return nil
}

func isSummableAggregateType(
	t typing.Type,
) bool {
	if !t.IsValid() {
		return false
	}

	return IsNumeric(t.ValueType())
}

func isAverageableAggregateType(
	t typing.Type,
) bool {
	if !t.IsValid() {
		return false
	}

	return IsNumeric(t.ValueType())
}

func averageResultType(
	t typing.Type,
) typing.Type {
	u, hasUnit := t.Unit()

	var result typing.Type

	if hasUnit {
		result = typing.NewWithUnit(
			value.Decimal,
			u,
		)
	} else {
		result = typing.New(value.Decimal)
	}

	return result.
		WithTemporalMode(t.TemporalMode()).
		WithNullable(t.Nullable())
}

func isOrderedAggregateType(
	t typing.Type,
) bool {
	if !t.IsValid() {
		return false
	}

	if IsNumeric(t.ValueType()) {
		return true
	}

	if t.HasUnit() {
		return false
	}

	switch t.ValueType() {
	case value.Date,
		value.DateTime,
		value.Duration:
		return true
	default:
		return false
	}
}
