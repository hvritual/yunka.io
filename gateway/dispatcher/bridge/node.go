package bridge

import (
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

/**
 * @BelongProject yunka
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
	Do(modName, srvName string, rt *request.Context, api *meta.RuntimeApi) (body []byte, err error)
}
