package evaluator

import (
	"math/big"
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/inference"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// evaluateAggregate executes the Core v1 semantic graph aggregate pipeline:
//
//	traverse
//	  -> relation applicability
//	  -> filter
//	  -> map
//	  -> aggregate
//
// Traversal order is deterministic and follows Relation UUID order.
//
// Strict runtime semantics:
//
//	Sum(empty)     -> Defined(0)
//	Count(empty)   -> Defined(0)
//	Average(empty) -> Undefined(EmptyAggregation)
//	Min(empty)     -> Undefined(EmptyAggregation)
//	Max(empty)     -> Undefined(EmptyAggregation)
//	mapped Null    -> Null
//	mapped Undefined -> Undefined
//
// Null/Undefined applicability or filter predicates are not silently treated
// as false. They produce Undefined(IndeterminateCondition).
func (s *evaluationState) evaluateAggregate(
	expression ast.Aggregate,
	context expressionContext,
) (Result, error) {
	if !expression.IsValid() {
		return Result{},
			invalidCommittedExpressionError()
	}

	staticContext, err := context.inferenceContext(s)
	if err != nil {
		return Result{}, err
	}

	staticResultType, err := inference.Infer(
		expression,
		staticContext,
		s.version,
	)
	if err != nil || !staticResultType.IsValid() {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"committed aggregate could not be statically inferred at runtime",
				err,
			)
	}

	if staticResultType.TemporalMode() != typing.TemporalScalar {
		return Result{},
			newError(
				ErrorUnsupportedSeries,
				"Series aggregate evaluation is not implemented yet",
				nil,
			)
	}

	staticBinding, err := inference.BindingFromTraversal(
		expression.Source(),
		staticContext,
		s.version,
	)
	if err != nil || !staticBinding.IsValid() {
		return Result{},
			newError(
				ErrorInvariantViolation,
				"committed aggregate traversal binding could not be reconstructed at runtime",
				err,
			)
	}

	origin, err := s.aggregateTraversalOrigin(
		context,
		expression.Source(),
	)
	if err != nil {
		return Result{}, err
	}

	items, err := s.aggregateTraversalItems(
		origin,
		expression.Source(),
	)
	if err != nil {
		return Result{}, err
	}

	provenanceChildren := make(
		[]provenance.Node,
		0,
		len(items)*2,
	)

	mapped := make(
		[]enginevalue.Resolved,
		0,
		len(items),
	)

	count := int64(0)

	for _, item := range items {
		itemProvenance := make([]provenance.Node, 0, 3)

		relation, ok := s.version.Relation(
			item.relation.ID(),
		)
		if !ok || !relation.IsValid() {
			return Result{},
				newError(
					ErrorInvariantViolation,
					"traversed Relation could not be resolved from committed ModelVersion",
					nil,
				)
		}

		applicable,
			applicabilityProvenance,
			indeterminate,
			err := s.evaluateRelationApplicability(relation)
		if err != nil {
			return Result{}, err
		}

		if applicabilityProvenance.IsValid() {
			itemProvenance = append(
				itemProvenance,
				applicabilityProvenance,
			)
		}

		if indeterminate {
			provenanceChildren = append(provenanceChildren,
				aggregateItemProvenance(
					item,
					itemProvenance,
					undefined(enginevalue.UndefinedIndeterminateCondition),
				),
			)
			return newResult(
				undefined(
					enginevalue.UndefinedIndeterminateCondition,
				),
				provenance.NewOperation(
					"Aggregate"+expression.Operation().String(),
					provenanceChildren...,
				),
			), nil
		}

		if !applicable {
			provenanceChildren = append(provenanceChildren,
				aggregateItemProvenance(item, itemProvenance, boolResolved(false)),
			)
			continue
		}

		childContext := context.withBinding(
			item,
			staticBinding,
			staticContext,
		)

		if filterExpression, hasFilter := expression.Filter(); hasFilter {
			filterResult, err := s.evaluateExpression(
				filterExpression,
				childContext,
			)
			if err != nil {
				return Result{}, err
			}

			itemProvenance = append(
				itemProvenance,
				provenance.NewOperation(
					"AggregateFilter",
					filterResult.provenance,
				).WithResolved(filterResult.value),
			)

			passes,
				filterIndeterminate,
				err := aggregatePredicate(filterResult.value)
			if err != nil {
				return Result{}, err
			}

			if filterIndeterminate {
				provenanceChildren = append(provenanceChildren,
					aggregateItemProvenance(
						item,
						itemProvenance,
						undefined(enginevalue.UndefinedIndeterminateCondition),
					),
				)
				return newResult(
					undefined(
						enginevalue.UndefinedIndeterminateCondition,
					),
					provenance.NewOperation(
						"Aggregate"+expression.Operation().String(),
						provenanceChildren...,
					),
				), nil
			}

			if !passes {
				provenanceChildren = append(provenanceChildren,
					aggregateItemProvenance(item, itemProvenance, boolResolved(false)),
				)
				continue
			}
		}

		if expression.Operation() == ast.AggregateOperationCount {
			count++
			provenanceChildren = append(provenanceChildren,
				aggregateItemProvenance(item, itemProvenance, boolResolved(true)),
			)
			continue
		}

		mapExpression, hasMap := expression.Map()
		if !hasMap || mapExpression == nil {
			return Result{},
				newError(
					ErrorInvariantViolation,
					"mapped aggregate has no map expression",
					nil,
				)
		}

		mapResult, err := s.evaluateExpression(
			mapExpression,
			childContext,
		)
		if err != nil {
			return Result{}, err
		}

		itemProvenance = append(
			itemProvenance,
			provenance.NewOperation(
				"AggregateMap",
				mapResult.provenance,
			).WithResolved(mapResult.value),
		)

		if mapResult.value.IsNull() ||
			mapResult.value.IsUndefined() {
			provenanceChildren = append(provenanceChildren,
				aggregateItemProvenance(item, itemProvenance, mapResult.value),
			)
			return newResult(
				mapResult.value,
				provenance.NewOperation(
					"Aggregate"+expression.Operation().String(),
					provenanceChildren...,
				),
			), nil
		}

		mapped = append(
			mapped,
			mapResult.value,
		)

		provenanceChildren = append(provenanceChildren,
			aggregateItemProvenance(item, itemProvenance, mapResult.value),
		)
	}

	resolved, err := s.finishAggregate(
		expression.Operation(),
		staticResultType,
		mapped,
		count,
	)
	if err != nil {
		return Result{}, err
	}

	return newResult(
		resolved,
		provenance.NewOperation(
			"Aggregate"+expression.Operation().String(),
			provenanceChildren...,
		),
	), nil
}

