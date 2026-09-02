package proxy

import (
	"context"
	"net"
	"sync"

	"github.com/valyala/fasthttp"
	coremiddleware "github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/gateway/dispatcher/router"
	"github.com/hvritual/yunka.io/pkg/logExt"
)

// Proxy owns routing and middleware composition. HTTP request contexts are
// allocated per request and are never stored in a sync.Pool.
type Proxy struct {
	tree           *router.Tree
	logFn          func() logExt.Logger
	contextLogFn   func(context.Context) logExt.Logger
	middles        Next
	runtimeMiddles coremiddleware.Chain
	server         *fasthttp.Server
	mu             sync.Mutex
}

func NewProxy(logFn func() logExt.Logger, configure func(*router.Tree)) *Proxy {
	proxy := &Proxy{
		tree:           router.New(),
		logFn:          logFn,
		runtimeMiddles: coremiddleware.New(),
	}
	if configure != nil {
		configure(proxy.tree)
	}
	return proxy
}

func (proxy *Proxy) UriTree() *router.Tree { return proxy.tree }

func (proxy *Proxy) Run(address string) {
	if logger := proxy.logger(); logger != nil {
		logger.Debugf("start server:%s", address)
	}
	server := &fasthttp.Server{Handler: proxy.serverHTTP}
	proxy.setServer(server)
	if err := server.ListenAndServe(address); err != nil {
		if logger := proxy.logger(); logger != nil {
			logger.Errorf("start serve error, err:%v", err)
		}
	}
}

func (proxy *Proxy) RunLn(listener net.Listener) {
	server := &fasthttp.Server{Handler: proxy.serverHTTP}
	proxy.setServer(server)
	if err := server.Serve(listener); err != nil {
		if logger := proxy.logger(); logger != nil {
			logger.Errorf("start serve error, err:%v", err)
		}
	}
}

func (proxy *Proxy) Stop() {
	proxy.mu.Lock()
	server := proxy.server
	proxy.server = nil
	proxy.mu.Unlock()
	if server != nil {
		if err := server.Shutdown(); err != nil {
			if logger := proxy.logger(); logger != nil {
				logger.Errorf("stop serve error, err:%v", err)
			}
		}
	}
}

func (proxy *Proxy) setServer(server *fasthttp.Server) {
	proxy.mu.Lock()
	proxy.server = server
	proxy.mu.Unlock()
}

func (proxy *Proxy) logger() logExt.Logger {
	if proxy == nil || proxy.logFn == nil {
		return nil
	}
	return proxy.logFn()
}
