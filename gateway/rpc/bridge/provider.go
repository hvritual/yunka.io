package bridge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/rpcbridge"
)

const DefaultGatewayServiceName = "GatewayService"

var (
	ErrServicePoolUnavailable = errors.New("rpc bridge: service pool is unavailable")
	ErrServiceNotFound        = errors.New("rpc bridge: service was not found")
	ErrServiceTypeMismatch    = errors.New("rpc bridge: service does not implement GatewayService")
)

// GatewayBusinessService is the protected business-facing ABI. Existing
// services already implementing the generated server methods satisfy this
// interface without changing their structs or method bodies.
type GatewayBusinessService interface {
	BatchAddRuntimeApi(context.Context, *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error)
	DeleteRuntimeApi(context.Context, *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error)
	OperateRoleAPI(context.Context, *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error)
}

var _ meta.GatewayServiceServer = (GatewayBusinessService)(nil)

// ServicePool is the narrow compatibility surface needed from the current
// reflective module container. C7 replaces this adapter with constructors and
// request scopes while keeping GatewayBusinessService unchanged.
type ServicePool interface {
	GetService(string, request.Runtime) (core.Service, error)
	PutService(core.Service)
}

// ModuleGatewayProvider adapts the current module service pool to a typed
// per-call provider. It must not be used as a new general service locator.
type ModuleGatewayProvider struct {
	pool           ServicePool
	serviceName    string
	runtimeFactory RuntimeFactory
}

func NewModuleGatewayProvider(pool ServicePool, serviceName string, runtimeFactory RuntimeFactory) (*ModuleGatewayProvider, error) {
	if pool == nil {
		return nil, ErrServicePoolUnavailable
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		serviceName = DefaultGatewayServiceName
	}
	if runtimeFactory == nil {
		runtimeFactory = WorkRuntimeFactory{}
	}
	return &ModuleGatewayProvider{
		pool:           pool,
		serviceName:    serviceName,
		runtimeFactory: runtimeFactory,
	}, nil
}

func (provider *ModuleGatewayProvider) Acquire(ctx context.Context) (GatewayBusinessService, rpcbridge.ReleaseFunc, error) {
	if provider == nil || provider.pool == nil {
		return nil, nil, ErrServicePoolUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runtime, err := provider.runtimeFactory.New(ctx, provider.serviceName)
	if err != nil {
		return nil, nil, fmt.Errorf("rpc bridge: create runtime: %w", err)
	}
	if runtime == nil {
		return nil, nil, errors.New("rpc bridge: runtime factory returned nil")
	}

	service, err := provider.pool.GetService(provider.serviceName, runtime)
	if err != nil {
		return nil, nil, fmt.Errorf("rpc bridge: acquire %s: %w", provider.serviceName, err)
	}
	if service == nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrServiceNotFound, provider.serviceName)
	}
	service.SetRuntime(runtime)

	var (
		once       sync.Once
		releaseErr error
	)
	release := func(callErr error) error {
		once.Do(func() {
			if finisher, ok := runtime.(interface {
				FinishRequest(error) error
			}); ok {
				releaseErr = errors.Join(releaseErr, finisher.FinishRequest(callErr))
			}
			service.SetRuntime(nil)
			releaseErr = errors.Join(releaseErr, safePutService(provider.pool, service))
		})
		return releaseErr
	}

	typed, ok := service.(GatewayBusinessService)
	if !ok {
		typeErr := fmt.Errorf("%w: %T", ErrServiceTypeMismatch, service)
		return nil, nil, errors.Join(typeErr, release(typeErr))
	}
	return typed, rpcbridge.Once(release), nil
}

func safePutService(pool ServicePool, service core.Service) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("rpc bridge: PutService panicked: %v", recovered)
		}
	}()
	pool.PutService(service)
	return nil
}
