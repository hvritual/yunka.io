package grpc

import (
	"sync"
	"errors"
	"fmt"

	grpcgo "google.golang.org/grpc"
	"yunka.io/gateway/rpc/bridge"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/rpcbridge"
)

var (
	ErrGRPCServerUnavailable = errors.New("grpc transport: server is unavailable")
	ErrRPCServiceInvalid     = errors.New("grpc transport: service has an incompatible type")
)

var ErrRPCServiceDuplicate = errors.New("grpc transport: GatewayService is already registered")

// GrpcServer is a handwritten typed registration adapter over grpc.Server.
type GrpcServer struct {
	grpcSrv *grpcgo.Server
	registerMu sync.Mutex
}

func NewTypedGrpcServer(grpcSrv *grpcgo.Server) (*GrpcServer, error) {
	if grpcSrv == nil {
		return nil, ErrGRPCServerUnavailable
	}
	return &GrpcServer{grpcSrv: grpcSrv}, nil
}

// NewGrpcServer preserves the historical constructor name while returning the
// typed registrar directly. A nil grpc.Server produces nil.
func NewGrpcServer(grpcSrv *grpcgo.Server) *GrpcServer {
	server, err := NewTypedGrpcServer(grpcSrv)
	if err != nil {
		return nil
	}
	return server
}

func (server *GrpcServer) GetGrpcServer() *grpcgo.Server {
	if server == nil {
		return nil
	}
	return server.grpcSrv
}

func (server *GrpcServer) RegisterGatewayService(service meta.GatewayServiceServer) error {
	if server == nil || server.grpcSrv == nil {
		return ErrGRPCServerUnavailable
	}
	if service == nil {
		return ErrRPCServiceInvalid
	}

	server.registerMu.Lock()
	defer server.registerMu.Unlock()
	if _, exists := server.grpcSrv.GetServiceInfo()[meta.GatewayService_ServiceDesc.ServiceName]; exists {
		return ErrRPCServiceDuplicate
	}
	meta.RegisterGatewayServiceServer(server.grpcSrv, service)
	return nil
}

func (server *GrpcServer) RegisterGatewayProvider(provider rpcbridge.Provider[bridge.GatewayBusinessService]) error {
	if server == nil || server.grpcSrv == nil {
		return ErrGRPCServerUnavailable
	}
	service, err := bridge.NewGatewayServiceBridge(provider)
	if err != nil {
		return err
	}
	return server.RegisterGatewayService(service)
}

func safeRegister(register func()) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("grpc transport: service registration panicked: %v", recovered)
		}
	}()
	register()
	return nil
}
