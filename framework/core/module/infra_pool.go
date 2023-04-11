package module

import (
	"reflect"
	"sync"
	"yunka.io/framework/core"
)

func (mod *module) getInfra(fType reflect.Type) (*infra, bool) {
	if fType.Kind() != reflect.Interface {

		_infra, ok := mod.singleInfras.Load(fType)
		if ok {
			infra := _infra.(*infra)
			return infra, true
		}

		pool, ok := mod.infras[fType]
		if !ok {
			return nil, false
		}

		return mod.getPoolInfra(pool, fType)
	} else {
		var inf *infra
		mod.singleInfras.Range(func(key, value interface{}) bool {
			iType := key.(reflect.Type)
			if !iType.Implements(fType) {
				return true
			}
			inf = value.(*infra)
			if inf != nil {
				return false
			}
			return false
		})

		if inf != nil {
			return inf, true
		}

		for iType, infraPool := range mod.infras {
			if !iType.Implements(fType) {
				continue
			}
			return mod.getPoolInfra(infraPool, fType)
		}

	}

	return nil, false

}

func (mod *module) getPoolInfra(pool *sync.Pool, rType reflect.Type) (*infra, bool) {
	_infra := pool.Get()

	init, _ := _infra.(core.RuntimeInit)
	return &infra{
		obj:         _infra,
		runTimeInit: init,
		rType:       rType,
	}, true
}
