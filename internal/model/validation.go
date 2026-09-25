package model

import (
	"fmt"
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
)

type ValidationIssueKind uint16

const (
	ValidationIssueInvalid ValidationIssueKind = iota

	ValidationIssueInvalidEntityType
	ValidationIssueInvalidRelationType
	ValidationIssueInvalidTrait
	ValidationIssueInvalidPropertyContract
	ValidationIssueInvalidPropertyDefinition
	ValidationIssueInvalidScopeDefinition
	ValidationIssueInvalidConstraint
	ValidationIssueInvalidRule

	ValidationIssueInvalidEntity
	ValidationIssueInvalidRelation

	ValidationIssueDuplicateEntityTypeID
	ValidationIssueDuplicateRelationTypeID
	ValidationIssueDuplicateTraitID
	ValidationIssueDuplicatePropertyContractID
	ValidationIssueDuplicatePropertyDefinitionID
	ValidationIssueDuplicateScopeDefinitionID
	ValidationIssueDuplicateConstraintID
	ValidationIssueDuplicateRuleID

	ValidationIssueDuplicateEntityID
	ValidationIssueDuplicateRelationID

	ValidationIssueUnknownEntityType
	ValidationIssueUnknownRelationType
	ValidationIssueUnknownTrait
	ValidationIssueUnknownPropertyContract
	ValidationIssueUnknownPropertyDefinition
	ValidationIssueUnknownScopeDefinition

	ValidationIssueUnknownEntity

	ValidationIssuePropertyOwnerNotFound
	ValidationIssuePropertyOwnerMismatch

	ValidationIssueTraitRequirementNotSatisfied

	ValidationIssueDuplicateContractImplementation
	ValidationIssueContractValueTypeMismatch
	ValidationIssueContractDimensionMismatch
	ValidationIssueContractTemporalModeMismatch
	ValidationIssueUnsupportedSchemaCombination

	ValidationIssueInvalidRelationEndpoint

	ValidationIssueInvalidValueBinding
	ValidationIssueValueBindingTargetNotFound
	ValidationIssueValueBindingDefinitionNotFound
	ValidationIssueValueBindingTargetMismatch
	ValidationIssueBaseValueForDerivedProperty
	ValidationIssueValueTypeMismatch
	ValidationIssueValueUnitMismatch
	ValidationIssueTemporalValueMismatch
	ValidationIssueNullNotAllowed
	ValidationIssueInvalidEffectivePeriod
	ValidationIssueDuplicateScopeSelector
	ValidationIssueScopeSelectorTypeMismatch
	ValidationIssueConflictingValueBindings

	ValidationIssueInvalidDerivedExpression
	ValidationIssueInvalidRelationApplicability
	ValidationIssueInvalidConstraintApplicability
	ValidationIssueInvalidConstraintAssertion
	ValidationIssueInvalidRuleCondition

	ValidationIssueDependencyGraphBuildFailure
	ValidationIssueCycle

	// Dataset-specific lifecycle and ownership restrictions intentionally reuse
	// ordinary semantic validation issue kinds wherever possible. The cases
	// below have no equivalent in an authored self-contained model.
	ValidationIssueInvalidDataset
	ValidationIssueDatasetTypeBindingNotAllowed
	ValidationIssueDatasetBindingTargetNotOwned
	ValidationIssueDatasetRelationRequiresDatasetEndpoint
)

