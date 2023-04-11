package module

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
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
 * @Date:  2020/12/5 9:49 下午
 * @Version V1.0
 */

type infra struct {
	runTimeInit core.RuntimeInit
	rType       reflect.Type
	ptr         reflect.Value
	obj         interface{}
}

func buildInfra(f interface{}, rType reflect.Type) *infra {
	values := reflect.ValueOf(f).Call([]reflect.Value{})
	infraObj := values[0].Interface()
	infra := &infra{}
	if _, ok := infraObj.(core.RuntimeInit); ok {
		infra.runTimeInit = infraObj.(core.RuntimeInit)
	}
	infra.rType = rType
	infra.obj = infraObj
	return infra
}

func (mod *module) BindInfra(single bool, f interface{}) error {
	if single {
		iType, err := parsePoolFunc(f)
		if err != nil {
			return err
		}
		_, ok := mod.singleInfras.Load(iType)
		if ok {
			return errors.New(fmt.Sprintf("infra :%v exist", iType))
		}

		core.RegisterInitiator(func(app *core.App) {
			mod.singleInfras.Store(iType, buildInfra(f, iType))
		})
		return nil
	} else {
		return mod.bind(f, func() map[reflect.Type]*sync.Pool {
			return mod.infras
		})
	}

}

func (mod *module) GetInfra(rt request.Runtime, rType reflect.Type) interface{} {
	infra, ok := mod.getInfra(rType)
	if !ok {
		infra, _ = mod.getInfra(rType.Elem())
	}
	if infra != nil {
		if infra.runTimeInit != nil {
			err := infra.runTimeInit.Init(rt)
			if err != nil {
				return nil
			}
		}
		return infra.obj
	}

	return nil
}

// 如果infra是单例，则数据不需要放入
func (mod *module) PutInfra(rType reflect.Type, infra interface{}) {

	if _, ok := mod.singleInfras.Load(rType); ok {
		return
	}

	if rType.Kind() != reflect.Interface {
		pool, ok := mod.infras[rType]
		if ok {
			pool.Put(infra)
		}
	} else {
		for iType, infraPool := range mod.infras {
			if !iType.Implements(rType) {
				continue
			}
			infraPool.Put(infra)
		}
	}
}
