package observability

import (
	grpcgo "google.golang.org/grpc"
	"github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/resilience"
)

// UnaryClientInterceptor instruments the single grpc-go unary client runtime.
func (provider *Provider) UnaryClientInterceptor() grpcgo.UnaryClientInterceptor {
	if provider == nil {
		return middleware.UnaryClientInterceptor()
	}
	return middleware.UnaryClientInterceptor(provider.Middleware())
}

// GovernedUnaryClientInterceptor creates one logical RPC observability span
// around W3 policy execution and records policy snapshots after the call.
func (provider *Provider) GovernedUnaryClientInterceptor(policy *resilience.RPCPolicy) grpcgo.UnaryClientInterceptor {
	middlewares := make([]middleware.Middleware, 0, 7)
	if provider != nil {
		middlewares = append(middlewares, provider.Middleware())
		if policy != nil {
			middlewares = append(middlewares, provider.ResilienceMiddleware(policy))
		}
	}
	if policy != nil {
		middlewares = append(middlewares, policy.Middlewares()...)
	}
	return middleware.UnaryClientInterceptor(middlewares...)
}
