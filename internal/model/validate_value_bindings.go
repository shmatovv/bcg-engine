package model

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

// validateValueBindings validates committed Base Value semantics.
//
// Base bindings must:
//   - be structurally valid;
//   - target an existing object/type and PropertyDefinition;
//   - target a PropertyDefinition owned by the object's/type's schema owner;
//   - target an Input PropertyDefinition;
//   - carry a value with the declared ValueType and exact declared Unit;
//   - use Null only when the PropertyContract allows it;
//   - use a valid half-open EffectivePeriod;
//   - not repeat one ScopeDefinition inside the same binding;
//   - not create two equal-precedence candidates for the same target when
//     their scope selectors can match the same context and effective periods
//     overlap.
//
// Defined base values for TemporalSeries are rejected until the engine has a
// first-class Series value representation. Null remains legal for a Series
// property when its contract is nullable because Null denotes absence of the
// property value rather than one scalar sample.
func (d *Draft) validateValueBindings(
	report *ValidationReport,
	indexes validationIndexes,
) {
	groups := make(
		map[semantic.ValueTarget][]validatedBindingCandidate,
	)

	for _, binding := range d.valueBindings {
		if !binding.IsValid() {
			report.append(
				newValidationIssue(
					ValidationIssueInvalidValueBinding,
					"value binding is structurally invalid",
				),
			)
			continue
		}

		resolved, ok := resolveValueBindingTarget(
			report,
			binding.Target(),
			indexes,
		)
		if !ok {
			continue
		}

		candidateValid := true

		if resolved.definition.Computation().Kind() !=
			schema.ComputationInput {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueBaseValueForDerivedProperty,
					resolved.subjectID,
					resolved.definition.ID(),
					"base value binding targets a derived property definition",
				),
			)
			candidateValid = false
		}

		if !validateBindingValue(
			report,
			binding,
			resolved,
			indexes,
		) {
			candidateValid = false
		}

		if !validateBindingEffectivePeriod(
			report,
			binding,
			resolved,
		) {
			candidateValid = false
		}

		if !validateBindingScopeSelectors(
			report,
			binding,
			resolved,
			indexes,
		) {
			candidateValid = false
		}

		if candidateValid {
			groups[binding.Target()] = append(
				groups[binding.Target()],
				validatedBindingCandidate{
					binding:      binding,
					subjectID:    resolved.subjectID,
					definitionID: resolved.definition.ID(),
				},
			)
		}
	}

	validateBindingConflicts(
		report,
		groups,
	)
}

type resolvedValueBindingTarget struct {
	subjectID  identity.UUID
	definition schema.PropertyDefinition
}

func resolveValueBindingTarget(
	report *ValidationReport,
	target semantic.ValueTarget,
	indexes validationIndexes,
) (resolvedValueBindingTarget, bool) {
	switch target.Kind() {
	case semantic.ValueTargetObjectProperty:
		address, ok := target.ObjectProperty()
		if !ok {
			return resolvedValueBindingTarget{}, false
		}

		definition, definitionExists :=
			indexes.propertyDefinitionByID[address.DefinitionID()]
		if !definitionExists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueBindingDefinitionNotFound,
					address.Object().ID(),
					address.DefinitionID(),
					"value binding references unknown property definition",
				),
			)
			return resolvedValueBindingTarget{}, false
		}

		owner, ownerResolved := objectPropertyOwner(
			report,
			address.Object(),
			address.DefinitionID(),
			indexes,
		)
		if !ownerResolved {
			return resolvedValueBindingTarget{}, false
		}

		if definition.Owner() != owner {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueBindingTargetMismatch,
					address.Object().ID(),
					definition.ID(),
					"property definition is not available on the value binding target object type",
				),
			)
			return resolvedValueBindingTarget{}, false
		}

		return resolvedValueBindingTarget{
			subjectID:  address.Object().ID(),
			definition: definition,
		}, true

	case semantic.ValueTargetTypeProperty:
		owner, definitionID, ok := target.TypeProperty()
		if !ok {
			return resolvedValueBindingTarget{}, false
		}

		if !propertyOwnerExists(owner, indexes) {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueBindingTargetNotFound,
					owner.ID(),
					definitionID,
					"value binding references unknown target type",
				),
			)
			return resolvedValueBindingTarget{}, false
		}

		definition, definitionExists :=
			indexes.propertyDefinitionByID[definitionID]
		if !definitionExists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueBindingDefinitionNotFound,
					owner.ID(),
					definitionID,
					"value binding references unknown property definition",
				),
			)
			return resolvedValueBindingTarget{}, false
		}

		if definition.Owner() != owner {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueBindingTargetMismatch,
					owner.ID(),
					definition.ID(),
					"property definition is not owned by the value binding target type",
				),
			)
			return resolvedValueBindingTarget{}, false
		}

		return resolvedValueBindingTarget{
			subjectID:  owner.ID(),
			definition: definition,
		}, true

	default:
		return resolvedValueBindingTarget{}, false
	}
}

