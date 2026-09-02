package request

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"github.com/valyala/fasthttp"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
	"github.com/hvritual/yunka.io/pkg/define"
	"github.com/hvritual/yunka.io/pkg/logExt"
	"github.com/hvritual/yunka.io/pkg/memstore"
	"github.com/hvritual/yunka.io/pkg/response"
	"github.com/hvritual/yunka.io/pkg/stringsExt"
)

const (
	DataKey = `data`
	codeKey = `code`
)

// Context is the concrete request-owned HTTP execution boundary. A Context is
// created for exactly one inbound or composed request and is never pooled or
// retained by application-owned services.
type Context struct {
	request *RequestCtx
	base    context.Context
	store   *memstore.Store
	logger  logExt.Logger
	status  string
}

// NewContext creates a fresh request context. raw may be nil for transport-
// neutral tests; base defaults to context.Background().
func NewContext(base context.Context, raw *fasthttp.RequestCtx) *Context {
	if base == nil {
		base = context.Background()
	}
	return &Context{
		request: &RequestCtx{RequestCtx: raw},
		base:    base,
		store:   &memstore.Store{},
	}
}

// NewHTTPRequestContext creates a fresh context for one fasthttp request.
func NewHTTPRequestContext(raw *fasthttp.RequestCtx) *Context {
	return NewContext(context.Background(), raw)
}

// CloneRequest creates a fresh child Context for a composed call. Only the
// request transport is copied; response state and mutable per-request storage
// are intentionally not shared. Trusted context values and cancellation are
// inherited through the parent context.Context.
func (ctx *Context) CloneRequest() *Context {
	raw := &fasthttp.RequestCtx{}
	if ctx != nil && ctx.request != nil && ctx.request.RequestCtx != nil {
		ctx.request.Request.CopyTo(&raw.Request)
	}
	child := NewContext(ctx, raw)
	if ctx != nil {
		child.logger = ctx.logger
	}
	return child
}

func (ctx *Context) GetRequestCtx() *RequestCtx {
	if ctx == nil {
		return nil
	}
	return ctx.request
}

func (ctx *Context) SetRequestCtx(raw *fasthttp.RequestCtx) {
	if ctx == nil {
		return
	}
	if ctx.request == nil {
		ctx.request = &RequestCtx{}
	}
	ctx.request.RequestCtx = raw
}

func (ctx *Context) SetContext(base context.Context) {
	if ctx == nil {
		return
	}
	if base == nil {
		base = context.Background()
	}
	ctx.base = base
}

func (ctx *Context) baseContext() context.Context {
	if ctx == nil || ctx.base == nil {
		return context.Background()
	}
	return ctx.base
}

func (ctx *Context) Deadline() (time.Time, bool) { return ctx.baseContext().Deadline() }
func (ctx *Context) Done() <-chan struct{}       { return ctx.baseContext().Done() }
func (ctx *Context) Err() error                  { return ctx.baseContext().Err() }

func (ctx *Context) Value(key any) any {
	if ctx == nil {
		return nil
	}
	if key == 0 && ctx.request != nil && ctx.request.RequestCtx != nil {
		return &ctx.request.Request.Header
	}
	if value := ctx.baseContext().Value(key); value != nil {
		return value
	}
	if keyText, ok := key.(string); ok && ctx.request != nil && ctx.request.RequestCtx != nil {
		return stringsExt.SliceToString(ctx.request.Request.Header.Peek(keyText))
	}
	return nil
}

func (ctx *Context) Set(key, value string) {
	if ctx == nil || ctx.request == nil || ctx.request.RequestCtx == nil {
		return
	}
	ctx.request.Request.Header.Set(key, value)
}

func (ctx *Context) ForeachKey(handler func(key, value string) error) error {
	if ctx == nil || ctx.request == nil || ctx.request.RequestCtx == nil {
		return errors.New("request context is unavailable")
	}
	var result error
	ctx.request.Request.Header.VisitAll(func(key, value []byte) {
		if result != nil {
			return
		}
		result = handler(stringsExt.SliceToString(key), stringsExt.SliceToString(value))
	})
	return result
}

func (ctx *Context) Logger() logExt.Logger {
	if ctx == nil {
		return nil
	}
	if trace, ok := ctx.logger.(logExt.Trace); ok {
		traceID := ctx.TraceID()
		if traceID == "" && ctx.request != nil && ctx.request.RequestCtx != nil {
			if value, ok := ctx.request.UserValue(define.TraceId).(string); ok {
				traceID = value
			}
		}
		if traceID != "" {
			trace.Set(traceID)
		}
	}
	return ctx.logger
}

