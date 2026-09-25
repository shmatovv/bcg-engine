package schema

type Cardinality struct {
	min    uint64
	max    uint64
	hasMax bool
}

func NewCardinality(min uint64, max uint64, hasMax bool) Cardinality {
	if hasMax && min > max {
		panic("schema: cardinality min exceeds max")
	}
	return Cardinality{min: min, max: max, hasMax: hasMax}
}
func UnboundedCardinality(min uint64) Cardinality { return Cardinality{min: min} }
func (c Cardinality) Min() uint64                 { return c.min }
func (c Cardinality) Max() (uint64, bool)         { return c.max, c.hasMax }
func (c Cardinality) IsValid() bool               { return !c.hasMax || c.min <= c.max }
