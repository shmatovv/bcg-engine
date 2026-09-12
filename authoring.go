package bcg

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	internalmodel "github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
)

// RoundingMode specifies how finite-precision decimal operations round.
// It is part of a committed Model's immutable arithmetic semantics.
type RoundingMode uint8

const (
	// RoundTruncate rounds toward zero.
	RoundTruncate RoundingMode = iota + 1
	// RoundFloor rounds toward negative infinity.
	RoundFloor
	// RoundCeil rounds toward positive infinity.
	RoundCeil
	// RoundHalfUp rounds ties away from zero.
	RoundHalfUp
	// RoundHalfEven rounds ties to an even final digit.
	RoundHalfEven
)

// IsValid reports whether r is a supported rounding mode.
func (r RoundingMode) IsValid() bool {
	return r >= RoundTruncate && r <= RoundHalfEven
}

// String returns the stable rounding-mode name.
func (r RoundingMode) String() string {
	switch r {
	case RoundTruncate:
		return "Truncate"
	case RoundFloor:
		return "Floor"
	case RoundCeil:
		return "Ceil"
	case RoundHalfUp:
		return "HalfUp"
	case RoundHalfEven:
		return "HalfEven"
	default:
		return "Invalid"
	}
}

// PrecisionPolicy defines the decimal arithmetic policy of a committed
// Model version. Its zero value is invalid.
type PrecisionPolicy struct {
	scale    uint64
	rounding RoundingMode
}

// NewPrecisionPolicy creates a valid arithmetic policy. The maximum scale is
// MaxPrecisionScale, the maximum supported by BCG Decimal values.
func NewPrecisionPolicy(scale uint64, rounding RoundingMode) (PrecisionPolicy, error) {
	policy := PrecisionPolicy{scale: scale, rounding: rounding}
	if !policy.IsValid() {
		return PrecisionPolicy{}, fmt.Errorf("%w: precision policy is invalid", ErrInvalidArgument)
	}
	return policy, nil
}

// MaxPrecisionScale is the largest decimal scale supported in a committed
// Model precision policy.
const MaxPrecisionScale = decimal.MaxScale

// Scale returns the number of retained fractional decimal digits.
func (p PrecisionPolicy) Scale() uint64 { return p.scale }

// Rounding returns the policy's rounding mode.
func (p PrecisionPolicy) Rounding() RoundingMode { return p.rounding }

// IsValid reports whether p can be used to commit a Model.
func (p PrecisionPolicy) IsValid() bool {
	return p.rounding.IsValid() && p.scale <= MaxPrecisionScale
}

func (p PrecisionPolicy) internal() (precision.Policy, error) {
	if !p.IsValid() {
		return precision.Policy{}, fmt.Errorf("%w: precision policy is invalid", ErrInvalidArgument)
	}
	return precision.New(p.scale, p.rounding.internal()), nil
}

func precisionPolicyFromInternal(policy precision.Policy) PrecisionPolicy {
	return PrecisionPolicy{scale: policy.Scale(), rounding: roundingModeFromInternal(policy.Rounding())}
}

func (r RoundingMode) internal() precision.Mode {
	switch r {
	case RoundTruncate:
		return precision.Truncate
	case RoundFloor:
		return precision.Floor
	case RoundCeil:
		return precision.Ceil
	case RoundHalfUp:
		return precision.HalfUp
	case RoundHalfEven:
		return precision.HalfEven
	default:
		return precision.Invalid
	}
}

func roundingModeFromInternal(mode precision.Mode) RoundingMode {
	switch mode {
	case precision.Truncate:
		return RoundTruncate
	case precision.Floor:
		return RoundFloor
	case precision.Ceil:
		return RoundCeil
	case precision.HalfUp:
		return RoundHalfUp
	case precision.HalfEven:
		return RoundHalfEven
	default:
		return 0
	}
}

// CommitOptions identifies and configures one new immutable Model version.
// Version identity is always explicit; Commit never generates one silently.
type CommitOptions struct {
	versionID ID
	label     string
	precision PrecisionPolicy
}

// NewCommitOptions creates valid options for Draft.Commit.
func NewCommitOptions(versionID ID, label string, precision PrecisionPolicy) (CommitOptions, error) {
	if _, err := versionID.internal(); err != nil {
		return CommitOptions{}, err
	}
	if !precision.IsValid() {
		return CommitOptions{}, fmt.Errorf("%w: precision policy is invalid", ErrInvalidArgument)
	}
	return CommitOptions{versionID: versionID, label: label, precision: precision}, nil
}

// VersionID returns the identity reserved for the committed version.
func (o CommitOptions) VersionID() ID { return o.versionID }

// VersionLabel returns the user-facing version label.
func (o CommitOptions) VersionLabel() string { return o.label }

// PrecisionPolicy returns the committed arithmetic policy.
func (o CommitOptions) PrecisionPolicy() PrecisionPolicy { return o.precision }

// IsValid reports whether o can be used by Draft.Commit.
func (o CommitOptions) IsValid() bool {
	return !o.versionID.IsZero() && o.precision.IsValid()
}

func (o CommitOptions) internal() (internalmodel.CommitOptions, error) {
	versionID, err := o.versionID.internal()
	if err != nil {
		return internalmodel.CommitOptions{}, err
	}
	policy, err := o.precision.internal()
	if err != nil {
		return internalmodel.CommitOptions{}, err
	}
	return internalmodel.NewCommitOptions(versionID, o.label, policy), nil
}

