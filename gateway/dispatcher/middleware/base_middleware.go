package middleware

import (
	"github.com/valyala/fasthttp"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
)

const (
	strAuthorization = `Authorization`

	strServerHead = `Server`
	strServer     = `yunka`

	strContentTypeHead = `Content-Type`
	strContentType     = `application/json; charset=utf-8`

	strACAllowOriginHead = `Access-Control-Allow-Origin`
	strACAllowOrigin     = `*`

	strACAllowCredentialsHead = `Access-Control-Allow-Credentials`
	strACAllowCredentials     = `false`

	strACAllowMethodHead = `Access-Control-Allow-Methods`
	strACAllowMethod     = `GET, POST, PUT, PATCH, DELETE, OPTIONS`

	strACAllowHeadersHead = `Access-Control-Allow-Headers`
	strACAllowHeaders     = `Origin, X-Requested-With, Content-Type, Accept, Authorization, X-Code, version`

	strAcExposeHeaderHead = `Access-Control-Expose-Headers`
	strAcExposeHeader     = `Authorization, X-Trace-Id`

	strXForwardFor = `X-Forwarded-For`
	baseName       = `base`
)

type BaseMiddleware struct {
	proxy.Next
}

func (*BaseMiddleware) Name() string {
	return baseName
}

func (base *BaseMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {
	rCtx := rt.GetRequestCtx()
	rCtx.Response.Header.Set(strServerHead, strServer)
	rCtx.Response.Header.Set(strContentTypeHead, strContentType)
	rCtx.Response.Header.Set(strACAllowOriginHead, strACAllowOrigin)
	rCtx.Response.Header.Set(strACAllowCredentialsHead, strACAllowCredentials)
	rCtx.Response.Header.Set(strACAllowMethodHead, strACAllowMethod)
	rCtx.Response.Header.Set(strACAllowHeadersHead, strACAllowHeaders)
	rCtx.Response.Header.Set(strAcExposeHeaderHead, strAcExposeHeader)

	if api == nil {
		rCtx.Error(`not found`, fasthttp.StatusNotFound)
		return
	}

	if rCtx.IsOptions() {
		rCtx.Response.SetStatusCode(200)
		return
	}

	(rCtx.Request.Header).Set(strXForwardFor, rCtx.ClientIP())
	base.Next.Do(authStatus, rt, api)
	return
}