func (k ValidationIssueKind) IsValid() bool {
	switch k {
	case ValidationIssueInvalidEntityType,
		ValidationIssueInvalidRelationType,
		ValidationIssueInvalidTrait,
		ValidationIssueInvalidPropertyContract,
		ValidationIssueInvalidPropertyDefinition,
		ValidationIssueInvalidScopeDefinition,
		ValidationIssueInvalidConstraint,
		ValidationIssueInvalidRule,

		ValidationIssueInvalidEntity,
		ValidationIssueInvalidRelation,

		ValidationIssueDuplicateEntityTypeID,
		ValidationIssueDuplicateRelationTypeID,
		ValidationIssueDuplicateTraitID,
		ValidationIssueDuplicatePropertyContractID,
		ValidationIssueDuplicatePropertyDefinitionID,
		ValidationIssueDuplicateScopeDefinitionID,
		ValidationIssueDuplicateConstraintID,
		ValidationIssueDuplicateRuleID,

		ValidationIssueDuplicateEntityID,
		ValidationIssueDuplicateRelationID,

		ValidationIssueUnknownEntityType,
		ValidationIssueUnknownRelationType,
		ValidationIssueUnknownTrait,
		ValidationIssueUnknownPropertyContract,
		ValidationIssueUnknownPropertyDefinition,
		ValidationIssueUnknownScopeDefinition,

		ValidationIssueUnknownEntity,

		ValidationIssuePropertyOwnerNotFound,
		ValidationIssuePropertyOwnerMismatch,

		ValidationIssueTraitRequirementNotSatisfied,

		ValidationIssueDuplicateContractImplementation,
		ValidationIssueContractValueTypeMismatch,
		ValidationIssueContractDimensionMismatch,
		ValidationIssueContractTemporalModeMismatch,
		ValidationIssueUnsupportedSchemaCombination,

		ValidationIssueInvalidRelationEndpoint,

		ValidationIssueInvalidValueBinding,
		ValidationIssueValueBindingTargetNotFound,
		ValidationIssueValueBindingDefinitionNotFound,
		ValidationIssueValueBindingTargetMismatch,
		ValidationIssueBaseValueForDerivedProperty,
		ValidationIssueValueTypeMismatch,
		ValidationIssueValueUnitMismatch,
		ValidationIssueTemporalValueMismatch,
		ValidationIssueNullNotAllowed,
		ValidationIssueInvalidEffectivePeriod,
		ValidationIssueDuplicateScopeSelector,
		ValidationIssueScopeSelectorTypeMismatch,
		ValidationIssueConflictingValueBindings,

		ValidationIssueInvalidDerivedExpression,
		ValidationIssueInvalidRelationApplicability,
		ValidationIssueInvalidConstraintApplicability,
		ValidationIssueInvalidConstraintAssertion,
		ValidationIssueInvalidRuleCondition,

		ValidationIssueDependencyGraphBuildFailure,
		ValidationIssueCycle,
		ValidationIssueInvalidDataset,
		ValidationIssueDatasetTypeBindingNotAllowed,
		ValidationIssueDatasetBindingTargetNotOwned,
		ValidationIssueDatasetRelationRequiresDatasetEndpoint:
		return true

	default:
		return false
	}
}

