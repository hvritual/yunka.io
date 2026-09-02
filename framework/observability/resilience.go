package observability

import (
	"context"

	"go.opentelemetry.io/otel/metric"
	"github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/resilience"
)

// ResilienceMiddleware records the current W3 state after each logical call.
// It does not own or mutate policy state and therefore remains safe to compose
// around an existing RPCPolicy.
func (provider *Provider) ResilienceMiddleware(policy *resilience.RPCPolicy) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context) error {
			err := next(ctx)
			if provider != nil && policy != nil {
				key := resilience.OperationKey(ctx)
				provider.RecordPolicySnapshot(ctx, policy.Snapshot(key))
			}
			return err
		}
	}
}

// RecordPolicySnapshot exports W3 point-in-time state as synchronous gauges.
// The operation dimension comes from runtime metadata; raw policy keys are not
// used as labels so custom keys cannot create unbounded metric cardinality.
func (provider *Provider) RecordPolicySnapshot(ctx context.Context, snapshot resilience.PolicySnapshot) {
	if provider == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	options := metric.WithAttributes(metricAttributes(ctx)...)
	if snapshot.Circuit.State != "" {
		provider.circuitState.Record(ctx, circuitStateValue(snapshot.Circuit.State), options)
		provider.circuitFailures.Record(ctx, int64(snapshot.Circuit.Failures), options)
	}
	if snapshot.Rate.Burst > 0 || snapshot.Rate.Rate > 0 {
		provider.rateTokens.Record(ctx, snapshot.Rate.Tokens, options)
	}
	if snapshot.Load.Limit > 0 {
		provider.loadLimit.Record(ctx, int64(snapshot.Load.Limit), options)
		provider.loadInFlight.Record(ctx, int64(snapshot.Load.InFlight), options)
	}
}

func circuitStateValue(state resilience.CircuitState) int64 {
	switch state {
	case resilience.CircuitHalfOpen:
		return 1
	case resilience.CircuitOpen:
		return 2
	default:
		return 0
	}
}
