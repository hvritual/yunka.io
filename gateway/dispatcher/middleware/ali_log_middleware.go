package middleware

import (
	"fmt"
	"time"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/aliLogStore"
	"yunka.io/pkg/define"
)

const (
	aliLogName    = "ali-log"
	deviceUUIDKey = "device_uuid"
)

type AliLogMiddleware struct {
	proxy.Next
	log             *aliLogStore.Log
	whiteList       []string
	whiteListHandle map[string]LogHandle
}

type LogHandle func(rt request.Runtime, api *meta.RuntimeApi)

func NewAliLogMiddleware(log *aliLogStore.Log, whiteList []string, whiteListHandle map[string]LogHandle) *AliLogMiddleware {
	return &AliLogMiddleware{log: log, whiteList: whiteList, whiteListHandle: whiteListHandle}
}

func (erm *AliLogMiddleware) Name() string {
	return aliLogName
}

func (erm *AliLogMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {
	// 白名单过滤
	var uri = api.GetUri()
	var cancelFlag = false
	if erm.whiteList != nil && len(erm.whiteList) > 0 {
		for i := 0; i < len(erm.whiteList); i++ {
			if uri == erm.whiteList[i] {
				cancelFlag = true
				// 执行 handle
				if erm.whiteListHandle != nil {
					if handle, ok := erm.whiteListHandle[uri]; ok {
						handle(rt, api)
					}
				}
				break
			}
		}
	}
	if !cancelFlag {
		erm.Next.Do(authStatus, rt, api)
		return
	}

	var (
		traceId = rt.GetRequestCtx().RequestCtx.UserValue(define.TraceId)
		start   = time.Now().Unix()
	)

	erm.Next.Do(authStatus, rt, api)

	var (
		end          = time.Now().Unix()
		bodyByte     = rt.GetRequestCtx().Request.Body()
		paramByte    = rt.GetRequestCtx().Request.URI().QueryString()
		responseByte = rt.GetRequestCtx().Response.Body()
	)

	responseByte = rt.GetRequestCtx().Response.Body()
	var deviceUUID = rt.GetRequestCtx().RequestCtx.UserValue(deviceUUIDKey)
	if deviceUUID == nil {
		deviceUUID = ""
	}
	var body = aliLogStore.YunKaLog{
		TraceId:      traceId.(string),
		OrgUuid:      rt.GetRequestCtx().GetOrgUUID(),
		ProveUuid:    rt.GetRequestCtx().GetProveUUID(),
		CustomerUuid: rt.GetRequestCtx().GetOrgUUID(),
		DeviceUuid:   deviceUUID.(string),
		ModuleName:   api.GetModuleName(),
		ServiceName:  rt.GetServiceName(),
		Start:        fmt.Sprintf("%d", start),
		End:          fmt.Sprintf("%d", end),
		ClientIp:     rt.GetRequestCtx().ClientIP(),
		Path:         uri,
		Method:       string(rt.GetRequestCtx().Method()),
		RequestBody:  string(bodyByte),
		RequestParam: string(paramByte),
		ResponseBody: string(responseByte),
	}

	err := erm.log.Put(body)
	if err != nil {
		rt.Logger().Error(err)
	}

	return
}