func aggregateItemProvenance(
	item concreteExpressionBinding,
	children []provenance.Node,
	resolved enginevalue.Resolved,
) provenance.Node {
	if !item.IsValid() || !resolved.IsValid() {
		panic("evaluator: invalid aggregate item provenance")
	}

	return provenance.NewAggregateItem(
		item.relation,
		item.entity,
		children...,
	).WithResolved(resolved)
}

func (s *evaluationState) aggregateTraversalOrigin(
	context expressionContext,
	source ast.TraversalSource,
) (address.ObjectRef, error) {
	if !source.IsValid() {
		return address.ObjectRef{}, newError(
			ErrorInvariantViolation,
			"aggregate traversal source is invalid",
			nil,
		)
	}

	switch source.Origin() {
	case ast.TraversalOriginSelf:
		if !context.self.IsValid() ||
			context.self.Kind() != address.ObjectRefEntity {
			return address.ObjectRef{}, newError(
				ErrorInvariantViolation,
				"aggregate traversal requires Entity self",
				nil,
			)
		}
		return context.self, nil

	case ast.TraversalOriginAbsoluteEntity:
		origin, ok := source.AbsoluteOrigin()
		if !ok || !origin.IsValid() ||
			origin.Kind() != address.ObjectRefEntity {
			return address.ObjectRef{}, newError(
				ErrorInvariantViolation,
				"aggregate traversal explicit origin must be an Entity object",
				nil,
			)
		}
		return origin, nil

	case ast.TraversalOriginBoundEntity:
		origin, ok := context.boundObject(ast.BoundTargetEntity)
		if !ok || !origin.IsValid() ||
			origin.Kind() != address.ObjectRefEntity {
			return address.ObjectRef{}, newError(
				ErrorInvariantViolation,
				"aggregate traversal bound origin is unavailable",
				nil,
			)
		}
		return origin, nil

	default:
		return address.ObjectRef{}, newError(
			ErrorInvariantViolation,
			"aggregate traversal origin is invalid",
			nil,
		)
	}
}

