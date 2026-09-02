package middleware

import (
	"strings"

	"github.com/google/uuid"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/dispatcher/proxy"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
	"github.com/hvritual/yunka.io/pkg/define"
	"github.com/hvritual/yunka.io/pkg/stringsExt"
)

type TraceIdMiddleware struct {
	proxy.Next
}

func (erm *TraceIdMiddleware) Name() string {
	return apiName
}

const xTraceIDHeader = "X-Trace-Id"

func (erm *TraceIdMiddleware) Do(authStatus bool, rt *request.Context, api *meta.RuntimeApi) {
	// W4 span context is canonical when observability middleware is active.
	// Keep the legacy trace_id request header only as a compatibility fallback.
	traceId := strings.TrimSpace(rt.TraceID())
	if traceId == "" {
		traceId = strings.TrimSpace(stringsExt.SliceToString(rt.GetRequestCtx().Request.Header.Peek(define.TraceId)))
	}
	if traceId == "" {
		traceId = uuid.New().String()
	}

	rt.SetTraceID(traceId)
	rt.GetRequestCtx().SetUserValue(define.TraceId, traceId)
	rt.GetRequestCtx().Response.Header.Set(xTraceIDHeader, traceId)

	erm.Next.Do(authStatus, rt, api)
}
