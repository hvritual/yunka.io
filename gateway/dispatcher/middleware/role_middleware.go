package middleware

import (
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/intercept"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/internal/resp"
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

type EnterpriseRoleMiddleware struct {
	intercept intercept.Intercept
	proxy.Next
}

func NewEnterpriseRoleMiddleware(v intercept.Intercept) *EnterpriseRoleMiddleware {
	return &EnterpriseRoleMiddleware{
		intercept: v,
	}
}

func (erm *EnterpriseRoleMiddleware) Name() string {
	return ermName
}

func (erm *EnterpriseRoleMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {

	if api.Auth > 0 && (api.Auth&meta.AuthBit_AuthRole != 0) {
		principal, ok := identity.FromContext(rt)
		if !ok || !principal.Authenticated || principal.TenantID == "" || len(principal.Roles) == 0 || erm.intercept == nil {
			rt.GetRequestCtx().Write(resp.SysNotRightBys)
			return
		}
		if bys, allowed := erm.intercept.VerifyRoleApiRight(rt, api.Uuid, principal.TenantID, principal.Roles); !allowed {
			rt.GetRequestCtx().Write(bys)
			return
		}
		authStatus = true
	}

	erm.Next.Do(authStatus, rt, api)

	return
}