func (s *evaluationState) evaluateRelationApplicability(
	relation semantic.Relation,
) (
	bool,
	provenance.Node,
	bool,
	error,
) {
	expression, hasApplicability := relation.Applicability()
	if !hasApplicability {
		return true,
			provenance.Node{},
			false,
			nil
	}

	relationType, ok := s.version.RelationType(relation.TypeID())
	if !ok || !relationType.IsValid() {
		return false,
			provenance.Node{},
			false,
			newError(
				ErrorInvariantViolation,
				"Relation applicability owner could not be resolved",
				nil,
			)
	}

	result, err := s.evaluateExpression(
		expression,
		newExpressionContext(
			address.RelationRef(relation.ID()),
			schema.RelationTypeOwner(relationType.ID()),
		),
	)
	if err != nil {
		return false,
			provenance.Node{},
			false,
			err
	}

	applicable,
		indeterminate,
		err := aggregatePredicate(result.value)
	if err != nil {
		return false,
			provenance.Node{},
			false,
			err
	}

	return applicable,
		provenance.NewOperation(
			"RelationApplicability",
			result.provenance,
		).WithResolved(result.value),
		indeterminate,
		nil
}

func aggregatePredicate(
	resolved enginevalue.Resolved,
) (
	bool,
	bool,
	error,
) {
	if !resolved.IsValid() {
		return false,
			false,
			newError(
				ErrorInvariantViolation,
				"aggregate predicate produced invalid resolved value",
				nil,
			)
	}

	if resolved.IsNull() ||
		resolved.IsUndefined() {
		return false, true, nil
	}

	typed := resolved.Value()

	if typed.Type() != enginevalue.Bool ||
		typed.HasUnit() {
		return false,
			false,
			newError(
				ErrorInvariantViolation,
				"validated aggregate predicate did not evaluate to unitless Bool",
				nil,
			)
	}

	return typed.Raw().Bool(),
		false,
		nil
}

func (s *evaluationState) finishAggregate(
	operation ast.AggregateOperation,
	staticType typing.Type,
	mapped []enginevalue.Resolved,
	count int64,
) (enginevalue.Resolved, error) {
	switch operation {
	case ast.AggregateOperationCount:
		return enginevalue.Defined(
			enginevalue.NewTyped(
				enginevalue.NewInt64(count),
			),
		), nil

	case ast.AggregateOperationSum:
		if len(mapped) == 0 {
			return zeroResolvedForStaticType(staticType)
		}

		result := mapped[0]

		for _, current := range mapped[1:] {
			combined, err := s.evaluateArithmetic(
				ast.ArithmeticAdd,
				result,
				current,
			)
			if err != nil {
				return enginevalue.Resolved{}, err
			}

			result = combined
		}

		return s.normalizeResolvedToStaticType(
			result,
			staticType,
		)

	case ast.AggregateOperationAverage:
		if len(mapped) == 0 {
			return undefined(
				enginevalue.UndefinedEmptyAggregation,
			), nil
		}

		sum := mapped[0]

		for _, current := range mapped[1:] {
			combined, err := s.evaluateArithmetic(
				ast.ArithmeticAdd,
				sum,
				current,
			)
			if err != nil {
				return enginevalue.Resolved{}, err
			}

			sum = combined
		}

		divisor := enginevalue.Defined(
			enginevalue.NewTyped(
				enginevalue.NewInteger(
					big.NewInt(int64(len(mapped))),
				),
			),
		)

		average, err := s.evaluateArithmetic(
			ast.ArithmeticDivide,
			sum,
			divisor,
		)
		if err != nil {
			return enginevalue.Resolved{}, err
		}

		return s.normalizeResolvedToStaticType(
			average,
			staticType,
		)

	case ast.AggregateOperationMin,
		ast.AggregateOperationMax:

		if len(mapped) == 0 {
			return undefined(
				enginevalue.UndefinedEmptyAggregation,
			), nil
		}

		selected := mapped[0]

		for _, current := range mapped[1:] {
			comparison, err := compareDefined(
				current.Value(),
				selected.Value(),
			)
			if err != nil {
				return enginevalue.Resolved{}, err
			}

			if operation == ast.AggregateOperationMin &&
				comparison < 0 {
				selected = current
			}

			if operation == ast.AggregateOperationMax &&
				comparison > 0 {
				selected = current
			}
		}

		return s.normalizeResolvedToStaticType(
			selected,
			staticType,
		)

	default:
		return enginevalue.Resolved{},
			newError(
				ErrorInvariantViolation,
				"committed aggregate has invalid operation",
				nil,
			)
	}
}

