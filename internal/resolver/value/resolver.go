package value

import (
	"errors"
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	"github.com/shmatovv/bcg-engine/v2/internal/types/series"
	"github.com/shmatovv/bcg-engine/v2/internal/types/unit"
)

// Resolve resolves one Input PropertyDefinition for the supplied committed
// ModelVersion and CalculationContext.
//
// Resolution precedence in v1:
//
//  1. Calculation override
//  2. Scenario override
//  3. Base object ValueBinding
//  4. Base type ValueBinding
//  5. Undefined(MissingValue)
//
// Within every layer:
//
//	scope matching
//	-> time matching
//	-> scope specificity
//	-> ambiguity detection
//
// Equal-specificity candidates are never resolved by insertion or map order.
func Resolve(
	version ModelSource,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Resolution, error) {
	if !version.IsCommitted() {
		return Resolution{},
			newError(
				ErrorInvalidModelVersion,
				"value resolution requires a committed ModelVersion",
			)
	}

	if !target.IsValid() {
		return Resolution{},
			newError(
				ErrorInvalidPropertyAddress,
				"target property address is invalid",
			)
	}

	if err := ValidateContext(version, context); err != nil {
		return Resolution{}, err
	}

	return ResolveValidated(version, target, context)
}

// ResolveValidated resolves an Input property after the caller has already
// validated this exact committed ModelVersion and CalculationContext with
// ValidateContext. It retains all concrete target and resolution checks but
// intentionally avoids repeatedly validating every override while evaluating
// a large, single effective plan.
//
// It is an internal execution API. Callers handling arbitrary runtime input
// must use Resolve instead.
func ResolveValidated(
	version ModelSource,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Resolution, error) {
	if !version.IsCommitted() {
		return Resolution{},
			newError(
				ErrorInvalidModelVersion,
				"value resolution requires a committed ModelVersion",
			)
	}

	if !target.IsValid() {
		return Resolution{},
			newError(
				ErrorInvalidPropertyAddress,
				"target property address is invalid",
			)
	}

	owner, err :=
		resolveObjectOwner(
			version,
			target.Object(),
		)

	if err != nil {
		return Resolution{},
			err
	}

	definition, ok :=
		version.PropertyDefinition(
			target.DefinitionID(),
		)

	if !ok ||
		!definition.IsValid() {

		return Resolution{},
			newError(
				ErrorDefinitionNotFound,
				"target property definition could not be resolved",
			)
	}

	if definition.Owner() !=
		owner {

		return Resolution{},
			newError(
				ErrorDefinitionNotAvailable,
				"target property definition is not available on the concrete object type",
			)
	}

	if definition.
		Computation().
		Kind() !=
		schema.ComputationInput {

		return Resolution{},
			newError(
				ErrorNotInputProperty,
				"base value resolver can only resolve Input properties",
			)
	}

	if definition.TemporalMode() !=
		schema.TemporalScalar {

		return Resolution{},
			newError(
				ErrorUnsupportedSeriesProperty,
				"scalar Resolve does not support Series properties; use ResolveSeries",
			)
	}

	if override, ok :=
		context.Override(
			target,
		); ok {

		if err :=
			validateOverride(
				version,
				definition,
				override,
			); err != nil {

			return Resolution{},
				err
		}

		outcome :=
			newSelectedOutcome(
				resolvedFromBindingValue(
					override.Value(),
				),
				calculationOverrideSource(
					override,
				),
			)

		switch context.
			TimeContext().
			Kind() {

		case domaintime.ContextAt:
			return newAtResolution(
				outcome,
			), nil

		case domaintime.ContextPeriod:
			from,
				to,
				ok :=
				context.
					TimeContext().
					Period()

			if !ok {
				return Resolution{},
					newError(
						ErrorInvalidCalculationContext,
						"Period calculation context has invalid bounds",
					)
			}

			return newPeriodResolution(
				[]Segment{
					newSegment(
						from,
						to,
						outcome,
					),
				},
			), nil

		default:
			return Resolution{},
				newError(
					ErrorInvalidCalculationContext,
					"calculation time context kind is invalid",
				)
		}
	}

	var scenarioCandidates []scenarioCandidate

	if selectedScenario, ok :=
		context.Scenario(); ok {

		scenarioCandidates =
			collectScenarioCandidates(
				selectedScenario,
				target,
			)
	}

	objectCandidates,
		typeCandidates :=
		collectBaseCandidates(
			version,
			target,
			owner,
		)

	scopeValues :=
		context.ScopeValues()

	scenarioCandidates =
		filterScenarioByScope(
			scenarioCandidates,
			scopeValues,
		)

	objectCandidates =
		filterByScope(
			objectCandidates,
			scopeValues,
		)

	typeCandidates =
		filterByScope(
			typeCandidates,
			scopeValues,
		)

	timeContext :=
		context.TimeContext()

	if instant, ok :=
		timeContext.At(); ok {

		return newAtResolution(
			selectAt(
				scenarioCandidates,
				objectCandidates,
				typeCandidates,
				instant,
			),
		), nil
	}

	from,
		to,
		ok :=
		timeContext.Period()

	if !ok {
		return Resolution{},
			newError(
				ErrorInvalidCalculationContext,
				"calculation time context is neither At nor Period",
			)
	}

	segments :=
		resolvePeriod(
			scenarioCandidates,
			objectCandidates,
			typeCandidates,
			from,
			to,
		)

	return newPeriodResolution(
		segments,
	), nil
}

