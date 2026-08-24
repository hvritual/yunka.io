package proxy

import (
	"context"
	"net/http"
	"runtime/debug"

	"github.com/valyala/fasthttp"
	"yunka.io/framework/core/request"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/observability"
	"yunka.io/gateway/internal/resp"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
)

func (proxy *Proxy) serverHTTP(raw *fasthttp.RequestCtx) {
	rt := request.NewHTTPRequestContext(raw)
	defer func() {
		if recovered := recover(); recovered != nil {
			_, _ = raw.Write(response.ErrSysError)
			if logger := rt.Logger(); logger != nil {
				logger.Error(recovered, string(debug.Stack()))
			}
		}
	}()

	base := observability.Extract(context.Background(), fastHTTPHeaderCarrier{header: &raw.Request.Header})
	rt.SetContext(base)
	if proxy.contextLogFn != nil {
		rt.SetLogger(proxy.contextLogFn(rt))
	} else {
		rt.SetLogger(proxy.logger())
	}

	path := stringsExt.SliceToString(raw.Path())
	api, ok := proxy.tree.Get(path)
	if !ok {
		if logger := rt.Logger(); logger != nil {
			logger.Warn("[gateway]", path, " not found ip:", rt.GetRequestCtx().ClientIP())
		}
		raw.Response.SetStatusCode(http.StatusNotFound)
		_, _ = raw.Response.BodyWriter().Write(resp.SysCheckBys)
		return
	}

	rt.SetMetadata(runtimecontext.Metadata{
		Transport: "http",
		Protocol:  "http",
		Operation: api.Uri,
		Route:     path,
		Service:   api.SrvName,
		Module:    api.ModuleName,
		Method:    stringsExt.SliceToString(raw.Method()),
	})

	if err := proxy.runtimeMiddles.Handle(rt, func(child context.Context) error {
		rt.SetContext(child)
		proxy.middles.Do(false, rt, api)
		return nil
	}); err != nil {
		if logger := rt.Logger(); logger != nil {
			logger.Error("runtime middleware error:", err)
		}
		raw.Response.SetStatusCode(runtimeMiddlewareStatus(err))
		_, _ = raw.Write(response.ErrSysError)
	}
}
