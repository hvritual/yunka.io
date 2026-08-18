package observability

import (
	"yunka.io/framework/core/middleware"
	"yunka.io/framework/core/resilience"
	"yunka.io/pkg/invoke"
)

// WrapRPCClient instruments an existing RPC client without modifying generated
// transport code.
func (provider *Provider) WrapRPCClient(client invoke.RpcClient) invoke.RpcClient {
	if provider == nil || client == nil {
		return client
	}
	return middleware.WrapRPCClient(client, provider.Middleware())
}

// WrapGovernedRPCClient creates one logical RPC observability span around a W3
// policy and records policy snapshots after the governed call. If individual
// retry-attempt spans are also desired, instrument the transport client before
// passing it to this method.
func (provider *Provider) WrapGovernedRPCClient(client invoke.RpcClient, policy *resilience.RPCPolicy) invoke.RpcClient {
	if client == nil {
		return nil
	}
	if policy == nil {
		return provider.WrapRPCClient(client)
	}
	governed := policy.Wrap(client)
	if provider == nil {
		return governed
	}
	return middleware.WrapRPCClient(governed, provider.Middleware(), provider.ResilienceMiddleware(policy))
}
