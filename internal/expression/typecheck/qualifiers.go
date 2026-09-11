package typecheck

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/expression/typing"
)

// requireSameTemporalMode verifies that operands have exactly the same
// temporal shape.
//
// Core v1 deliberately performs no implicit Scalar -> Series broadcast and
// no implicit Series resampling/aggregation.
func requireSameTemporalMode(
	left typing.Type,
	right typing.Type,
) (typing.TemporalMode, error) {
	if !left.IsValid() || !right.IsValid() {
		return typing.TemporalInvalid, fmt.Errorf(
			"typecheck: temporal compatibility requires valid operand types",
		)
	}

	if left.TemporalMode() != right.TemporalMode() {
		return typing.TemporalInvalid, fmt.Errorf(
			"typecheck: incompatible temporal modes: %s and %s",
			left.TemporalMode(),
			right.TemporalMode(),
		)
	}

	return left.TemporalMode(), nil
}

// propagateStrictBinaryQualifiers applies the qualifiers of a strict binary
// operation.
//
// A strict binary operation:
//
//   - requires identical temporal modes;
//   - produces Null whenever either operand evaluates to Null.
//
// Therefore result nullability is the logical OR of operand nullability.
func propagateStrictBinaryQualifiers(
	result typing.Type,
	left typing.Type,
	right typing.Type,
) (typing.Type, error) {
	if !result.IsValid() {
		return typing.Type{}, fmt.Errorf(
			"typecheck: cannot qualify invalid result type",
		)
	}

	mode, err := requireSameTemporalMode(left, right)
	if err != nil {
		return typing.Type{}, err
	}

	return result.
		WithTemporalMode(mode).
		WithNullable(left.Nullable() || right.Nullable()), nil
}

// propagateStrictUnaryQualifiers applies the qualifiers of a strict unary
// operation.
//
// Unary strict operations preserve both temporal shape and nullability.
func propagateStrictUnaryQualifiers(
	result typing.Type,
	operand typing.Type,
) (typing.Type, error) {
	if !result.IsValid() || !operand.IsValid() {
		return typing.Type{}, fmt.Errorf(
			"typecheck: unary qualifier propagation requires valid types",
		)
	}

	return result.
		WithTemporalMode(operand.TemporalMode()).
		WithNullable(operand.Nullable()), nil
}
