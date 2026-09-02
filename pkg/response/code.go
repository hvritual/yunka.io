package response

import (
	"github.com/hvritual/yunka.io/pkg/define"
)

/**
* @Description: TODO
* @author fworld
* @date 2018/8/15
* @version V1.0
 */

const (
	sysSuccess = (define.Reserve) + iota
	sysError
	sysParamIllegal
	sysUnionResponseEmpty
	sysNotFound
	sysSmsExist
	sysSmsCodeCheckFail
	SystemCodeEnd
)

var (
	SysSuccess = NewHttpResponse(sysSuccess, "请求成功")

	ErrSysError = NewHttpResponse(sysError, "系统异常,请稍后再试")

	ErrParamIllegal = NewHttpResponse(sysParamIllegal, "参数不满足要求")

	SysUnionResponseEmptyBys = NewHttpResponse(sysUnionResponseEmpty, "聚合响应为空")

	SysNotFoundErr = NewHttpResponse(sysNotFound, "接口不存在")

	SysSmsCodeCheckFailErr = NewHttpResponse(sysSmsCodeCheckFail, "验证码不匹配")

	SysSmsExistErr = NewHttpResponse(sysSmsExist, "发送过于频繁")
)
