package bcg

import (
	"fmt"
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/invariant"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	invariantcheck "github.com/shmatovv/bcg-engine/v2/internal/invariant/check"
	"github.com/shmatovv/bcg-engine/v2/internal/runtime"
)

// EnumOptionSpec describes a stable named enum option.
type EnumOptionSpec struct {
	ID   ID
	Name string
}
type EnumOption struct{ value schema.EnumOption }

func (o EnumOption) ID() ID        { return idFromInternal(o.value.ID()) }
func (o EnumOption) Name() string  { return o.value.Name() }
func (o EnumOption) IsValid() bool { return o.value.IsValid() }

type EnumDefinitionSpec struct {
	ID      ID
	Name    string
	Options []EnumOptionSpec
}
type EnumDefinition struct{ value schema.EnumDefinition }

func NewEnumDefinition(spec EnumDefinitionSpec) (EnumDefinition, error) {
	id, err := requiredSchemaID(spec.ID, "enum definition ID")
	if err != nil {
		return EnumDefinition{}, err
	}
	opts := make([]schema.EnumOption, len(spec.Options))
	seen := map[ID]bool{}
	for i, o := range spec.Options {
		if o.ID.IsZero() || o.Name == "" {
			return EnumDefinition{}, invalidSchemaArgument("enum option is invalid")
		}
		if seen[o.ID] {
			return EnumDefinition{}, invalidSchemaArgument("enum option IDs must be unique")
		}
		seen[o.ID] = true
		opts[i] = schema.NewEnumOption(o.ID.value, o.Name)
	}
	if len(opts) == 0 {
		return EnumDefinition{}, invalidSchemaArgument("enum definition requires an option")
	}
	return EnumDefinition{schema.NewEnumDefinition(id, spec.Name, opts)}, nil
}
func (e EnumDefinition) ID() ID       { return idFromInternal(e.value.ID()) }
func (e EnumDefinition) Name() string { return e.value.Name() }
func (e EnumDefinition) Options() []EnumOption {
	items := e.value.Options()
	out := make([]EnumOption, len(items))
	for i, v := range items {
		out[i] = EnumOption{v}
	}
	return out
}
func (e EnumDefinition) Option(id ID) (EnumOption, bool) {
	v, ok := e.value.Option(id.value)
	return EnumOption{v}, ok
}
func (e EnumDefinition) IsValid() bool { return e.value.IsValid() }

// EntityKeyDefinition represents business uniqueness for one EntityType.
type EntityKeyDefinitionSpec struct {
	ID                    ID
	Name                  string
	EntityTypeID          ID
	PropertyDefinitionIDs []ID
}
type EntityKeyDefinition struct{ value schema.EntityKeyDefinition }

func NewEntityKeyDefinition(spec EntityKeyDefinitionSpec) (EntityKeyDefinition, error) {
	id, err := requiredSchemaID(spec.ID, "entity key ID")
	if err != nil {
		return EntityKeyDefinition{}, err
	}
	et, err := requiredSchemaID(spec.EntityTypeID, "entity key entity type ID")
	if err != nil {
		return EntityKeyDefinition{}, err
	}
	p, err := schemaIDs(spec.PropertyDefinitionIDs, "entity key property definition")
	if err != nil {
		return EntityKeyDefinition{}, err
	}
	if spec.Name == "" {
		return EntityKeyDefinition{}, invalidSchemaArgument("entity key name must not be empty")
	}
	return EntityKeyDefinition{schema.NewEntityKeyDefinition(id, spec.Name, et, p)}, nil
}
func (k EntityKeyDefinition) ID() ID           { return idFromInternal(k.value.ID()) }
func (k EntityKeyDefinition) Name() string     { return k.value.Name() }
func (k EntityKeyDefinition) EntityTypeID() ID { return idFromInternal(k.value.EntityTypeID()) }
func (k EntityKeyDefinition) PropertyDefinitionIDs() []ID {
	return publicSchemaIDs(k.value.PropertyDefinitionIDs())
}
func (k EntityKeyDefinition) IsValid() bool { return k.value.IsValid() }

type InvariantSubjectKind uint8

const (
	InvariantSubjectEntityType InvariantSubjectKind = iota + 1
	InvariantSubjectRelationType
)