func (s *evaluationState) aggregateTraversalItems(
	self address.ObjectRef,
	source ast.TraversalSource,
) ([]concreteExpressionBinding, error) {
	if !self.IsValid() ||
		self.Kind() != address.ObjectRefEntity ||
		!source.IsValid() {
		return nil,
			newError(
				ErrorInvariantViolation,
				"invalid aggregate traversal runtime input",
				nil,
			)
	}

	if relationType, ok := s.version.RelationType(
		source.RelationTypeID(),
	); !ok || !relationType.IsValid() {
		return nil,
			newError(
				ErrorInvariantViolation,
				"aggregate traversal RelationType could not be resolved",
				nil,
			)
	}

	selfID := self.ID()

	switch source.Direction() {
	case ast.TraversalDirectionOutgoing:
		return aggregateBindingsFromRelations(
			s.version.OutgoingRelations(selfID, source.RelationTypeID()),
			true,
		), nil

	case ast.TraversalDirectionIncoming:
		return aggregateBindingsFromRelations(
			s.version.IncomingRelations(selfID, source.RelationTypeID()),
			false,
		), nil

	case ast.TraversalDirectionBoth:
		return aggregateBindingsFromBothDirections(
			s.version.OutgoingRelations(selfID, source.RelationTypeID()),
			s.version.IncomingRelations(selfID, source.RelationTypeID()),
		), nil

	default:
		return nil, newError(
			ErrorInvariantViolation,
			"aggregate traversal direction is invalid",
			nil,
		)
	}
}

func aggregateBindingsFromRelations(
	relations []semantic.Relation,
	outgoing bool,
) []concreteExpressionBinding {
	result := make([]concreteExpressionBinding, 0, len(relations))
	for _, relation := range relations {
		entityID := relation.From()
		if outgoing {
			entityID = relation.To()
		}
		result = append(result, newAggregateBinding(relation, entityID))
	}
	return result
}

func aggregateBindingsFromBothDirections(
	outgoing []semantic.Relation,
	incoming []semantic.Relation,
) []concreteExpressionBinding {
	type item struct {
		binding concreteExpressionBinding
	}

	items := make([]item, 0, len(outgoing)+len(incoming))
	seen := make(map[identity.UUID]struct{}, len(outgoing)+len(incoming))
	for _, relation := range outgoing {
		items = append(items, item{binding: newAggregateBinding(relation, relation.To())})
		seen[relation.ID()] = struct{}{}
	}
	for _, relation := range incoming {
		// The same relation is in both indexes only for a self-loop. Parallel
		// relations have distinct UUIDs and therefore remain distinct items.
		if _, exists := seen[relation.ID()]; exists {
			continue
		}
		items = append(items, item{binding: newAggregateBinding(relation, relation.From())})
	}

	sort.Slice(items, func(i, j int) bool {
		return compareEvaluatorUUID(
			items[i].binding.relation.ID(),
			items[j].binding.relation.ID(),
		) < 0
	})

	result := make([]concreteExpressionBinding, 0, len(items))
	for _, item := range items {
		result = append(result, item.binding)
	}
	return result
}

func newAggregateBinding(
	relation semantic.Relation,
	entityID identity.UUID,
) concreteExpressionBinding {
	if !relation.IsValid() ||
		entityID.IsZero() {
		panic("evaluator: invalid aggregate traversal binding")
	}

	return concreteExpressionBinding{
		relation: address.RelationRef(
			relation.ID(),
		),
		entity: address.EntityRef(
			entityID,
		),
	}
}

func compareEvaluatorUUID(
	left identity.UUID,
	right identity.UUID,
) int {
	leftBytes := left.Bytes()
	rightBytes := right.Bytes()

	for index := range leftBytes {
		switch {
		case leftBytes[index] < rightBytes[index]:
			return -1
		case leftBytes[index] > rightBytes[index]:
			return 1
		}
	}

	return 0
}
