package proxy

import (
	"context"
	"errors"
	"net/http"

	"yunka.io/framework/core/resilience"
)

// runtimeMiddlewareStatus adapts transport-neutral resilience errors to HTTP
// semantics. The resilience core itself intentionally has no HTTP dependency.
func runtimeMiddlewareStatus(err error) int {
	switch {
	case errors.Is(err, resilience.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, resilience.ErrCircuitOpen), errors.Is(err, resilience.ErrLoadShed):
		return http.StatusServiceUnavailable
	case errors.Is(err, resilience.ErrTimeoutBudgetExceeded), errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
