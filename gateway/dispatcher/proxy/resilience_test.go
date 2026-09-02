package proxy

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/resilience"
)

func TestRuntimeMiddlewareStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "rate limited", err: fmt.Errorf("wrapped: %w", resilience.ErrRateLimited), want: http.StatusTooManyRequests},
		{name: "circuit open", err: resilience.ErrCircuitOpen, want: http.StatusServiceUnavailable},
		{name: "load shed", err: resilience.ErrLoadShed, want: http.StatusServiceUnavailable},
		{name: "budget", err: resilience.ErrTimeoutBudgetExceeded, want: http.StatusGatewayTimeout},
		{name: "deadline", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
		{name: "unknown", err: fmt.Errorf("boom"), want: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := runtimeMiddlewareStatus(test.err); got != test.want {
				t.Fatalf("status=%d want=%d", got, test.want)
			}
		})
	}
}
