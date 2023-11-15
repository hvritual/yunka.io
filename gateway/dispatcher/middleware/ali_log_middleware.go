package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/aliLogStore"
	"yunka.io/pkg/define"
)

const (
	aliLogName              = "ali-log"
	deviceUUIDKey           = "device_uuid"
	ServiceNameKey          = "service_name"
	ModuleNameKey           = "module_name"
	ProveNameHeaderKey      = "proveName"
	ReplaceOrgNameHeaderKey = "replaceOrgName"
	OrgNameHeaderKey        = "orgName"
)

type AliLogMiddleware struct {
	proxy.Next
	log            *aliLogStore.Log
	beforeHandle   []AliLogHandle
	afterHandle    []AliLogHandle
	whiteHandleMap map[string][]AliLogHandle
}

func NewAliLogMiddleware(log *aliLogStore.Log, whiteHandleMap map[string][]AliLogHandle) *AliLogMiddleware {
	var m = &AliLogMiddleware{
		log:            log,
		whiteHandleMap: whiteHandleMap,
	}

	m.beforeHandle = []AliLogHandle{
		m.setTraceId,
		m.setStart,
		m.setOrgUUID,
		m.setProveUuid,
		m.setProveName,
		m.setModuleName,
		m.setActionType,
		m.setActionDesc,
		m.setCustomerUuid,
		m.setDeviceUUID,
		m.setServiceName,
		m.setClientIp,
		m.setPath,
		m.setMethod,
		m.setRequestBody,
		m.setRequestParam,
	}

	m.afterHandle = []AliLogHandle{
		m.setEnd,
		m.setActionResult,
		m.setResponseBody,
	}

	return m
}

type AliLogHandle func(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi)

func (aliLog *AliLogMiddleware) Name() string {
	return aliLogName
}
func (aliLog *AliLogMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {
	var (
		key = api.GetUri()
	)
	if _, ok := aliLog.whiteHandleMap[key]; ok {
		var (
			body = &AliYunKaLogBody{}
		)
		// 前置
		if aliLog.beforeHandle != nil {
			for i := 0; i < len(aliLog.beforeHandle); i++ {
				aliLog.beforeHandle[i](body, rt, api)
			}
		}

		aliLog.Next.Do(authStatus, rt, api)

		// 后置
		if aliLog.afterHandle != nil {
			for i := 0; i < len(aliLog.afterHandle); i++ {
				aliLog.afterHandle[i](body, rt, api)
			}
		}
		// 覆盖
		if aliLog.whiteHandleMap[key] != nil {
			for i := 0; i < len(aliLog.whiteHandleMap[key]); i++ {
				aliLog.whiteHandleMap[key][i](body, rt, api)
			}
		}

		err := aliLog.log.Put(body)
		if err != nil {
			rt.Logger().Error(err)
		}
		return
	}

	aliLog.Next.Do(authStatus, rt, api)
}

func (aliLog *AliLogMiddleware) setTraceId(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	traceId := rt.GetRequestCtx().UserValue(define.TraceId)
	if traceId != nil {
		body.TraceId = traceId.(string)
	}
}
func (aliLog *AliLogMiddleware) setStart(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.Start = fmt.Sprintf("%d", time.Now().UnixMilli())
}
func (aliLog *AliLogMiddleware) setEnd(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.End = fmt.Sprintf("%d", time.Now().UnixMilli())
}
func (aliLog *AliLogMiddleware) setOrgUUID(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.OrgUuid = rt.GetRequestCtx().GetOrgUUID()
}
func (aliLog *AliLogMiddleware) setProveUuid(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ProveUuid = rt.GetRequestCtx().GetProveUUID()
}
func (aliLog *AliLogMiddleware) setProveName(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ProveName = string(rt.GetRequestCtx().Request.Header.Peek(ProveNameHeaderKey))
}
func (aliLog *AliLogMiddleware) setModuleName(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ModuleName = api.GetModuleName()
}

// -table:yunka_system_api;-field:eng_name;
func (aliLog *AliLogMiddleware) setActionType(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ActionType = api.GetName()
}
func (aliLog *AliLogMiddleware) setActionResult(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ActionResult = "失败"
	if rt.GetRequestCtx().Response.StatusCode() == http.StatusOK {
		var respBody = rt.GetRequestCtx().Response.Body()
		var mapRes = make(map[string]string)
		_ = json.Unmarshal(respBody, &mapRes)
		if mapRes["code"] == "0" {
			body.ActionResult = "成功"
		}
	}
}
func (aliLog *AliLogMiddleware) setActionDesc(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	// TODO: add api desc
}
func (aliLog *AliLogMiddleware) setCustomerUuid(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.CustomerUuid = rt.GetRequestCtx().GetOrgUUID()
}
func (aliLog *AliLogMiddleware) setServiceName(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ServiceName = rt.GetServiceName()
}
func (aliLog *AliLogMiddleware) setClientIp(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ClientIp = rt.GetRequestCtx().LocalIP().String()
}
func (aliLog *AliLogMiddleware) setPath(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.Path = api.GetUri()
}
func (aliLog *AliLogMiddleware) setMethod(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.Method = string(rt.GetRequestCtx().Method())
}
func (aliLog *AliLogMiddleware) setRequestBody(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.RequestBody = string(rt.GetRequestCtx().Request.Body())
}
func (aliLog *AliLogMiddleware) setRequestParam(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.RequestParam = string(rt.GetRequestCtx().Request.URI().QueryString())
}
func (aliLog *AliLogMiddleware) setResponseBody(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ResponseBody = string(rt.GetRequestCtx().Response.Body())
}
func (aliLog *AliLogMiddleware) setDeviceUUID(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	var deviceUUID = rt.GetRequestCtx().RequestCtx.UserValue(deviceUUIDKey)
	if deviceUUID == nil {
		deviceUUID = ""
	}
	body.DeviceUuid = deviceUUID.(string)
}
func (aliLog *AliLogMiddleware) setReplaceOrgName(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.ReplaceOrgName = string(rt.GetRequestCtx().Request.Header.Peek(ReplaceOrgNameHeaderKey))
}
func (aliLog *AliLogMiddleware) setOrgName(body *AliYunKaLogBody, rt request.Runtime, api *meta.RuntimeApi) {
	body.OrgName = string(rt.GetRequestCtx().Request.Header.Peek(OrgNameHeaderKey))
}