func validateScopeContext(
	version ModelSource,
	context runtime.CalculationContext,
) error {
	for _, selector := range context.
		ScopeValues().
		Selectors() {

		definition, ok :=
			version.ScopeDefinition(
				selector.DefinitionID(),
			)

		if !ok ||
			!definition.IsValid() {

			return newError(
				ErrorInvalidCalculationContext,
				"calculation context references an unknown scope definition",
			)
		}

		if selector.
			Value().
			Type() !=
			definition.ValueType() {

			return newError(
				ErrorInvalidCalculationContext,
				"calculation scope value type does not match its scope definition",
			)
		}
	}

	return nil
}

func resolveObjectOwner(
	version ModelSource,
	object address.ObjectRef,
) (schema.PropertyOwner, error) {
	switch object.Kind() {
	case address.ObjectRefEntity:
		entity, ok :=
			version.Entity(
				object.ID(),
			)

		if !ok ||
			!entity.IsValid() {

			return schema.PropertyOwner{},
				newError(
					ErrorObjectNotFound,
					"target entity could not be resolved",
				)
		}

		entityType, ok :=
			version.EntityType(
				entity.TypeID(),
			)

		if !ok ||
			entityType.ID() != entity.TypeID() {

			return schema.PropertyOwner{},
				newError(
					ErrorInvalidModelVersion,
					"target entity type could not be resolved from committed model",
				)
		}

		return schema.EntityTypeOwner(
			entityType.ID(),
		), nil

	case address.ObjectRefRelation:
		relation, ok :=
			version.Relation(
				object.ID(),
			)

		if !ok ||
			!relation.IsValid() {

			return schema.PropertyOwner{},
				newError(
					ErrorObjectNotFound,
					"target relation could not be resolved",
				)
		}

		relationType, ok :=
			version.RelationType(
				relation.TypeID(),
			)

		if !ok ||
			relationType.ID() != relation.TypeID() {

			return schema.PropertyOwner{},
				newError(
					ErrorInvalidModelVersion,
					"target relation type could not be resolved from committed model",
				)
		}

		return schema.RelationTypeOwner(
			relationType.ID(),
		), nil

	default:
		return schema.PropertyOwner{},
			newError(
				ErrorInvalidPropertyAddress,
				"target object reference kind is invalid",
			)
	}
}

