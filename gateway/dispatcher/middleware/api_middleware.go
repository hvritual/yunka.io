package middleware

import (
	"crypto/subtle"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/stringsExt"
)

/**
* @Description:
* @date 2019-03-18
* @version V1.0
 */
const (
	apiName = `api`
	xCode   = `X-Code`
)

type APIMiddleware struct {
	proxy.Next
	key string
}

func NewAPIMiddleware(key string) *APIMiddleware {
	return &APIMiddleware{
		key: key,
	}
}

func (erm *APIMiddleware) Name() string {
	return apiName
}

func (erm *APIMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {
	if api.Auth > 0 && (api.Auth&meta.AuthBit_AuthApi != 0) {
		code := stringsExt.SliceToString(rt.GetRequestCtx().Request.Header.Peek(xCode))
		if len(erm.key) >= 32 && len(code) == len(erm.key) &&
			subtle.ConstantTimeCompare([]byte(code), []byte(erm.key)) == 1 {
			authStatus = true
			if principal, ok := request.PrincipalFromRuntime(rt); !ok || !principal.Authenticated {
				request.SetPrincipal(rt, identity.Principal{
					Subject:       "api-key",
					AuthMethod:    identity.AuthMethodAPIKey,
					Authenticated: true,
				})
			}
		}
	}

	erm.Next.Do(authStatus, rt, api)

	return
}
