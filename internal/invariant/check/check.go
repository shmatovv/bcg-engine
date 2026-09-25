package check

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	domaininvariant "github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
	"sort"
)

type State uint8

const (
	StateInvalid State = iota
	StateSatisfied
	StateViolated
	StateNotApplicable
	StateUndefined
)

func (s State) IsValid() bool { return s >= StateSatisfied && s <= StateUndefined }
func (s State) String() string {
	switch s {
	case StateSatisfied:
		return "Satisfied"
	case StateViolated:
		return "Violated"
	case StateNotApplicable:
		return "NotApplicable"
	case StateUndefined:
		return "Undefined"
	}
	return "Invalid"
}

type Source interface {
	evaluator.Source
	Invariant(identity.UUID) (domaininvariant.Invariant, bool)
	Invariants() []domaininvariant.Invariant
}
type Result struct {
	invariant domaininvariant.Invariant
	subject   address.ObjectRef
	state     State
}

func (r Result) Invariant() domaininvariant.Invariant { return r.invariant }
func (r Result) Subject() address.ObjectRef           { return r.subject }
func (r Result) State() State                         { return r.state }
func (r Result) IsValid() bool {
	return r.invariant.IsValid() && r.subject.IsValid() && r.state.IsValid()
}

type Report struct{ results []Result }

func (r Report) Results() []Result { return append([]Result(nil), r.results...) }
func (r Report) Len() int          { return len(r.results) }
func One(source Source, id identity.UUID, subject address.ObjectRef, context runtime.CalculationContext) (Result, error) {
	inv, ok := source.Invariant(id)
	if !ok {
		return Result{}, &Error{"invariant not found"}
	}
	return evaluate(source, inv, subject, context)
}
func ForObject(source Source, subject address.ObjectRef, context runtime.CalculationContext) (Report, error) {
	items := source.Invariants()
	sort.Slice(items, func(i, j int) bool { return items[i].ID().String() < items[j].ID().String() })
	out := make([]Result, 0, len(items))
	for _, v := range items {
		if !matches(v, subject, source) {
			continue
		}
		r, e := evaluate(source, v, subject, context)
		if e != nil {
			return Report{}, e
		}
		out = append(out, r)
	}
	return Report{out}, nil
}
func All(source Source, context runtime.CalculationContext) (Report, error) {
	var out []Result
	for _, e := range source.Entities() {
		r, err := ForObject(source, address.EntityRef(e.ID()), context)
		if err != nil {
			return Report{}, err
		}
		out = append(out, r.Results()...)
	}
	for _, rln := range source.Relations() {
		r, err := ForObject(source, address.RelationRef(rln.ID()), context)
		if err != nil {
			return Report{}, err
		}
		out = append(out, r.Results()...)
	}
	return Report{out}, nil
}
func matches(inv domaininvariant.Invariant, subject address.ObjectRef, source Source) bool {
	owner, ok := source.ObjectType(subject)
	if !ok {
		return false
	}
	s := inv.Subject()
	return (s.Kind() == domaininvariant.SubjectEntityType && owner.Kind().String() == "EntityType" || s.Kind() == domaininvariant.SubjectRelationType && owner.Kind().String() == "RelationType") && owner.ID() == s.TypeID()
}
func evaluate(source Source, inv domaininvariant.Invariant, subject address.ObjectRef, context runtime.CalculationContext) (Result, error) {
	if !matches(inv, subject, source) {
		return Result{}, &Error{"invariant subject does not match object"}
	}
	if app, ok := inv.Applicability(); ok {
		r, e := evaluator.EvaluateExpressionForSelf(source, app, subject, context)
		if e != nil {
			return Result{}, e
		}
		b, u, e := evaluator.InterpretScalarBool(r.Value())
		if e != nil {
			return Result{}, e
		}
		if u {
			return Result{inv, subject, StateUndefined}, nil
		}
		if !b {
			return Result{inv, subject, StateNotApplicable}, nil
		}
	}
	r, e := evaluator.EvaluateExpressionForSelf(source, inv.Assertion(), subject, context)
	if e != nil {
		return Result{}, e
	}
	b, u, e := evaluator.InterpretScalarBool(r.Value())
	if e != nil {
		return Result{}, e
	}
	if u {
		return Result{inv, subject, StateUndefined}, nil
	}
	if b {
		return Result{inv, subject, StateSatisfied}, nil
	}
	return Result{inv, subject, StateViolated}, nil
}

type Error struct{ message string }

func (e *Error) Error() string { return "invariant check: " + e.message }
