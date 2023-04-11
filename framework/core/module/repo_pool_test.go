package module

import (
	"reflect"
	"sync"
	"testing"
	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
)

/**
 * @BelongProject yunka
 * @BelongPackage module
 * @Description:
 *
 * @Copyright 2021 - Powered By 云咖
 * @Author: fworld
 * @Date:  2021/4/11 下午10:14
 * @Version V1.0
 */

type (
	TestRepo struct {
		rt request.Runtime
		core.BaseRepository
		Ti *TestInfra
		//Tri *TestRuntimeInfra
	}

	Test2Repo struct {
		rt request.Runtime
		core.BaseRepository
		Tri *TestRuntimeInfra
	}
)

func Test_module_getRepo(t *testing.T) {
	module := NewModule(`test`, nil)
	err := module.BindInfra(false, func() *TestInfra {
		return &TestInfra{}
	})

	if err != nil {
		t.Fatal(err)
	}

	err = module.BindRepository(func() *TestRepo {
		return &TestRepo{}
	})
	if err != nil {
		t.Fatal(err)
	}

	iType := reflect.TypeOf(&TestRepo{})
	wg := sync.WaitGroup{}
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			repository, ok := module.getRepo(iType)
			if ok {
				module.PutRepo(iType, repository.obj)
			}
			wg.Done()
		}()
	}

	wg.Wait()

}
