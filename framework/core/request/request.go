package request

import (
	"github.com/valyala/fasthttp"
	"net"
	"strings"
	"github.com/hvritual/yunka.io/framework/core/binding"
	"github.com/hvritual/yunka.io/pkg/define"
	"github.com/hvritual/yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage application
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 4:43 下午
 * @Version V1.0
 */

type RequestCtx struct {
	*fasthttp.RequestCtx
}

func (ctx *RequestCtx) GetOrgUUID() string {
	return stringsExt.SliceToString(ctx.QueryArgs().Peek(define.OrgUUID))
}

func (ctx *RequestCtx) GetProveUUID() string {
	return stringsExt.SliceToString(ctx.QueryArgs().Peek(define.UserUUID))
}

func (ctx *RequestCtx) GetRoleUUID() []string {
	role := stringsExt.SliceToString(ctx.QueryArgs().Peek(define.RoleUUID))
	if role == `` {
		return nil
	}
	return strings.Split(role, define.RoleContactFLag)
}

func (ctx *RequestCtx) ClientIP() string {
	clientIP := stringsExt.SliceToString(ctx.Request.Header.Peek("X-Forwarded-For"))
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])
	if clientIP == "" {
		clientIP = strings.TrimSpace(stringsExt.SliceToString(ctx.Request.Header.Peek("X-Real-Ip")))
	}
	if clientIP != "" {
		return clientIP
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(ctx.RemoteAddr().String())); err == nil {
		return ip
	}

	return ""
}

func (ctx *RequestCtx) Query(key string) string {
	return stringsExt.SliceToString(ctx.QueryArgs().Peek(key))
}

func (ctx *RequestCtx) ShouldBindJSON(obj interface{}) error {
	return ctx.ShouldBindWith(obj, binding.JSON)
}

func (ctx *RequestCtx) ShouldBindQuery(obj interface{}) error {
	return ctx.ShouldBindWith(obj, binding.Query)
}

func (ctx *RequestCtx) ShouldBindWith(obj interface{}, b binding.Bind) error {
	return b.Bind(&(ctx.Request), obj)
}
