package bridge_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"yunka.io/framework/core"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/request"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/gateway/rpc/bridge"
	rpcclient "yunka.io/gateway/rpc/client"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/rpcbridge"
)

type unchangedGatewayService struct {
	core.BaseService

	mu            sync.Mutex
	callErr       error
	lastPrincipal identity.Principal
	lastMetadata  runtimecontext.Metadata
}

func (*unchangedGatewayService) GetName() string { return bridge.DefaultGatewayServiceName }

func (service *unchangedGatewayService) BatchAddRuntimeApi(ctx context.Context, _ *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	service.capture(ctx)
	if service.callErr != nil {
		return nil, service.callErr
	}
	return &meta.OperateRuntimeApiResponse{Msg: "batch"}, nil
}

func (service *unchangedGatewayService) DeleteRuntimeApi(ctx context.Context, _ *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	service.capture(ctx)
	if service.callErr != nil {
		return nil, service.callErr
	}
	return &meta.OperateRuntimeApiResponse{Msg: "delete"}, nil
}

func (service *unchangedGatewayService) OperateRoleAPI(ctx context.Context, _ *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	service.capture(ctx)
	if service.callErr != nil {
		return nil, service.callErr
	}
	return &meta.OperateRoleResponse{Msg: "role"}, nil
}

func (service *unchangedGatewayService) capture(ctx context.Context) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if runtime := service.GetRuntime(); runtime != nil {
		service.lastPrincipal, _ = request.PrincipalFromRuntime(runtime)
		service.lastMetadata, _ = request.MetadataFromRuntime(runtime)
		return
	}
	service.lastPrincipal, _ = identity.FromContext(ctx)
	service.lastMetadata, _ = runtimecontext.MetadataFrom(ctx)
}

func (service *unchangedGatewayService) snapshot() (identity.Principal, runtimecontext.Metadata) {
	service.mu.Lock()
	defer service.mu.Unlock()
	metadata := service.lastMetadata
	if service.lastMetadata.Attributes != nil {
		metadata.Attributes = make(map[string]string, len(service.lastMetadata.Attributes))
		for key, value := range service.lastMetadata.Attributes {
			metadata.Attributes[key] = value
		}
	}
	return service.lastPrincipal.Clone(), metadata
}

type fakeServicePool struct {
	service core.Service
	gets    atomic.Int32
	puts    atomic.Int32
}

func (pool *fakeServicePool) GetService(_ string, _ request.Runtime) (core.Service, error) {
	pool.gets.Add(1)
	return pool.service, nil
}

func (pool *fakeServicePool) PutService(core.Service) {
	pool.puts.Add(1)
}

