package middleware

import (
	"strings"
	"time"
	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/internal/resp"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/cryptoExt"
	"yunka.io/pkg/define"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
)

const (
	DefaultRefreshTime = 30 * time.Minute
	TokenLifeCycle     = 7 * 24 * time.Hour
	jwtName            = `jwt`
	jwtKeyName         = `jwt`
	jwtIssuerName      = `jwtIssuer`
	jwtAudienceName    = `jwtAudience`
	userToken          = `token`
)

type TokenHook interface {
	// 检查token 方向
	//  token str 本身
	//  token 映射用户与 api 本身
	//  请求IP 与 API 本身
	CheckToken(rt request.Runtime, api *meta.RuntimeApi, token string) bool

	ResetToken(oldToken, token string) error
}

type JwtMiddleware struct {
	secretKey []byte
	issuer    string
	audience  string
	hooks     []TokenHook
	proxy.Next
}

func init() {
	core.RegisterConfType(jwtKeyName, ``)
	core.RegisterConfType(jwtIssuerName, ``)
	core.RegisterConfType(jwtAudienceName, ``)
}

func NewJwtMiddleware() *JwtMiddleware {

	secretKey := core.GetConfV2(jwtKeyName, ``)
	issuer := core.GetConfV2(jwtIssuerName, ``)
	audience := core.GetConfV2(jwtAudienceName, ``)

	if len(secretKey) < 32 {
		panic("jwt key must contain at least 32 bytes")
	}
	if issuer == `` || audience == `` {
		panic("jwt issuer and audience are required")
	}
	return &JwtMiddleware{
		secretKey: []byte(secretKey),
		issuer:    issuer,
		audience:  audience,
	}
}

func (jwt *JwtMiddleware) Name() string {
	return jwtName
}

func (jwt *JwtMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {
	reqCtx := rt.GetRequestCtx()
	args := (&(reqCtx.Request)).URI().QueryArgs()
	// Identity is derived from the validated token only. Remove client supplied
	// identity and legacy query-string tokens before forwarding the request.
	args.Del(userToken)
	args.Del(define.OrgUUID)
	args.Del(define.UserUUID)
	args.Del(define.RoleUUID)

	authorization := strings.TrimSpace(stringsExt.SliceToString(reqCtx.Request.Header.Peek(strAuthorization)))
	token := ""
	if len(authorization) > len("Bearer ") && strings.EqualFold(authorization[:len("Bearer ")], "Bearer ") {
		token = strings.TrimSpace(authorization[len("Bearer "):])
	}
	if token != `` {
		ok, body := cryptoExt.ParseTokenBodyWithValidation(jwt.secretKey, token, jwt.issuer, jwt.audience)
		if !ok {
			if api.Auth&meta.AuthBit_AuthToken != 0 {
				rt.Logger().Debug("not right not ok")
				reqCtx.Write(resp.SysNotRightBys)
				return
			} else {
				authStatus = false
				jwt.Next.Do(authStatus, rt, api)
				return
			}
		}

		// APIs marked with AuthTimeLimit require server-side token hooks on every
		// request, not only when the token is close to expiry.
		if api.Auth&meta.AuthBit_AuthTimeLimit != 0 {
			for _, hook := range jwt.hooks {
				if !hook.CheckToken(rt, api, token) {
					reqCtx.Write(resp.SysNotRightBys)
					return
				}
			}
		}

		expiresAt, err := body.GetExpirationTime()
		if err != nil || expiresAt == nil {
			reqCtx.Write(resp.SysNotRightBys)
			return
		}
		if time.Until(expiresAt.Time) <= DefaultRefreshTime {
			newToken := cryptoExt.ProduceJwtTokenTime(jwt.secretKey, body, TokenLifeCycle)
			if newToken == "" {
				reqCtx.Write(response.ErrSysError)
				return
			}
			for _, hook := range jwt.hooks {
				if err := hook.ResetToken(token, newToken); err != nil {
					reqCtx.Write(response.ErrSysError)
					return
				}
			}
			token = newToken
		}

		for key, v := range body {
			id, ok := v.(string)
			if ok {
				args.Add(key, id)
			}
		}

		// 更新uri 防止网关、模块一体化时导致uri 没有更新
		(&(reqCtx.Request)).URI().SetQueryString(args.String())
		authStatus = true
		jwt.Next.Do(authStatus, rt, api)
		reqCtx.Response.Header.Set(strAuthorization, token)

	} else {
		if api.Auth > 0 {
			if api.Auth&meta.AuthBit_AuthApi == 0 {
				// API只满足token 授权
				reqCtx.Write(resp.SysNotRightBys)
				return
			}
		}
		authStatus = false
		jwt.Next.Do(authStatus, rt, api)
		return

	}
}
