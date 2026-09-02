package grpc

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"github.com/hvritual/yunka.io/framework/core/identity"
	coremiddleware "github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
	"github.com/hvritual/yunka.io/framework/requestscope"
	"github.com/hvritual/yunka.io/gateway/rpc/bridge"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
	"github.com/hvritual/yunka.io/pkg/rpcbridge"
)

type authenticatedScopedGatewayService struct {
	scopes requestscope.ScopeFactory[grpcScopeRepositories]
	seen   chan authenticatedScopedObservation
}

type authenticatedScopedObservation struct {
	principal identity.Principal
	metadata  runtimecontext.Metadata
	traceID   string
}

func (service *authenticatedScopedGatewayService) BatchAddRuntimeApi(ctx context.Context, _ *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[grpcScopeRepositories]) (*meta.OperateRuntimeApiResponse, error) {
		principal, _ := scope.Principal()
		metadata, _ := scope.Metadata()
		select {
		case service.seen <- authenticatedScopedObservation{
			principal: principal,
			metadata:  metadata.Clone(),
			traceID:   scope.TraceID(),
		}:
		default:
		}
		return &meta.OperateRuntimeApiResponse{Msg: "authenticated"}, nil
	})
}

func (service *authenticatedScopedGatewayService) DeleteRuntimeApi(ctx context.Context, _ *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	return requestscope.ExecuteValue(ctx, service.scopes, func(*requestscope.Scope[grpcScopeRepositories]) (*meta.OperateRuntimeApiResponse, error) {
		return &meta.OperateRuntimeApiResponse{}, nil
	})
}

func (service *authenticatedScopedGatewayService) OperateRoleAPI(ctx context.Context, _ *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	return requestscope.ExecuteValue(ctx, service.scopes, func(*requestscope.Scope[grpcScopeRepositories]) (*meta.OperateRoleResponse, error) {
		return &meta.OperateRoleResponse{}, nil
	})
}

type grpcScopeUnitFactory struct {
	begins    atomic.Int32
	commits   atomic.Int32
	rollbacks atomic.Int32
	closes    atomic.Int32
}

type grpcScopeUnit struct{ owner *grpcScopeUnitFactory }

func (factory *grpcScopeUnitFactory) Begin(context.Context) (requestscope.UnitOfWork, error) {
	factory.begins.Add(1)
	return &grpcScopeUnit{owner: factory}, nil
}
func (unit *grpcScopeUnit) Commit(context.Context) error {
	unit.owner.commits.Add(1)
	return nil
}
func (unit *grpcScopeUnit) Rollback(context.Context) error {
	unit.owner.rollbacks.Add(1)
	return nil
}
func (unit *grpcScopeUnit) Close() error {
	unit.owner.closes.Add(1)
	return nil
}

type grpcScopeRepositories struct{}

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

	units := &grpcScopeUnitFactory{}
	scopes, err := requestscope.NewFactory(requestscope.FactoryOptions[grpcScopeRepositories]{
		UnitOfWork: units,
		Repositories: func(context.Context, requestscope.UnitOfWork) (grpcScopeRepositories, error) {
			return grpcScopeRepositories{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan authenticatedScopedObservation, 1)
	service := &authenticatedScopedGatewayService{scopes: scopes, seen: seen}
	provider := rpcbridge.Static[bridge.GatewayBusinessService](service)

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
	case observation := <-seen:
		principal := observation.principal
		if principal.Subject != "typed-gateway" || !principal.Authenticated || principal.AuthMethod != identity.AuthMethodServiceToken {
			t.Fatalf("principal = %+v", principal)
		}
		if observation.metadata.Transport != "rpc" || observation.metadata.Protocol != "grpc" || observation.metadata.Operation != meta.GatewayService_BatchAddRuntimeApi_FullMethodName {
			t.Fatalf("metadata = %+v", observation.metadata)
		}
	case <-ctx.Done():
		t.Fatal("service did not observe authenticated request scope")
	}
	if units.begins.Load() != 1 || units.commits.Load() != 1 || units.rollbacks.Load() != 0 || units.closes.Load() != 1 {
		t.Fatalf("scope lifecycle begins=%d commits=%d rollbacks=%d closes=%d", units.begins.Load(), units.commits.Load(), units.rollbacks.Load(), units.closes.Load())
	}
}
