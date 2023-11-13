package middleware

import (
	"encoding/json"
	sls "github.com/aliyun/aliyun-log-go-sdk"
)

type AliYunKaLogBody struct {
	TraceId      string `json:"traceId" local:"链路ID"`
	OrgUuid      string `json:"orgUUID" local:"经销商uuid"`
	ProveUuid    string `json:"proveUUID" local:"操作人uuid"`
	ProveName    string `json:"proveName" local:"操作人名称"`
	ModuleName   string `json:"moduleName" local:"模块名称"`
	ActionType   string `json:"actionType" local:"操作类型"`
	ActionResult string `json:"actionResult" local:"操作结果"`
	ActionDesc   string `json:"actionDesc" local:"操作描述"`
	CustomerUuid string `json:"customerUUID" local:"客户uuid"`
	DeviceUuid   string `json:"deviceUUID" local:"设备uuid"`
	ServiceName  string `json:"serviceName" local:"服务名称"`
	Start        string `json:"start" local:"操作开始时间"`
	End          string `json:"end" local:"操作结束时间"`
	ClientIp     string `json:"clientIP" local:"IP地址"`
	Path         string `json:"path" local:"请求路径"`
	Method       string `json:"method" local:"请求类型"`
	RequestBody  string `json:"requestBody" local:"请求BODY"`
	RequestParam string `json:"requestParam" local:"请求Query"`
	ResponseBody string `json:"responseBody" local:"返回结果"`
}

func (body AliYunKaLogBody) Body() map[string]string {
	var res = make(map[string]string)
	bodyByte, _ := json.Marshal(body)
	_ = json.Unmarshal(bodyByte, &res)
	return res
}

func (body AliYunKaLogBody) Index() sls.Index {
	var res = make(map[string]string)
	bodyByte, _ := json.Marshal(body)
	_ = json.Unmarshal(bodyByte, &res)

	var idx = make(map[string]sls.IndexKey)
	for key, _ := range res {
		idx[key] = sls.IndexKey{
			Token:         []string{",", ":", " ", "&", "="},
			CaseSensitive: false,
			Type:          "text",
		}
	}
	return sls.Index{
		Keys: idx,
		// 全文索引。
		Line: &sls.IndexLine{
			Token:         []string{",", ":", " ", "&", "="},
			CaseSensitive: false,
			IncludeKeys:   []string{},
			ExcludeKeys:   []string{},
		},
	}
}
