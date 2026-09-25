package bcg_test

import (
	bcg "github.com/shmatovv/bcg-engine/v2"
	"testing"
)

func TestExpressionV2PublicInspection(t *testing.T) {
	hello := mustData[bcg.Expression](t)(bcg.Constant(bcg.String(" Привет ")))
	trimmed := mustData[bcg.Expression](t)(bcg.Trim(hello))
	upper := mustData[bcg.Expression](t)(bcg.Upper(trimmed))
	length := mustData[bcg.Expression](t)(bcg.Length(upper))
	if length.Kind() != bcg.ExpressionUnaryV2 || len(length.Children()) != 1 {
		t.Fatalf("unary inspection = %s %#v", length.Kind(), length.Children())
	}
	operation, ok := length.UnaryV2Operation()
	if !ok || operation.Operation() != bcg.UnaryLength || operation.Operand().Kind() != bcg.ExpressionUnaryV2 {
		t.Fatalf("unary accessor = %#v %v", operation, ok)
	}
	suffix := mustData[bcg.Expression](t)(bcg.Constant(bcg.String("вет")))
	contains := mustData[bcg.Expression](t)(bcg.Contains(trimmed, suffix))
	if contains.Kind() != bcg.ExpressionBinaryV2 || len(contains.Children()) != 2 {
		t.Fatal("binary inspection lost children")
	}
	concat := mustData[bcg.Expression](t)(bcg.Concat(trimmed, suffix))
	if concat.Kind() != bcg.ExpressionVariadicV2 || len(concat.Children()) != 2 {
		t.Fatal("variadic inspection lost children")
	}
	count := 0
	if err := bcg.WalkExpression(concat, bcg.ExpressionVisitorFunc(func(bcg.Expression) error { count++; return nil })); err != nil || count != 4 {
		t.Fatalf("walk = %d, %v", count, err)
	}
}
