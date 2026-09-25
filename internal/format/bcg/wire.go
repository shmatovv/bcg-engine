package bcg

// These DTOs are deliberately separate from the domain objects. Their stable
// JSON names are the BCG v1 compatibility surface.
type wireDocument struct {
	FormatVersion       int          `json:"formatVersion"`
	EngineCompatibility string       `json:"engineCompatibility"`
	Artifact            wireArtifact `json:"artifact"`
}

type wireArtifact struct {
	Model     wireModel      `json:"model"`
	Version   wireVersion    `json:"version"`
	Scenarios []wireScenario `json:"scenarios"`
}

type wireModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type wireVersion struct {
	ID                   string                    `json:"id"`
	ModelID              string                    `json:"modelId"`
	ParentVersionID      *string                   `json:"parentVersionId"`
	Label                string                    `json:"label"`
	Precision            wirePrecision             `json:"precision"`
	EntityTypes          []wireEntityType          `json:"entityTypes"`
	EnumDefinitions      []wireEnumDefinition      `json:"enumDefinitions,omitempty"`
	EntityKeys           []wireEntityKey           `json:"entityKeyDefinitions,omitempty"`
	RelationTypes        []wireRelationType        `json:"relationTypes"`
	Traits               []wireTrait               `json:"traits"`
	Contracts            []wireContract            `json:"propertyContracts"`
	Properties           []wireProperty            `json:"propertyDefinitions"`
	Scopes               []wireScope               `json:"scopeDefinitions"`
	Entities             []wireEntity              `json:"entities"`
	Relations            []wireRelation            `json:"relations"`
	Bindings             []wireBinding             `json:"valueBindings"`
	Constraints          []wireConstraint          `json:"constraints"`
	Rules                []wireRule                `json:"rules"`
	Invariants           []wireInvariant           `json:"invariants,omitempty"`
	FunctionRequirements []wireFunctionRequirement `json:"functionRequirements,omitempty"`
}
type wireEnumOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type wireEnumDefinition struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Options []wireEnumOption `json:"options"`
}
type wireEntityKey struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	EntityTypeID          string   `json:"entityTypeId"`
	PropertyDefinitionIDs []string `json:"propertyDefinitionIds"`
}
type wireCardinality struct {
	Min string  `json:"min"`
	Max *string `json:"max"`
}
type wireInvariantSubject struct {
	Kind   string `json:"kind"`
	TypeID string `json:"typeId"`
}
type wireInvariant struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Subject       wireInvariantSubject `json:"subject"`
	Applicability *wireExpression      `json:"applicability"`
	Assertion     wireExpression       `json:"assertion"`
}
type wireFunctionRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Major     uint32 `json:"major"`
	Minor     uint32 `json:"minor"`
	Patch     uint32 `json:"patch"`
}
type wireFunctionType struct {
	ValueType        string    `json:"valueType"`
	EnumDefinitionID *string   `json:"enumDefinitionId"`
	Unit             *wireUnit `json:"unit"`
	TemporalMode     string    `json:"temporalMode"`
	Nullable         bool      `json:"nullable"`
}
type wireFunctionRequirement struct {
	Ref            wireFunctionRef    `json:"ref"`
	Parameters     []wireFunctionType `json:"parameters"`
	Result         wireFunctionType   `json:"result"`
	NullableResult bool               `json:"nullableResult"`
}

type wirePrecision struct {
	Scale    string `json:"scale"`
	Rounding string `json:"rounding"`
}

type wireEntityType struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Traits              []string `json:"traits"`
	PropertyDefinitions []string `json:"propertyDefinitions"`
}

type wireTypeConstraint struct {
	Kind string  `json:"kind"`
	ID   *string `json:"id"`
}

type wireRelationType struct {
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	From                *wireTypeConstraint `json:"from"`
	To                  *wireTypeConstraint `json:"to"`
	PropertyDefinitions []string            `json:"propertyDefinitions"`
	OutgoingCardinality *wireCardinality    `json:"outgoingCardinality,omitempty"`
	IncomingCardinality *wireCardinality    `json:"incomingCardinality,omitempty"`
}

type wireTrait struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	RequiredContracts []string `json:"requiredPropertyContracts"`
}

type wireDimensionFactor struct {
	ID       string `json:"id"`
	Exponent int    `json:"exponent"`
}

type wireDimension struct {
	Factors []wireDimensionFactor `json:"factors"`
}

type wireRational struct {
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}

type wireUnitFactor struct {
	ID       string `json:"id"`
	Exponent int    `json:"exponent"`
}

type wireUnit struct {
	Dimension         wireDimension    `json:"dimension"`
	Factors           []wireUnitFactor `json:"factors"`
	ConversionFactors []wireUnitFactor `json:"conversionFactors"`
	Scale             wireRational     `json:"scale"`
}

type wireOwner struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type wireContract struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	ValueType        string         `json:"valueType"`
	Dimension        *wireDimension `json:"dimension"`
	TemporalMode     string         `json:"temporalMode"`
	Nullable         *bool          `json:"nullable"`
	EnumDefinitionID *string        `json:"enumDefinitionId,omitempty"`
}

type wireProperty struct {
	ID               string          `json:"id"`
	Owner            wireOwner       `json:"owner"`
	ContractID       *string         `json:"contractId"`
	Name             string          `json:"name"`
	ValueType        string          `json:"valueType"`
	Unit             *wireUnit       `json:"unit"`
	TemporalMode     string          `json:"temporalMode"`
	Computation      wireComputation `json:"computation"`
	EnumDefinitionID *string         `json:"enumDefinitionId,omitempty"`
}

