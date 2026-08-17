package resp

import (
	"strconv"
	"yunka.io/pkg/response"
)

/**
* @Description: TODO
* @author fworld
* @date 2018/8/15
* @version V1.0
 */

const (
	SysParamIllegalCode = (response.SystemCodeEnd | 0x1) + iota
	SysErrorCode
	SysBizDataCode
	SysNotApiCode
	_
	SysNetworkCode
	SysNotRightCode
	SysNodeNotExistCode
	SysBlackCode
	SysEnterpriseUnbindApiCode
	SysEnterpriseApiExpireCode
	SysRoleNotMatchCode
	SysServerNotExitPubCode
	SysBusyCode
)

var (
	SysBizData = response.HttpBaseMsg{
		Code: strconv.FormatInt(SysBizDataCode, 10),
		Msg:  "业务返回数据异常",
		Data: nil,
	}
	SysBizDataBys = SysBizData.Marshal()

	SysCheck = response.HttpBaseMsg{
		Code: strconv.FormatInt(SysNotApiCode, 10),
		Msg:  "映射路径错误,风控介入",
		Data: nil,
	}
	SysCheckBys = SysCheck.Marshal()

	SysNotRight = response.HttpBaseMsg{
		Code: strconv.FormatInt(SysNotRightCode, 10),
		Msg:  "您暂无对应操作权限",
		Data: nil,
	}
	SysNotRightBys = SysNotRight.Marshal()

	SysNodeNotExist = response.HttpBaseMsg{
		Code: strconv.FormatInt(SysNodeNotExistCode, 10),
		Msg:  "暂无响应结点,请稍后再试",
		Data: nil,
	}
	SysNodeNotExistBys = SysNodeNotExist.Marshal()

	SysBlack = response.HttpBaseMsg{
		Code: strconv.FormatInt(SysBlackCode, 10),
		Msg:  "您无权限访问系统",
		Data: nil,
	}
	SysBlackBys = SysBlack.Marshal()

	SysRoleNotMatch = response.HttpBaseMsg{
		Code: strconv.FormatInt(SysRoleNotMatchCode, 10),
		Msg:  "权限不足，请联系企业管理员开通授权",
		Data: nil,
	}
	SysRoleNotMatchBys = SysRoleNotMatch.Marshal()

	SysBusy = response.HttpBaseMsg{
		Code: strconv.FormatInt(SysBusyCode, 10),
		Msg:  "业务繁忙请稍后再试",
		Data: nil,
	}
	SysBusyBys = SysBusy.Marshal()
)
