package proxy

import coremiddleware "yunka.io/framework/core/middleware"

/**
 * @BelongProject yunka
 * @BelongPackage proxy
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 1:44 下午
 * @Version V1.0
 */

// Use keeps the legacy gateway-specific middleware chain source-compatible.
func (p *Proxy) Use(h MiddleWare) MiddleWare {
	return p.middles.Use(h)
}

// UseRuntime adds transport-neutral middleware around the gateway request.
// The same middleware type can be reused by RPC, event and job transports.
func (p *Proxy) UseRuntime(middlewares ...coremiddleware.Middleware) *Proxy {
	p.runtimeMiddles = p.runtimeMiddles.Use(middlewares...)
	return p
}
