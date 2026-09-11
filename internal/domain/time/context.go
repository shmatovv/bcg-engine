package time

import "github.com/shmatovv/bcg-engine/internal/types/datetime"

// ContextKind identifies the temporal mode of one calculation.
type ContextKind uint8

const (
	ContextInvalid ContextKind = iota
	ContextAt
	ContextPeriod
)

// IsValid reports whether k is a supported calculation time mode.
func (k ContextKind) IsValid() bool {
	return k == ContextAt || k == ContextPeriod
}

// String returns the canonical name of k.
func (k ContextKind) String() string {
	switch k {
	case ContextAt:
		return "At"
	case ContextPeriod:
		return "Period"
	default:
		return "Invalid"
	}
}

// Context defines the temporal dimension of one calculation.
//
// Exactly one mode is active:
//
//	At(t)
//	Period(from, to)
//
// Period uses half-open semantics [from, to) and therefore requires
// from < to.
//
// The zero value is invalid.
type Context struct {
	kind ContextKind

	at datetime.DateTime

	from datetime.DateTime
	to   datetime.DateTime
}

// At constructs a calculation time context for one instant.
func At(at datetime.DateTime) Context {
	if !at.IsValid() {
		panic("time: calculation At instant is invalid")
	}

	return Context{
		kind: ContextAt,
		at:   at,
	}
}

// Period constructs a half-open calculation interval [from, to).
func Period(
	from datetime.DateTime,
	to datetime.DateTime,
) Context {
	if !from.IsValid() {
		panic("time: calculation period from is invalid")
	}

	if !to.IsValid() {
		panic("time: calculation period to is invalid")
	}

	if from.Cmp(to) >= 0 {
		panic("time: calculation period must satisfy from < to")
	}

	return Context{
		kind: ContextPeriod,
		from: from,
		to:   to,
	}
}

// Kind returns the temporal mode represented by c.
func (c Context) Kind() ContextKind {
	return c.kind
}

// At returns the instant stored in an At context.
//
// The boolean result is false for every other context kind.
func (c Context) At() (datetime.DateTime, bool) {
	if c.kind != ContextAt {
		return datetime.DateTime{}, false
	}

	return c.at, true
}

// Period returns the bounds stored in a Period context.
//
// The interval is half-open: [from, to).
// The boolean result is false for every other context kind.
func (c Context) Period() (
	datetime.DateTime,
	datetime.DateTime,
	bool,
) {
	if c.kind != ContextPeriod {
		return datetime.DateTime{},
			datetime.DateTime{},
			false
	}

	return c.from, c.to, true
}

// IsValid reports whether c represents exactly one valid temporal mode.
func (c Context) IsValid() bool {
	switch c.kind {
	case ContextAt:
		return c.at.IsValid() &&
			c.from.IsZero() &&
			c.to.IsZero()

	case ContextPeriod:
		return c.at.IsZero() &&
			c.from.IsValid() &&
			c.to.IsValid() &&
			c.from.Cmp(c.to) < 0

	default:
		return false
	}
}
