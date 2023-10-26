package middleware

import (
	"github.com/google/uuid"
	"strings"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/define"
	"yunka.io/pkg/stringsExt"
)

type TraceIdMiddleware struct {
	proxy.Next
}

func (erm *TraceIdMiddleware) Name() string {
	return apiName
}

func (erm *TraceIdMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {

	traceId := strings.TrimSpace(stringsExt.SliceToString(rt.GetRequestCtx().Request.Header.Peek(define.TraceId)))

	if traceId == "" {
		traceId = uuid.New().String()
	}

	rt.GetRequestCtx().RequestCtx.SetUserValue(define.TraceId, traceId)

	erm.Next.Do(authStatus, rt, api)

	return
}
