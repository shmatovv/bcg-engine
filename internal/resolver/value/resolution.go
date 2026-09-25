package value

import (
	"github.com/shmatovv/bcg-engine/v2/internal/types/datetime"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// ResolutionKind identifies the temporal shape returned by Resolve.
type ResolutionKind uint8

const (
	ResolutionInvalid ResolutionKind = iota
	ResolutionAt
	ResolutionPeriod
)

func (k ResolutionKind) IsValid() bool {
	return k == ResolutionAt ||
		k == ResolutionPeriod
}

// Outcome is one resolved scalar value plus its selected source when one
// exists.
//
// MissingValue and AmbiguousValue have no single selected source.
type Outcome struct {
	resolved enginevalue.Resolved

	source    Source
	hasSource bool
}

func newSelectedOutcome(
	resolved enginevalue.Resolved,
	source Source,
) Outcome {
	if !resolved.IsValid() {
		panic("resolver/value: invalid selected resolved value")
	}

	if !source.IsValid() {
		panic("resolver/value: invalid selected source")
	}

	return Outcome{
		resolved:  resolved,
		source:    source,
		hasSource: true,
	}
}

func newUndefinedOutcome(
	kind enginevalue.UndefinedKind,
) Outcome {
	return Outcome{
		resolved: enginevalue.Undefined(
			enginevalue.NewUndefinedInfo(
				kind,
			),
		),
	}
}

func (o Outcome) Value() enginevalue.Resolved {
	return o.resolved
}

func (o Outcome) Source() (
	Source,
	bool,
) {
	return o.source,
		o.hasSource
}

func (o Outcome) IsValid() bool {
	if !o.resolved.IsValid() {
		return false
	}

	if o.hasSource {
		return o.source.IsValid()
	}

	return !o.source.IsValid() &&
		o.resolved.IsUndefined()
}

// Segment is one finite half-open [from, to) scalar resolution segment.
type Segment struct {
	from datetime.DateTime
	to   datetime.DateTime

	outcome Outcome
}

func newSegment(
	from datetime.DateTime,
	to datetime.DateTime,
	outcome Outcome,
) Segment {
	if !from.IsValid() ||
		!to.IsValid() ||
		from.Cmp(to) >= 0 {
		panic("resolver/value: invalid period segment")
	}

	if !outcome.IsValid() {
		panic("resolver/value: invalid period segment outcome")
	}

	return Segment{
		from:    from,
		to:      to,
		outcome: outcome,
	}
}

func (s Segment) From() datetime.DateTime {
	return s.from
}

func (s Segment) To() datetime.DateTime {
	return s.to
}

func (s Segment) Outcome() Outcome {
	return s.outcome
}

func (s Segment) IsValid() bool {
	return s.from.IsValid() &&
		s.to.IsValid() &&
		s.from.Cmp(s.to) < 0 &&
		s.outcome.IsValid()
}

// Resolution is either one At outcome or an ordered sequence of Period
// segments.
type Resolution struct {
	kind ResolutionKind

	at Outcome

	segments []Segment
}

func newAtResolution(
	outcome Outcome,
) Resolution {
	if !outcome.IsValid() {
		panic("resolver/value: invalid At outcome")
	}

	return Resolution{
		kind: ResolutionAt,
		at:   outcome,
	}
}

func newPeriodResolution(
	segments []Segment,
) Resolution {
	if len(segments) == 0 {
		panic("resolver/value: period resolution must contain segments")
	}

	copied := make(
		[]Segment,
		len(segments),
	)

	copy(
		copied,
		segments,
	)

	for index, segment := range copied {

		if !segment.IsValid() {
			panic("resolver/value: invalid period resolution segment")
		}

		if index > 0 &&
			!copied[index-1].
				to.
				Equal(segment.from) {
			panic("resolver/value: period resolution segments must be contiguous")
		}
	}

	return Resolution{
		kind:     ResolutionPeriod,
		segments: copied,
	}
}

func (r Resolution) Kind() ResolutionKind {
	return r.kind
}

func (r Resolution) At() (
	Outcome,
	bool,
) {
	if r.kind != ResolutionAt {
		return Outcome{},
			false
	}

	return r.at,
		true
}

func (r Resolution) Segments() []Segment {
	if r.kind != ResolutionPeriod ||
		len(r.segments) == 0 {
		return nil
	}

	result := make(
		[]Segment,
		len(r.segments),
	)

	copy(
		result,
		r.segments,
	)

	return result
}

func (r Resolution) IsValid() bool {
	switch r.kind {
	case ResolutionAt:
		return r.at.IsValid() &&
			len(r.segments) == 0

	case ResolutionPeriod:
		if r.at.IsValid() ||
			len(r.segments) == 0 {
			return false
		}

		for index, segment := range r.segments {

			if !segment.IsValid() {
				return false
			}

			if index > 0 &&
				!r.segments[index-1].
					to.
					Equal(segment.from) {
				return false
			}
		}

		return true

	default:
		return false
	}
}