func (k ValidationIssueKind) String() string {
	switch k {
	case ValidationIssueInvalidEntityType:
		return "InvalidEntityType"
	case ValidationIssueInvalidRelationType:
		return "InvalidRelationType"
	case ValidationIssueInvalidTrait:
		return "InvalidTrait"
	case ValidationIssueInvalidPropertyContract:
		return "InvalidPropertyContract"
	case ValidationIssueInvalidPropertyDefinition:
		return "InvalidPropertyDefinition"
	case ValidationIssueInvalidScopeDefinition:
		return "InvalidScopeDefinition"
	case ValidationIssueInvalidConstraint:
		return "InvalidConstraint"
	case ValidationIssueInvalidRule:
		return "InvalidRule"
	case ValidationIssueInvalidEntity:
		return "InvalidEntity"
	case ValidationIssueInvalidRelation:
		return "InvalidRelation"
	case ValidationIssueDuplicateEntityTypeID:
		return "DuplicateEntityTypeID"
	case ValidationIssueDuplicateRelationTypeID:
		return "DuplicateRelationTypeID"
	case ValidationIssueDuplicateTraitID:
		return "DuplicateTraitID"
	case ValidationIssueDuplicatePropertyContractID:
		return "DuplicatePropertyContractID"
	case ValidationIssueDuplicatePropertyDefinitionID:
		return "DuplicatePropertyDefinitionID"
	case ValidationIssueDuplicateScopeDefinitionID:
		return "DuplicateScopeDefinitionID"
	case ValidationIssueDuplicateConstraintID:
		return "DuplicateConstraintID"
	case ValidationIssueDuplicateRuleID:
		return "DuplicateRuleID"
	case ValidationIssueDuplicateEntityID:
		return "DuplicateEntityID"
	case ValidationIssueDuplicateRelationID:
		return "DuplicateRelationID"
	case ValidationIssueUnknownEntityType:
		return "UnknownEntityType"
	case ValidationIssueUnknownRelationType:
		return "UnknownRelationType"
	case ValidationIssueUnknownTrait:
		return "UnknownTrait"
	case ValidationIssueUnknownPropertyContract:
		return "UnknownPropertyContract"
	case ValidationIssueUnknownPropertyDefinition:
		return "UnknownPropertyDefinition"
	case ValidationIssueUnknownScopeDefinition:
		return "UnknownScopeDefinition"
	case ValidationIssueUnknownEntity:
		return "UnknownEntity"
	case ValidationIssuePropertyOwnerNotFound:
		return "PropertyOwnerNotFound"
	case ValidationIssuePropertyOwnerMismatch:
		return "PropertyOwnerMismatch"
	case ValidationIssueTraitRequirementNotSatisfied:
		return "TraitRequirementNotSatisfied"
	case ValidationIssueDuplicateContractImplementation:
		return "DuplicateContractImplementation"
	case ValidationIssueContractValueTypeMismatch:
		return "ContractValueTypeMismatch"
	case ValidationIssueContractDimensionMismatch:
		return "ContractDimensionMismatch"
	case ValidationIssueContractTemporalModeMismatch:
		return "ContractTemporalModeMismatch"
	case ValidationIssueUnsupportedSchemaCombination:
		return "UnsupportedSchemaCombination"
	case ValidationIssueInvalidRelationEndpoint:
		return "InvalidRelationEndpoint"
	case ValidationIssueInvalidValueBinding:
		return "InvalidValueBinding"
	case ValidationIssueValueBindingTargetNotFound:
		return "ValueBindingTargetNotFound"
	case ValidationIssueValueBindingDefinitionNotFound:
		return "ValueBindingDefinitionNotFound"
	case ValidationIssueValueBindingTargetMismatch:
		return "ValueBindingTargetMismatch"
	case ValidationIssueBaseValueForDerivedProperty:
		return "BaseValueForDerivedProperty"
	case ValidationIssueValueTypeMismatch:
		return "ValueTypeMismatch"
	case ValidationIssueValueUnitMismatch:
		return "ValueUnitMismatch"
	case ValidationIssueTemporalValueMismatch:
		return "TemporalValueMismatch"
	case ValidationIssueNullNotAllowed:
		return "NullNotAllowed"
	case ValidationIssueInvalidEffectivePeriod:
		return "InvalidEffectivePeriod"
	case ValidationIssueDuplicateScopeSelector:
		return "DuplicateScopeSelector"
	case ValidationIssueScopeSelectorTypeMismatch:
		return "ScopeSelectorTypeMismatch"
	case ValidationIssueConflictingValueBindings:
		return "ConflictingValueBindings"
	case ValidationIssueInvalidDerivedExpression:
		return "InvalidDerivedExpression"
	case ValidationIssueInvalidRelationApplicability:
		return "InvalidRelationApplicability"
	case ValidationIssueInvalidConstraintApplicability:
		return "InvalidConstraintApplicability"
	case ValidationIssueInvalidConstraintAssertion:
		return "InvalidConstraintAssertion"
	case ValidationIssueInvalidRuleCondition:
		return "InvalidRuleCondition"
	case ValidationIssueDependencyGraphBuildFailure:
		return "DependencyGraphBuildFailure"
	case ValidationIssueCycle:
		return "Cycle"
	case ValidationIssueInvalidDataset:
		return "InvalidDataset"
	case ValidationIssueDatasetTypeBindingNotAllowed:
		return "DatasetTypeBindingNotAllowed"
	case ValidationIssueDatasetBindingTargetNotOwned:
		return "DatasetBindingTargetNotOwned"
	case ValidationIssueDatasetRelationRequiresDatasetEndpoint:
		return "DatasetRelationRequiresDatasetEndpoint"
	default:
		return "Invalid"
	}
}

type ValidationIssue struct {
	kind ValidationIssueKind

	subjectID  identity.UUID
	hasSubject bool

	relatedID  identity.UUID
	hasRelated bool

	message string
}

func newValidationIssue(
	kind ValidationIssueKind,
	message string,
) ValidationIssue {
	if !kind.IsValid() {
		panic(
			"model: invalid validation issue kind",
		)
	}

	if message == "" {
		panic(
			"model: validation issue message must not be empty",
		)
	}

	return ValidationIssue{
		kind:    kind,
		message: message,
	}
}

