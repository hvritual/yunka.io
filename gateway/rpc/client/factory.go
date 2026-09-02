package client

import (
	"context"
	"errors"
	"strings"

	grpcgo "google.golang.org/grpc"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

var (
	ErrClientUnavailable       = errors.New("rpc client: GatewayService client is unavailable")
	ErrTargetClientUnavailable = errors.New("rpc client: target GatewayService client is unavailable")
	ErrTargetRequired          = errors.New("rpc client: target is required")
)

// Factory owns typed GatewayService clients and any underlying connections.
// Returned connections/clients are borrowed; callers do not close them.
type Factory interface {
	Client(context.Context) (meta.GatewayServiceClient, error)
	ClientForTarget(context.Context, string) (meta.GatewayServiceClient, error)
}

type FactoryFunc struct {
	Default func(context.Context) (meta.GatewayServiceClient, error)
	Target  func(context.Context, string) (meta.GatewayServiceClient, error)
}

func (factory FactoryFunc) Client(ctx context.Context) (meta.GatewayServiceClient, error) {
	if factory.Default == nil {
		return nil, ErrClientUnavailable
	}
	return factory.Default(ctx)
}

func (factory FactoryFunc) ClientForTarget(ctx context.Context, target string) (meta.GatewayServiceClient, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, ErrTargetRequired
	}
	if factory.Target == nil {
		return nil, ErrTargetClientUnavailable
	}
	return factory.Target(ctx, target)
}

// TargetConnectionResolver resolves one explicitly requested target. The
// resolver owns connection caching and shutdown.
type TargetConnectionResolver func(context.Context, string) (grpcgo.ClientConnInterface, error)

// NewConnectionFactory adapts standard grpc-go connections into typed clients.
func NewConnectionFactory(defaultConnection grpcgo.ClientConnInterface, targetResolver TargetConnectionResolver) Factory {
	return FactoryFunc{
		Default: func(context.Context) (meta.GatewayServiceClient, error) {
			if defaultConnection == nil {
				return nil, ErrClientUnavailable
			}
			return meta.NewGatewayServiceClient(defaultConnection), nil
		},
		Target: func(ctx context.Context, target string) (meta.GatewayServiceClient, error) {
			if targetResolver == nil {
				return nil, ErrTargetClientUnavailable
			}
			connection, err := targetResolver(ctx, target)
			if err != nil {
				return nil, err
			}
			if connection == nil {
				return nil, ErrTargetClientUnavailable
			}
			return meta.NewGatewayServiceClient(connection), nil
		},
	}
}

// NewTypedFactory uses one application-owned typed client and an optional
// target-aware resolver.
func NewTypedFactory(
	defaultClient meta.GatewayServiceClient,
	targetResolver func(context.Context, string) (meta.GatewayServiceClient, error),
) Factory {
	return FactoryFunc{
		Default: func(context.Context) (meta.GatewayServiceClient, error) {
			if defaultClient == nil {
				return nil, ErrClientUnavailable
			}
			return defaultClient, nil
		},
		Target: targetResolver,
	}
}
