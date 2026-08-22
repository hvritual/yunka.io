package resilience

import (
	grpcgo "google.golang.org/grpc"
	"yunka.io/framework/core/middleware"
)

// RPCPolicyConfig composes one outbound RPC governance policy. Stateful
// policies are isolated by Key, which defaults to runtime operation metadata.
type RPCPolicyConfig struct {
	Key       KeyFunc
	Timeout   TimeoutBudgetConfig
	Retry     RetryConfig
	Circuit   CircuitBreakerConfig
	RateLimit RateLimitConfig
	LoadShed  LoadShedConfig
}

type RPCPolicy struct {
	config   RPCPolicyConfig
	breakers *CircuitBreakerGroup
	limiters *RateLimiterGroup
	shedders *LoadShedderGroup
}

type PolicySnapshot struct {
	Circuit CircuitSnapshot
	Rate    RateLimitSnapshot
	Load    LoadShedSnapshot
}

func NewRPCPolicy(config RPCPolicyConfig) *RPCPolicy {
	policy := &RPCPolicy{config: config}
	if config.Circuit.Enabled {
		policy.breakers = NewCircuitBreakerGroup(config.Circuit)
	}
	if config.RateLimit.Enabled {
		policy.limiters = NewRateLimiterGroup(config.RateLimit)
	}
	if config.LoadShed.Enabled {
		policy.shedders = NewLoadShedderGroup(config.LoadShed)
	}
	return policy
}

// Middlewares uses this order:
// timeout budget -> retry -> rate limit -> load shed -> circuit breaker -> transport.
// Because retry wraps the remaining policies, every retry attempt is still governed.
func (policy *RPCPolicy) Middlewares() []middleware.Middleware {
	if policy == nil {
		return nil
	}
	middlewares := make([]middleware.Middleware, 0, 5)
	if policy.config.Timeout.Timeout > 0 || policy.config.Timeout.Minimum > 0 || policy.config.Timeout.Reserve > 0 {
		middlewares = append(middlewares, TimeoutBudget(policy.config.Timeout))
	}
	if policy.config.Retry.MaxAttempts > 1 {
		middlewares = append(middlewares, Retry(policy.config.Retry))
	}
	if policy.limiters != nil {
		middlewares = append(middlewares, policy.limiters.Middleware(policy.config.Key))
	}
	if policy.shedders != nil {
		middlewares = append(middlewares, policy.shedders.Middleware(policy.config.Key))
	}
	if policy.breakers != nil {
		middlewares = append(middlewares, policy.breakers.Middleware(policy.config.Key))
	}
	return middlewares
}

// UnaryClientInterceptor exposes the policy through the standard grpc-go
// interceptor contract. Generated clients and connection ownership remain
// outside the policy.
func (policy *RPCPolicy) UnaryClientInterceptor() grpcgo.UnaryClientInterceptor {
	if policy == nil {
		return middleware.UnaryClientInterceptor()
	}
	return middleware.UnaryClientInterceptor(policy.Middlewares()...)
}

func (policy *RPCPolicy) Snapshot(key string) PolicySnapshot {
	var snapshot PolicySnapshot
	if policy == nil {
		return snapshot
	}
	if policy.breakers != nil {
		snapshot.Circuit = policy.breakers.Breaker(key).Snapshot()
	}
	if policy.limiters != nil {
		snapshot.Rate = policy.limiters.Bucket(key).Snapshot()
	}
	if policy.shedders != nil {
		snapshot.Load = policy.shedders.Shedder(key).Snapshot()
	}
	return snapshot
}
