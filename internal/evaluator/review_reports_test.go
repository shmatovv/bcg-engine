package evaluator_test

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainscenario "github.com/shmatovv/bcg-engine/internal/domain/scenario"
	"github.com/shmatovv/bcg-engine/internal/domain/scope"
	domaintime "github.com/shmatovv/bcg-engine/internal/domain/time"
	"github.com/shmatovv/bcg-engine/internal/model"
	ruleeval "github.com/shmatovv/bcg-engine/internal/rule/eval"
	"github.com/shmatovv/bcg-engine/internal/runtime"
	"github.com/shmatovv/bcg-engine/internal/types/datetime"
)

func TestEmptyReportsValidateContext(t *testing.T) {
	v := reviewCommit(t, model.NewDraft())
	from, _ := datetime.Parse("2026-01-01T00:00:00Z")
	to, _ := datetime.Parse("2026-02-01T00:00:00Z")
	for _, context := range []runtime.CalculationContext{
		reviewContext().WithScenario(domainscenario.New(identity.New(), "wrong model", identity.New(), nil)),
		runtime.NewCalculationContext(domaintime.Period(from, to), scope.Values{}, nil),
	} {
		if _, err := check.All(v, context); err == nil {
			t.Fatal("empty constraint report accepted invalid/unsupported context")
		}
		if _, err := ruleeval.All(v, context); err == nil {
			t.Fatal("empty rule report accepted invalid/unsupported context")
		}
	}
}