func objectPropertyOwner(
	report *ValidationReport,
	object semantic.ObjectRef,
	definitionID identity.UUID,
	indexes validationIndexes,
) (schema.PropertyOwner, bool) {
	switch object.Kind() {
	case semantic.ObjectRefEntity:
		entity, exists := indexes.entityByID[object.ID()]
		if !exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueBindingTargetNotFound,
					object.ID(),
					definitionID,
					"value binding references unknown entity",
				),
			)
			return schema.PropertyOwner{}, false
		}

		if _, exists := indexes.entityTypeByID[entity.TypeID()]; !exists {
			// Semantic validation already reports UnknownEntityType.
			return schema.PropertyOwner{}, false
		}

		return schema.EntityTypeOwner(entity.TypeID()), true

	case semantic.ObjectRefRelation:
		relation, exists := indexes.relationByID[object.ID()]
		if !exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueBindingTargetNotFound,
					object.ID(),
					definitionID,
					"value binding references unknown relation",
				),
			)
			return schema.PropertyOwner{}, false
		}

		if _, exists := indexes.relationTypeByID[relation.TypeID()]; !exists {
			// Semantic validation already reports UnknownRelationType.
			return schema.PropertyOwner{}, false
		}

		return schema.RelationTypeOwner(relation.TypeID()), true

	default:
		return schema.PropertyOwner{}, false
	}
}

func propertyOwnerExists(
	owner schema.PropertyOwner,
	indexes validationIndexes,
) bool {
	switch owner.Kind() {
	case schema.PropertyOwnerEntityType:
		_, exists := indexes.entityTypeByID[owner.ID()]
		return exists

	case schema.PropertyOwnerRelationType:
		_, exists := indexes.relationTypeByID[owner.ID()]
		return exists

	default:
		return false
	}
}

func validateBindingValue(
	report *ValidationReport,
	binding semantic.ValueBinding,
	resolved resolvedValueBindingTarget,
	indexes validationIndexes,
) bool {
	bindingValue := binding.Value()
	definition := resolved.definition

	switch bindingValue.Kind() {
	case semantic.BindingValueDefined:
		typedValue, ok := bindingValue.Value()
		if !ok || !typedValue.IsValid() {
			return false
		}

		valid := true

		if definition.TemporalMode() == schema.TemporalSeries {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueUnsupportedSeriesBaseValue,
					resolved.subjectID,
					definition.ID(),
					"defined base value cannot target a Series property until Series values are represented explicitly",
				),
			)
			valid = false
		}

		if typedValue.Type() != definition.ValueType() {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueTypeMismatch,
					resolved.subjectID,
					definition.ID(),
					"base value type does not match property definition value type",
				),
			)
			valid = false
		}

		if !bindingUnitMatchesDefinition(
			typedValue,
			definition,
		) {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueValueUnitMismatch,
					resolved.subjectID,
					definition.ID(),
					"base value unit does not exactly match property definition unit",
				),
			)
			valid = false
		}

		return valid

	case semantic.BindingValueNull:
		contractID, hasContract := definition.ContractID()
		if !hasContract {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueNullNotAllowed,
					resolved.subjectID,
					definition.ID(),
					"null base value is not allowed for a property without a nullable contract",
				),
			)
			return false
		}

		contract, exists := indexes.propertyContractByID[contractID]
		if !exists {
			// Schema reference validation already reports the missing contract.
			return false
		}

		if !contract.Nullable() {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueNullNotAllowed,
					resolved.subjectID,
					definition.ID(),
					"null base value is not allowed by the property contract",
				),
			)
			return false
		}

		return true

	default:
		return false
	}
}

func bindingUnitMatchesDefinition(
	typedValue value.Typed,
	definition schema.PropertyDefinition,
) bool {
	definitionUnit, definitionHasUnit := definition.Unit()
	valueHasUnit := typedValue.HasUnit()

	if definitionHasUnit != valueHasUnit {
		return false
	}

	if !definitionHasUnit {
		return true
	}

	// Committed Base Values are canonical model data. They are stored in the
	// exact Unit declared by PropertyDefinition. Import/UI layers may perform
	// an explicit fixed conversion before constructing the binding.
	return typedValue.Unit().Equal(definitionUnit)
}

func validateBindingEffectivePeriod(
	report *ValidationReport,
	binding semantic.ValueBinding,
	resolved resolvedValueBindingTarget,
) bool {
	period, hasPeriod := binding.EffectivePeriod()
	if !hasPeriod {
		return true
	}

	if !validEffectivePeriod(period) {
		report.append(
			newRelatedValidationIssue(
				ValidationIssueInvalidEffectivePeriod,
				resolved.subjectID,
				resolved.definition.ID(),
				"value binding effective period must satisfy from < to when both bounds are present",
			),
		)
		return false
	}

	return true
}

func validEffectivePeriod(
	period domaintime.EffectivePeriod,
) bool {
	if !period.IsValid() {
		return false
	}

	from, hasFrom := period.From()
	to, hasTo := period.To()

	if hasFrom && hasTo {
		return from.Cmp(to) < 0
	}

	return true
}

