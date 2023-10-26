package logExt

import sls "github.com/aliyun/aliyun-log-go-sdk"

type aliLogBody struct {
	traceId string
	content string
}

func (body aliLogBody) Body() map[string]string {
	return map[string]string{
		"content":  body.content,
		"trace_id": body.traceId,
	}
}
func (body aliLogBody) Index() sls.Index {
	return sls.Index{
		Keys: map[string]sls.IndexKey{
			"content": {
				Token:         []string{",", " ", "'", "\"", ";", "=", "(", ")", "[", "]", "{", "}", "?", "@", "<", ">", "/", ":", "\n", "\t", "\r"},
				CaseSensitive: false,
				Type:          "text",
				DocValue:      false,
				Alias:         "",
				Chn:           true,
				JsonKeys:      nil,
			},
			"trace_id": {
				Token: []string{""},
				Type:  "text",
			},
		},
	}
}
