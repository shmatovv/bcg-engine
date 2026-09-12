package scenario

import (
	"errors"
	"sort"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/types/series"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
)

// ValidationIssueKind identifies one Scenario compatibility failure.
type ValidationIssueKind uint8

const (
	ValidationIssueInvalid ValidationIssueKind = iota
	ValidationIssueInvalidModelVersion
	ValidationIssueInvalidScenario
	ValidationIssueBaseModelVersionMismatch
	ValidationIssueTargetObjectNotFound
	ValidationIssueTargetDefinitionNotFound
	ValidationIssueTargetDefinitionMismatch
	ValidationIssueTemporalValueMismatch
	ValidationIssueValueTypeMismatch
	ValidationIssueValueUnitMismatch
	ValidationIssueNullNotAllowed
	ValidationIssueUnknownScopeDefinition
	ValidationIssueScopeValueTypeMismatch
	ValidationIssueInvalidEffectivePeriod
)

func (k ValidationIssueKind) IsValid() bool {
	switch k {
	case ValidationIssueInvalidModelVersion,
		ValidationIssueInvalidScenario,
		ValidationIssueBaseModelVersionMismatch,
		ValidationIssueTargetObjectNotFound,
		ValidationIssueTargetDefinitionNotFound,
		ValidationIssueTargetDefinitionMismatch,
		ValidationIssueTemporalValueMismatch,
		ValidationIssueValueTypeMismatch,
		ValidationIssueValueUnitMismatch,
		ValidationIssueNullNotAllowed,
		ValidationIssueUnknownScopeDefinition,
		ValidationIssueScopeValueTypeMismatch,
		ValidationIssueInvalidEffectivePeriod:
		return true

	default:
		return false
	}
}

func (k ValidationIssueKind) String() string {
	switch k {
	case ValidationIssueInvalidModelVersion:
		return "InvalidModelVersion"

	case ValidationIssueInvalidScenario:
		return "InvalidScenario"

	case ValidationIssueBaseModelVersionMismatch:
		return "BaseModelVersionMismatch"

	case ValidationIssueTargetObjectNotFound:
		return "TargetObjectNotFound"

	case ValidationIssueTargetDefinitionNotFound:
		return "TargetDefinitionNotFound"

	case ValidationIssueTargetDefinitionMismatch:
		return "TargetDefinitionMismatch"

	case ValidationIssueTemporalValueMismatch:
		return "TemporalValueMismatch"

	case ValidationIssueValueTypeMismatch:
		return "ValueTypeMismatch"

	case ValidationIssueValueUnitMismatch:
		return "ValueUnitMismatch"

	case ValidationIssueNullNotAllowed:
		return "NullNotAllowed"

	case ValidationIssueUnknownScopeDefinition:
		return "UnknownScopeDefinition"

	case ValidationIssueScopeValueTypeMismatch:
		return "ScopeValueTypeMismatch"

	case ValidationIssueInvalidEffectivePeriod:
		return "InvalidEffectivePeriod"

	default:
		return "Invalid"
	}
}

// ValidationIssue is one independent Scenario compatibility failure.
type ValidationIssue struct {
	kind ValidationIssueKind

	target    address.PropertyAddress
	hasTarget bool

	message string
}

func newIssue(
	kind ValidationIssueKind,
	message string,
) ValidationIssue {
	if !kind.IsValid() {
		panic("scenario: invalid validation issue kind")
	}

	if message == "" {
		panic("scenario: validation issue message must not be empty")
	}

	return ValidationIssue{
		kind:    kind,
		message: message,
	}
}

func newTargetIssue(
	kind ValidationIssueKind,
	target address.PropertyAddress,
	message string,
) ValidationIssue {
	issue := newIssue(
		kind,
		message,
	)

	if target.IsValid() {
		issue.target = target
		issue.hasTarget = true
	}

	return issue
}

func (i ValidationIssue) Kind() ValidationIssueKind {
	return i.kind
}

func (i ValidationIssue) Target() (
	address.PropertyAddress,
	bool,
) {
	return i.target,
		i.hasTarget
}

func (i ValidationIssue) Message() string {
	return i.message
}

func (i ValidationIssue) IsValid() bool {
	return i.kind.IsValid() &&
		i.message != ""
}

