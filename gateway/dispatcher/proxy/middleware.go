package proxy

import (
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

/**
 * @BelongProject yunka
 * @BelongPackage proxy
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 1:38 下午
 * @Version V1.0
 */

var (
	_ MiddleWare = (*Next)(nil)
)

type MiddleWare interface {
	Name() string

	Use(middle MiddleWare) MiddleWare

	Do(bool, *request.Context, *meta.RuntimeApi)
}

type Next struct {
	next MiddleWare
}

func (m *Next) Use(middle MiddleWare) MiddleWare {
	m.next = middle
	return m.next
}

func (m *Next) Name() string {
	return "yunka"
}

func (m *Next) Do(authStatus bool, ctx *request.Context, api *meta.RuntimeApi) {
	if m.next != nil {
		m.next.Do(authStatus, ctx, api)
	}
	return
}