type InvariantSubject struct {
	kind   InvariantSubjectKind
	typeID ID
}

func EntityInvariantSubject(id ID) (InvariantSubject, error) {
	if id.IsZero() {
		return InvariantSubject{}, invalidSchemaArgument("entity invariant subject ID is invalid")
	}
	return InvariantSubject{InvariantSubjectEntityType, id}, nil
}
func RelationInvariantSubject(id ID) (InvariantSubject, error) {
	if id.IsZero() {
		return InvariantSubject{}, invalidSchemaArgument("relation invariant subject ID is invalid")
	}
	return InvariantSubject{InvariantSubjectRelationType, id}, nil
}
func (s InvariantSubject) Kind() InvariantSubjectKind { return s.kind }
func (s InvariantSubject) TypeID() ID                 { return s.typeID }
func (s InvariantSubject) IsValid() bool {
	return (s.kind == InvariantSubjectEntityType || s.kind == InvariantSubjectRelationType) && !s.typeID.IsZero()
}
func (s InvariantSubject) internal() (invariant.Subject, error) {
	if !s.IsValid() {
		return invariant.Subject{}, invalidSchemaArgument("invariant subject is invalid")
	}
	if s.kind == InvariantSubjectEntityType {
		return invariant.EntityTypeSubject(s.typeID.value), nil
	}
	return invariant.RelationTypeSubject(s.typeID.value), nil
}
func invariantSubjectFromInternal(s invariant.Subject) InvariantSubject {
	if s.Kind() == invariant.SubjectEntityType {
		return InvariantSubject{InvariantSubjectEntityType, idFromInternal(s.TypeID())}
	}
	return InvariantSubject{InvariantSubjectRelationType, idFromInternal(s.TypeID())}
}

type InvariantSpec struct {
	ID            ID
	Name          string
	Subject       InvariantSubject
	Applicability Expression
	Assertion     Expression
}
type Invariant struct{ value invariant.Invariant }

func NewInvariant(spec InvariantSpec) (Invariant, error) {
	id, err := requiredSchemaID(spec.ID, "invariant ID")
	if err != nil {
		return Invariant{}, err
	}
	s, err := spec.Subject.internal()
	if err != nil {
		return Invariant{}, err
	}
	if spec.Name == "" || !spec.Assertion.IsValid() {
		return Invariant{}, invalidSchemaArgument("invariant name or assertion is invalid")
	}
	app, has := spec.Applicability.value, spec.Applicability.IsValid()
	return Invariant{invariant.New(id, spec.Name, s, app, has, spec.Assertion.value)}, nil
}
func (i Invariant) ID() ID                    { return idFromInternal(i.value.ID()) }
func (i Invariant) Name() string              { return i.value.Name() }
func (i Invariant) Subject() InvariantSubject { return invariantSubjectFromInternal(i.value.Subject()) }
func (i Invariant) Applicability() (Expression, bool) {
	v, ok := i.value.Applicability()
	return Expression{value: v}, ok
}
func (i Invariant) Assertion() Expression { return Expression{value: i.value.Assertion()} }
func (i Invariant) IsValid() bool         { return i.value.IsValid() }