// Draft is a mutable candidate for one logical Model. It is not safe for
// concurrent mutation. A Draft may be invalid while an author is editing it;
// only Commit produces an immutable, valid Model.
//
// Draft exposes typed schema, semantic-data, persistent-binding, Constraint,
// and Rule operations while keeping all internal domain objects private.
type Draft struct {
	modelID identity.UUID
	name    string
	value   *internalmodel.Draft

	parentVersionID identity.UUID
	hasParent       bool
}

// NewModel creates a new logical Model identity. It has no committed version
// until one of its Drafts is committed.
func NewModel(id ID, name string) (Model, error) {
	modelID, err := id.internal()
	if err != nil {
		return Model{}, err
	}
	if name == "" {
		return Model{}, fmt.Errorf("%w: model name must not be empty", ErrInvalidArgument)
	}
	return Model{modelID: modelID, name: name}, nil
}

// IsLogical reports whether m contains a valid logical Model identity. It may
// be true for a new model that has not yet been committed; use IsValid to test
// whether it is an immutable evaluatable Model version.
func (m Model) IsLogical() bool { return !m.modelID.IsZero() && m.name != "" }

// PrecisionPolicy returns the immutable arithmetic policy for a committed
// Model. It returns false for a new logical Model or an invalid wrapper.
func (m Model) PrecisionPolicy() (PrecisionPolicy, bool) {
	if !m.IsValid() {
		return PrecisionPolicy{}, false
	}
	return precisionPolicyFromInternal(m.version.PrecisionPolicy()), true
}

// Draft creates an editable Draft. For a committed Model, it makes an
// independent copy of that version's model content and records it as the
// parent of the next successful commit. For a new logical Model, it creates
// an empty Draft with no parent.
func (m Model) Draft() (*Draft, error) {
	if !m.IsLogical() {
		return nil, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}
	if m.version.IsCommitted() && !m.IsValid() {
		return nil, fmt.Errorf("%w: model is invalid", ErrInvalidModel)
	}

	draft := &Draft{modelID: m.modelID, name: m.name}
	if m.IsValid() {
		draft.value = internalmodel.NewDraftFromVersion(m.version)
		draft.parentVersionID = m.version.ID()
		draft.hasParent = true
		return draft, nil
	}
	draft.value = internalmodel.NewDraft()
	return draft, nil
}

// NewDraft is an alias for Draft. It is provided so a new logical Model reads
// naturally as model.NewDraft(); committed Models receive the same independent
// editable copy and parent behavior as Draft.
func (m Model) NewDraft() (*Draft, error) { return m.Draft() }

// Edit creates an editable Draft from a committed Model. Unlike Draft, it
// rejects a new logical Model that has not been committed yet.
func (m Model) Edit() (*Draft, error) {
	if !m.IsValid() {
		return nil, fmt.Errorf("%w: committed model is required", ErrInvalidModel)
	}
	return m.Draft()
}

// IsValid reports whether d is a usable Draft lifecycle wrapper. It does not
// report model-wide validity; use Validate for that.
func (d *Draft) IsValid() bool {
	return d != nil && !d.modelID.IsZero() && d.name != "" && d.value != nil &&
		(!d.hasParent || (!d.parentVersionID.IsZero()))
}

// ModelID returns the long-lived identity that d will preserve on commit.
func (d *Draft) ModelID() ID {
	if d == nil {
		return ID{}
	}
	return idFromInternal(d.modelID)
}

// Name returns the draft's mutable model presentation name.
func (d *Draft) Name() string {
	if d == nil {
		return ""
	}
	return d.name
}

// SetName changes the presentation name that the next successful commit will
// carry. It does not mutate any committed Model version.
func (d *Draft) SetName(name string) error {
	if d == nil || d.value == nil {
		return fmt.Errorf("%w: draft is invalid", ErrInvalidArgument)
	}
	if name == "" {
		return fmt.Errorf("%w: model name must not be empty", ErrInvalidArgument)
	}
	d.name = name
	return nil
}

// ParentVersionID returns the source version when d was created from a
// committed Model. A Draft from a new logical Model has no parent.
func (d *Draft) ParentVersionID() (ID, bool) {
	if d == nil || !d.hasParent {
		return ID{}, false
	}
	return idFromInternal(d.parentVersionID), true
}

// Validate runs the authoritative model validation machinery without
// changing d or committing it. Validation reports every discovered issue in
// deterministic order; a non-nil error indicates misuse of the Draft wrapper,
// not an invalid candidate model.
func (d *Draft) Validate() (ValidationReport, error) {
	if !d.IsValid() {
		return ValidationReport{}, fmt.Errorf("%w: draft is invalid", ErrInvalidArgument)
	}
	return validationReportFromInternal(d.value.Validate()), nil
}

// Commit validates d authoritatively and, only when it is valid, creates a
// new immutable Model version. A Draft from a committed Model automatically
// uses that source version as the new version's parent. Commit does not mutate
// d or any existing committed Model, including on failure.
func (d *Draft) Commit(options CommitOptions) (Model, error) {
	if !d.IsValid() {
		return Model{}, fmt.Errorf("%w: draft is invalid", ErrInvalidArgument)
	}
	if d.hasParent && options.versionID.value == d.parentVersionID {
		return Model{}, fmt.Errorf("%w: new version ID must differ from its parent", ErrInvalidArgument)
	}
	internalOptions, err := options.internal()
	if err != nil {
		return Model{}, err
	}
	if d.hasParent {
		internalOptions = internalOptions.WithParent(d.parentVersionID)
	}
	version, err := internalmodel.Commit(
		internalmodel.NewModel(d.modelID, d.name),
		d.value,
		internalOptions,
	)
	if err != nil {
		return Model{}, publicError(fmt.Errorf("%w: %w", ErrModelValidation, err))
	}
	return newModel(d.name, version), nil
}
