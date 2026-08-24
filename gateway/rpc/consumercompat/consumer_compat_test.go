package consumercompat_test

import (
	"context"
	"net"
	"testing"
	"time"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"yunka.io/gateway/rpc/bridge"
	"yunka.io/gateway/rpc/client"
	"yunka.io/gateway/rpc/handle"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/rpcbridge"
)

// existingService is a plain application-owned service with no runtime mutation.
type existingService struct{ calls int }

func (service *existingService) BatchAddRuntimeApi(context.Context, *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	service.calls++
	return &meta.OperateRuntimeApiResponse{Msg: "batch"}, nil
}
func (service *existingService) DeleteRuntimeApi(context.Context, *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	service.calls++
	return &meta.OperateRuntimeApiResponse{Msg: "delete"}, nil
}
func (service *existingService) OperateRoleAPI(context.Context, *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	service.calls++
	return &meta.OperateRoleResponse{Msg: "role"}, nil
}

var _ meta.GatewayServiceServer = (*existingService)(nil)
var _ bridge.GatewayBusinessService = (*existingService)(nil)

func TestExistingBusinessServiceAndHistoricalFacadeUseSingleTypedRuntime(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := grpcgo.NewServer()
	service := &existingService{}
	if err := bridge.RegisterGatewayService(server, rpcbridge.Static[bridge.GatewayBusinessService](service)); err != nil {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpcgo.DialContext(
		ctx,
		"passthrough:///consumer-compat",
		grpcgo.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpcgo.WithTransportCredentials(insecure.NewCredentials()),
		grpcgo.WithBlock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	var targetSeen string
	factory := client.NewConnectionFactory(connection, func(_ context.Context, target string) (grpcgo.ClientConnInterface, error) {
		targetSeen = target
		return connection, nil
	})
	facade := handle.NewGatewayServiceClient(factory, client.WithTimeout(time.Second))
	batch, err := facade.BatchAddRuntimeApi(ctx, &meta.BatchRuntimeApiRequest{})
	if err != nil || batch.GetMsg() != "batch" {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	role, err := facade.OperateRoleAPISpecial(ctx, &meta.RoleModuleBtn{}, "node-a")
	if err != nil || role.GetMsg() != "role" || targetSeen != "node-a" {
		t.Fatalf("role=%+v target=%q err=%v", role, targetSeen, err)
	}
	if service.calls != 2 {
		t.Fatalf("calls=%d", service.calls)
	}
}
