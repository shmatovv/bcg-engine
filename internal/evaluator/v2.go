package evaluator

import (
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/provenance"
	"github.com/shmatovv/bcg-engine/v2/internal/types/decimal"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func (s *evaluationState) evaluateV2Unary(e ast.Unary, c expressionContext) (Result, error) {
	x, err := s.evaluateExpression(e.Operand(), c)
	if err != nil {
		return Result{}, err
	}
	var r enginevalue.Resolved
	switch e.Operator() {
	case ast.UnaryOperatorIsDefined:
		r = boolResolved(x.value.IsDefined())
	case ast.UnaryOperatorIsUndefined:
		r = boolResolved(x.value.IsUndefined())
	default:
		if x.value.IsUndefined() {
			r = dependencyUndefined()
		} else if x.value.IsNull() {
			r = enginevalue.Null()
		} else {
			r, err = s.v2UnaryDefined(e.Operator(), x.value.Value())
		}
	}
	if err != nil {
		return Result{}, err
	}
	return newResult(r, provenance.NewOperation(e.Operator().String(), x.provenance)), nil
}
func (s *evaluationState) v2UnaryDefined(op ast.UnaryOperator, v enginevalue.Typed) (enginevalue.Resolved, error) {
	raw := v.Raw()
	switch op {
	case ast.UnaryOperatorAbs:
		if v.Type() == enginevalue.Integer {
			n := raw.Integer()
			n.Abs(n)
			return enginevalue.Defined(v2Typed(enginevalue.NewInteger(n), v)), nil
		}
		if v.Type() == enginevalue.Decimal {
			return enginevalue.Defined(v2Typed(enginevalue.NewDecimal(raw.Decimal().Abs()), v)), nil
		}
	case ast.UnaryOperatorRound, ast.UnaryOperatorFloor, ast.UnaryOperatorCeil:
		if v.Type() != enginevalue.Integer && v.Type() != enginevalue.Decimal {
			return enginevalue.Resolved{}, newError(ErrorInvariantViolation, "numeric operand required", nil)
		}
		d := numericDecimal(v)
		var e error
		if op == ast.UnaryOperatorRound {
			d, e = d.Round(s.version.PrecisionPolicy())
		} else {
			d = v2IntegralDecimal(d, op == ast.UnaryOperatorCeil)
		}
		if e != nil {
			return enginevalue.Resolved{}, e
		}
		return enginevalue.Defined(v2Typed(enginevalue.NewDecimal(d), v)), nil
	case ast.UnaryOperatorLength:
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(utf8.RuneCountInString(raw.StringValue()))))), nil
	case ast.UnaryOperatorLower:
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewString(strings.ToLower(raw.StringValue())))), nil
	case ast.UnaryOperatorUpper:
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewString(strings.ToUpper(raw.StringValue())))), nil
	case ast.UnaryOperatorTrim:
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewString(strings.TrimSpace(raw.StringValue())))), nil
	case ast.UnaryOperatorYear:
		if v.Type() == enginevalue.DateTime {
			return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(raw.DateTime().Date().Year())))), nil
		}
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(raw.Date().Year())))), nil
	case ast.UnaryOperatorMonth:
		if v.Type() == enginevalue.DateTime {
			return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(raw.DateTime().Date().Month())))), nil
		}
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(raw.Date().Month())))), nil
	case ast.UnaryOperatorDay:
		if v.Type() == enginevalue.DateTime {
			return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(raw.DateTime().Date().Day())))), nil
		}
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(raw.Date().Day())))), nil
	case ast.UnaryOperatorHour:
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(raw.DateTime().Hour())))), nil
	case ast.UnaryOperatorMinute:
		return enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInt64(int64(raw.DateTime().Minute())))), nil
	}
	return enginevalue.Resolved{}, newError(ErrorInvariantViolation, "invalid unary operation", nil)
}

func v2Typed(raw enginevalue.Raw, source enginevalue.Typed) enginevalue.Typed {
	if source.HasUnit() {
		return enginevalue.NewTypedWithUnit(raw, source.Unit())
	}
	return enginevalue.NewTyped(raw)
}
func v2IntegralDecimal(value decimal.Decimal, ceil bool) decimal.Decimal {
	coefficient := value.Coefficient()
	scale := value.Scale()
	if scale == 0 {
		return value
	}
	power := new(big.Int).Exp(big.NewInt(10), new(big.Int).SetUint64(scale), nil)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(coefficient, power, remainder)
	if remainder.Sign() != 0 && ((ceil && coefficient.Sign() > 0) || (!ceil && coefficient.Sign() < 0)) {
		quotient.Add(quotient, big.NewInt(1))
	}
	return decimal.New(quotient, 0)
}
func (s *evaluationState) evaluateV2Binary(e ast.Binary, c expressionContext) (Result, error) {
	l, err := s.evaluateExpression(e.Left(), c)
	if err != nil {
		return Result{}, err
	}
	q, err := s.evaluateExpression(e.Right(), c)
	if err != nil {
		return Result{}, err
	}
	p := provenance.NewOperation(e.Operator().String(), l.provenance, q.provenance)
	if l.value.IsUndefined() || q.value.IsUndefined() {
		return newResult(dependencyUndefined(), p), nil
	}
	if l.value.IsNull() || q.value.IsNull() {
		return newResult(enginevalue.Null(), p), nil
	}
	a, b := l.value.Value(), q.value.Value()
	var r enginevalue.Resolved
	switch e.Operator() {
	case ast.BinaryOperatorModulo:
		divisor := b.Raw().Integer()
		if divisor.Sign() == 0 {
			return newResult(undefined(enginevalue.UndefinedDivisionByZero), p), nil
		}
		r = enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewInteger(new(big.Int).Rem(a.Raw().Integer(), divisor))))
	case ast.BinaryOperatorContains:
		r = boolResolved(strings.Contains(a.Raw().StringValue(), b.Raw().StringValue()))
	case ast.BinaryOperatorStartsWith:
		r = boolResolved(strings.HasPrefix(a.Raw().StringValue(), b.Raw().StringValue()))
	case ast.BinaryOperatorEndsWith:
		r = boolResolved(strings.HasSuffix(a.Raw().StringValue(), b.Raw().StringValue()))
	default:
		return Result{}, newError(ErrorUnsupportedExpression, "temporal operation is not supported by current temporal value implementation", nil)
	}
	return newResult(r, p), nil
}
func (s *evaluationState) evaluateV2Variadic(e ast.Variadic, c expressionContext) (Result, error) {
	xs := e.Operands()
	results := make([]Result, len(xs))
	children := make([]provenance.Node, len(xs))
	for i, x := range xs {
		r, err := s.evaluateExpression(x, c)
		if err != nil {
			return Result{}, err
		}
		results[i] = r
		children[i] = r.provenance
		if r.value.IsUndefined() {
			return newResult(dependencyUndefined(), provenance.NewOperation(e.Operator().String(), children[:i+1]...)), nil
		}
		if r.value.IsNull() {
			return newResult(enginevalue.Null(), provenance.NewOperation(e.Operator().String(), children[:i+1]...)), nil
		}
	}
	if e.Operator() != ast.VariadicOperatorConcat {
		return Result{}, newError(ErrorUnsupportedExpression, "scalar min/max evaluation is not implemented", nil)
	}
	var b strings.Builder
	for _, r := range results {
		b.WriteString(r.value.Value().Raw().StringValue())
	}
	return newResult(enginevalue.Defined(enginevalue.NewTyped(enginevalue.NewString(b.String()))), provenance.NewOperation("Concat", children...)), nil
}
