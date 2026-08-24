package bridge

import (
	"context"
	"errors"

	"yunka.io/gateway/rpc/meta"
)

const DefaultGatewayServiceName = "GatewayService"

var ErrServiceNotFound = errors.New("rpc bridge: GatewayService was not found")

// GatewayBusinessService is the explicit business-facing RPC ABI. Application
// composition owns the implementation; request-time service lookup, runtime
// mutation, and lifecycle pooling are not supported.
type GatewayBusinessService interface {
	BatchAddRuntimeApi(context.Context, *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error)
	DeleteRuntimeApi(context.Context, *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error)
	OperateRoleAPI(context.Context, *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error)
}

var _ meta.GatewayServiceServer = (GatewayBusinessService)(nil)
