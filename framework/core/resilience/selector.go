package resilience

import (
	"context"
	"errors"

	"yunka.io/pkg/invoke"
	"yunka.io/pkg/selector"
)

// WrapSelected composes W3 resilience outside W5 adaptive selection. This
// ordering is intentional: every Retry attempt performs a fresh Pick, while
// rate-limit/load-shed/circuit rejections happen before a node is selected and
// therefore cannot poison passive node health.
func (policy *RPCPolicy) WrapSelected(next invoke.RpcClient, selected selector.Selector, opts ...selector.RPCOption) invoke.RpcClient {
	return policy.Wrap(selector.WrapRPCClient(next, selected, opts...))
}

// SelectorFeedback classifies resilience errors when a selector wrapper is
// composed manually. Local policy rejections are not node failures; transport
// and downstream errors remain passive-health failures.
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