func validateOverride(
	version ModelSource,
	definition schema.PropertyDefinition,
	override runtime.Override,
) error {
	if !override.IsValid() {
		return newError(
			ErrorInvalidCalculationOverride,
			"calculation override is structurally invalid",
		)
	}

	bindingValue :=
		override.Value()

	switch bindingValue.Kind() {
	case semantic.BindingValueDefined:
		if definition.TemporalMode() != schema.TemporalScalar {
			return newError(ErrorInvalidCalculationOverride, "scalar calculation override cannot target a Series property")
		}
		defined, ok :=
			bindingValue.Value()

		if !ok ||
			!defined.IsValid() {

			return newError(
				ErrorInvalidCalculationOverride,
				"defined calculation override value is invalid",
			)
		}

		if defined.Type() !=
			definition.ValueType() {

			return newError(
				ErrorInvalidCalculationOverride,
				"calculation override value type does not match property definition",
			)
		}

		expectedUnit,
			expectedHasUnit :=
			definition.Unit()

		if defined.HasUnit() !=
			expectedHasUnit {

			return newError(
				ErrorInvalidCalculationOverride,
				"calculation override unit presence does not match property definition",
			)
		}

		if expectedHasUnit &&
			!defined.
				Unit().
				Equal(expectedUnit) {

			return newError(
				ErrorInvalidCalculationOverride,
				"calculation override unit does not exactly match property definition unit",
			)
		}

		return nil

	case semantic.BindingValueSeries:
		stored, ok := bindingValue.Series()
		if !ok || !stored.IsValid() {
			return newError(ErrorInvalidCalculationOverride, "Series calculation override value is invalid")
		}
		if definition.TemporalMode() != schema.TemporalSeries {
			return newError(ErrorInvalidCalculationOverride, "Series calculation override cannot target a Scalar property")
		}
		if err := stored.ValidateAgainst(definition.ValueType(), calculationOverrideUnit(definition), calculationOverrideHasUnit(definition), calculationOverrideAllowsNull(version, definition)); err != nil {
			switch {
			case errors.Is(err, series.ErrElementTypeMismatch):
				return newError(ErrorInvalidCalculationOverride, "Series calculation override element type does not match property definition")
			case errors.Is(err, series.ErrUnitMismatch):
				return newError(ErrorInvalidCalculationOverride, "Series calculation override unit does not exactly match property definition")
			case errors.Is(err, series.ErrNullNotAllowed):
				return newError(ErrorInvalidCalculationOverride, "Null Series sample is not allowed by property contract")
			default:
				return newError(ErrorInvalidCalculationOverride, "Series calculation override value is invalid")
			}
		}
		return nil

	case semantic.BindingValueNull:
		if definition.TemporalMode() == schema.TemporalSeries {
			return newError(ErrorInvalidCalculationOverride, "whole-Series Null calculation overrides are not supported")
		}
		contractID,
			hasContract :=
			definition.ContractID()

		if !hasContract {
			return newError(
				ErrorInvalidCalculationOverride,
				"Null calculation override requires a nullable property contract",
			)
		}

		contract, ok :=
			version.PropertyContract(
				contractID,
			)

		if !ok ||
			!contract.IsValid() {

			return newError(
				ErrorInvalidModelVersion,
				"property contract could not be resolved from committed model",
			)
		}

		if !contract.Nullable() {
			return newError(
				ErrorInvalidCalculationOverride,
				"property contract does not allow Null calculation overrides",
			)
		}

		return nil

	default:
		return newError(
			ErrorInvalidCalculationOverride,
			"calculation override value kind is invalid",
		)
	}
}

func calculationOverrideUnit(definition schema.PropertyDefinition) unit.Unit {
	u, _ := definition.Unit()
	return u
}

func calculationOverrideHasUnit(definition schema.PropertyDefinition) bool {
	_, hasUnit := definition.Unit()
	return hasUnit
}

func calculationOverrideAllowsNull(version ModelSource, definition schema.PropertyDefinition) bool {
	contractID, hasContract := definition.ContractID()
	if !hasContract {
		return false
	}
	contract, ok := version.PropertyContract(contractID)
	return ok && contract.IsValid() && contract.Nullable()
}

func collectBaseCandidates(
	version ModelSource,
	target address.PropertyAddress,
	owner schema.PropertyOwner,
) ([]candidate, []candidate) {
	objectBindings := version.ValueBindingsForTarget(semantic.ObjectPropertyTarget(target))
	typeBindings := version.ValueBindingsForTarget(semantic.TypePropertyTarget(owner, target.DefinitionID()))
	objectCandidates := make([]candidate, 0, len(objectBindings))
	typeCandidates := make([]candidate, 0, len(typeBindings))
	for _, binding := range objectBindings {
		objectCandidates = append(objectCandidates, newCandidate(binding, SourceBaseObjectBinding))
	}
	for _, binding := range typeBindings {
		typeCandidates = append(typeCandidates, newCandidate(binding, SourceBaseTypeBinding))
	}
	return objectCandidates, typeCandidates
}

