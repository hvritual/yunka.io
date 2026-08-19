package resilience

import (
	"context"
	"errors"
	"testing"
	"yunka.io/pkg/selector"
)

func TestSelectorFeedbackIgnoresLocalPolicyRejections(t *testing.T) {
	for _, err := range []error{context.Canceled, ErrTimeoutBudgetExceeded, ErrCircuitOpen, ErrRateLimited, ErrLoadShed} {
		if got := SelectorFeedback(err); got != selector.OutcomeIgnore {
			t.Fatalf("err=%v outcome=%v", err, got)
		}
	}
	if got := SelectorFeedback(errors.New("transport")); got != selector.OutcomeFailure {
		t.Fatalf("outcome=%v", got)
	}
	if got := SelectorFeedback(nil); got != selector.OutcomeSuccess {
		t.Fatalf("outcome=%v", got)
	}
}
