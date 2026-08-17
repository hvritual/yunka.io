package middleware

import (
	"time"
	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/internal/resp"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/cryptoExt"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
)

const (
	DefaultRefreshTime = 30 * 60
	TokenLifeCycle     = 7 * 24 * time.Hour
	jwtName            = `jwt`
	jwtKeyName         = `jwt`
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
	hooks     []TokenHook
	proxy.Next
}

func init() {
	core.RegisterConfType(jwtKeyName, ``)
}

func NewJwtMiddleware() *JwtMiddleware {

	secretKey := core.GetConfV2(jwtKeyName, ``)

	if secretKey == `` {
		panic("jwt not found key")
	}
	return &JwtMiddleware{
		secretKey: []byte(secretKey),
	}
}

func (jwt *JwtMiddleware) Name() string {
	return jwtName
}

func (jwt *JwtMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {
	reqCtx := rt.GetRequestCtx()
	args := (&(reqCtx.Request)).URI().QueryArgs()
	// 清除Token 减少网络传输数据
	token := stringsExt.SliceToString(args.Peek(userToken))
	if token != `` {
		//ok, body := cryptoExt.ParseTokenBody(jwt.secretKey, token)
		// FIXME  后期需要收敛应用token 时间问题
		ok, body := cryptoExt.ParseTokenBodyNotValidation(jwt.secretKey, token)
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

		// 需要强制检查Token
		if api.Auth&meta.AuthBit_AuthTimeLimit != 0 {
			if !body.VerifyExpiresAt(time.Now().Unix()+DefaultRefreshTime, true) {
				newToken := cryptoExt.ProduceJwtTokenTime(jwt.secretKey, body, TokenLifeCycle)
				for _, hook := range jwt.hooks {
					if !hook.CheckToken(rt, api, token) {
						reqCtx.Write(resp.SysNotRightBys)
						return
					}
					if err := hook.ResetToken(token, newToken); err != nil {
						reqCtx.Write(response.ErrSysError)
						return
					}
				}
				token = newToken
			} else {
				for _, hook := range jwt.hooks {
					if !hook.CheckToken(rt, api, token) {
						reqCtx.Write(resp.SysNotRightBys)
						return
					}
				}
			}
		} else {
			if !body.VerifyExpiresAt(time.Now().Unix()+DefaultRefreshTime, true) {
				token = cryptoExt.ProduceJwtTokenTime(jwt.secretKey, body, TokenLifeCycle)
			}
		}

		for key, v := range body {
			id, ok := v.(string)
			if ok {
				args.Add(key, id)
			}
		}

		args.Del(userToken)
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
