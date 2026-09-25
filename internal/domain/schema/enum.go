package schema

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
)

// MaxEnumOptions bounds one enum declaration. It is deliberately generous:
// the limit is a host-safety bound for untrusted artifacts, not a business
// modelling recommendation.
const MaxEnumOptions = 4096

type EnumOption struct {
	id   identity.UUID
	name string
}

func NewEnumOption(id identity.UUID, name string) EnumOption {
	if id.IsZero() || name == "" {
		panic("schema: invalid enum option")
	}
	return EnumOption{id: id, name: name}
}
func (o EnumOption) ID() identity.UUID { return o.id }
func (o EnumOption) Name() string      { return o.name }
func (o EnumOption) IsValid() bool     { return !o.id.IsZero() && o.name != "" }

type EnumDefinition struct {
	id      identity.UUID
	name    string
	options []EnumOption
}

func NewEnumDefinition(id identity.UUID, name string, options []EnumOption) EnumDefinition {
	if id.IsZero() || name == "" {
		panic("schema: invalid enum definition")
	}
	if len(options) == 0 || len(options) > MaxEnumOptions {
		panic("schema: invalid enum option count")
	}
	seen := map[identity.UUID]struct{}{}
	result := make([]EnumOption, len(options))
	for i, option := range options {
		if !option.IsValid() || option.ID().IsZero() {
			panic("schema: invalid enum option")
		}
		if _, ok := seen[option.ID()]; ok {
			panic("schema: duplicate enum option id")
		}
		seen[option.ID()] = struct{}{}
		result[i] = option
	}
	// Enum option order is explicitly non-semantic. Canonicalize it at the
	// ownership boundary so all public inspection paths are deterministic.
	sort.Slice(result, func(i, j int) bool { return result[i].ID().String() < result[j].ID().String() })
	return EnumDefinition{id: id, name: name, options: result}
}
func (d EnumDefinition) ID() identity.UUID     { return d.id }
func (d EnumDefinition) Name() string          { return d.name }
func (d EnumDefinition) Options() []EnumOption { return append([]EnumOption(nil), d.options...) }
func (d EnumDefinition) Option(id identity.UUID) (EnumOption, bool) {
	for _, option := range d.options {
		if option.ID() == id {
			return option, true
		}
	}
	return EnumOption{}, false
}
func (d EnumDefinition) IsValid() bool {
	if d.id.IsZero() || d.name == "" || len(d.options) == 0 || len(d.options) > MaxEnumOptions {
		return false
	}
	seen := map[identity.UUID]struct{}{}
	for _, o := range d.options {
		if !o.IsValid() {
			return false
		}
		if _, ok := seen[o.ID()]; ok {
			return false
		}
		seen[o.ID()] = struct{}{}
	}
	return true
}
