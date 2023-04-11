package module

import (
	"fmt"
	"github.com/pkg/errors"
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
 * @Date:  2020/12/5 9:30 下午
 * @Version V1.0
 */

type repository struct {
	rType   reflect.Type // 测试使用
	obj     core.Repository
	runtime []core.RuntimeInit
	infras  []*infra
}

var (
	_repoType = reflect.TypeOf(new(core.Repository)).Elem()
)

func (mod *module) BindRepository(f interface{}) error {
	outType, err := parsePoolFunc(f)
	if err != nil {
		return err
	}

	if !outType.Implements(_repoType) {
		return errors.New(fmt.Sprintf("info :%v must implement core.Repository", outType))
	}

	if mod.repos == nil {
		return errors.New(fmt.Sprintf("not found %v", outType))
	}

	_, ok := mod.repos[outType]
	if ok {
		return errors.New(fmt.Sprintf("info :%v exist", outType))
	}

	mod.repos[outType] = &sync.Pool{
		New: func() interface{} {
			values := reflect.ValueOf(f).Call([]reflect.Value{})
			repo := values[0].Interface()
			iRepo := repo.(core.Repository)
			repository := &repository{
				rType: outType,
				obj:   iRepo,
			}
			init, ok := repo.(core.RuntimeInit)
			if ok {
				repository.runtime = append(repository.runtime, init)
			}

			allFields(repo, func(value reflect.Value, r reflect.Type) {
				infra, ok := mod.getInfra(r)
				if ok {
					if infra.runTimeInit != nil {
						repository.runtime = append(repository.runtime, infra.runTimeInit)
					}
					repository.infras = append(repository.infras, infra)
					value.Set(reflect.ValueOf(infra.obj))
				}

			})
			iRepo.SetRepo(repository)
			return repository
		},
	}
	return nil
}

func (mod *module) GetRepo(rt request.Runtime, rType reflect.Type) core.Repository {
	repo, _ := mod.getRepo(rType)
	if repo == nil {
		return nil
	}
	for _, is := range repo.runtime {
		if err := is.Init(rt); err != nil {
			return nil
		}
	}
	return repo.obj
}

func (mod *module) PutRepo(rType reflect.Type, repo core.Repository) {
	if rType.Kind() != reflect.Interface {
		pool, ok := mod.repos[rType]
		if ok {
			pool.Put(repo.GetRepo())
		}
	} else {
		for iType, p := range mod.repos {
			if !iType.Implements(rType) {
				continue
			}
			rp := repo.GetRepo().(*repository)
			objType := reflect.TypeOf(rp.obj)
			if !objType.Implements(rType) {
			}
			p.Put(repo.GetRepo())
		}
	}
}

func (mod *module) putRepo(rType reflect.Type, repo *repository) {
	if rType.Kind() != reflect.Interface {
		pool, ok := mod.repos[rType]
		if ok {
			pool.Put(repo)
		}
	} else {
		for iType, p := range mod.repos {
			if !iType.Implements(rType) {
				continue
			}
			p.Put(repo)
		}
	}
}
