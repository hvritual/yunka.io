package role

import (
	"github.com/hvritual/yunka.io/gateway/rpc/bridge"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

// These compile-time assertions exercise the real in-repository business
// service without changing its RPC method bodies.
var _ meta.GatewayServiceServer = (*RoleIntercept)(nil)
var _ bridge.GatewayBusinessService = (*RoleIntercept)(nil)
