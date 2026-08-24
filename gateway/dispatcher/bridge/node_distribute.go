package bridge

import (
	"context"
	"github.com/valyala/fasthttp"

	"yunka.io/framework/core/request"
	"yunka.io/gateway/internal/resp"
	utils "yunka.io/gateway/internal/util"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/response"
)

/**
 * @BelongProject yunka
 * @BelongPackage node
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/19 5:01 下午
 * @Version V1.0
 */

var (
	_ Executor = (*distribute)(nil)
)

type ServiceNodeDiscovery interface {
	GetNode(ctx context.Context, serviceName string) (*meta.ServerNode, error)
}

type distribute struct {
	opt    *utils.HTTPOption
	client *utils.FastHTTPClient
	snd    ServiceNodeDiscovery
}

func (d *distribute) Do(modName, srvName string, rt *request.Context, api *meta.RuntimeApi) (body []byte, err error) {
	serverNode, err := d.snd.GetNode(rt, modName)
	if err != nil {
		if logger := rt.Logger(); logger != nil {
			logger.Error(err)
		}
		return nil, resp.SysNodeNotExistBys
	}
	r, err := d.client.Do(&(rt.GetRequestCtx().Request), serverNode.IpPort, d.opt)

	if err != nil {
		if logger := rt.Logger(); logger != nil {
			logger.Error(err)
		}
		return nil, err
	}
	if r.StatusCode() != fasthttp.StatusOK {
		return nil, response.ErrSysError
	}
	return r.Body(), nil
}
