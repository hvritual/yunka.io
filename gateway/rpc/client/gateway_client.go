package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	grpcgo "google.golang.org/grpc"
	"yunka.io/gateway/rpc/meta"
)

const DefaultTimeout = 500 * time.Millisecond

var ErrClientSourceUnsupported = errors.New("rpc client: unsupported GatewayService client source")

// Option configures the source-compatible client facade.
type Option func(*GatewayServiceClient)

// WithTimeout preserves the historical per-call deadline when positive. A
// non-positive duration delegates deadline ownership entirely to the caller.
func WithTimeout(timeout time.Duration) Option {
	return func(client *GatewayServiceClient) {
		client.timeout = timeout
	}
}

// GatewayServiceClient preserves the historical business-facing method set
// while every operation delegates to the standard generated typed client.
type GatewayServiceClient struct {
	factory   Factory
	timeout   time.Duration
	sourceErr error
}

// NewGatewayServiceClient preserves the historical constructor name without
// retaining the old invoke transport. Supported sources are Factory,
// meta.GatewayServiceClient, and grpc.ClientConnInterface.
func NewGatewayServiceClient(source interface{}, options ...Option) *GatewayServiceClient {
	switch value := source.(type) {
	case Factory:
		return NewGatewayServiceClientWithFactory(value, options...)
	case meta.GatewayServiceClient:
		return NewTypedGatewayServiceClient(value, options...)
	case grpcgo.ClientConnInterface:
		return NewGatewayServiceClientWithFactory(NewConnectionFactory(value, nil), options...)
	case *GatewayServiceClient:
		if value == nil {
			return unavailableClient(fmt.Errorf("%w: <nil>", ErrClientSourceUnsupported), options...)
		}
		return value
	case nil:
		return unavailableClient(ErrClientUnavailable, options...)
	default:
		return unavailableClient(fmt.Errorf("%w: %T", ErrClientSourceUnsupported, source), options...)
	}
}

func unavailableClient(err error, options ...Option) *GatewayServiceClient {
	client := &GatewayServiceClient{timeout: DefaultTimeout, sourceErr: err}
	for _, option := range options {
		if option != nil {
			option(client)
		}
	}
	return client
}

func NewGatewayServiceClientWithFactory(factory Factory, options ...Option) *GatewayServiceClient {
	client := &GatewayServiceClient{factory: factory, timeout: DefaultTimeout}
	for _, option := range options {
		if option != nil {
			option(client)
		}
	}
	return client
}

func NewTypedGatewayServiceClient(typed meta.GatewayServiceClient, options ...Option) *GatewayServiceClient {
	return NewGatewayServiceClientWithFactory(NewTypedFactory(typed, nil), options...)
}

func (client *GatewayServiceClient) BatchAddRuntimeApi(parent context.Context, request *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	ctx, cancel := client.callContext(parent)
	defer cancel()
	typed, err := client.defaultClient(ctx)
	if err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}
	response, err := typed.BatchAddRuntimeApi(ctx, request)
	if response == nil {
		response = &meta.OperateRuntimeApiResponse{}
	}
	return response, err
}

func (client *GatewayServiceClient) BatchAddRuntimeApiSpecial(parent context.Context, request *meta.BatchRuntimeApiRequest, nodeIP string) (*meta.OperateRuntimeApiResponse, error) {
	ctx, cancel := client.callContext(parent)
	defer cancel()
	typed, err := client.targetClient(ctx, nodeIP)
	if err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}
	response, err := typed.BatchAddRuntimeApi(ctx, request)
	if response == nil {
		response = &meta.OperateRuntimeApiResponse{}
	}
	return response, err
}

func (client *GatewayServiceClient) DeleteRuntimeApi(parent context.Context, request *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	ctx, cancel := client.callContext(parent)
	defer cancel()
	typed, err := client.defaultClient(ctx)
	if err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}
	response, err := typed.DeleteRuntimeApi(ctx, request)
	if response == nil {
		response = &meta.OperateRuntimeApiResponse{}
	}
	return response, err
}

func (client *GatewayServiceClient) DeleteRuntimeApiSpecial(parent context.Context, request *meta.DeleteRuntimeApiRequest, nodeIP string) (*meta.OperateRuntimeApiResponse, error) {
	ctx, cancel := client.callContext(parent)
	defer cancel()
	typed, err := client.targetClient(ctx, nodeIP)
	if err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}
	response, err := typed.DeleteRuntimeApi(ctx, request)
	if response == nil {
		response = &meta.OperateRuntimeApiResponse{}
	}
	return response, err
}

func (client *GatewayServiceClient) OperateRoleAPI(parent context.Context, request *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	ctx, cancel := client.callContext(parent)
	defer cancel()
	typed, err := client.defaultClient(ctx)
	if err != nil {
		return &meta.OperateRoleResponse{}, err
	}
	response, err := typed.OperateRoleAPI(ctx, request)
	if response == nil {
		response = &meta.OperateRoleResponse{}
	}
	return response, err
}

func (client *GatewayServiceClient) OperateRoleAPISpecial(parent context.Context, request *meta.RoleModuleBtn, nodeIP string) (*meta.OperateRoleResponse, error) {
	ctx, cancel := client.callContext(parent)
	defer cancel()
	typed, err := client.targetClient(ctx, nodeIP)
	if err != nil {
		return &meta.OperateRoleResponse{}, err
	}
	response, err := typed.OperateRoleAPI(ctx, request)
	if response == nil {
		response = &meta.OperateRoleResponse{}
	}
	return response, err
}

func (client *GatewayServiceClient) defaultClient(ctx context.Context) (meta.GatewayServiceClient, error) {
	if client == nil {
		return nil, ErrClientUnavailable
	}
	if client.sourceErr != nil {
		return nil, client.sourceErr
	}
	if client.factory == nil {
		return nil, ErrClientUnavailable
	}
	return client.factory.Client(ctx)
}

func (client *GatewayServiceClient) targetClient(ctx context.Context, target string) (meta.GatewayServiceClient, error) {
	if client == nil {
		return nil, ErrTargetClientUnavailable
	}
	if client.sourceErr != nil {
		return nil, client.sourceErr
	}
	if client.factory == nil {
		return nil, ErrTargetClientUnavailable
	}
	return client.factory.ClientForTarget(ctx, target)
}

func (client *GatewayServiceClient) callContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if client == nil || client.timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, client.timeout)
}

type compatibilityABI interface {
	BatchAddRuntimeApi(context.Context, *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error)
	BatchAddRuntimeApiSpecial(context.Context, *meta.BatchRuntimeApiRequest, string) (*meta.OperateRuntimeApiResponse, error)
	DeleteRuntimeApi(context.Context, *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error)
	DeleteRuntimeApiSpecial(context.Context, *meta.DeleteRuntimeApiRequest, string) (*meta.OperateRuntimeApiResponse, error)
	OperateRoleAPI(context.Context, *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error)
	OperateRoleAPISpecial(context.Context, *meta.RoleModuleBtn, string) (*meta.OperateRoleResponse, error)
}

var _ compatibilityABI = (*GatewayServiceClient)(nil)
