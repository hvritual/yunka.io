package grpc

import (
	"context"
	"errors"
	"testing"

	grpcgo "google.golang.org/grpc"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

type typedTestGatewayService struct{}

func (*typedTestGatewayService) BatchAddRuntimeApi(context.Context, *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	return &meta.OperateRuntimeApiResponse{}, nil
}
func (*typedTestGatewayService) DeleteRuntimeApi(context.Context, *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	return &meta.OperateRuntimeApiResponse{}, nil
}
func (*typedTestGatewayService) OperateRoleAPI(context.Context, *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	return &meta.OperateRoleResponse{}, nil
}

func TestTypedServerRegistration(t *testing.T) {
	if NewGrpcServer(nil) != nil {
		t.Fatal("nil grpc server must not produce a registrar")
	}
	registrar, err := NewTypedGrpcServer(grpcgo.NewServer())
	if err != nil {
		t.Fatal(err)
	}
	if err := registrar.RegisterGatewayService(&typedTestGatewayService{}); err != nil {
		t.Fatal(err)
	}
	if err := registrar.RegisterGatewayService(nil); !errors.Is(err, ErrRPCServiceInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestDuplicateTypedRegistrationIsContained(t *testing.T) {
	registrar := NewGrpcServer(grpcgo.NewServer())
	service := &typedTestGatewayService{}
	if err := registrar.RegisterGatewayService(service); err != nil {
		t.Fatal(err)
	}
	if err := registrar.RegisterGatewayService(service); err == nil {
		t.Fatal("duplicate registration must fail")
	}
}
