package module

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
)

// come from github.com/8treenet/freedom
// modify by fworld

var (
	ErrNotFoundService = errors.New("not found service")
)

type (
	serviceContainer struct {
		pool      *sync.Pool
		buildFunc interface{}
	}
)

func (mod *module) register(srv core.Service, f interface{}) error {
	_, ok := mod.srvContainer[srv.GetName()]

	if ok {
		return errors.New(fmt.Sprintf("service :%s exist", srv.GetName()))
	}
	_, err := parsePoolFunc(f)
	if err != nil {
		return errors.New(fmt.Sprintf("[Yunka] RegisterService: function is incorrect, %v : %s", f, err.Error()))
	}
	srvContainer := &serviceContainer{
		buildFunc: f,
	}
	srvContainer.pool = &sync.Pool{
		New: func() interface{} {
			return mod.buildService(srvContainer)
		},
	}

	mod.srvContainer[srv.GetName()] = srvContainer
	return nil
}

func (mod *module) buildService(srvC *serviceContainer) interface{} {
	values := reflect.ValueOf(srvC.buildFunc).Call([]reflect.Value{})
	newService := values[0].Interface()
	var (
		inits  []core.RuntimeInit
		infras []*core.ServiceItem
		repos  []*core.ServiceItem
	)
	baseSrv := newService.(core.RuntimeService)
	allFields(newService, func(fieldValue reflect.Value, fType reflect.Type) {
		// 配置 基础设施
		infra, ok := mod.getInfra(fType)
		if ok {
			infras = append(infras, &core.ServiceItem{
				Type: fType,
				Ptr:  fieldValue,
			})
			mod.PutInfra(fType, infra)
			return
		} else {
			// 配置资源
			repo, ok := mod.getRepo(fType)
			if ok {
				repos = append(repos, &core.ServiceItem{
					Type: fType,
					Ptr:  fieldValue,
				})
				mod.putRepo(fType, repo)
				return
			}
		}
	})

	baseSrv.RegisterInit(inits)
	baseSrv.RegisterSrvItems(infras, repos)

	return newService
}

func (mod *module) getService(name string, rt request.Runtime) (core.Service, error) {
	container, ok := mod.srvContainer[name]
	if !ok {
		return nil, nil
	}

	srv := container.pool.Get()
	if srv == nil {
		return nil, errors.New("not found service")
	}
	baseSrv := srv.(core.RuntimeService)
	inits := []core.RuntimeInit(nil)

	infras, repos := baseSrv.GetSrvItems()
	for _, inf := range infras {
		_infra, _ := mod.getInfra(inf.Type)
		if _infra.obj != nil {
			inf.Ptr.Set(reflect.ValueOf(_infra.obj))
			inf.Obj = _infra
			i := inf.Obj.(*infra)
			if i.runTimeInit != nil {
				inits = append(inits, i.runTimeInit)
			}
		}

	}

	for _, repo := range repos {
		_repo, _ := mod.getRepo(repo.Type)
		if _repo.obj != nil {
			repo.Ptr.Set(reflect.ValueOf(_repo.obj))
			repo.Obj = _repo
			inits = append(inits, _repo.runtime...)
		}

	}

	for idx, _ := range inits {
		if err := inits[idx].Init(rt); err != nil {
			return nil, err
		}
	}
	return baseSrv, nil
}

func (mod *module) putService(srv core.Service) {
	container, ok := mod.srvContainer[srv.GetName()]
	if !ok {
		return
	}
	baseSrv := srv.(core.RuntimeService)
	infras, repos := baseSrv.GetSrvItems()
	for _, repo := range repos {
		r := repo.Obj.(*repository)
		mod.putRepo(repo.Type, r)
		repo.Obj = nil
	}
	for _, inf := range infras {
		mod.PutInfra(inf.Type, inf.Obj.(*infra).obj)
		inf.Obj = nil
	}
	container.pool.Put(srv)
}
