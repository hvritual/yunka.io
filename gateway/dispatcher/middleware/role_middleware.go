package middleware

import (
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/intercept"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
)

/**
* @Description:
* @date 2019-03-18
* @version V1.0
 */
const (
	ermName = `enterpriseRole`
)

// EnterpriseRoleMiddleware is retained as a source-compatible chain element.
// Deprecated: C8.3 moved authorization to gateway/authz.Authorizer at the
// execution boundary. This middleware must not grant or deny access.
type EnterpriseRoleMiddleware struct {
	proxy.Next
}

// NewEnterpriseRoleMiddleware preserves the historical constructor ABI. The
// intercept argument is intentionally ignored: role control-plane services are
// no longer authorization decision points.
func NewEnterpriseRoleMiddleware(_ intercept.Intercept) *EnterpriseRoleMiddleware {
	return &EnterpriseRoleMiddleware{}
}

func (erm *EnterpriseRoleMiddleware) Name() string {
	return ermName
}

func (erm *EnterpriseRoleMiddleware) Do(authStatus bool, rt *request.Context, api *meta.RuntimeApi) {
	erm.Next.Do(authStatus, rt, api)
}
