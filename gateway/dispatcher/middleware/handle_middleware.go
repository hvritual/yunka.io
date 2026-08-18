package middleware

import (
	"github.com/buger/jsonparser"
	"github.com/panjf2000/ants"
	"github.com/pkg/errors"
	"strings"
	"sync"
	"yunka.io/framework/core/request"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/gateway/dispatcher/bridge"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/internal/resp"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
	"yunka.io/pkg/syncExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage middleware
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/19 2:42 下午
 * @Version V1.0
 */

const (
	defaultSize   = 1024
	handleName    = `handle`
	basicJsonCode = `code`
	basicJsonMsg  = `msg`
	basicJsonData = `data`
)

type HandleMiddleware struct {
	proxy.Next
	pool *ants.Pool
	exec bridge.Executor
}

func NewHandleMiddleware(exec bridge.Executor) *HandleMiddleware {
	pool, err := ants.NewPool(defaultSize)
	if err != nil {
		panic(err)
	}
	return &HandleMiddleware{
		pool: pool,
		exec: exec,
	}
}

func (hm *HandleMiddleware) Name() string {
	return handleName
}

func (hm *HandleMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {
	if api.Auth > 0 && !authStatus {
		rt.Write(resp.SysNotRightBys)
		return
	}

	if len(api.Composes) == 0 {
		// 重定向api
		if api.IsRedirect {
			reqCtx := rt.GetRequestCtx()
			(&(reqCtx.Request)).URI().SetPath(api.SrvApi)
		}
		body, err := hm.exec.Do(api.ModuleName, api.SrvName, rt, api)
		if err != nil {
			if bys, ok := err.(response.HttpResponse); ok {
				rt.Write(bys)
			} else {
				bys, _ := rt.ResponseError(err)
				rt.Write(bys)
			}
			return
		}
		if len(body) != 0 {
			rt.Write(body)
		}
	} else {
		body := stringsExt.StringToSlice(`{}`)
		lock := sync.RWMutex{}
		args := rt.GetRequestCtx().URI().QueryArgs()
		factory := func(item *meta.RuntimeApi) func() error {
			newRt := request.NewWorkRuntime()
			newRt.CopyFrom(rt.GetRequestCtx())
			newRt.SetLogger(rt.Logger())
			// Composed calls inherit trusted identity, trace and deadlines from the
			// parent context while deriving operation metadata for the child call.
			newRt.SetContext(rt)
			metadata, _ := runtimecontext.MetadataFrom(rt)
			metadata.Operation = item.Uri
			metadata.Route = item.Uri
			metadata.Service = item.SrvName
			metadata.Module = item.ModuleName
			newRt.SetMetadata(metadata)

			if args.Len() != 0 {
				var builder strings.Builder
				builder.WriteString(item.Uri)
				builder.WriteString(`?`)
				builder.WriteString(args.String())
				(&(newRt.GetRequestCtx().Request)).URI().Update(builder.String())
			} else {
				(&(newRt.GetRequestCtx().Request)).URI().Update(item.Uri)
			}

			// 重定向api
			if item.IsRedirect {
				reqCtx := newRt.GetRequestCtx()
				(&(reqCtx.Request)).URI().SetPath(item.SrvApi)
			}

			return func() error {

				resp, err := hm.exec.Do(item.ModuleName, item.SrvName, newRt, item)
				if err != nil {
					rt.Logger().Error(err)
					return err
				}

				strValue, _ := jsonparser.GetString(resp, basicJsonCode)
				if strValue != `0` {
					v, _ := jsonparser.GetString(resp, basicJsonMsg)
					rt.Logger().Error(v)
					return errors.New(v)
				}
				value, dataType, _, err := jsonparser.Get(resp, basicJsonData)
				if err != nil {
					rt.Logger().Error(err)
					return err
				}
				switch dataType {
				case jsonparser.String:
					lenVal := len(value)
					data := make([]byte, lenVal+2)
					data[0] = '"'
					copy(data[1:], value)
					data[lenVal+1] = '"'
					value = data
				}
				lock.Lock()
				defer lock.Unlock()
				body, err = jsonparser.Set(body, value, item.Name)
				return err
			}
		}

		var h = make([]func() error, len(api.Composes))
		for idx, item := range api.Composes {
			h[idx] = factory(item)
		}

		err := syncExt.Finish(h...)
		if err != nil {
			if bys, ok := err.(response.HttpResponse); ok {
				rt.Write(bys)
			} else {
				bys, _ := rt.ResponseError(err)
				rt.Write(bys)
			}
			return
		}
		if len(body) != 0 {
			bys, err := jsonparser.Set(response.SysSuccess, body, `data`)
			if err != nil {
				rt.Logger().Error(err)
				bys, _ = rt.ResponseError(err)
			}
			rt.Write(bys)

		} else {
			rt.Write(response.SysUnionResponseEmptyBys)
		}
	}
	return
}
