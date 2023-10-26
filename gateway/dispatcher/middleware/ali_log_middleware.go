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
	aliLogName     = "ali-log"
	deviceUUIDKey  = "device_uuid"
	ServiceNameKey = "service_name"
	ModuleNameKey  = "module_name"
)

type AliLogMiddleware struct {
	proxy.Next
	log             *aliLogStore.Log
	whiteList       []string
	whiteListHandle map[string][]LogHandle
}

type LogHandle func(rt request.Runtime, api *meta.RuntimeApi)

func NewAliLogMiddleware(log *aliLogStore.Log, whiteList []string, whiteListHandle map[string][]LogHandle) *AliLogMiddleware {
	return &AliLogMiddleware{log: log, whiteList: whiteList, whiteListHandle: whiteListHandle}
}

func (erm *AliLogMiddleware) Name() string {
	return aliLogName
}

func (erm *AliLogMiddleware) doHandle(key string, handleMap map[string][]LogHandle, rt request.Runtime, api *meta.RuntimeApi) {
	if handleMap == nil {
		return
	}
	if _, ok := handleMap[key]; !ok {
		return
	}
	if handleMap[key] == nil || len(handleMap[key]) == 0 {
		return
	}
	for i := 0; i < len(handleMap[key]); i++ {
		handleMap[key][i](rt, api)
	}
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
				erm.doHandle(uri, erm.whiteListHandle, rt, api)
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
		serviceName  = rt.GetServiceName()
		moduleName   = api.GetModuleName()
	)
	sKey := rt.GetRequestCtx().UserValue(ServiceNameKey)
	if sKey != nil {
		serviceName = sKey.(string)
	}
	mKey := rt.GetRequestCtx().UserValue(ModuleNameKey)
	if sKey != nil {
		moduleName = mKey.(string)
	}

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
		ModuleName:   moduleName,
		ServiceName:  serviceName,
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
