package inference

import (
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func inferV2Unary(e ast.Unary, c Context, r Resolver) (typing.Type, error) {
	x, err := Infer(e.Operand(), c, r)
	if err != nil {
		return typing.Type{}, err
	}
	switch e.Operator() {
	case ast.UnaryOperatorIsDefined, ast.UnaryOperatorIsUndefined:
		return typing.New(value.Bool), nil
	case ast.UnaryOperatorAbs, ast.UnaryOperatorRound, ast.UnaryOperatorFloor, ast.UnaryOperatorCeil:
		if x.ValueType() != value.Integer && x.ValueType() != value.Decimal {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return x, nil
	case ast.UnaryOperatorLength:
		if x.ValueType() != value.String {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return typing.New(value.Integer).WithNullable(x.Nullable()), nil
	case ast.UnaryOperatorLower, ast.UnaryOperatorUpper, ast.UnaryOperatorTrim:
		if x.ValueType() != value.String {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return x, nil
	case ast.UnaryOperatorYear, ast.UnaryOperatorMonth, ast.UnaryOperatorDay:
		if x.ValueType() != value.Date && x.ValueType() != value.DateTime {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return typing.New(value.Integer).WithNullable(x.Nullable()), nil
	case ast.UnaryOperatorHour, ast.UnaryOperatorMinute:
		if x.ValueType() != value.DateTime {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return typing.New(value.Integer).WithNullable(x.Nullable()), nil
	}
	return typing.Type{}, Error{kind: ErrorTypecheck}
}
func inferV2Binary(e ast.Binary, c Context, r Resolver) (typing.Type, error) {
	l, err := Infer(e.Left(), c, r)
	if err != nil {
		return typing.Type{}, err
	}
	q, err := Infer(e.Right(), c, r)
	if err != nil {
		return typing.Type{}, err
	}
	nullable := l.Nullable() || q.Nullable()
	switch e.Operator() {
	case ast.BinaryOperatorModulo:
		if l.ValueType() != value.Integer || q.ValueType() != value.Integer || l.HasUnit() || q.HasUnit() {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return typing.New(value.Integer).WithNullable(nullable), nil
	case ast.BinaryOperatorContains, ast.BinaryOperatorStartsWith, ast.BinaryOperatorEndsWith:
		if l.ValueType() != value.String || q.ValueType() != value.String {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return typing.New(value.Bool).WithNullable(nullable), nil
	case ast.BinaryOperatorAddDuration, ast.BinaryOperatorSubtractDuration:
		if l.ValueType() != value.DateTime || q.ValueType() != value.Duration {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return typing.New(value.DateTime).WithNullable(nullable), nil
	case ast.BinaryOperatorDifference:
		if l.ValueType() != value.DateTime || q.ValueType() != value.DateTime {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
		return typing.New(value.Duration).WithNullable(nullable), nil
	}
	return typing.Type{}, Error{kind: ErrorTypecheck}
}
func inferV2Variadic(e ast.Variadic, c Context, r Resolver) (typing.Type, error) {
	xs := e.Operands()
	first, err := Infer(xs[0], c, r)
	if err != nil {
		return typing.Type{}, err
	}
	for _, x := range xs[1:] {
		t, err := Infer(x, c, r)
		if err != nil {
			return typing.Type{}, err
		}
		if !sameV2Type(t, first) {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
	}
	if e.Operator() == ast.VariadicOperatorConcat {
		if first.ValueType() != value.String {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
	}
	if e.Operator() == ast.VariadicOperatorScalarMin || e.Operator() == ast.VariadicOperatorScalarMax {
		if first.ValueType() != value.Integer && first.ValueType() != value.Decimal {
			return typing.Type{}, Error{kind: ErrorTypecheck}
		}
	}
	return first, nil
}
func sameV2Type(left, right typing.Type) bool {
	if left.ValueType() != right.ValueType() || left.TemporalMode() != right.TemporalMode() || left.Nullable() != right.Nullable() || left.HasUnit() != right.HasUnit() {
		return false
	}
	if left.HasUnit() {
		a, _ := left.Unit()
		b, _ := right.Unit()
		if !a.Equal(b) {
			return false
		}
	}
	a, aok := left.EnumDefinitionID()
	b, bok := right.EnumDefinitionID()
	return aok == bok && (!aok || a == b)
}
