package module

import (
	"reflect"
)

func (mod *module) getRepo(fType reflect.Type) (*repository, bool) {
	if fType.Kind() != reflect.Interface {
		pool, ok := mod.repos[fType]
		if ok {
			p := pool.Get()
			rp, ok := p.(*repository)
			return rp, ok
		}
		return nil, false

	}

	for iType, pool := range mod.repos {
		if !iType.Implements(fType) {
			continue
		}
		p := pool.Get()
		rp, ok := p.(*repository)
		return rp, ok
	}
	return nil, false
}
