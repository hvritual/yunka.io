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
		ctx := rt.GetRequestCtx()
		orgUUID := ctx.GetOrgUUID()
		roleUUID := ctx.GetRoleUUID()
		if len(roleUUID) != 0 {
			if bys, ok := erm.intercept.VerifyRoleApiRight(api.Uuid, orgUUID, roleUUID); !ok {
				ctx.Write(bys)
				return
			}
			authStatus = true
		}
	}


	erm.Next.Do(authStatus, rt, api)

	return
}