func (ctx *Context) SetLogger(logger logExt.Logger) {
	if ctx != nil {
		ctx.logger = logger
	}
}

func (ctx *Context) Store() *memstore.Store {
	if ctx == nil {
		return nil
	}
	if ctx.store == nil {
		ctx.store = &memstore.Store{}
	}
	return ctx.store
}

func (ctx *Context) Status() string {
	if ctx == nil {
		return ""
	}
	return ctx.status
}

func (ctx *Context) GetParam(target any) error {
	if ctx == nil || ctx.request == nil || ctx.request.RequestCtx == nil {
		return errors.New("request context is unavailable")
	}
	return ctx.request.ShouldBindQuery(target)
}

func (ctx *Context) GetBody(target any) error {
	if ctx == nil || ctx.request == nil || ctx.request.RequestCtx == nil {
		return errors.New("request context is unavailable")
	}
	return ctx.request.ShouldBindJSON(target)
}

func (ctx *Context) WriteBys(responseBytes, data []byte) ([]byte, error) {
	result, err := jsonparser.Set(responseBytes, data, DataKey)
	if err != nil {
		return responseBytes, response.ErrSysError
	}
	return result, nil
}

func (ctx *Context) ResponseObject(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return ctx.ResponseByte(data)
}

func (ctx *Context) ResponseString(value string) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return ctx.ResponseByte(data)
}

func (ctx *Context) ResponseError(err error) ([]byte, error) {
	if err == nil {
		err = response.ErrSysError
	}
	data, marshalErr := json.Marshal(err.Error())
	if marshalErr != nil {
		return nil, marshalErr
	}
	return ctx.WriteBys(response.ErrSysError, data)
}

func (ctx *Context) ResponseByte(data []byte) ([]byte, error) {
	return ctx.WriteBys(response.SysSuccess, data)
}

func (ctx *Context) ResponseHtml(html string) ([]byte, error) {
	if ctx != nil && ctx.request != nil && ctx.request.RequestCtx != nil {
		ctx.request.SetContentType("text/html; charset=utf-8")
	}
	return []byte(html), nil
}

func (ctx *Context) Write(data []byte) error {
	if ctx == nil || ctx.request == nil || ctx.request.RequestCtx == nil {
		return errors.New("request context is unavailable")
	}
	_, err := ctx.request.Response.BodyWriter().Write(data)
	ctx.status, _ = jsonparser.GetString(data, codeKey)
	return err
}

func (ctx *Context) JSONWrite(value any) (int, error) {
	if ctx == nil || ctx.request == nil || ctx.request.RequestCtx == nil {
		return 0, errors.New("request context is unavailable")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return ctx.request.Response.BodyWriter().Write(data)
}

func (ctx *Context) SetPrincipal(principal identity.Principal) {
	if ctx == nil {
		return
	}
	ctx.base = identity.WithPrincipal(ctx.baseContext(), principal)
	if ctx.request == nil || ctx.request.RequestCtx == nil {
		return
	}
	ctx.request.SetUserValue(define.OrgUUID, principal.TenantID)
	ctx.request.SetUserValue(define.UserUUID, principal.UserID)
	ctx.request.SetUserValue(define.RoleUUID, append([]string(nil), principal.Roles...))
}

func (ctx *Context) Principal() (identity.Principal, bool) {
	return identity.FromContext(ctx.baseContext())
}

func (ctx *Context) SetMetadata(metadata runtimecontext.Metadata) {
	if ctx != nil {
		ctx.base = runtimecontext.WithMetadata(ctx.baseContext(), metadata)
	}
}

func (ctx *Context) Metadata() (runtimecontext.Metadata, bool) {
	return runtimecontext.MetadataFrom(ctx.baseContext())
}

func (ctx *Context) SetTraceID(traceID string) {
	if ctx == nil {
		return
	}
	traceID = strings.TrimSpace(traceID)
	ctx.base = runtimecontext.WithTraceID(ctx.baseContext(), traceID)
	if ctx.request != nil && ctx.request.RequestCtx != nil {
		ctx.request.SetUserValue(define.TraceId, traceID)
	}
}

func (ctx *Context) TraceID() string {
	return runtimecontext.TraceIDFrom(ctx.baseContext())
}

func (ctx *Context) GetServiceName() string {
	metadata, _ := ctx.Metadata()
	return metadata.Service
}