// ValidationReport contains every Scenario compatibility issue discovered in
// one deterministic validation pass.
type ValidationReport struct {
	issues []ValidationIssue
}

func (r ValidationReport) OK() bool {
	return len(r.issues) == 0
}

func (r ValidationReport) Len() int {
	return len(r.issues)
}

func (r ValidationReport) Issues() []ValidationIssue {
	if len(r.issues) == 0 {
		return nil
	}

	result := make(
		[]ValidationIssue,
		len(r.issues),
	)

	copy(
		result,
		r.issues,
	)

	return result
}

func (r ValidationReport) Has(
	kind ValidationIssueKind,
) bool {
	for _, issue := range r.issues {
		if issue.kind == kind {
			return true
		}
	}

	return false
}

func (r ValidationReport) IsValid() bool {
	for _, issue := range r.issues {
		if !issue.IsValid() {
			return false
		}
	}

	return true
}

func (r ValidationReport) Error() string {
	if len(r.issues) == 0 {
		return "scenario: validation succeeded"
	}

	if len(r.issues) == 1 {
		return "scenario: validation failed: " +
			r.issues[0].kind.String() +
			": " +
			r.issues[0].message
	}

	return "scenario: validation failed with multiple issues"
}

func (r *ValidationReport) append(
	issue ValidationIssue,
) {
	if r == nil {
		panic("scenario: nil validation report")
	}

	if !issue.IsValid() {
		panic("scenario: invalid validation issue")
	}

	r.issues = append(
		r.issues,
		issue,
	)
}

func (r *ValidationReport) sortDeterministically() {
	if r == nil {
		panic("scenario: nil validation report")
	}

	sort.SliceStable(
		r.issues,
		func(i, j int) bool {
			left :=
				r.issues[i]

			right :=
				r.issues[j]

			if left.kind != right.kind {
				return left.kind <
					right.kind
			}

			if left.hasTarget !=
				right.hasTarget {

				return !left.hasTarget
			}

			if left.hasTarget {
				if comparison :=
					compareAddress(
						left.target,
						right.target,
					); comparison != 0 {

					return comparison < 0
				}
			}

			return left.message <
				right.message
		},
	)
}

// Validate verifies that candidate is a value-only overlay compatible with
// version.
//
// Validate never mutates ModelVersion or Scenario.
func Validate(
	version model.ModelVersion,
	candidate domainscenario.Scenario,
) ValidationReport {
	var report ValidationReport

	if !version.IsCommitted() {
		report.append(
			newIssue(
				ValidationIssueInvalidModelVersion,
				"Scenario validation requires a committed ModelVersion",
			),
		)

		report.sortDeterministically()

		return report
	}

	if !candidate.IsValid() {
		report.append(
			newIssue(
				ValidationIssueInvalidScenario,
				"Scenario is structurally invalid",
			),
		)

		report.sortDeterministically()

		return report
	}

	if candidate.BaseModelVersionID() !=
		version.ID() {

		report.append(
			newIssue(
				ValidationIssueBaseModelVersionMismatch,
				"Scenario is bound to a different ModelVersion",
			),
		)
	}

	for _, override := range candidate.Overrides() {

		validateOverride(
			&report,
			version,
			override,
		)
	}

	report.sortDeterministically()

	return report
}

func validateOverride(
	report *ValidationReport,
	version model.ModelVersion,
	override domainscenario.Override,
) {
	target :=
		override.Target()

	owner, ok :=
		ownerForObject(
			version,
			target.Object(),
		)

	if !ok {
		report.append(
			newTargetIssue(
				ValidationIssueTargetObjectNotFound,
				target,
				"Scenario override target object does not exist in the base ModelVersion",
			),
		)

		return
	}

	definition, exists :=
		version.PropertyDefinition(
			target.DefinitionID(),
		)

	if !exists ||
		!definition.IsValid() {

		report.append(
			newTargetIssue(
				ValidationIssueTargetDefinitionNotFound,
				target,
				"Scenario override PropertyDefinition does not exist in the base ModelVersion",
			),
		)

		return
	}

	if definition.Owner() != owner {
		report.append(
			newTargetIssue(
				ValidationIssueTargetDefinitionMismatch,
				target,
				"Scenario override PropertyDefinition is not available on the target object type",
			),
		)

		return
	}

	validateOverrideValue(
		report,
		version,
		target,
		definition,
		override.Value(),
	)

	validateOverrideScope(
		report,
		version,
		target,
		override,
	)

	validateOverridePeriod(
		report,
		target,
		override,
	)
}

