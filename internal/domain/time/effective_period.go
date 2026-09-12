package time

import (
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
)

type EffectivePeriod struct {
	from    datetime.DateTime
	hasFrom bool

	to    datetime.DateTime
	hasTo bool
}

func NewEffectivePeriod(
	from datetime.DateTime,
	hasFrom bool,
	to datetime.DateTime,
	hasTo bool,
) EffectivePeriod {
	if hasFrom && !from.IsValid() {
		panic("time: effective period from is invalid")
	}

	if hasTo && !to.IsValid() {
		panic("time: effective period to is invalid")
	}

	if !hasFrom {
		from = datetime.DateTime{}
	}

	if !hasTo {
		to = datetime.DateTime{}
	}

	// from < to будем проверять validation-слоем,
	// если текущий DateTime API ещё не предоставляет
	// нужную операцию сравнения.

	return EffectivePeriod{
		from:    from,
		hasFrom: hasFrom,
		to:      to,
		hasTo:   hasTo,
	}
}

func (p EffectivePeriod) From() (datetime.DateTime, bool) {
	return p.from, p.hasFrom
}

func (p EffectivePeriod) To() (datetime.DateTime, bool) {
	return p.to, p.hasTo
}

func (p EffectivePeriod) IsValid() bool {
	if p.hasFrom && !p.from.IsValid() {
		return false
	}

	if p.hasTo && !p.to.IsValid() {
		return false
	}

	return true
}

// Equal reports semantic equality of two optional-ended effective periods.
func (p EffectivePeriod) Equal(other EffectivePeriod) bool {
	if !p.IsValid() || !other.IsValid() ||
		p.hasFrom != other.hasFrom || p.hasTo != other.hasTo {
		return false
	}
	if p.hasFrom && !p.from.Equal(other.from) {
		return false
	}
	if p.hasTo && !p.to.Equal(other.to) {
		return false
	}
	return true
}
