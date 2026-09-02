package resilience

import (
	"context"
	"errors"

	"github.com/hvritual/yunka.io/pkg/selector"
)

// PickTarget delegates node selection to W5 without creating a second RPC
// transport. The caller resolves the returned address through its grpc-go
// connection factory and must report the final call result with Selection.Done.
func (policy *RPCPolicy) PickTarget(selected selector.Selector, service string, options ...selector.SelectOption) (*selector.Selection, error) {
	return selector.Pick(selected, service, options...)
}

// SelectorFeedback classifies resilience errors when target selection is
// composed around grpc-go. Local policy rejections are not node failures;
// transport and downstream errors remain passive-health failures.
func SelectorFeedback(err error) selector.Outcome {
	switch {
	case err == nil:
		return selector.OutcomeSuccess
	case errors.Is(err, context.Canceled),
		errors.Is(err, ErrTimeoutBudgetExceeded),
		errors.Is(err, ErrCircuitOpen),
		errors.Is(err, ErrRateLimited),
		errors.Is(err, ErrLoadShed):
		return selector.OutcomeIgnore
	default:
		return selector.OutcomeFailure
	}
}
