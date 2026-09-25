package ast

import (
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func TestDepthIsIterativeForDeepExpression(t *testing.T) {
	var expression Expression = NewConstant(value.NewTyped(value.NewInt64(1)))
	for index := 0; index < 5_000; index++ {
		expression = NewNegate(expression)
	}

	depth, valid := Depth(expression)
	if !valid || depth != 5_001 {
		t.Fatalf("deep expression depth = %d, valid=%t", depth, valid)
	}
}
