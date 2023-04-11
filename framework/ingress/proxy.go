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
	err := fasthttp.ListenAndServe(address, p.serverHttp)
	if err != nil {
		p.logFn().Errorf("start serve error, err:%v", err)
	}
}

func (p *Proxy) Stop() {
	// TODO graceful stop
}