func validateOverrideValue(
	report *ValidationReport,
	version model.ModelVersion,
	target address.PropertyAddress,
	definition schema.PropertyDefinition,
	bindingValue semantic.BindingValue,
) {
	switch bindingValue.Kind() {
	case semantic.BindingValueDefined:
		if definition.TemporalMode() != schema.TemporalScalar {
			report.append(newTargetIssue(ValidationIssueTemporalValueMismatch, target, "scalar Scenario override cannot target a Series property"))
			return
		}
		typed, ok :=
			bindingValue.Value()

		if !ok ||
			!typed.IsValid() {

			report.append(
				newTargetIssue(
					ValidationIssueInvalidScenario,
					target,
					"Scenario override contains an invalid Defined value",
				),
			)

			return
		}

		if typed.Type() !=
			definition.ValueType() {

			report.append(
				newTargetIssue(
					ValidationIssueValueTypeMismatch,
					target,
					"Scenario override value type does not match PropertyDefinition",
				),
			)
		}

		expectedUnit,
			expectedHasUnit :=
			definition.Unit()

		if typed.HasUnit() !=
			expectedHasUnit ||
			(expectedHasUnit &&
				!typed.
					Unit().
					Equal(expectedUnit)) {

			report.append(
				newTargetIssue(
					ValidationIssueValueUnitMismatch,
					target,
					"Scenario override unit does not exactly match PropertyDefinition unit",
				),
			)
		}

	case semantic.BindingValueSeries:
		stored, ok := bindingValue.Series()
		if !ok || !stored.IsValid() {
			report.append(newTargetIssue(ValidationIssueInvalidScenario, target, "Scenario override contains an invalid Series value"))
			return
		}
		if definition.TemporalMode() != schema.TemporalSeries {
			report.append(newTargetIssue(ValidationIssueTemporalValueMismatch, target, "Series Scenario override cannot target a Scalar property"))
			return
		}
		if err := stored.ValidateAgainst(definition.ValueType(), overrideUnit(definition), overrideHasUnit(definition), overrideAllowsNull(version, definition)); err != nil {
			switch {
			case errors.Is(err, series.ErrElementTypeMismatch):
				report.append(newTargetIssue(ValidationIssueValueTypeMismatch, target, "Scenario Series element type does not match PropertyDefinition"))
			case errors.Is(err, series.ErrUnitMismatch):
				report.append(newTargetIssue(ValidationIssueValueUnitMismatch, target, "Scenario Series unit does not exactly match PropertyDefinition unit"))
			case errors.Is(err, series.ErrNullNotAllowed):
				report.append(newTargetIssue(ValidationIssueNullNotAllowed, target, "Null Scenario Series sample is not allowed by PropertyContract"))
			default:
				report.append(newTargetIssue(ValidationIssueInvalidScenario, target, "Scenario Series value is invalid"))
			}
		}

	case semantic.BindingValueNull:
		if definition.TemporalMode() == schema.TemporalSeries {
			report.append(newTargetIssue(ValidationIssueTemporalValueMismatch, target, "whole-Series Null Scenario overrides are not supported"))
			return
		}
		contractID,
			hasContract :=
			definition.ContractID()

		if !hasContract {
			report.append(
				newTargetIssue(
					ValidationIssueNullNotAllowed,
					target,
					"Null Scenario override requires a nullable PropertyContract",
				),
			)

			return
		}

		contract, exists :=
			version.PropertyContract(
				contractID,
			)

		if !exists ||
			!contract.IsValid() ||
			!contract.Nullable() {

			report.append(
				newTargetIssue(
					ValidationIssueNullNotAllowed,
					target,
					"PropertyContract does not allow Null Scenario override",
				),
			)
		}

	default:
		report.append(
			newTargetIssue(
				ValidationIssueInvalidScenario,
				target,
				"Scenario override contains an invalid value kind",
			),
		)
	}
}

func overrideUnit(definition schema.PropertyDefinition) unit.Unit {
	u, _ := definition.Unit()
	return u
}

