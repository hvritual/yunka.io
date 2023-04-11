package core

import (
	"reflect"
	"yunka.io/framework/core/request"
)

/**
 * @BelongProject yunka
 * @BelongPackage core
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/5 7:25 下午
 * @Version V1.0
 */
const (
	moduleServiceContactFlag = `/`
)

var (
	_ Service = (*BaseService)(nil)
)

// Service is interface, all domain service implement it.
type Service interface {
	request.BaseRuntime
	GetName() string
}

type RuntimeInit interface {
	Init(rt request.Runtime) error
}

type (
	RuntimeService interface {
		Service
		RegisterInit(infras []RuntimeInit)
		GetRuntimeInit() []RuntimeInit
		RegisterSrvItems(infra, repos []*ServiceItem)
		GetSrvItems() (infra, repos []*ServiceItem)
	}

	ServiceItem struct {
		Type reflect.Type
		Ptr  reflect.Value
		Obj  interface{} // 存放infra 或者 repository
	}
)

type BaseService struct {
	request.Runtime
	inits []RuntimeInit  //所依赖的RuntimeInit
	infra []*ServiceItem //所依赖的RuntimeInit
	repos []*ServiceItem //所依赖的RuntimeInit
}

func (srv *BaseService) GetName() string {
	panic("implement me")
}

func (srv *BaseService) SetRuntime(rt request.Runtime) {
	srv.Runtime = rt
}

func (srv *BaseService) GetRuntime() request.Runtime {
	return srv.Runtime
}

func (srv *BaseService) RegisterInit(infras []RuntimeInit) {
	srv.inits = infras
}

func (srv *BaseService) GetRuntimeInit() []RuntimeInit {
	return srv.inits
}

//请勿调用 维持服务所需要的基础设施
func (srv *BaseService) RegisterSrvItems(infra, repos []*ServiceItem) {
	srv.infra, srv.repos = infra, repos
}

//请勿调用 维持服务所需要的基础设施
func (srv *BaseService) GetSrvItems() (infra, repos []*ServiceItem) {
	return srv.infra, srv.repos
}
