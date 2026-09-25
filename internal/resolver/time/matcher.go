package time

import (
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
)

// ContainsAt reports whether an effective-dated scalar binding applies at
// instant.
//
// Effective periods use half-open semantics [from, to).
//
// A binding without an EffectivePeriod is always effective.
func ContainsAt(
	period domaintime.EffectivePeriod,
	hasPeriod bool,
	instant datetime.DateTime,
) bool {
	if !instant.IsValid() {
		return false
	}

	if !hasPeriod {
		return true
	}

	if !period.IsValid() {
		return false
	}

	from, hasFrom := period.From()
	to, hasTo := period.To()

	if hasFrom && from.Cmp(instant) > 0 {
		return false
	}

	if hasTo && instant.Cmp(to) >= 0 {
		return false
	}

	return true
}

// Intersection represents the non-empty intersection of a calculation
// Period and one effective-dated binding.
//
// It is always a finite half-open interval [from, to).
type Intersection struct {
	from datetime.DateTime
	to   datetime.DateTime
}

// From returns the inclusive intersection start.
func (i Intersection) From() datetime.DateTime {
	return i.from
}

// To returns the exclusive intersection end.
func (i Intersection) To() datetime.DateTime {
	return i.to
}

// IsValid reports whether i is a non-empty finite half-open interval.
func (i Intersection) IsValid() bool {
	return i.from.IsValid() &&
		i.to.IsValid() &&
		i.from.Cmp(i.to) < 0
}

// IntersectPeriod intersects one effective-dated binding with a finite
// calculation window [calculationFrom, calculationTo).
//
// A binding without an EffectivePeriod intersects the entire calculation
// window.
func IntersectPeriod(
	period domaintime.EffectivePeriod,
	hasPeriod bool,
	calculationFrom datetime.DateTime,
	calculationTo datetime.DateTime,
) (Intersection, bool) {
	if !calculationFrom.IsValid() ||
		!calculationTo.IsValid() ||
		calculationFrom.Cmp(calculationTo) >= 0 {
		return Intersection{}, false
	}

	from := calculationFrom
	to := calculationTo

	if hasPeriod {
		if !period.IsValid() {
			return Intersection{}, false
		}

		effectiveFrom, hasFrom := period.From()
		effectiveTo, hasTo := period.To()

		if hasFrom && effectiveFrom.Cmp(from) > 0 {
			from = effectiveFrom
		}

		if hasTo && effectiveTo.Cmp(to) < 0 {
			to = effectiveTo
		}
	}

	if from.Cmp(to) >= 0 {
		return Intersection{}, false
	}

	return Intersection{
		from: from,
		to:   to,
	}, true
}
