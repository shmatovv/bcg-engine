package typecheck

import "github.com/shmatovv/bcg-engine/v2/internal/types/unit"

// AdditiveUnit returns the result Unit for addition/subtraction.
//
// Rules:
//
//   - unitless + unitless -> unitless result (no Unit)
//   - unit + unit         -> allowed only when exact, context-free automatic
//     conversion is possible
//   - unit + unitless     -> invalid
//   - unitless + unit     -> invalid
//
// Equal Dimension alone is intentionally insufficient for addition and
// subtraction. The evaluator must be able to convert the right value into the
// left Unit without external context.
//
// When both operands have automatically convertible Units, the left operand's
// Unit becomes the result Unit.
//
// Examples:
//
//	kg + g       -> kg
//	g + kg       -> g
//	kg + m       -> invalid
//	RUB + kopeck -> RUB
//	RUB + EUR    -> invalid
func AdditiveUnit(
	left unit.Unit,
	leftHasUnit bool,
	right unit.Unit,
	rightHasUnit bool,
) (unit.Unit, bool, bool) {
	if !leftHasUnit && !rightHasUnit {
		return unit.Unit{}, false, true
	}

	if leftHasUnit != rightHasUnit {
		return unit.Unit{}, false, false
	}

	if !left.IsValid() || !right.IsValid() {
		return unit.Unit{}, false, false
	}

	if !left.CanConvertTo(right) {
		return unit.Unit{}, false, false
	}

	return left, true, true
}

// MultiplicativeUnit returns the result Unit for multiplication.
//
// Rules:
//
//   - unitless * unitless -> unitless
//   - unit * unitless     -> unit
//   - unitless * unit     -> unit
//   - unit * unit         -> composed Unit
//
// Multiplication does not require the operands to be mutually convertible.
// Their Dimensions and conversion signatures are composed algebraically.
//
// Examples:
//
//	2 * kg          -> kg
//	kg * 2          -> kg
//	m * m           -> m^2
//	RUB/item * item -> RUB
func MultiplicativeUnit(
	left unit.Unit,
	leftHasUnit bool,
	right unit.Unit,
	rightHasUnit bool,
) (result unit.Unit, hasUnit bool, ok bool) {
	defer rejectExponentOverflow(&result, &hasUnit, &ok)
	switch {
	case !leftHasUnit && !rightHasUnit:
		return unit.Unit{}, false, true

	case leftHasUnit && !rightHasUnit:
		if !left.IsValid() {
			return unit.Unit{}, false, false
		}

		return left, true, true

	case !leftHasUnit && rightHasUnit:
		if !right.IsValid() {
			return unit.Unit{}, false, false
		}

		return right, true, true

	default:
		if !left.IsValid() || !right.IsValid() {
			return unit.Unit{}, false, false
		}

		return left.Mul(right), true, true
	}
}

// DivisionUnit returns the result Unit for division.
//
// Rules:
//
//   - unitless / unitless -> unitless
//   - unit / unitless     -> unit
//   - unitless / unit     -> 1/unit
//   - unit / unit         -> composed quotient Unit
//
// Division does not require the operands to be mutually convertible.
// Their Dimensions and conversion signatures are composed algebraically.
//
// Examples:
//
//	RUB / 2  -> RUB
//	1 / kg   -> 1/kg
//	RUB / kg -> RUB/kg
//	kg / kg  -> dimensionless Unit
//
// A dimensionless Unit produced by unit algebra is intentionally retained as
// a Unit. Its exact scale and conversion signature may still carry meaningful
// conversion information.
func DivisionUnit(
	left unit.Unit,
	leftHasUnit bool,
	right unit.Unit,
	rightHasUnit bool,
) (result unit.Unit, hasUnit bool, ok bool) {
	defer rejectExponentOverflow(&result, &hasUnit, &ok)
	switch {
	case !leftHasUnit && !rightHasUnit:
		return unit.Unit{}, false, true

	case leftHasUnit && !rightHasUnit:
		if !left.IsValid() {
			return unit.Unit{}, false, false
		}

		return left, true, true

	case !leftHasUnit && rightHasUnit:
		if !right.IsValid() {
			return unit.Unit{}, false, false
		}

		result, err := unit.DimensionlessUnit().Div(right)
		if err != nil {
			return unit.Unit{}, false, false
		}

		return result, true, true

	default:
		if !left.IsValid() || !right.IsValid() {
			return unit.Unit{}, false, false
		}

		result, err := left.Div(right)
		if err != nil {
			return unit.Unit{}, false, false
		}

		return result, true, true
	}
}

// Only the documented algebra overflow is a typechecking failure. Preserve
// unexpected panics so programming defects are not hidden as invalid model data.
func rejectExponentOverflow(result *unit.Unit, hasUnit, ok *bool) {
	if recovered := recover(); recovered != nil {
		if recovered != unit.ErrExponentOverflow {
			panic(recovered)
		}
		*result, *hasUnit, *ok = unit.Unit{}, false, false
	}
}
