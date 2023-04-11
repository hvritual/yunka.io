package proxy

import "yunka.io/framework/core/request"

/**
 * @BelongProject yunka
 * @BelongPackage proxy
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 1:15 下午
 * @Version V1.0
 */

func (p *Proxy) AcquireMultiContext() *multiContext {
	m := p.pool.Get().(*multiContext)
	m.Init()
	return m
}

func (p *Proxy) PutMultiContext(m *multiContext) {
	p.pool.Put(m)
}

func (p *Proxy) acquireControllerContext() *request.WorkRuntime {
	return p.ctxPool.Get().(*request.WorkRuntime)
}

func (p *Proxy) putControllerContext(m *request.WorkRuntime) {
	m.Store().Reset()
	p.ctxPool.Put(m)
}
