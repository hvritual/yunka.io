package response

import (
	"encoding/json"
	"github.com/buger/jsonparser"
	"strconv"
)

/**
 * @BelongProject yunka
 * @BelongPackage response
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 11:30 上午
 * @Version V1.0
 */

type (
	HttpBaseMsg struct {
		Code string      `json:"code"`
		Msg  string      `json:"msg"`
		Data interface{} `json:"data"`
	}

	HttpResponse []byte
)

func NewHttpResponse(code int64, msg string) HttpResponse {
	return HttpBaseMsg{
		Code: strconv.FormatInt(code, 10),
		Msg:  msg,
	}.Marshal()
}

func (h HttpResponse) Error() string {
	val, _ := jsonparser.GetString(h, "msg")
	return val
}

func (h HttpBaseMsg) Marshal() HttpResponse {
	bys, _ := json.Marshal(h)
	return bys
}

func IllegalParamError(err error) HttpResponse {
	return HttpBaseMsg{
		Code: strconv.FormatInt(sysParamIllegal, 10),
		Msg:  "参数不满足要求:" + err.Error(),
		Data: nil,
	}.Marshal()
}
