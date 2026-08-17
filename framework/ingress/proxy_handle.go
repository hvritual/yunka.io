package ingress

import (
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"runtime/debug"
	"strings"
	"yunka.io/framework/core"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject xirang
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
	handle, ok := p.exec.IsExist(path)
	if !ok {
		core.Log().Warn("[gateway]", path, " not found")
		ctx.Response.BodyWriter().Write(response.SysNotFoundErr)
		return
	}
	uris := strings.Split(path, `/`)

	if len(uris) < 2 {
		core.Log().Warn("[gateway]", path, " not found")
		ctx.Response.BodyWriter().Write(response.IllegalParamError(errors.New("不规范api")))
		return
	}

	const (
		modIdx = 2
		srvIdx = 3
	)

	bys, err := p.exec.Do(uris[modIdx], uris[srvIdx], rt, handle)
	if err != nil {
		if bys, ok := err.(response.HttpResponse); ok {
			ctx.Write(bys)
		} else {
			ctx.Write(response.ErrSysError)
		}
	} else {
		ctx.Write(bys)
	}
	return
}