func overrideHasUnit(definition schema.PropertyDefinition) bool {
	_, hasUnit := definition.Unit()
	return hasUnit
}

func overrideAllowsNull(version model.ModelVersion, definition schema.PropertyDefinition) bool {
	contractID, hasContract := definition.ContractID()
	if !hasContract {
		return false
	}
	contract, ok := version.PropertyContract(contractID)
	return ok && contract.IsValid() && contract.Nullable()
}

func validateOverrideScope(
	report *ValidationReport,
	version model.ModelVersion,
	target address.PropertyAddress,
	override domainscenario.Override,
) {
	for _, selector := range override.ScopeSelectors() {

		definition, exists :=
			version.ScopeDefinition(
				selector.DefinitionID(),
			)

		if !exists ||
			!definition.IsValid() {

			report.append(
				newTargetIssue(
					ValidationIssueUnknownScopeDefinition,
					target,
					"Scenario override references an unknown ScopeDefinition",
				),
			)

			continue
		}

		if selector.
			Value().
			Type() !=
			definition.ValueType() {

			report.append(
				newTargetIssue(
					ValidationIssueScopeValueTypeMismatch,
					target,
					"Scenario override scope value type does not match ScopeDefinition",
				),
			)
		}
	}
}

func validateOverridePeriod(
	report *ValidationReport,
	target address.PropertyAddress,
	override domainscenario.Override,
) {
	period, hasPeriod :=
		override.EffectivePeriod()

	if !hasPeriod {
		return
	}

	if !period.IsValid() {
		report.append(
			newTargetIssue(
				ValidationIssueInvalidEffectivePeriod,
				target,
				"Scenario override EffectivePeriod is invalid",
			),
		)

		return
	}

	from, hasFrom :=
		period.From()

	to, hasTo :=
		period.To()

	if hasFrom &&
		hasTo &&
		from.Cmp(to) >= 0 {

		report.append(
			newTargetIssue(
				ValidationIssueInvalidEffectivePeriod,
				target,
				"Scenario override EffectivePeriod must satisfy from < to",
			),
		)
	}
}

func ownerForObject(
	version model.ModelVersion,
	object address.ObjectRef,
) (schema.PropertyOwner, bool) {
	switch object.Kind() {
	case address.ObjectRefEntity:
		entity, ok :=
			version.Entity(
				object.ID(),
			)

		if !ok ||
			!entity.IsValid() {

			return schema.PropertyOwner{},
				false
		}

		entityType, ok :=
			version.EntityType(
				entity.TypeID(),
			)

		if !ok ||
			entityType.ID() != entity.TypeID() {

			return schema.PropertyOwner{},
				false
		}

		return schema.EntityTypeOwner(
			entityType.ID(),
		), true

	case address.ObjectRefRelation:
		relation, ok :=
			version.Relation(
				object.ID(),
			)

		if !ok ||
			!relation.IsValid() {

			return schema.PropertyOwner{},
				false
		}

		relationType, ok :=
			version.RelationType(
				relation.TypeID(),
			)

		if !ok ||
			relationType.ID() != relation.TypeID() {

			return schema.PropertyOwner{},
				false
		}

		return schema.RelationTypeOwner(
			relationType.ID(),
		), true

	default:
		return schema.PropertyOwner{},
			false
	}
}

func compareAddress(
	left address.PropertyAddress,
	right address.PropertyAddress,
) int {
	if left.Object().Kind() <
		right.Object().Kind() {

		return -1
	}

	if left.Object().Kind() >
		right.Object().Kind() {

		return 1
	}

	leftObject :=
		left.
			Object().
			ID().
			Bytes()

	rightObject :=
		right.
			Object().
			ID().
			Bytes()

	for index := range leftObject {

		switch {
		case leftObject[index] <
			rightObject[index]:
			return -1

		case leftObject[index] >
			rightObject[index]:
			return 1
		}
	}

	leftDefinition :=
		left.
			DefinitionID().
			Bytes()

	rightDefinition :=
		right.
			DefinitionID().
			Bytes()

	for index := range leftDefinition {

		switch {
		case leftDefinition[index] <
			rightDefinition[index]:
			return -1

		case leftDefinition[index] >
			rightDefinition[index]:
			return 1
		}
	}

	return 0
}

var _ error = ValidationReport{}
