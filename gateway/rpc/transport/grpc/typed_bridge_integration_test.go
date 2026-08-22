package grpc

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"yunka.io/framework/core"
	"yunka.io/framework/core/identity"
	coremiddleware "yunka.io/framework/core/middleware"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/rpc/bridge"
	"yunka.io/gateway/rpc/meta"
)

type authenticatedGatewayService struct {
	core.BaseService
	seen chan identity.Principal
}

func (*authenticatedGatewayService) GetName() string { return bridge.DefaultGatewayServiceName }

func (service *authenticatedGatewayService) BatchAddRuntimeApi(context.Context, *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	principal, _ := request.PrincipalFromRuntime(service.GetRuntime())
	select {
	case service.seen <- principal:
	default:
	}
	return &meta.OperateRuntimeApiResponse{Msg: "authenticated"}, nil
}

func (*authenticatedGatewayService) DeleteRuntimeApi(context.Context, *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	return &meta.OperateRuntimeApiResponse{}, nil
}

func (*authenticatedGatewayService) OperateRoleAPI(context.Context, *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	return &meta.OperateRoleResponse{}, nil
}

type authenticatedGatewayPool struct {
	service core.Service
	puts    atomic.Int32
}

func (pool *authenticatedGatewayPool) GetService(string, request.Runtime) (core.Service, error) {
	return pool.service, nil
}

func (pool *authenticatedGatewayPool) PutService(core.Service) {
	pool.puts.Add(1)
}

func TestTypedGatewayBridgeOverAuthenticatedTLSBufconn(t *testing.T) {
	serverTLS, clientTLS := testTLSCredentials(t)
	verifier, err := NewStaticServiceTokenVerifier([]StaticServiceCredential{{
		Token: testServiceTokenA,
		Principal: identity.Principal{
			Subject: "typed-gateway", Roles: []string{"rpc-caller"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	service := &authenticatedGatewayService{seen: make(chan identity.Principal, 1)}
	pool := &authenticatedGatewayPool{service: service}
	provider, err := bridge.NewModuleGatewayProvider(pool, "", bridge.WorkRuntimeFactory{})
	if err != nil {
		t.Fatal(err)
	}

	listener := bufconn.Listen(1 << 20)
	server := grpcgo.NewServer(
		grpcgo.Creds(serverTLS),
		grpcgo.UnaryInterceptor(AuthenticatedUnaryServerInterceptor(coremiddleware.New(), verifier)),
	)
	transport, err := NewTypedGrpcServer(server)
	if err != nil {
		t.Fatal(err)
	}
	if err := transport.RegisterGatewayProvider(provider); err != nil {
		t.Fatal(err)
	}

	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
		select {
		case <-serveResult:
		default:
		}
	})

	dialer := func(context.Context, string) (net.Conn, error) { return listener.Dial() }
	credentials, err := NewStaticServiceTokenCredentials(testServiceTokenA)
	if err != nil {
		t.Fatal(err)
	}
	connection := dialTestConnection(t, dialer, clientTLS, credentials)
	client := meta.NewGatewayServiceClient(connection)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := client.BatchAddRuntimeApi(ctx, &meta.BatchRuntimeApiRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetMsg() != "authenticated" {
		t.Fatalf("response = %+v", response)
	}
	select {
	case principal := <-service.seen:
		if principal.Subject != "typed-gateway" || !principal.Authenticated || principal.AuthMethod != identity.AuthMethodServiceToken {
			t.Fatalf("principal = %+v", principal)
		}
	case <-ctx.Done():
		t.Fatal("service did not observe authenticated runtime principal")
	}
	if pool.puts.Load() != 1 {
		t.Fatalf("PutService calls = %d", pool.puts.Load())
	}
	if service.GetRuntime() != nil {
		t.Fatal("service retained request runtime after typed call")
	}
}
