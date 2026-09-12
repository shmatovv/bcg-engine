package provenance

import "testing"

func TestOperationDecisionValidation(t *testing.T) {
	if !NewOperationDecision("And", DecisionShortCircuited).IsValid() {
		t.Fatal("valid logical short-circuit decision rejected")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("invalid operation decision was accepted")
		}
	}()
	_ = NewOperationDecision("Add", DecisionThen)
}
