package middleware

import (
	"crypto/subtle"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/dispatcher/proxy"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
	"github.com/hvritual/yunka.io/pkg/stringsExt"
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

func (erm *APIMiddleware) Do(authStatus bool, rt *request.Context, api *meta.RuntimeApi) {
	if api.Auth > 0 && (api.Auth&meta.AuthBit_AuthApi != 0) {
		code := stringsExt.SliceToString(rt.GetRequestCtx().Request.Header.Peek(xCode))
		if len(erm.key) >= 32 && len(code) == len(erm.key) &&
			subtle.ConstantTimeCompare([]byte(code), []byte(erm.key)) == 1 {
			authStatus = true
			if principal, ok := rt.Principal(); !ok || !principal.Authenticated {
				rt.SetPrincipal(identity.Principal{
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
