package meta

import (
	"context"
	"testing"
)

// c5GatewayServiceABI freezes the business-facing method signatures used by
// existing framework services. The standard generator may change internals,
// but these signatures and historical message fields remain source compatible.
type c5GatewayServiceABI interface {
	BatchAddRuntimeApi(context.Context, *BatchRuntimeApiRequest) (*OperateRuntimeApiResponse, error)
	DeleteRuntimeApi(context.Context, *DeleteRuntimeApiRequest) (*OperateRuntimeApiResponse, error)
	OperateRoleAPI(context.Context, *RoleModuleBtn) (*OperateRoleResponse, error)
}

type c5GatewayServiceStub struct{}

func (*c5GatewayServiceStub) BatchAddRuntimeApi(context.Context, *BatchRuntimeApiRequest) (*OperateRuntimeApiResponse, error) {
	return &OperateRuntimeApiResponse{}, nil
}
func (*c5GatewayServiceStub) DeleteRuntimeApi(context.Context, *DeleteRuntimeApiRequest) (*OperateRuntimeApiResponse, error) {
	return &OperateRuntimeApiResponse{}, nil
}
func (*c5GatewayServiceStub) OperateRoleAPI(context.Context, *RoleModuleBtn) (*OperateRoleResponse, error) {
	return &OperateRoleResponse{}, nil
}

var _ c5GatewayServiceABI = (*c5GatewayServiceStub)(nil)
var _ GatewayServiceServer = (*c5GatewayServiceStub)(nil)

func TestC5GatewayMessageSourceABI(t *testing.T) {
	request := &BatchRuntimeApiRequest{
		Apis: []*RuntimeApi{{Uuid: "api", SrvName: "gateway", ModuleName: "core"}},
		Buttons: []*RuntimeApiModuleButton{{
			ApiUUID: "api", ModuleBtnUUID: "button", Permissions: []string{"drink.execute"},
		}},
	}
	role := &RoleModuleBtn{
		OrgUUID: "org", RoleUUID: "role",
		ModuleBtnUUID: []string{"a"}, DeleteModuleBtnUUID: []string{"b"},
		Permissions: []string{"drink.execute"}, DeletePermissions: []string{"drink.admin"},
	}
	if request.GetApis()[0].GetUuid() != "api" || request.GetButtons()[0].GetModuleBtnUUID() != "button" {
		t.Fatalf("gateway request ABI changed: %+v", request)
	}
	if request.GetButtons()[0].GetPermissions()[0] != "drink.execute" {
		t.Fatalf("button permission ABI missing: %+v", request.GetButtons()[0])
	}
	if role.GetOrgUUID() != "org" || role.GetDeleteModuleBtnUUID()[0] != "b" {
		t.Fatalf("historical role ABI changed: %+v", role)
	}
	if role.GetPermissions()[0] != "drink.execute" || role.GetDeletePermissions()[0] != "drink.admin" {
		t.Fatalf("permission role ABI missing: %+v", role)
	}
	if GatewayService_BatchAddRuntimeApi_FullMethodName != "/io.yunka.gateway.rpc.GatewayService/BatchAddRuntimeApi" {
		t.Fatalf("unexpected full method name %q", GatewayService_BatchAddRuntimeApi_FullMethodName)
	}
}
