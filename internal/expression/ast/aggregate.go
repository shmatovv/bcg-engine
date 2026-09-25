package ast

// AggregateOperation identifies an aggregation operation.
type AggregateOperation uint8

const (
	AggregateOperationInvalid AggregateOperation = iota
	AggregateOperationSum
	AggregateOperationAverage
	AggregateOperationCount
	AggregateOperationMin
	AggregateOperationMax
	AggregateOperationExists
	AggregateOperationAny
	AggregateOperationAll
	AggregateOperationDistinctCount
)

// IsValid reports whether o is a valid aggregate operation.
func (o AggregateOperation) IsValid() bool {
	switch o {
	case AggregateOperationSum,
		AggregateOperationAverage,
		AggregateOperationCount,
		AggregateOperationMin,
		AggregateOperationMax,
		AggregateOperationExists,
		AggregateOperationAny,
		AggregateOperationAll,
		AggregateOperationDistinctCount:
		return true
	default:
		return false
	}
}

// RequiresMap reports whether the operation requires a map expression.
//
// Count operates directly on traversal items.
// Sum, Average, Min, Max, Any, All and DistinctCount operate on mapped values.
func (o AggregateOperation) RequiresMap() bool {
	switch o {
	case AggregateOperationSum,
		AggregateOperationAverage,
		AggregateOperationMin,
		AggregateOperationMax,
		AggregateOperationAny,
		AggregateOperationAll,
		AggregateOperationDistinctCount:
		return true
	default:
		return false
	}
}

// String returns the canonical name of o.
func (o AggregateOperation) String() string {
	switch o {
	case AggregateOperationSum:
		return "Sum"
	case AggregateOperationAverage:
		return "Average"
	case AggregateOperationCount:
		return "Count"
	case AggregateOperationMin:
		return "Min"
	case AggregateOperationMax:
		return "Max"
	case AggregateOperationExists:
		return "Exists"
	case AggregateOperationAny:
		return "Any"
	case AggregateOperationAll:
		return "All"
	case AggregateOperationDistinctCount:
		return "DistinctCount"
	default:
		return "Invalid"
	}
}

// Aggregate represents aggregation over a graph traversal.
//
// There are four valid construction forms:
//
//	COUNT(source)
//	COUNT(source, filter)
//
//	SUM/AVERAGE/MIN/MAX(source, map)
//	SUM/AVERAGE/MIN/MAX(source, filter, map)
//
// Evaluation order:
//
//	traverse
//	  -> applicability
//	  -> filter (when present)
//	  -> map (SUM/AVERAGE/MIN/MAX only)
//	  -> aggregate
//
// Count operates directly on traversal items remaining after
// applicability and filtering.
//
// Type, unit, Null, Undefined and empty-set semantics are handled
// by static validation and evaluation.
type Aggregate struct {
	source TraversalSource

	filter    Expression
	hasFilter bool

	mapExpr Expression
	hasMap  bool

	operation AggregateOperation
}

// NewCountAggregate constructs an unfiltered Count aggregate.
//
// Count operates directly on traversal items and does not have
// a map expression.
func NewCountAggregate(
	source TraversalSource,
) Aggregate {
	validateAggregateSource(source)

	return Aggregate{
		source:    source,
		operation: AggregateOperationCount,
	}
}

// NewFilteredCountAggregate constructs a filtered Count aggregate.
//
// filter is evaluated for each traversal item and must statically
// resolve to Boolean.
func NewFilteredCountAggregate(
	source TraversalSource,
	filter Expression,
) Aggregate {
	validateAggregateSource(source)
	validateAggregateFilter(filter)

	return Aggregate{
		source:    source,
		filter:    filter,
		hasFilter: true,
		operation: AggregateOperationCount,
	}
}