type wireComputation struct {
	Kind       string          `json:"kind"`
	Expression *wireExpression `json:"expression"`
	Presence   string          `json:"presence,omitempty"`
}

type wireScope struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ValueType string `json:"valueType"`
}

type wireEntity struct {
	ID     string  `json:"id"`
	TypeID string  `json:"typeId"`
	Name   *string `json:"name"`
}

type wireRelation struct {
	ID            string          `json:"id"`
	TypeID        string          `json:"typeId"`
	From          string          `json:"from"`
	To            string          `json:"to"`
	Applicability *wireExpression `json:"applicability"`
}

type wireObjectRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type wireAddress struct {
	Object       wireObjectRef `json:"object"`
	DefinitionID string        `json:"definitionId"`
}

type wireTarget struct {
	Kind         string       `json:"kind"`
	Address      *wireAddress `json:"address"`
	Owner        *wireOwner   `json:"owner"`
	DefinitionID *string      `json:"definitionId"`
}

type wireTyped struct {
	Type             string    `json:"type"`
	Data             string    `json:"data"`
	EnumDefinitionID string    `json:"enumDefinitionId,omitempty"`
	EnumOptionID     string    `json:"enumOptionId,omitempty"`
	Unit             *wireUnit `json:"unit"`
}

type wireBindingValue struct {
	Kind   string      `json:"kind"`
	Value  *wireTyped  `json:"value"`
	Series *wireSeries `json:"series"`
}

type wireSelector struct {
	DefinitionID string    `json:"definitionId"`
	Value        wireTyped `json:"value"`
}

type wireEffectivePeriod struct {
	From *string `json:"from"`
	To   *string `json:"to"`
}

type wireBinding struct {
	Target          wireTarget           `json:"target"`
	Value           wireBindingValue     `json:"value"`
	ScopeSelectors  []wireSelector       `json:"scopeSelectors"`
	EffectivePeriod *wireEffectivePeriod `json:"effectivePeriod"`
}

type wireSeriesSample struct {
	Null  *bool    `json:"null"`
	Value *wireRaw `json:"value"`
}

type wireRaw struct {
	Type             string `json:"type"`
	Data             string `json:"data"`
	EnumDefinitionID string `json:"enumDefinitionId,omitempty"`
	EnumOptionID     string `json:"enumOptionId,omitempty"`
}

type wirePoint struct {
	At     string           `json:"at"`
	Sample wireSeriesSample `json:"sample"`
}

type wireInterval struct {
	From   string           `json:"from"`
	To     string           `json:"to"`
	Sample wireSeriesSample `json:"sample"`
}

type wireSeries struct {
	Axis        string         `json:"axis"`
	ElementType string         `json:"elementType"`
	Unit        *wireUnit      `json:"unit"`
	Points      []wirePoint    `json:"points"`
	Intervals   []wireInterval `json:"intervals"`
}

type wireConstraint struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Applicability *wireExpression `json:"applicability"`
	Assertion     wireExpression  `json:"assertion"`
	Severity      string          `json:"severity"`
	Message       *string         `json:"message"`
}

type wireMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type wireRule struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Condition wireExpression `json:"condition"`
	Metadata  []wireMetadata `json:"metadata"`
}

type wireScenario struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	BaseModelVersionID string         `json:"baseModelVersionId"`
	Overrides          []wireOverride `json:"overrides"`
}

type wireOverride struct {
	Target          wireAddress          `json:"target"`
	Value           wireBindingValue     `json:"value"`
	ScopeSelectors  []wireSelector       `json:"scopeSelectors"`
	EffectivePeriod *wireEffectivePeriod `json:"effectivePeriod"`
}

type wireTraversalSource struct {
	Origin         string         `json:"origin"`
	AbsoluteOrigin *wireObjectRef `json:"absoluteOrigin"`
	Direction      string         `json:"direction"`
	RelationTypeID string         `json:"relationTypeId"`
}

// wireExpression is a tagged AST union. Only fields relevant to Kind may be
// populated; conversion rejects cross-kind fields rather than silently
// accepting a malformed tree.
type wireExpression struct {
	Kind string `json:"kind"`

	Value *wireTyped `json:"value,omitempty"`

	Operator string          `json:"operator,omitempty"`
	Left     *wireExpression `json:"left,omitempty"`
	Right    *wireExpression `json:"right,omitempty"`
	Operand  *wireExpression `json:"operand,omitempty"`

	Condition *wireExpression `json:"condition,omitempty"`
	Then      *wireExpression `json:"then,omitempty"`
	Else      *wireExpression `json:"else,omitempty"`
	Primary   *wireExpression `json:"primary,omitempty"`
	Fallback  *wireExpression `json:"fallback,omitempty"`

	DefinitionID string       `json:"definitionId,omitempty"`
	ContractID   string       `json:"contractId,omitempty"`
	Address      *wireAddress `json:"address,omitempty"`
	Target       string       `json:"target,omitempty"`

	Source    *wireTraversalSource `json:"source,omitempty"`
	Filter    *wireExpression      `json:"filter,omitempty"`
	Map       *wireExpression      `json:"map,omitempty"`
	Operation string               `json:"operation,omitempty"`
	Operands  []wireExpression     `json:"operands,omitempty"`
	Arguments []wireExpression     `json:"arguments,omitempty"`
	Function  *wireFunctionRef     `json:"function,omitempty"`
}