func TestModuleProviderPreservesUnchangedServiceABIAndRuntime(t *testing.T) {
	service := &unchangedGatewayService{}
	pool := &fakeServicePool{service: service}
	provider, err := bridge.NewModuleGatewayProvider(pool, "", bridge.WorkRuntimeFactory{})
	if err != nil {
		t.Fatal(err)
	}
	server, err := bridge.NewGatewayServiceBridge(provider)
	if err != nil {
		t.Fatal(err)
	}

	principal := identity.Principal{
		Subject:       "service-a",
		TenantID:      "tenant-a",
		AuthMethod:    identity.AuthMethodServiceToken,
		Authenticated: true,
	}
	ctx := identity.WithPrincipal(context.Background(), principal)
	ctx = runtimecontext.WithTraceID(ctx, "trace-a")

	response, err := server.BatchAddRuntimeApi(ctx, &meta.BatchRuntimeApiRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetMsg() != "batch" {
		t.Fatalf("response = %+v", response)
	}
	if pool.gets.Load() != 1 || pool.puts.Load() != 1 {
		t.Fatalf("pool gets/puts = %d/%d", pool.gets.Load(), pool.puts.Load())
	}
	if service.GetRuntime() != nil {
		t.Fatal("service runtime was retained after release")
	}
	lastPrincipal, lastMetadata := service.snapshot()
	if lastPrincipal.Subject != principal.Subject || !lastPrincipal.Authenticated {
		t.Fatalf("principal = %+v", lastPrincipal)
	}
	if lastMetadata.Operation != meta.GatewayService_BatchAddRuntimeApi_FullMethodName {
		t.Fatalf("operation = %q", lastMetadata.Operation)
	}
}

func TestModuleProviderReturnsCallAndFinishErrors(t *testing.T) {
	callErr := errors.New("business failed")
	finishErr := errors.New("finish failed")
	service := &unchangedGatewayService{callErr: callErr}
	pool := &fakeServicePool{service: service}

	factory := bridge.RuntimeFactoryFunc(func(ctx context.Context, serviceName string) (request.Runtime, error) {
		runtime := request.NewWorkRuntime()
		runtime.SetContext(ctx)
		runtime.SetSrvName(serviceName)
		runtime.BindFinishHook(func(error) error { return finishErr })
		return runtime, nil
	})
	provider, err := bridge.NewModuleGatewayProvider(pool, "", factory)
	if err != nil {
		t.Fatal(err)
	}
	server, err := bridge.NewGatewayServiceBridge(provider)
	if err != nil {
		t.Fatal(err)
	}

	_, err = server.DeleteRuntimeApi(context.Background(), &meta.DeleteRuntimeApiRequest{})
	if !errors.Is(err, callErr) || !errors.Is(err, finishErr) {
		t.Fatalf("error = %v", err)
	}
	if pool.puts.Load() != 1 {
		t.Fatalf("PutService calls = %d", pool.puts.Load())
	}
}

type nilServiceProvider struct {
	releases atomic.Int32
}

func (provider *nilServiceProvider) Acquire(context.Context) (bridge.GatewayBusinessService, rpcbridge.ReleaseFunc, error) {
	return nil, func(error) error {
		provider.releases.Add(1)
		return nil
	}, nil
}

func TestNilAcquiredServiceIsReleasedExactlyOnce(t *testing.T) {
	provider := &nilServiceProvider{}
	server, err := bridge.NewGatewayServiceBridge(provider)
	if err != nil {
		t.Fatal(err)
	}
	_, err = server.BatchAddRuntimeApi(context.Background(), &meta.BatchRuntimeApiRequest{})
	if !errors.Is(err, bridge.ErrServiceNotFound) {
		t.Fatalf("error = %v", err)
	}
	if provider.releases.Load() != 1 {
		t.Fatalf("release calls = %d", provider.releases.Load())
	}
}

func TestTypedBridgeAndCompatibilityClientOverBufconn(t *testing.T) {
	service := &unchangedGatewayService{}
	listener := bufconn.Listen(1024 * 1024)
	server := grpcgo.NewServer()
	if err := bridge.RegisterGatewayService(
		server,
		rpcbridge.Static[bridge.GatewayBusinessService](service),
	); err != nil {
		t.Fatal(err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
		<-serveDone
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpcgo.DialContext(
		ctx,
		"bufnet",
		grpcgo.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpcgo.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	var target string
	factory := rpcclient.NewConnectionFactory(
		connection,
		func(_ context.Context, current string) (grpcgo.ClientConnInterface, error) {
			target = current
			return connection, nil
		},
	)
	client := rpcclient.NewGatewayServiceClientWithFactory(factory, rpcclient.WithTimeout(time.Second))

	response, err := client.BatchAddRuntimeApi(ctx, &meta.BatchRuntimeApiRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetMsg() != "batch" {
		t.Fatalf("response = %+v", response)
	}
	role, err := client.OperateRoleAPISpecial(ctx, &meta.RoleModuleBtn{}, "127.0.0.1:19090")
	if err != nil {
		t.Fatal(err)
	}
	if role.GetMsg() != "role" || target != "127.0.0.1:19090" {
		t.Fatalf("role=%+v target=%q", role, target)
	}
	_, lastMetadata := service.snapshot()
	if lastMetadata.Operation != meta.GatewayService_OperateRoleAPI_FullMethodName {
		t.Fatalf("operation = %q", lastMetadata.Operation)
	}
}
