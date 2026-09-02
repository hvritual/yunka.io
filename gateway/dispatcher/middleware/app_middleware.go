package middleware

import (
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/dispatcher/proxy"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

/**
 * @BelongProject yunka
 * @BelongPackage middleware
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 5:45 下午
 * @Version V1.0
 */

const (
	testName = `test`
)

type TestMiddleware struct {
	proxy.Next
}

func (t TestMiddleware) Name() string {
	return testName
}

func (t TestMiddleware) Do(b bool, rt *request.Context, api *meta.RuntimeApi) {
	t.Next.Do(true, rt, api)
}

func NewTestMiddleware() proxy.MiddleWare {
	return &TestMiddleware{}
}
