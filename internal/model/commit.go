package model

import (
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/types/decimal"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
)

type CommitOptions struct {
	versionID identity.UUID

	parentVersionID identity.UUID
	hasParent       bool

	versionLabel string

	precisionPolicy precision.Policy
}

// NewCommitOptions panics for a zero version ID, invalid rounding mode, or
// a scale exceeding decimal.MaxScale. Committed arithmetic must be executable.
func NewCommitOptions(
	versionID identity.UUID,
	versionLabel string,
	precisionPolicy precision.Policy,
) CommitOptions {
	if versionID.IsZero() {
		panic("model: model version id must not be zero")
	}

	if !precisionPolicy.IsValid() || precisionPolicy.Scale() > decimal.MaxScale {
		panic("model: precision policy must be valid and scale must not exceed decimal.MaxScale")
	}

	return CommitOptions{
		versionID:       versionID,
		versionLabel:    versionLabel,
		precisionPolicy: precisionPolicy,
	}
}

func (o CommitOptions) WithParent(
	parentVersionID identity.UUID,
) CommitOptions {
	o.requireValid()

	if parentVersionID.IsZero() {
		panic("model: parent model version id must not be zero")
	}

	if parentVersionID == o.versionID {
		panic("model: model version cannot be its own parent")
	}

	o.parentVersionID = parentVersionID
	o.hasParent = true

	return o
}

func (o CommitOptions) VersionID() identity.UUID {
	return o.versionID
}

func (o CommitOptions) ParentVersionID() (
	identity.UUID,
	bool,
) {
	return o.parentVersionID, o.hasParent
}

func (o CommitOptions) VersionLabel() string {
	return o.versionLabel
}

func (o CommitOptions) PrecisionPolicy() precision.Policy {
	return o.precisionPolicy
}

func (o CommitOptions) IsValid() bool {
	if o.versionID.IsZero() ||
		!o.precisionPolicy.IsValid() || o.precisionPolicy.Scale() > decimal.MaxScale {
		return false
	}

	if o.hasParent {
		return !o.parentVersionID.IsZero() &&
			o.parentVersionID != o.versionID
	}

	return o.parentVersionID.IsZero()
}

func (o CommitOptions) requireValid() {
	if !o.IsValid() {
		panic("model: invalid commit options")
	}
}

func Commit(
	model Model,
	draft *Draft,
	options CommitOptions,
) (ModelVersion, error) {
	if !model.IsValid() {
		panic("model: invalid model")
	}

	if draft == nil {
		panic("model: nil draft")
	}

	options.requireValid()

	report := draft.Validate()

	if !report.OK() {
		return ModelVersion{}, report
	}

	return newCommittedModelVersion(
		options.versionID,
		model.ID(),
		options.parentVersionID,
		options.hasParent,
		options.versionLabel,
		options.precisionPolicy,
		draft.entityTypes,
		draft.relationTypes,
		draft.traits,
		draft.propertyContracts,
		draft.propertyDefinitions,
		draft.scopeDefinitions,
		draft.entities,
		draft.relations,
		draft.valueBindings,
		draft.constraints,
		draft.rules,
	), nil
}
