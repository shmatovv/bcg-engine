package provenance

// Decision identifies an actual control-flow decision made by a lazy
// operation. It is semantic data, so consumers need not reconstruct branch
// selection from the static expression tree.
type Decision uint8

const (
	DecisionNone Decision = iota
	DecisionShortCircuited
	DecisionThen
	DecisionElse
	DecisionPrimary
	DecisionFallback
	DecisionIndeterminate
)

func (d Decision) IsValid() bool {
	switch d {
	case DecisionNone,
		DecisionShortCircuited,
		DecisionThen,
		DecisionElse,
		DecisionPrimary,
		DecisionFallback,
		DecisionIndeterminate:
		return true
	default:
		return false
	}
}

func (d Decision) String() string {
	switch d {
	case DecisionNone:
		return "None"
	case DecisionShortCircuited:
		return "ShortCircuited"
	case DecisionThen:
		return "Then"
	case DecisionElse:
		return "Else"
	case DecisionPrimary:
		return "Primary"
	case DecisionFallback:
		return "Fallback"
	case DecisionIndeterminate:
		return "Indeterminate"
	default:
		return "Invalid"
	}
}

// IsAllowedForOperation reports whether d is meaningful for an operation in
// the recorded evaluator vocabulary. DecisionNone is valid for every strict
// operation; branch decisions are deliberately limited to their lazy forms.
func (d Decision) IsAllowedForOperation(operation string) bool {
	if !d.IsValid() {
		return false
	}
	switch d {
	case DecisionNone:
		return true
	case DecisionShortCircuited:
		return operation == "And" || operation == "Or"
	case DecisionThen, DecisionElse, DecisionIndeterminate:
		return operation == "If"
	case DecisionPrimary, DecisionFallback:
		return operation == "Coalesce"
	default:
		return false
	}
}

// Kind identifies one node in an evaluation provenance tree.
type Kind uint8

const (
	KindInvalid Kind = iota
	KindInputProperty
	KindDerivedProperty
	KindReference
	KindConstant
	KindOperation
	KindCalculationOverride
	KindScenarioOverride
	KindBaseObjectBinding
	KindBaseTypeBinding
	KindMissingValue
	KindAmbiguousValue
	KindAggregateItem
)

func (k Kind) IsValid() bool {
	switch k {
	case KindInputProperty,
		KindDerivedProperty,
		KindReference,
		KindConstant,
		KindOperation,
		KindCalculationOverride,
		KindScenarioOverride,
		KindBaseObjectBinding,
		KindBaseTypeBinding,
		KindMissingValue,
		KindAmbiguousValue,
		KindAggregateItem:
		return true
	default:
		return false
	}
}

func (k Kind) String() string {
	switch k {
	case KindInputProperty:
		return "InputProperty"
	case KindDerivedProperty:
		return "DerivedProperty"
	case KindReference:
		return "Reference"
	case KindConstant:
		return "Constant"
	case KindOperation:
		return "Operation"
	case KindCalculationOverride:
		return "CalculationOverride"
	case KindScenarioOverride:
		return "ScenarioOverride"
	case KindBaseObjectBinding:
		return "BaseObjectBinding"
	case KindBaseTypeBinding:
		return "BaseTypeBinding"
	case KindMissingValue:
		return "MissingValue"
	case KindAmbiguousValue:
		return "AmbiguousValue"
	case KindAggregateItem:
		return "AggregateItem"
	default:
		return "Invalid"
	}
}