func resolvePeriod(
	scenarioCandidates []scenarioCandidate,
	objectCandidates []candidate,
	typeCandidates []candidate,
	from datetime.DateTime,
	to datetime.DateTime,
) []Segment {
	boundaries :=
		[]datetime.DateTime{
			from,
			to,
		}

	boundaries =
		appendScenarioCandidateBoundaries(
			boundaries,
			scenarioCandidates,
			from,
			to,
		)

	boundaries =
		appendCandidateBoundaries(
			boundaries,
			objectCandidates,
			from,
			to,
		)

	boundaries =
		appendCandidateBoundaries(
			boundaries,
			typeCandidates,
			from,
			to,
		)

	sort.Slice(
		boundaries,
		func(i, j int) bool {
			return boundaries[i].
				Cmp(
					boundaries[j],
				) < 0
		},
	)

	boundaries =
		uniqueDateTimes(
			boundaries,
		)

	segments :=
		make(
			[]Segment,
			0,
			len(boundaries)-1,
		)

	for index := 0; index+1 < len(boundaries); index++ {

		segmentFrom :=
			boundaries[index]

		segmentTo :=
			boundaries[index+1]

		if segmentFrom.
			Cmp(segmentTo) >= 0 {

			continue
		}

		outcome :=
			selectAt(
				scenarioCandidates,
				objectCandidates,
				typeCandidates,
				segmentFrom,
			)

		segments =
			append(
				segments,
				newSegment(
					segmentFrom,
					segmentTo,
					outcome,
				),
			)
	}

	if len(segments) == 0 {
		panic(
			"resolver/value: valid calculation Period produced no segments",
		)
	}

	return segments
}

func appendScenarioCandidateBoundaries(
	boundaries []datetime.DateTime,
	candidates []scenarioCandidate,
	calculationFrom datetime.DateTime,
	calculationTo datetime.DateTime,
) []datetime.DateTime {
	for _, candidate := range candidates {

		period,
			hasPeriod :=
			candidate.
				override.
				EffectivePeriod()

		if !hasPeriod {
			continue
		}

		if effectiveFrom,
			hasFrom :=
			period.From(); hasFrom &&
			effectiveFrom.
				Cmp(calculationFrom) > 0 &&
			effectiveFrom.
				Cmp(calculationTo) < 0 {

			boundaries =
				append(
					boundaries,
					effectiveFrom,
				)
		}

		if effectiveTo,
			hasTo :=
			period.To(); hasTo &&
			effectiveTo.
				Cmp(calculationFrom) > 0 &&
			effectiveTo.
				Cmp(calculationTo) < 0 {

			boundaries =
				append(
					boundaries,
					effectiveTo,
				)
		}
	}

	return boundaries
}

func appendCandidateBoundaries(
	boundaries []datetime.DateTime,
	candidates []candidate,
	calculationFrom datetime.DateTime,
	calculationTo datetime.DateTime,
) []datetime.DateTime {
	for _, candidate := range candidates {

		period,
			hasPeriod :=
			candidate.
				binding.
				EffectivePeriod()

		if !hasPeriod {
			continue
		}

		if effectiveFrom,
			hasFrom :=
			period.From(); hasFrom &&
			effectiveFrom.
				Cmp(calculationFrom) > 0 &&
			effectiveFrom.
				Cmp(calculationTo) < 0 {

			boundaries =
				append(
					boundaries,
					effectiveFrom,
				)
		}

		if effectiveTo,
			hasTo :=
			period.To(); hasTo &&
			effectiveTo.
				Cmp(calculationFrom) > 0 &&
			effectiveTo.
				Cmp(calculationTo) < 0 {

			boundaries =
				append(
					boundaries,
					effectiveTo,
				)
		}
	}

	return boundaries
}

func uniqueDateTimes(
	values []datetime.DateTime,
) []datetime.DateTime {
	if len(values) < 2 {
		return values
	}

	result :=
		values[:1]

	for _, current := range values[1:] {

		if result[len(result)-1].Equal(current) {

			continue
		}

		result =
			append(
				result,
				current,
			)
	}

	return result
}