func validateBindingScopeSelectors(
	report *ValidationReport,
	binding semantic.ValueBinding,
	resolved resolvedValueBindingTarget,
	indexes validationIndexes,
) bool {
	seen := make(map[identity.UUID]struct{})
	valid := true

	for _, selector := range binding.ScopeSelectors() {
		definitionID := selector.DefinitionID()

		if _, exists := seen[definitionID]; exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueDuplicateScopeSelector,
					resolved.subjectID,
					resolved.definition.ID(),
					"value binding contains more than one selector for the same scope definition",
				),
			)
			valid = false
			continue
		}

		seen[definitionID] = struct{}{}

		scopeDefinition, exists := indexes.scopeDefinitionByID[definitionID]
		if !exists {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueUnknownScopeDefinition,
					resolved.subjectID,
					definitionID,
					"value binding selector references unknown scope definition",
				),
			)
			valid = false
			continue
		}

		selectorValue := selector.Value()
		if selectorValue.Type() != scopeDefinition.ValueType() {
			report.append(
				newRelatedValidationIssue(
					ValidationIssueScopeSelectorTypeMismatch,
					resolved.subjectID,
					definitionID,
					"scope selector value type does not match scope definition value type",
				),
			)
			valid = false
		}
	}

	return valid
}

type validatedBindingCandidate struct {
	binding      semantic.ValueBinding
	subjectID    identity.UUID
	definitionID identity.UUID
}

func validateBindingConflicts(
	report *ValidationReport,
	groups map[semantic.ValueTarget][]validatedBindingCandidate,
) {
	for _, candidates := range groups {
		for i := 0; i < len(candidates); i++ {
			for j := i + 1; j < len(candidates); j++ {
				left := candidates[i]
				right := candidates[j]

				leftSelectors := left.binding.ScopeSelectors()
				rightSelectors := right.binding.ScopeSelectors()

				if !scopeSpecificityEqual(
					leftSelectors,
					rightSelectors,
				) {
					continue
				}

				if !scopeSelectorSetsCanMatchSameContext(
					leftSelectors,
					rightSelectors,
				) {
					continue
				}

				if !bindingEffectivePeriodsOverlap(
					left.binding,
					right.binding,
				) {
					continue
				}

				report.append(
					newRelatedValidationIssue(
						ValidationIssueConflictingValueBindings,
						left.subjectID,
						left.definitionID,
						"multiple base value bindings have equal target, scope specificity and overlapping effective time",
					),
				)
			}
		}
	}
}

func scopeSpecificityEqual(
	left []scope.Selector,
	right []scope.Selector,
) bool {
	return len(left) == len(right)
}

// scopeSelectorSetsCanMatchSameContext reports whether one CalculationContext
// can satisfy both selector sets at the same time.
//
// Selectors constrain independent scope dimensions. Two sets are therefore
// incompatible only when both constrain the same ScopeDefinition to different
// values. Disjoint selector sets are compatible and may be simultaneously
// applicable.
//
// Duplicate selectors are rejected earlier by validateBindingScopeSelectors.
// The duplicate checks below are defensive so this helper remains correct if
// it is reused independently later.
func scopeSelectorSetsCanMatchSameContext(
	left []scope.Selector,
	right []scope.Selector,
) bool {
	leftByDefinition := make(
		map[identity.UUID]value.Typed,
		len(left),
	)

	for _, selector := range left {
		definitionID := selector.DefinitionID()

		if _, exists := leftByDefinition[definitionID]; exists {
			return false
		}

		leftByDefinition[definitionID] = selector.Value()
	}

	rightSeen := make(
		map[identity.UUID]struct{},
		len(right),
	)

	for _, selector := range right {
		definitionID := selector.DefinitionID()

		if _, exists := rightSeen[definitionID]; exists {
			return false
		}

		rightSeen[definitionID] = struct{}{}

		leftValue, constrainedByLeft :=
			leftByDefinition[definitionID]
		if !constrainedByLeft {
			continue
		}

		if !leftValue.Equal(selector.Value()) {
			return false
		}
	}

	return true
}

func bindingEffectivePeriodsOverlap(
	left semantic.ValueBinding,
	right semantic.ValueBinding,
) bool {
	leftPeriod, leftHasPeriod := left.EffectivePeriod()
	rightPeriod, rightHasPeriod := right.EffectivePeriod()

	if leftHasPeriod && !validEffectivePeriod(leftPeriod) {
		return false
	}
	if rightHasPeriod && !validEffectivePeriod(rightPeriod) {
		return false
	}

	if !leftHasPeriod && !rightHasPeriod {
		return true
	}

	leftFromValue, leftHasFrom := leftPeriod.From()
	leftToValue, leftHasTo := leftPeriod.To()
	rightFromValue, rightHasFrom := rightPeriod.From()
	rightToValue, rightHasTo := rightPeriod.To()

	if leftHasTo && rightHasFrom &&
		leftToValue.Cmp(rightFromValue) <= 0 {
		return false
	}

	if rightHasTo && leftHasFrom &&
		rightToValue.Cmp(leftFromValue) <= 0 {
		return false
	}

	return true
}
