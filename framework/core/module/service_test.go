package module

import (
	"fmt"
	"sync"
	"testing"
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
 * @Date:  2020/12/6 5:22 下午
 * @Version V1.0
 */

type TestService struct {
	core.BaseService
	Ti   *TestInfra
	Tri  *TestRuntimeInfra
	Repo *TestRepo
	Test Test
}

func (ts *TestService) GetName() string {
	return "test_service"
}

func (ts *TestService) Say(val string) {
	fmt.Println(ts.GetRuntime(), val)
}

type (
	Test interface {
		Test(string)
	}
)

func (t *TestRepo) Init(rt request.Runtime) {
	fmt.Println("init repo", rt)
}

func (t *TestRepo) Test(val string) {
	fmt.Println(" repo ", val)
}

func TestBaseService_GetInfras(t *testing.T) {
	mod := NewModule("test_module", func(mod core.Module) {

	})
	for _, err := range []error{
		mod.BindRepository(func() *TestRepo {
			return &TestRepo{}
		}),
		mod.BindRepository(func() *Test2Repo {
			return &Test2Repo{}
		}),
		mod.BindInfra(false, func() *TestInfra {
			return &TestInfra{}
		}),
		mod.BindInfra(true, func() *TestRuntimeInfra {
			return &TestRuntimeInfra{}
		}),
		mod.BindService(&TestService{}, func() *TestService {
			return &TestService{}
		}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			srv, _ := mod.GetService("test_service", &request.WorkRuntime{
				IsDirect: true,
			})

			if srv == nil {
				t.Fatal("service is empty")
			}
			srv.(*TestService).Say("hello")

			if srv.(*TestService).Test == nil {
				t.Fatal("reject object fail, test is nil")
			}

			mod.PutService(srv)
			wg.Done()
		}()
	}

	wg.Wait()

	//if srv == nil {
	//	t.Fatal("service is empty")
	//}
	//srv.(*TestService).Say("world")

}
