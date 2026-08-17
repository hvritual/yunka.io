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
 * @Date:  2020/12/7 10:17 下午
 * @Version V1.0
 */

type ModuleInit func(Module) error

type Module interface {
	Name() string

	Init(ModuleInit)

	BindInfra(single bool, f interface{}) error

	BindService(srv Service, f interface{}) error

	BindRepository(f interface{}) error

	GetService(name string, rt request.Runtime) (Service, error)

	PutService(srv Service)

	GetRepo(rt request.Runtime, rType reflect.Type) Repository

	PutRepo(rType reflect.Type, repo Repository)

	GetInfra(rt request.Runtime, rType reflect.Type) interface{}

	PutInfra(rType reflect.Type, repo interface{})

	Stop()
}
