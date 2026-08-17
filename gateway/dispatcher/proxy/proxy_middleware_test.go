package proxy

import (
	"fmt"
	"testing"
	"yunka.io/framework/core/request"
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
}

func (t *TestHandle) Name() string {
	return t.HandlerName
}

func (t *TestHandle) Use(middle MiddleWare) MiddleWare {
	t.m = middle
	return t.m
}

func (t *TestHandle) Do(c request.Runtime, api *meta.RuntimeApi) {
	fmt.Println(t.HandlerName, " is call")
	if t.m != nil {
		fmt.Println(t.HandlerName, "next is call pre")
		t.m.Do(c, api)
		fmt.Println(t.HandlerName, "next is call post")
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

func (t Test4IntHandle) Do(c request.Runtime, api *meta.RuntimeApi) error {
	fmt.Println(t.HandlerName, " interrupt call")

	return nil
}

type Test4Handle struct {
	TestHandle
}

type Test5Handle struct {
	TestHandle
}

func TestProxy_Use(t *testing.T) {
	p := NewProxy(nil)

	p.Use(&Test1Handle{TestHandle{HandlerName: "1"}}).
		Use(&Test2Handle{TestHandle{HandlerName: "2"}}).
		Use(&Test3Handle{TestHandle{HandlerName: "3"}}).
		Use(&Test4Handle{TestHandle{HandlerName: "4"}}).
		Use(&Test5Handle{TestHandle{HandlerName: "5"}})

	p.middles.Do(nil, nil)

	p.Use(&Test1Handle{TestHandle{HandlerName: "1"}}).
		Use(&Test2Handle{TestHandle{HandlerName: "2"}}).
		Use(&Test3Handle{TestHandle{HandlerName: "3"}}).
		Use(&Test4IntHandle{TestHandle{HandlerName: "4"}}).
		Use(&Test5Handle{TestHandle{HandlerName: "5"}})

	p.middles.Do(nil, nil)

}
