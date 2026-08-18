package proxy

import (
	"context"
	"github.com/valyala/fasthttp"
	"net/http"
	"runtime/debug"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/gateway/internal/resp"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage proxy
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 1:42 下午
 * @Version V1.0
 */

func (p *Proxy) serverHttp(ctx *fasthttp.RequestCtx) {

	rt := p.acquireControllerContext()
	defer p.putControllerContext(rt)
	defer func() {
		if err := recover(); err != nil {
			ctx.Write(response.ErrSysError)
			rt.Logger().Error(string(debug.Stack()))
		}
	}()

	rt.SetRequestCtx(ctx)
	rt.SetLogger(p.logFn())
	path := stringsExt.SliceToString(ctx.Path())
	api, ok := p.tree.Get(path)
	if !ok {
		rt.Logger().Warn("[gateway]", path, " not found ip:", rt.GetRequestCtx().ClientIP())
		ctx.Response.SetStatusCode(http.StatusNotFound)
		ctx.Response.BodyWriter().Write(resp.SysCheckBys)
		return
	}
	rt.SetSrvName(api.SrvName)
	rt.SetMetadata(runtimecontext.Metadata{
		Transport: "http",
		Protocol:  "http",
		Operation: api.Uri,
		Route:     path,
		Service:   api.SrvName,
		Module:    api.ModuleName,
		Method:    stringsExt.SliceToString(ctx.Method()),
	})

	err := p.runtimeMiddles.Handle(rt, func(context.Context) error {
		p.middles.Do(false, rt, api)
		return nil
	})
	if err != nil {
		rt.Logger().Error("runtime middleware error:", err)
		ctx.Response.SetStatusCode(runtimeMiddlewareStatus(err))
		ctx.Write(response.ErrSysError)
	}
	return
}
