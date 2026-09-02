package middleware

import (
	"errors"
	"strings"
	"time"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/dispatcher/proxy"
	"github.com/hvritual/yunka.io/gateway/internal/resp"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
	"github.com/hvritual/yunka.io/pkg/cryptoExt"
	"github.com/hvritual/yunka.io/pkg/define"
	"github.com/hvritual/yunka.io/pkg/response"
	"github.com/hvritual/yunka.io/pkg/stringsExt"
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
	CheckToken(rt *request.Context, api *meta.RuntimeApi, token string) bool

	ResetToken(oldToken, token string) error
}

type JwtMiddleware struct {
	secretKey []byte
	issuer    string
	audience  string
	hooks     []TokenHook
	proxy.Next
}

type JWTConfig struct {
	Secret   []byte
	Issuer   string
	Audience string
}

func NewJwtMiddleware(config JWTConfig) (*JwtMiddleware, error) {
	config.Issuer = strings.TrimSpace(config.Issuer)
	config.Audience = strings.TrimSpace(config.Audience)
	if len(config.Secret) < 32 {
		return nil, errors.New("jwt key must contain at least 32 bytes")
	}
	if config.Issuer == "" || config.Audience == "" {
		return nil, errors.New("jwt issuer and audience are required")
	}
	return &JwtMiddleware{
		secretKey: append([]byte(nil), config.Secret...),
		issuer:    config.Issuer,
		audience:  config.Audience,
	}, nil
}

func (jwt *JwtMiddleware) Name() string {
	return jwtName
}

func claimString(claims map[string]interface{}, key string) string {
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	if stringer, ok := value.(interface{ String() string }); ok {
		return strings.TrimSpace(stringer.String())
	}
	return ""
}

func claimRoles(claims map[string]interface{}) []string {
	value, ok := claims[define.RoleUUID]
	if !ok || value == nil {
		return nil
	}
	var roles []string
	switch typed := value.(type) {
	case string:
		roles = strings.Split(typed, define.RoleContactFLag)
	case []string:
		roles = append(roles, typed...)
	case []interface{}:
		for _, item := range typed {
			if role, ok := item.(string); ok {
				roles = append(roles, role)
			}
		}
	}
	result := roles[:0]
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role != "" {
			result = append(result, role)
		}
	}
	return append([]string(nil), result...)
}

func principalFromClaims(claims map[string]interface{}) identity.Principal {
	userID := claimString(claims, define.UserUUID)
	subject := claimString(claims, "sub")
	if userID == "" {
		userID = subject
	}
	if subject == "" {
		subject = userID
	}
	return identity.Principal{
		Subject:       subject,
		TenantID:      claimString(claims, define.OrgUUID),
		UserID:        userID,
		Roles:         claimRoles(claims),
		AuthMethod:    identity.AuthMethodJWT,
		Authenticated: true,
	}
}

func (jwt *JwtMiddleware) Do(authStatus bool, rt *request.Context, api *meta.RuntimeApi) {
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

		rt.SetPrincipal(principalFromClaims(map[string]interface{}(body)))

		// Preserve validated string claims in the query only for source
		// compatibility with legacy handlers. Authorization must use Principal.
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
		if api.Auth > 0 && api.Auth&meta.AuthBit_AuthApi == 0 && !authStatus {
			// API只满足token 授权
			reqCtx.Write(resp.SysNotRightBys)
			return
		}
		jwt.Next.Do(authStatus, rt, api)
		return

	}
}