// NewCountLikeAggregate constructs a mapless Count or Exists aggregate.
func NewCountLikeAggregate(source TraversalSource, operation AggregateOperation) Aggregate {
	validateAggregateSource(source)
	if operation != AggregateOperationCount && operation != AggregateOperationExists {
		panic("ast: invalid mapless aggregate operation")
	}
	return Aggregate{source: source, operation: operation}
}

// NewFilteredCountLikeAggregate constructs a filtered Count or Exists aggregate.
func NewFilteredCountLikeAggregate(source TraversalSource, filter Expression, operation AggregateOperation) Aggregate {
	validateAggregateSource(source)
	validateAggregateFilter(filter)
	if operation != AggregateOperationCount && operation != AggregateOperationExists {
		panic("ast: invalid mapless aggregate operation")
	}
	return Aggregate{source: source, filter: filter, hasFilter: true, operation: operation}
}

// NewMappedAggregate constructs an unfiltered Sum, Average, Min or Max
// aggregate.
//
// mapExpr is evaluated for every traversal item.
func NewMappedAggregate(
	source TraversalSource,
	mapExpr Expression,
	operation AggregateOperation,
) Aggregate {
	validateMappedAggregateOperation(operation)
	validateAggregateSource(source)
	validateAggregateMap(mapExpr)

	return Aggregate{
		source:    source,
		mapExpr:   mapExpr,
		hasMap:    true,
		operation: operation,
	}
}

// NewFilteredMappedAggregate constructs a filtered Sum, Average, Min or
// Max aggregate.
//
// filter is evaluated before mapExpr.
func NewFilteredMappedAggregate(
	source TraversalSource,
	filter Expression,
	mapExpr Expression,
	operation AggregateOperation,
) Aggregate {
	validateMappedAggregateOperation(operation)
	validateAggregateSource(source)
	validateAggregateFilter(filter)
	validateAggregateMap(mapExpr)

	return Aggregate{
		source:    source,
		filter:    filter,
		hasFilter: true,
		mapExpr:   mapExpr,
		hasMap:    true,
		operation: operation,
	}
}

// Source returns the graph traversal source.
func (e Aggregate) Source() TraversalSource {
	return e.source
}

// Filter returns the optional filter expression.
func (e Aggregate) Filter() (Expression, bool) {
	return e.filter, e.hasFilter
}

// Map returns the optional map expression.
//
// Count has no map expression.
// Sum, Average, Min and Max always have one.
func (e Aggregate) Map() (Expression, bool) {
	return e.mapExpr, e.hasMap
}

// Operation returns the aggregate operation.
func (e Aggregate) Operation() AggregateOperation {
	return e.operation
}

// IsValid reports whether e is structurally valid.
func (e Aggregate) IsValid() bool {
	if !e.source.IsValid() || !e.operation.IsValid() {
		return false
	}

	if e.hasFilter {
		if e.filter == nil {
			return false
		}
	} else if e.filter != nil {
		return false
	}

	switch e.operation {
	case AggregateOperationCount, AggregateOperationExists:
		return !e.hasMap && e.mapExpr == nil

	case AggregateOperationSum,
		AggregateOperationAverage,
		AggregateOperationMin,
		AggregateOperationMax,
		AggregateOperationAny,
		AggregateOperationAll,
		AggregateOperationDistinctCount:
		return e.hasMap && e.mapExpr != nil

	default:
		return false
	}
}

func (Aggregate) expressionNode() {}

func validateAggregateSource(source TraversalSource) {
	if !source.IsValid() {
		panic("ast: aggregate source must be valid")
	}
}

func validateAggregateFilter(filter Expression) {
	if filter == nil {
		panic("ast: aggregate filter must not be nil")
	}
}

func validateAggregateMap(mapExpr Expression) {
	if mapExpr == nil {
		panic("ast: aggregate map expression must not be nil")
	}
}

func validateMappedAggregateOperation(operation AggregateOperation) {
	if !operation.IsValid() {
		panic("ast: aggregate operation must be valid")
	}

	if !operation.RequiresMap() {
		panic("ast: mapped aggregate operation must require a map expression")
	}
}
