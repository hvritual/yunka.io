package middleware

import (
	"errors"
	"strings"
	"sync"

	"github.com/buger/jsonparser"
	"yunka.io/framework/core/request"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/gateway/authz"
	"yunka.io/gateway/dispatcher/bridge"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/internal/resp"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
	"yunka.io/pkg/syncExt"
)

const (
	handleName    = `handle`
	basicJSONCode = `code`
	basicJSONMsg  = `msg`
	basicJSONData = `data`
)

var ErrExecutorUnavailable = errors.New("gateway handle middleware: executor is required")

type HandleMiddleware struct {
	proxy.Next
	exec       bridge.Executor
	authorized bool
}

func NewHandleMiddleware(executor bridge.Executor) *HandleMiddleware {
	if executor == nil {
		panic(ErrExecutorUnavailable)
	}
	return &HandleMiddleware{exec: executor}
}

func NewAuthorizedHandleMiddleware(executor bridge.Executor, authorizer authz.Authorizer) *HandleMiddleware {
	authorized, err := bridge.NewAuthorizedExecutor(executor, authorizer)
	if err != nil {
		panic(err)
	}
	return &HandleMiddleware{exec: authorized, authorized: true}
}

func (middleware *HandleMiddleware) Name() string { return handleName }

func (middleware *HandleMiddleware) Do(authStatus bool, rt *request.Context, api *meta.RuntimeApi) {
	if rt == nil || api == nil {
		return
	}
	if middleware.exec == nil {
		_ = rt.Write(response.ErrSysError)
		return
	}
	if api.Authorization != nil && !middleware.authorized {
		_ = rt.Write(resp.SysNotRightBys)
		return
	}
	if api.Auth > 0 && !authStatus {
		_ = rt.Write(resp.SysNotRightBys)
		return
	}

	if len(api.Composes) == 0 {
		if api.IsRedirect {
			rt.GetRequestCtx().Request.URI().SetPath(api.SrvApi)
		}
		body, err := middleware.exec.Do(api.ModuleName, api.SrvName, rt, api)
		if err != nil {
			middleware.writeError(rt, err)
			return
		}
		if len(body) != 0 {
			_ = rt.Write(body)
		}
		return
	}

	body := stringsExt.StringToSlice(`{}`)
	var lock sync.Mutex
	query := rt.GetRequestCtx().URI().QueryArgs().String()
	factory := func(item *meta.RuntimeApi) func() error {
		return func() error {
			child := rt.CloneRequest()
			metadata, _ := runtimecontext.MetadataFrom(rt)
			metadata.Operation = item.Uri
			metadata.Route = item.Uri
			metadata.Service = item.SrvName
			metadata.Module = item.ModuleName
			child.SetMetadata(metadata)

			if query != "" {
				var builder strings.Builder
				builder.WriteString(item.Uri)
				builder.WriteByte('?')
				builder.WriteString(query)
				child.GetRequestCtx().Request.URI().Update(builder.String())
			} else {
				child.GetRequestCtx().Request.URI().Update(item.Uri)
			}
			if item.IsRedirect {
				child.GetRequestCtx().Request.URI().SetPath(item.SrvApi)
			}

			result, err := middleware.exec.Do(item.ModuleName, item.SrvName, child, item)
			if err != nil {
				return err
			}
			code, _ := jsonparser.GetString(result, basicJSONCode)
			if code != `0` {
				message, _ := jsonparser.GetString(result, basicJSONMsg)
				return errors.New(message)
			}
			value, dataType, _, err := jsonparser.Get(result, basicJSONData)
			if err != nil {
				return err
			}
			if dataType == jsonparser.String {
				quoted := make([]byte, len(value)+2)
				quoted[0], quoted[len(quoted)-1] = '"', '"'
				copy(quoted[1:], value)
				value = quoted
			}
			lock.Lock()
			defer lock.Unlock()
			body, err = jsonparser.Set(body, value, item.Name)
			return err
		}
	}

	handlers := make([]func() error, len(api.Composes))
	for index, item := range api.Composes {
		handlers[index] = factory(item)
	}
	if err := syncExt.Finish(handlers...); err != nil {
		middleware.writeError(rt, err)
		return
	}
	if len(body) == 0 {
		_ = rt.Write(response.SysUnionResponseEmptyBys)
		return
	}
	result, err := jsonparser.Set(response.SysSuccess, body, basicJSONData)
	if err != nil {
		middleware.writeError(rt, err)
		return
	}
	_ = rt.Write(result)
}

func (*HandleMiddleware) writeError(rt *request.Context, err error) {
	if authz.IsDenied(err) {
		_ = rt.Write(resp.SysNotRightBys)
		return
	}
	if responseError, ok := err.(response.HttpResponse); ok {
		_ = rt.Write(responseError)
		return
	}
	data, encodeErr := rt.ResponseError(err)
	if encodeErr != nil {
		_ = rt.Write(response.ErrSysError)
		return
	}
	_ = rt.Write(data)
}
