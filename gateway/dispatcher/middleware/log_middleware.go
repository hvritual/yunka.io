package middleware

import (
	"fmt"
	"net/http"
	"os"
	"time"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/dispatcher/proxy"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
	"github.com/hvritual/yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage middleware
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 2:44 下午
 * @Version V1.0
 */

var (
	green        = string([]byte{27, 91, 57, 55, 59, 52, 50, 109})
	white        = string([]byte{27, 91, 57, 48, 59, 52, 55, 109})
	yellow       = string([]byte{27, 91, 57, 48, 59, 52, 51, 109})
	red          = string([]byte{27, 91, 57, 55, 59, 52, 49, 109})
	blue         = string([]byte{27, 91, 57, 55, 59, 52, 52, 109})
	magenta      = string([]byte{27, 91, 57, 55, 59, 52, 53, 109})
	cyan         = string([]byte{27, 91, 57, 55, 59, 52, 54, 109})
	reset        = string([]byte{27, 91, 48, 109})
	disableColor = false
)

const (
	logName    = `log`
	TimeLayout = "2006/01/02 - 15:04:05"
)

type LogMiddleware struct {
	proxy.Next
}

func (*LogMiddleware) Name() string {
	return logName
}

func (base *LogMiddleware) Do(authStatus bool, rt *request.Context, api *meta.RuntimeApi) {
	start := time.Now()
	base.Next.Do(authStatus, rt, api)

	end := time.Now()
	latency := end.Sub(start)

	clientIP := rt.GetRequestCtx().ClientIP()
	method := rt.GetRequestCtx().Method()
	statusCode := rt.GetRequestCtx().Response.StatusCode()

	var statusColor, methodColor, resetColor string
	statusColor = colorForStatus(statusCode)
	methodColor = colorForMethod(stringsExt.SliceToString(method))
	resetColor = reset

	fmt.Fprintf(os.Stdout, "[gateway] %v |%s %3d %s| %13v | %15s |%s %-7s %s %s\n%s",
		end.Format(TimeLayout),
		statusColor, statusCode, resetColor,
		latency,
		clientIP,
		methodColor, method, resetColor,
		string(rt.GetRequestCtx().Request.URI().Path()),
		"",
	)
}

func colorForStatus(code int) string {
	switch {
	case code >= http.StatusOK && code < http.StatusMultipleChoices:
		return green
	case code >= http.StatusMultipleChoices && code < http.StatusBadRequest:
		return white
	case code >= http.StatusBadRequest && code < http.StatusInternalServerError:
		return yellow
	default:
		return red
	}
}

func colorForMethod(method string) string {
	switch method {
	case "GET":
		return blue
	case "POST":
		return cyan
	case "PUT":
		return yellow
	case "DELETE":
		return red
	case "PATCH":
		return green
	case "HEAD":
		return magenta
	case "OPTIONS":
		return white
	default:
		return reset
	}
}
