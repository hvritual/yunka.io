package proxy

import (
	"github.com/valyala/fasthttp"
	"net"
	"sync"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/router"
	"yunka.io/pkg/logExt"
)

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

type Proxy struct {
	tree    *router.Tree
	pool    *sync.Pool
	ctxPool *sync.Pool
	logFn   func() logExt.Logger
	middles Next
}

func NewProxy(logFn func() logExt.Logger, fn func(rt *router.Tree)) *Proxy {
	p := &Proxy{
		pool: &sync.Pool{
			New: func() interface{} {
				return &multiContext{}
			},
		},
		tree: router.New(),
		ctxPool: &sync.Pool{
			New: func() interface{} {
				return request.NewWorkRuntime()
			},
		},
		logFn: logFn,
	}
	fn(p.tree)
	return p
}

func (p *Proxy) UriTree() *router.Tree {
	return p.tree
}

func (p *Proxy) Run(address string) {
	p.logFn().Debugf("start server:%s", address)
	err := fasthttp.ListenAndServe(address, p.serverHttp)
	if err != nil {
		p.logFn().Errorf("start serve error, err:%v", err)
	}
}

func (p *Proxy) RunLn(ln net.Listener) {
	s := &fasthttp.Server{
		Handler: p.serverHttp,
	}
	err := s.Serve(ln)
	if err != nil {
		p.logFn().Errorf("start serve error, err:%v", err)
	}
}

func (p *Proxy) Stop() {
	// TODO graceful stop
}
