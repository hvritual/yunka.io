package module

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"yunka.io/framework/core/request"
	log "yunka.io/pkg/logExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage module
 * @Description:
 *
 * @Copyright 2021 - Powered By 云咖
 * @Author: fworld
 * @Date:  2021/4/11 下午10:01
 * @Version V1.0
 */

type TestInfra struct {
}

type TestRuntimeInfra struct {
	rt request.Runtime
}

func (t *TestRuntimeInfra) Init(rt request.Runtime) {
	t.rt = rt

	fmt.Println("init runtime infra", rt)
}

func Test_module_BindSingleInfra(t *testing.T) {
	module := NewModule(`test`, nil)
	err := module.BindInfra(true, func() *TestInfra {
		return &TestInfra{}
	})

	if err != nil {
		log.Fatal(err)
	}

	iType := reflect.TypeOf(&TestInfra{})
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

func Test_module_BindInfra(t *testing.T) {
	module := NewModule(`test`, nil)
	err := module.BindInfra(false, func() *TestInfra {
		return &TestInfra{}
	})

	if err != nil {
		t.Fatal(err)
	}

	iType := reflect.TypeOf(&TestInfra{})
	wg := sync.WaitGroup{}
	errCh := make(chan error, 1000)
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repository, ok := module.getInfra(iType)
			if ok {
				module.PutInfra(iType, repository.obj)
			} else {
				errCh <- fmt.Errorf("not ok: %v", repository)
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}
