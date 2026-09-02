package proxy

import (
	"context"

	"github.com/valyala/fasthttp"
	"github.com/hvritual/yunka.io/framework/observability"
	"github.com/hvritual/yunka.io/pkg/logExt"
)

// UseObservability installs the unified W4 middleware around gateway requests.
// It is opt-in and leaves existing services unchanged until a provider is
// explicitly configured.
func (p *Proxy) UseObservability(provider *observability.Provider) *Proxy {
	if provider == nil {
		return p
	}
	p.contextLogFn = func(ctx context.Context) logExt.Logger {
		return provider.LegacyLogger(ctx)
	}
	return p.UseRuntime(provider.Middleware())
}

type fastHTTPHeaderCarrier struct {
	header *fasthttp.RequestHeader
}

func (carrier fastHTTPHeaderCarrier) Get(key string) string {
	if carrier.header == nil {
		return ""
	}
	return string(carrier.header.Peek(key))
}

func (carrier fastHTTPHeaderCarrier) Set(key, value string) {
	if carrier.header != nil {
		carrier.header.Set(key, value)
	}
}

func (carrier fastHTTPHeaderCarrier) Keys() []string {
	if carrier.header == nil {
		return nil
	}
	keys := make([]string, 0, carrier.header.Len())
	carrier.header.VisitAll(func(key, _ []byte) {
		keys = append(keys, string(key))
	})
	return keys
}
