package ingress

import (
	"github.com/valyala/fasthttp"
	"sync"
	"yunka.io/framework/core/request"
	"yunka.io/pkg/logExt"
)

/**
 * @BelongProject xirang
 * @BelongPackage proxy
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 1:15 下午
 * @Version V1.0
 */

type Proxy struct {
	pool    *sync.Pool
	ctxPool *sync.Pool
	logFn   func() logExt.Logger
	exec    Executor
	server  *fasthttp.Server
	mu      sync.Mutex
}

func NewProxy(logFn func() logExt.Logger, exec Executor) *Proxy {
	p := &Proxy{
		pool: &sync.Pool{
			New: func() interface{} {
				return &multiContext{}
			},
		},
		ctxPool: &sync.Pool{
			New: func() interface{} {
				return request.NewWorkRuntime()
			},
		},
		exec:  exec,
		logFn: logFn,
	}
	return p
}

func (p *Proxy) Run(address string) {
	p.logFn().Debugf("start server:%s", address)
	server := &fasthttp.Server{Handler: p.serverHttp}
	p.mu.Lock()
	p.server = server
	p.mu.Unlock()
	err := server.ListenAndServe(address)
	if err != nil {
		p.logFn().Errorf("start serve error, err:%v", err)
	}
}

func (p *Proxy) Stop() {
	p.mu.Lock()
	server := p.server
	p.server = nil
	p.mu.Unlock()
	if server != nil {
		if err := server.Shutdown(); err != nil {
			p.logFn().Errorf("stop serve error, err:%v", err)
		}
	}
}
