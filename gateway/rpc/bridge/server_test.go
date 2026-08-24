package bridge

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/rpcbridge"
)

type staticGatewayService struct {
	batch  atomic.Int32
	delete atomic.Int32
	role   atomic.Int32
	last   atomic.Value
}

func (service *staticGatewayService) record(ctx context.Context) {
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	service.last.Store(metadata)
}
func (service *staticGatewayService) BatchAddRuntimeApi(ctx context.Context, _ *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	service.record(ctx)
	service.batch.Add(1)
	return &meta.OperateRuntimeApiResponse{Msg: "batch"}, nil
}
func (service *staticGatewayService) DeleteRuntimeApi(ctx context.Context, _ *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	service.record(ctx)
	service.delete.Add(1)
	return &meta.OperateRuntimeApiResponse{Msg: "delete"}, nil
}
func (service *staticGatewayService) OperateRoleAPI(ctx context.Context, _ *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	service.record(ctx)
	service.role.Add(1)
	return &meta.OperateRoleResponse{Msg: "role"}, nil
}

func TestGatewayServiceBridgeDelegatesToExplicitService(t *testing.T) {
	service := &staticGatewayService{}
	server, err := NewGatewayServiceBridge(rpcbridge.Static[GatewayBusinessService](service))
	if err != nil {
		t.Fatal(err)
	}
	batch, err := server.BatchAddRuntimeApi(context.Background(), &meta.BatchRuntimeApiRequest{})
	if err != nil || batch.GetMsg() != "batch" {
		t.Fatalf("batch=%v err=%v", batch, err)
	}
	deleted, err := server.DeleteRuntimeApi(context.Background(), &meta.DeleteRuntimeApiRequest{})
	if err != nil || deleted.GetMsg() != "delete" {
		t.Fatalf("delete=%v err=%v", deleted, err)
	}
	if _, err := server.OperateRoleAPI(context.Background(), &meta.RoleModuleBtn{}); err != nil {
		t.Fatal(err)
	}
	if service.batch.Load() != 1 || service.delete.Load() != 1 || service.role.Load() != 1 {
		t.Fatalf("calls=%d/%d/%d", service.batch.Load(), service.delete.Load(), service.role.Load())
	}
	metadata := service.last.Load().(runtimecontext.Metadata)
	if metadata.Operation != meta.GatewayService_OperateRoleAPI_FullMethodName {
		t.Fatalf("operation=%q", metadata.Operation)
	}
}

func TestGatewayServiceBridgeRejectsMissingProvider(t *testing.T) {
	if _, err := NewGatewayServiceBridge(nil); !errors.Is(err, ErrGatewayProviderUnavailable) {
		t.Fatalf("error=%v", err)
	}
}

type nilServiceProvider struct{ releases atomic.Int32 }

func (provider *nilServiceProvider) Acquire(context.Context) (GatewayBusinessService, rpcbridge.ReleaseFunc, error) {
	return nil, func(error) error { provider.releases.Add(1); return nil }, nil
}
func TestNilAcquiredServiceIsReleasedExactlyOnce(t *testing.T) {
	provider := &nilServiceProvider{}
	server, err := NewGatewayServiceBridge(provider)
	if err != nil {
		t.Fatal(err)
	}
	_, err = server.BatchAddRuntimeApi(context.Background(), &meta.BatchRuntimeApiRequest{})
	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("error=%v", err)
	}
	if provider.releases.Load() != 1 {
		t.Fatalf("release=%d", provider.releases.Load())
	}
}

func TestTypedBridgeOverBufconn(t *testing.T) {
	service := &staticGatewayService{}
	listener := bufconn.Listen(1 << 20)
	server := grpcgo.NewServer()
	if err := RegisterGatewayService(server, rpcbridge.Static[GatewayBusinessService](service)); err != nil {
		t.Fatal(err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close(); <-serveDone })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpcgo.DialContext(ctx, "bufnet", grpcgo.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpcgo.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	response, err := meta.NewGatewayServiceClient(conn).BatchAddRuntimeApi(ctx, &meta.BatchRuntimeApiRequest{})
	if err != nil || response.GetMsg() != "batch" {
		t.Fatalf("response=%v err=%v", response, err)
	}
}
