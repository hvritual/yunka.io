package handle

import (
	"yunka.io/gateway/rpc/client"
	"yunka.io/gateway/rpc/meta"
)

// GatewayServiceClient is a source-compatible alias over the single typed
// client facade.
type GatewayServiceClient = client.GatewayServiceClient

type Factory = client.Factory

type Option = client.Option

func NewGatewayServiceClient(source interface{}, options ...Option) *GatewayServiceClient {
	return client.NewGatewayServiceClient(source, options...)
}

func NewGatewayServiceClientWithFactory(factory Factory, options ...Option) *GatewayServiceClient {
	return client.NewGatewayServiceClientWithFactory(factory, options...)
}

func NewTypedGatewayServiceClient(typed meta.GatewayServiceClient, options ...Option) *GatewayServiceClient {
	return client.NewTypedGatewayServiceClient(typed, options...)
}
