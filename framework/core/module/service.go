package module

import (
	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
)

/**
 * @BelongProject
 * @BelongPackage module
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/9 10:32 下午
 * @Version V1.0
 */
func (mod *module) BindService(srv core.Service, f interface{}) error {
	return mod.register(srv, f)
}

func (mod *module) GetService(name string, rt request.Runtime) (core.Service, error) {
	return mod.getService(name, rt)
}

func (mod *module) PutService(srv core.Service) {
	mod.putService(srv)
}