func newSubjectValidationIssue(
	kind ValidationIssueKind,
	subjectID identity.UUID,
	message string,
) ValidationIssue {
	issue :=
		newValidationIssue(
			kind,
			message,
		)

	if !subjectID.IsZero() {
		issue.subjectID =
			subjectID

		issue.hasSubject =
			true
	}

	return issue
}

func newRelatedValidationIssue(
	kind ValidationIssueKind,
	subjectID identity.UUID,
	relatedID identity.UUID,
	message string,
) ValidationIssue {
	issue :=
		newSubjectValidationIssue(
			kind,
			subjectID,
			message,
		)

	if !relatedID.IsZero() {
		issue.relatedID =
			relatedID

		issue.hasRelated =
			true
	}

	return issue
}

func (i ValidationIssue) Kind() ValidationIssueKind {
	return i.kind
}

func (i ValidationIssue) SubjectID() (
	identity.UUID,
	bool,
) {
	return i.subjectID,
		i.hasSubject
}

func (i ValidationIssue) RelatedID() (
	identity.UUID,
	bool,
) {
	return i.relatedID,
		i.hasRelated
}

func (i ValidationIssue) Message() string {
	return i.message
}

func (i ValidationIssue) IsValid() bool {
	return i.kind.IsValid() &&
		i.message != ""
}

func (i ValidationIssue) String() string {
	return fmt.Sprintf(
		"%s: %s",
		i.kind,
		i.message,
	)
}

type ValidationReport struct {
	issues []ValidationIssue
}

func (r ValidationReport) IsValid() bool {
	for _, issue := range r.issues {
		if !issue.IsValid() {
			return false
		}
	}

	return true
}

func (r ValidationReport) OK() bool {
	return len(r.issues) == 0
}

func (r ValidationReport) Len() int {
	return len(r.issues)
}

func (r ValidationReport) Issues() []ValidationIssue {
	return cloneSlice(
		r.issues,
	)
}

func (r ValidationReport) Has(
	kind ValidationIssueKind,
) bool {
	for _, issue := range r.issues {
		if issue.Kind() == kind {
			return true
		}
	}

	return false
}

func (r *ValidationReport) append(
	issue ValidationIssue,
) {
	if r == nil {
		panic(
			"model: nil validation report",
		)
	}

	if !issue.IsValid() {
		panic(
			"model: invalid validation issue",
		)
	}

	r.issues = append(
		r.issues,
		issue,
	)
}

func (r *ValidationReport) sortDeterministically() {
	if r == nil {
		panic(
			"model: nil validation report",
		)
	}

	sort.SliceStable(
		r.issues,
		func(
			leftIndex int,
			rightIndex int,
		) bool {
			return validationIssueLess(
				r.issues[leftIndex],
				r.issues[rightIndex],
			)
		},
	)
}

func validationIssueLess(
	left ValidationIssue,
	right ValidationIssue,
) bool {
	if left.kind != right.kind {
		return left.kind <
			right.kind
	}

	if left.hasSubject !=
		right.hasSubject {
		return !left.hasSubject
	}

	if left.hasSubject {
		comparison :=
			compareValidationUUID(
				left.subjectID,
				right.subjectID,
			)

		if comparison != 0 {
			return comparison < 0
		}
	}

	if left.hasRelated !=
		right.hasRelated {
		return !left.hasRelated
	}

	if left.hasRelated {
		comparison :=
			compareValidationUUID(
				left.relatedID,
				right.relatedID,
			)

		if comparison != 0 {
			return comparison < 0
		}
	}

	return left.message <
		right.message
}

func compareValidationUUID(
	left identity.UUID,
	right identity.UUID,
) int {
	leftBytes :=
		left.Bytes()

	rightBytes :=
		right.Bytes()

	for index := range leftBytes {
		switch {
		case leftBytes[index] <
			rightBytes[index]:
			return -1
		case leftBytes[index] >
			rightBytes[index]:
			return 1
		}
	}

	return 0
}

func (r ValidationReport) Error() string {
	switch len(r.issues) {
	case 0:
		return "model: validation succeeded"

	case 1:
		return fmt.Sprintf(
			"model: validation failed: %s",
			r.issues[0],
		)

	default:
		return fmt.Sprintf(
			"model: validation failed with %d issues",
			len(r.issues),
		)
	}
}

var _ error = ValidationReport{}
