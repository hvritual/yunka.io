package bridge

import (
	"context"
	"errors"
	"fmt"

	grpcgo "google.golang.org/grpc"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/rpcbridge"
)

var ErrGatewayProviderUnavailable = errors.New("rpc bridge: GatewayService provider is unavailable")

// GatewayServiceBridge is the standard generated gRPC server implementation
// that delegates each call to an explicitly owned provider.
type GatewayServiceBridge struct {
	provider rpcbridge.Provider[GatewayBusinessService]
}

var _ meta.GatewayServiceServer = (*GatewayServiceBridge)(nil)

func NewGatewayServiceBridge(provider rpcbridge.Provider[GatewayBusinessService]) (*GatewayServiceBridge, error) {
	if provider == nil {
		return nil, ErrGatewayProviderUnavailable
	}
	return &GatewayServiceBridge{provider: provider}, nil
}

func RegisterGatewayService(registrar grpcgo.ServiceRegistrar, provider rpcbridge.Provider[GatewayBusinessService]) error {
	if registrar == nil {
		return errors.New("rpc bridge: gRPC registrar is nil")
	}
	server, err := NewGatewayServiceBridge(provider)
	if err != nil {
		return err
	}
	meta.RegisterGatewayServiceServer(registrar, server)
	return nil
}

func (server *GatewayServiceBridge) BatchAddRuntimeApi(ctx context.Context, request *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	return invokeGateway(
		ctx,
		meta.GatewayService_BatchAddRuntimeApi_FullMethodName,
		server.provider,
		func(callContext context.Context, service GatewayBusinessService) (*meta.OperateRuntimeApiResponse, error) {
			return service.BatchAddRuntimeApi(callContext, request)
		},
	)
}

func (server *GatewayServiceBridge) DeleteRuntimeApi(ctx context.Context, request *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	return invokeGateway(
		ctx,
		meta.GatewayService_DeleteRuntimeApi_FullMethodName,
		server.provider,
		func(callContext context.Context, service GatewayBusinessService) (*meta.OperateRuntimeApiResponse, error) {
			return service.DeleteRuntimeApi(callContext, request)
		},
	)
}

func (server *GatewayServiceBridge) OperateRoleAPI(ctx context.Context, request *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	return invokeGateway(
		ctx,
		meta.GatewayService_OperateRoleAPI_FullMethodName,
		server.provider,
		func(callContext context.Context, service GatewayBusinessService) (*meta.OperateRoleResponse, error) {
			return service.OperateRoleAPI(callContext, request)
		},
	)
}

func invokeGateway[T any](
	ctx context.Context,
	fullMethod string,
	provider rpcbridge.Provider[GatewayBusinessService],
	call func(context.Context, GatewayBusinessService) (T, error),
) (result T, err error) {
	if provider == nil {
		return result, ErrGatewayProviderUnavailable
	}
	ctx = withRPCOperation(ctx, fullMethod)
	service, release, err := provider.Acquire(ctx)
	if err != nil {
		return result, err
	}
	release = rpcbridge.Once(release)
	if service == nil {
		acquireErr := ErrServiceNotFound
		if releaseErr := rpcbridge.SafeRelease(release, acquireErr); releaseErr != nil {
			return result, releaseErr
		}
		return result, acquireErr
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = rpcbridge.SafeRelease(release, fmt.Errorf("rpc call panicked: %v", recovered))
			panic(recovered)
		}
		if finalErr := rpcbridge.SafeRelease(release, err); finalErr != nil {
			err = finalErr
		}
	}()
	return call(ctx, service)
}

func withRPCOperation(ctx context.Context, fullMethod string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	metadata.Transport = "rpc"
	metadata.Protocol = "grpc"
	metadata.Operation = fullMethod
	metadata.Method = fullMethod
	if metadata.Attributes == nil {
		metadata.Attributes = make(map[string]string)
	}
	metadata.Attributes["rpc.direction"] = "server"
	return runtimecontext.WithMetadata(ctx, metadata)
}