func (d *Draft) AddEnumDefinition(v EnumDefinition) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !v.IsValid() {
		return invalidSchemaArgument("enum definition is invalid")
	}
	d.value.AddEnumDefinition(v.value)
	return nil
}
func (d *Draft) ReplaceEnumDefinition(id ID, v EnumDefinition) error {
	if err := d.requireSchemaReplacement(id, v.ID(), v.IsValid(), "enum definition"); err != nil {
		return err
	}
	if !d.value.ReplaceEnumDefinition(id.value, v.value) {
		return schemaItemNotFound("enum definition", id)
	}
	return nil
}
func (d *Draft) RemoveEnumDefinition(id ID) error {
	if err := d.requireSchemaDraftID(id, "enum definition ID"); err != nil {
		return err
	}
	if !d.value.RemoveEnumDefinition(id.value) {
		return schemaItemNotFound("enum definition", id)
	}
	return nil
}
func (d *Draft) AddEntityKeyDefinition(v EntityKeyDefinition) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !v.IsValid() {
		return invalidSchemaArgument("entity key definition is invalid")
	}
	d.value.AddEntityKeyDefinition(v.value)
	return nil
}
func (d *Draft) ReplaceEntityKeyDefinition(id ID, v EntityKeyDefinition) error {
	if err := d.requireSchemaReplacement(id, v.ID(), v.IsValid(), "entity key definition"); err != nil {
		return err
	}
	if !d.value.ReplaceEntityKeyDefinition(id.value, v.value) {
		return schemaItemNotFound("entity key definition", id)
	}
	return nil
}
func (d *Draft) RemoveEntityKeyDefinition(id ID) error {
	if err := d.requireSchemaDraftID(id, "entity key definition ID"); err != nil {
		return err
	}
	if !d.value.RemoveEntityKeyDefinition(id.value) {
		return schemaItemNotFound("entity key definition", id)
	}
	return nil
}
func (d *Draft) AddInvariant(v Invariant) error {
	if err := d.requireSchemaDraft(); err != nil {
		return err
	}
	if !v.IsValid() {
		return invalidSchemaArgument("invariant is invalid")
	}
	d.value.AddInvariant(v.value)
	return nil
}
func (d *Draft) ReplaceInvariant(id ID, v Invariant) error {
	if err := d.requireSchemaReplacement(id, v.ID(), v.IsValid(), "invariant"); err != nil {
		return err
	}
	if !d.value.ReplaceInvariant(id.value, v.value) {
		return schemaItemNotFound("invariant", id)
	}
	return nil
}
func (d *Draft) RemoveInvariant(id ID) error {
	if err := d.requireSchemaDraftID(id, "invariant ID"); err != nil {
		return err
	}
	if !d.value.RemoveInvariant(id.value) {
		return schemaItemNotFound("invariant", id)
	}
	return nil
}

func (m Model) EnumDefinitions() []EnumDefinition {
	if !m.IsValid() {
		return nil
	}
	v := m.version.EnumDefinitions()
	r := make([]EnumDefinition, len(v))
	for i, x := range v {
		r[i] = EnumDefinition{x}
	}
	sort.Slice(r, func(i, j int) bool { return r[i].ID().String() < r[j].ID().String() })
	return r
}
func (m Model) EntityKeyDefinitions() []EntityKeyDefinition {
	if !m.IsValid() {
		return nil
	}
	v := m.version.EntityKeyDefinitions()
	r := make([]EntityKeyDefinition, len(v))
	for i, x := range v {
		r[i] = EntityKeyDefinition{x}
	}
	sort.Slice(r, func(i, j int) bool { return r[i].ID().String() < r[j].ID().String() })
	return r
}
func (m Model) InvariantDefinitions() []Invariant {
	if !m.IsValid() {
		return nil
	}
	v := m.version.Invariants()
	r := make([]Invariant, len(v))
	for i, x := range v {
		r[i] = Invariant{x}
	}
	sort.Slice(r, func(i, j int) bool { return r[i].ID().String() < r[j].ID().String() })
	return r
}

type InvariantState uint8

const (
	InvariantSatisfied InvariantState = iota + 1
	InvariantViolated
	InvariantNotApplicable
	InvariantUndefined
)

func (s InvariantState) String() string {
	switch s {
	case InvariantSatisfied:
		return "Satisfied"
	case InvariantViolated:
		return "Violated"
	case InvariantNotApplicable:
		return "NotApplicable"
	case InvariantUndefined:
		return "Undefined"
	}
	return "Invalid"
}

type InvariantResult struct {
	invariant   Invariant
	subjectID   ID
	subjectKind ObjectKind
	state       InvariantState
}

func (r InvariantResult) Invariant() Invariant    { return r.invariant }
func (r InvariantResult) SubjectID() ID           { return r.subjectID }
func (r InvariantResult) SubjectKind() ObjectKind { return r.subjectKind }
func (r InvariantResult) State() InvariantState   { return r.state }

type InvariantReport struct{ results []InvariantResult }

