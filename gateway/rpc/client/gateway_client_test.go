package client

import (
	"context"
	"errors"
	"testing"
	"time"

	grpcgo "google.golang.org/grpc"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

type fakeTypedGatewayClient struct {
	deadlineSeen bool
	err          error
}

func (client *fakeTypedGatewayClient) BatchAddRuntimeApi(ctx context.Context, _ *meta.BatchRuntimeApiRequest, _ ...grpcgo.CallOption) (*meta.OperateRuntimeApiResponse, error) {
	_, client.deadlineSeen = ctx.Deadline()
	if client.err != nil {
		return nil, client.err
	}
	return &meta.OperateRuntimeApiResponse{Msg: "batch"}, nil
}

func (client *fakeTypedGatewayClient) DeleteRuntimeApi(context.Context, *meta.DeleteRuntimeApiRequest, ...grpcgo.CallOption) (*meta.OperateRuntimeApiResponse, error) {
	if client.err != nil {
		return nil, client.err
	}
	return &meta.OperateRuntimeApiResponse{Msg: "delete"}, nil
}

func (client *fakeTypedGatewayClient) OperateRoleAPI(context.Context, *meta.RoleModuleBtn, ...grpcgo.CallOption) (*meta.OperateRoleResponse, error) {
	if client.err != nil {
		return nil, client.err
	}
	return &meta.OperateRoleResponse{Msg: "role"}, nil
}

func TestCompatibilityFacadeUsesTypedClientAndHistoricalTimeout(t *testing.T) {
	typed := &fakeTypedGatewayClient{}
	client := NewGatewayServiceClient(typed)
	response, err := client.BatchAddRuntimeApi(context.Background(), &meta.BatchRuntimeApiRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetMsg() != "batch" || !typed.deadlineSeen {
		t.Fatalf("response=%+v deadline=%v", response, typed.deadlineSeen)
	}
}

func TestCompatibilityFacadeReturnsNonNilResponseOnError(t *testing.T) {
	want := errors.New("rpc failed")
	client := NewTypedGatewayServiceClient(&fakeTypedGatewayClient{err: want}, WithTimeout(0))
	response, err := client.DeleteRuntimeApi(context.Background(), &meta.DeleteRuntimeApiRequest{})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if response == nil {
		t.Fatal("historical facade must return a non-nil response on error")
	}
}

func TestCompatibilityConstructorRejectsOldRuntimeObjects(t *testing.T) {
	client := NewGatewayServiceClient(struct{}{}, WithTimeout(time.Second))
	_, err := client.OperateRoleAPI(context.Background(), &meta.RoleModuleBtn{})
	if !errors.Is(err, ErrClientSourceUnsupported) {
		t.Fatalf("error=%v", err)
	}
}

func TestTargetFactoryRequiresTarget(t *testing.T) {
	client := NewGatewayServiceClientWithFactory(NewTypedFactory(&fakeTypedGatewayClient{}, nil), WithTimeout(time.Second))
	_, err := client.OperateRoleAPISpecial(context.Background(), &meta.RoleModuleBtn{}, " ")
	if !errors.Is(err, ErrTargetRequired) {
		t.Fatalf("error = %v", err)
	}
}
