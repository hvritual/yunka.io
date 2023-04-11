package bridge

import (
	"errors"
	"runtime/debug"
	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/internal/resp"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage node
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/19 2:51 下午
 * @Version V1.0
 */

var (
	_ Executor = (*Single)(nil)
)

type Single struct {
	app *core.App
}

func NewSingleNode(app *core.App) *Single {
	return &Single{
		app: app,
	}
}

func (s *Single) Do(modName, srvName string, rt request.Runtime, api *meta.RuntimeApi) (bys []byte, err error) {
	// 如果服务与网关共存 此处需要将api uri更新为内部服务
	ctx := rt.GetRequestCtx()
	uri := stringsExt.SliceToString(ctx.Request.URI().Path())
	if api.IsRedirect {
		uri = api.SrvApi
	}
	h, param, ok := s.app.GetHandleTree().Get(uri)
	if !ok {
		rt.Logger().Debug("GetHandleTree() empty: ", api.Uri)
		return nil, resp.SysNodeNotExistBys
	}
	// auto di service field
	mod := s.app.GetModule(modName)
	if mod == nil {
		rt.Logger().Debug("GetModule() empty:", modName)
		return nil, resp.SysNodeNotExistBys
	}
	srv, err := mod.GetService(srvName, rt)
	if err != nil {
		rt.Logger().Debug("GetService() empty:", srvName)
		return nil, response.ErrSysError
	}
	if srv == nil {
		rt.Logger().Debug("service empty:", srvName)
		return nil, resp.SysNodeNotExistBys
	}

	if param != nil {
		args := ctx.QueryArgs()
		args.Add(param.Key, param.Val)
		(&(ctx.Request)).URI().SetQueryString(args.String())
	}

	srv.SetRuntime(rt)
	defer func() {
		_err := recover()
		if _err != nil {
			err = response.ErrSysError
			(rt.(*request.WorkRuntime)).FinishRequest(errors.New("panic"))
			rt.Logger().Error(_err, string(debug.Stack()))
		}
		mod.PutService(srv)
	}()
	bys, err = h(srv)
	(rt.(*request.WorkRuntime)).FinishRequest(err)
	return
}
