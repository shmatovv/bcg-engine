package provenance

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
		KindAmbiguousValue:
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
	default:
		return "Invalid"
	}
}
