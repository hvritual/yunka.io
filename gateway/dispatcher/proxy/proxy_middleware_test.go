package proxy

import (
	"reflect"
	"testing"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/router"
	"yunka.io/gateway/rpc/meta"
)

/**
 * @BelongProject yunka
 * @BelongPackage proxy
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 1:56 下午
 * @Version V1.0
 */

var (
	_ MiddleWare = (*TestHandle)(nil)
	_ MiddleWare = (*Test1Handle)(nil)
	_ MiddleWare = (*Test2Handle)(nil)
	_ MiddleWare = (*Test3Handle)(nil)
	_ MiddleWare = (*Test4Handle)(nil)
	_ MiddleWare = (*Test5Handle)(nil)
)

type TestHandle struct {
	HandlerName string
	m           MiddleWare
	calls       *[]string
}

func (t *TestHandle) Name() string {
	return t.HandlerName
}

func (t *TestHandle) Use(middle MiddleWare) MiddleWare {
	t.m = middle
	return t.m
}

func (t *TestHandle) Do(auth bool, c request.Runtime, api *meta.RuntimeApi) {
	*t.calls = append(*t.calls, t.HandlerName)
	if t.m != nil {
		t.m.Do(auth, c, api)
		return
	}
	return
}

type Test1Handle struct {
	TestHandle
}

type Test2Handle struct {
	TestHandle
}

type Test3Handle struct {
	TestHandle
}

type Test4IntHandle struct {
	TestHandle
}

func (t Test4IntHandle) Do(_ bool, _ request.Runtime, _ *meta.RuntimeApi) {
	*t.calls = append(*t.calls, t.HandlerName)
}

type Test4Handle struct {
	TestHandle
}

type Test5Handle struct {
	TestHandle
}

func TestProxy_Use(t *testing.T) {
	p := NewProxy(nil, func(*router.Tree) {})
	calls := make([]string, 0, 5)

	p.Use(&Test1Handle{TestHandle{HandlerName: "1", calls: &calls}}).
		Use(&Test2Handle{TestHandle{HandlerName: "2", calls: &calls}}).
		Use(&Test3Handle{TestHandle{HandlerName: "3", calls: &calls}}).
		Use(&Test4Handle{TestHandle{HandlerName: "4", calls: &calls}}).
		Use(&Test5Handle{TestHandle{HandlerName: "5", calls: &calls}})

	p.middles.Do(false, nil, nil)
	if want := []string{"1", "2", "3", "4", "5"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}

	calls = calls[:0]
	p = NewProxy(nil, func(*router.Tree) {})
	p.Use(&Test1Handle{TestHandle{HandlerName: "1", calls: &calls}}).
		Use(&Test2Handle{TestHandle{HandlerName: "2", calls: &calls}}).
		Use(&Test3Handle{TestHandle{HandlerName: "3", calls: &calls}}).
		Use(&Test4IntHandle{TestHandle{HandlerName: "4", calls: &calls}}).
		Use(&Test5Handle{TestHandle{HandlerName: "5", calls: &calls}})

	p.middles.Do(false, nil, nil)
	if want := []string{"1", "2", "3", "4"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("interrupt calls=%v want=%v", calls, want)
	}

}
