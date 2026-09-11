package value

import (
	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/model"
	"github.com/shmatovv/bcg-engine/internal/runtime"
)

// ResolveCalculationOverride resolves the highest-precedence calculation
// override for target when one exists.
//
// Unlike Resolve, this function is valid for both Input and Derived
// properties.
//
// A calculation override replaces the concrete property value before Scenario
// and normal property computation are considered.
func ResolveCalculationOverride(
	version model.ModelVersion,
	target address.PropertyAddress,
	context runtime.CalculationContext,
) (Outcome, bool, error) {
	if !version.IsCommitted() {
		return Outcome{},
			false,
			newError(
				ErrorInvalidModelVersion,
				"calculation override resolution requires a committed ModelVersion",
			)
	}

	if !target.IsValid() {
		return Outcome{},
			false,
			newError(
				ErrorInvalidPropertyAddress,
				"target property address is invalid",
			)
	}

	if err :=
		ValidateContext(
			version,
			context,
		); err != nil {

		return Outcome{},
			false,
			err
	}

	owner, err :=
		resolveObjectOwner(
			version,
			target.Object(),
		)

	if err != nil {
		return Outcome{},
			false,
			err
	}

	definition, ok :=
		version.PropertyDefinition(
			target.DefinitionID(),
		)

	if !ok ||
		!definition.IsValid() {

		return Outcome{},
			false,
			newError(
				ErrorDefinitionNotFound,
				"target property definition could not be resolved",
			)
	}

	if definition.Owner() != owner {
		return Outcome{},
			false,
			newError(
				ErrorDefinitionNotAvailable,
				"target property definition is not available on the concrete object type",
			)
	}

	if definition.TemporalMode() !=
		schema.TemporalScalar {

		return Outcome{},
			false,
			newError(
				ErrorUnsupportedSeriesProperty,
				"Series property calculation overrides are not implemented yet",
			)
	}

	override, ok :=
		context.Override(
			target,
		)

	if !ok {
		return Outcome{},
			false,
			nil
	}

	if err :=
		validateOverride(
			version,
			definition,
			override,
		); err != nil {

		return Outcome{},
			false,
			err
	}

	return newSelectedOutcome(
		resolvedFromBindingValue(
			override.Value(),
		),
		calculationOverrideSource(
			override,
		),
	), true, nil
}
