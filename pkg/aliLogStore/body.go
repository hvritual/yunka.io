package aliLogStore

import (
	"encoding/json"
	sls "github.com/aliyun/aliyun-log-go-sdk"
)

type YunKaLog struct {
	TraceId      string `json:"trace_id"`
	OrgUuid      string `json:"org_uuid"`
	ProveUuid    string `json:"prove_uuid"`
	CustomerUuid string `json:"customer_uuid"`
	DeviceUuid   string `json:"device_uuid"`
	ModuleName   string `json:"module_name"`
	ServiceName  string `json:"service_name"`
	Start        string `json:"start"`
	End          string `json:"end"`
	ClientIp     string `json:"client_ip"`
	Path         string `json:"path"`
	Method       string `json:"method"`
	RequestBody  string `json:"request_body"`
	RequestParam string `json:"request_param"`
	ResponseBody string `json:"response_body"`
}

func (body YunKaLog) Body() map[string]string {
	var res = make(map[string]string)
	bodyByte, _ := json.Marshal(body)
	_ = json.Unmarshal(bodyByte, &res)
	return res
}

func (body YunKaLog) Index() sls.Index {
	return sls.Index{
		// 字段索引。
		Keys: map[string]sls.IndexKey{
			"trace_id": {
				Token:         []string{","},
				CaseSensitive: false,
				Type:          "text",
			},
			"org_uuid": {
				Token:         []string{","},
				CaseSensitive: false,
				Type:          "text",
			},
			"prove_uuid": {
				Token:         []string{","},
				CaseSensitive: false,
				Type:          "text",
			},
			"customer_uuid": {
				Token:         []string{","},
				CaseSensitive: false,
				Type:          "text",
			},
			"device_uuid": {
				Token:         []string{","},
				CaseSensitive: false,
				Type:          "text",
			},
			"module_name": {
				Token:         []string{","},
				CaseSensitive: false,
				Type:          "text",
			},
			"service_name": {
				Token:         []string{","},
				CaseSensitive: false,
				Type:          "text",
			},
			"start": {
				Token:         []string{},
				CaseSensitive: false,
				Type:          "text",
			},
			"end": {
				Token:         []string{},
				CaseSensitive: false,
				Type:          "text",
			},
			"client_ip": {
				Token:         []string{""},
				CaseSensitive: false,
				Type:          "text",
			},
			"path": {
				Token:         []string{"/"},
				CaseSensitive: false,
				Type:          "text",
			},
			"method": {
				Token:         []string{","},
				CaseSensitive: false,
				Type:          "text",
			},
			"request_body": {
				Chn:           true,
				Token:         []string{":", ",", " "},
				CaseSensitive: false,
				Type:          "text",
			},
			"request_param": {
				Chn:           true,
				Token:         []string{"&", "="},
				CaseSensitive: false,
				Type:          "text",
			},
			"response_body": {
				Chn:           true,
				Token:         []string{":", ",", " "},
				CaseSensitive: false,
				Type:          "text",
			},
		},
		// 全文索引。
		Line: &sls.IndexLine{
			Token:         []string{",", ":", " ", "&", "="},
			CaseSensitive: false,
			IncludeKeys:   []string{},
			ExcludeKeys:   []string{},
		},
	}
}
