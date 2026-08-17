package ingress

import (
	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
)

/**
 * @BelongProject xirang
 * @BelongPackage node
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/19 2:51 下午
 * @Version V1.0
 */

// gateway 转发给node
type Executor interface {
	Do(modName, srvName string, rt request.Runtime, handle core.Handle) (body []byte, err error)

	IsExist(uri string) (core.Handle, bool)
}
