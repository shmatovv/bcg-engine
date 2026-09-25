// Package invariant defines subject-relative Boolean schema assertions.
package invariant

import (
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
)

type SubjectKind uint8

const (
	SubjectInvalid SubjectKind = iota
	SubjectEntityType
	SubjectRelationType
)

func (k SubjectKind) IsValid() bool { return k == SubjectEntityType || k == SubjectRelationType }

type Subject struct {
	kind   SubjectKind
	typeID identity.UUID
}

func EntityTypeSubject(id identity.UUID) Subject {
	if id.IsZero() {
		panic("invariant: zero entity type id")
	}
	return Subject{SubjectEntityType, id}
}
func RelationTypeSubject(id identity.UUID) Subject {
	if id.IsZero() {
		panic("invariant: zero relation type id")
	}
	return Subject{SubjectRelationType, id}
}
func (s Subject) Kind() SubjectKind     { return s.kind }
func (s Subject) TypeID() identity.UUID { return s.typeID }
func (s Subject) IsValid() bool         { return s.kind.IsValid() && !s.typeID.IsZero() }

type Invariant struct {
	id               identity.UUID
	name             string
	subject          Subject
	applicability    ast.Expression
	hasApplicability bool
	assertion        ast.Expression
}

func New(id identity.UUID, name string, subject Subject, app ast.Expression, hasApp bool, assertion ast.Expression) Invariant {
	if id.IsZero() || name == "" || !subject.IsValid() || assertion == nil || (hasApp && app == nil) {
		panic("invariant: invalid declaration")
	}
	if !hasApp {
		app = nil
	}
	return Invariant{id, name, subject, app, hasApp, assertion}
}
func (i Invariant) ID() identity.UUID                     { return i.id }
func (i Invariant) Name() string                          { return i.name }
func (i Invariant) Subject() Subject                      { return i.subject }
func (i Invariant) Applicability() (ast.Expression, bool) { return i.applicability, i.hasApplicability }
func (i Invariant) Assertion() ast.Expression             { return i.assertion }
func (i Invariant) IsValid() bool {
	return !i.id.IsZero() && i.name != "" && i.subject.IsValid() && i.assertion != nil && (!i.hasApplicability || i.applicability != nil)
}