func (r InvariantReport) Results() []InvariantResult {
	return append([]InvariantResult(nil), r.results...)
}
func (r InvariantReport) Len() int { return len(r.results) }
func invariantStateFromInternal(s invariantcheck.State) InvariantState {
	switch s {
	case invariantcheck.StateSatisfied:
		return InvariantSatisfied
	case invariantcheck.StateViolated:
		return InvariantViolated
	case invariantcheck.StateNotApplicable:
		return InvariantNotApplicable
	case invariantcheck.StateUndefined:
		return InvariantUndefined
	}
	return 0
}
func invariantResultFromInternal(r invariantcheck.Result) InvariantResult {
	k := ObjectEntity
	if r.Subject().Kind() == address.ObjectRefRelation {
		k = ObjectRelation
	}
	return InvariantResult{Invariant{r.Invariant()}, idFromInternal(r.Subject().ID()), k, invariantStateFromInternal(r.State())}
}

// CheckInvariant evaluates an Entity-subject invariant.
func (m Model) CheckInvariant(id ID, subject Entity, context CalculationContext) (InvariantResult, error) {
	return m.checkInvariantObject(id, address.EntityRef(subject.value.ID()), context)
}
func (m Model) CheckInvariantForRelation(id ID, subject Relation, context CalculationContext) (InvariantResult, error) {
	return m.checkInvariantObject(id, address.RelationRef(subject.value.ID()), context)
}
func (m Model) checkInvariantObject(id ID, subject address.ObjectRef, context CalculationContext) (InvariantResult, error) {
	if !m.IsValid() {
		return InvariantResult{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	if err := DefaultRuntime().ValidateModel(m); err != nil {
		return InvariantResult{}, err
	}
	ctx, err := context.internal()
	if err != nil {
		return InvariantResult{}, err
	}
	return checkInvariantObjectSource(evaluator.BindRuntime(m.version, DefaultRuntime()), id.value, subject, ctx)
}
func (m Model) CheckInvariantsForEntity(subject Entity, context CalculationContext) (InvariantReport, error) {
	return m.checkInvariantsForObject(address.EntityRef(subject.value.ID()), context)
}
func (m Model) CheckInvariantsForRelation(subject Relation, context CalculationContext) (InvariantReport, error) {
	return m.checkInvariantsForObject(address.RelationRef(subject.value.ID()), context)
}
func (m Model) checkInvariantsForObject(subject address.ObjectRef, context CalculationContext) (InvariantReport, error) {
	if !m.IsValid() {
		return InvariantReport{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	if err := DefaultRuntime().ValidateModel(m); err != nil {
		return InvariantReport{}, err
	}
	ctx, err := context.internal()
	if err != nil {
		return InvariantReport{}, err
	}
	return checkInvariantsForObjectSource(evaluator.BindRuntime(m.version, DefaultRuntime()), subject, ctx)
}

// CheckInvariants evaluates every invariant for every matching model object in
// deterministic subject and invariant ID order.
func (m Model) CheckInvariants(context CalculationContext) (InvariantReport, error) {
	if !m.IsValid() {
		return InvariantReport{}, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	if err := DefaultRuntime().ValidateModel(m); err != nil {
		return InvariantReport{}, err
	}
	ctx, err := context.internal()
	if err != nil {
		return InvariantReport{}, err
	}
	return checkAllInvariantsSource(evaluator.BindRuntime(m.version, DefaultRuntime()), ctx)
}

func checkInvariantObjectSource(source invariantcheck.Source, id identity.UUID, subject address.ObjectRef, context runtime.CalculationContext) (InvariantResult, error) {
	r, err := invariantcheck.One(source, id, subject, context)
	if err != nil {
		return InvariantResult{}, publicError(err)
	}
	return invariantResultFromInternal(r), nil
}

func checkInvariantsForObjectSource(source invariantcheck.Source, subject address.ObjectRef, context runtime.CalculationContext) (InvariantReport, error) {
	report, err := invariantcheck.ForObject(source, subject, context)
	if err != nil {
		return InvariantReport{}, publicError(err)
	}
	items := report.Results()
	out := make([]InvariantResult, len(items))
	for i, v := range items {
		out[i] = invariantResultFromInternal(v)
	}
	return InvariantReport{out}, nil
}

func checkAllInvariantsSource(source invariantcheck.Source, context runtime.CalculationContext) (InvariantReport, error) {
	report, err := invariantcheck.All(source, context)
	if err != nil {
		return InvariantReport{}, publicError(err)
	}
	items := report.Results()
	out := make([]InvariantResult, len(items))
	for i, v := range items {
		out[i] = invariantResultFromInternal(v)
	}
	return InvariantReport{out}, nil
}
