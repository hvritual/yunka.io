// Package runtimehost provides the canonical process-level runtime host for Yunka applications.
//
// It owns transport listeners, HTTP/gRPC runtime components, health and diagnostics
// endpoints, and failure cleanup. Business execution remains consumer-owned and is
// supplied through a typed Bootstrap callback that normally delegates to generated
// assembly code.
package runtimehost

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	grpcgo "google.golang.org/grpc"
	"yunka.io/framework/core"
	"yunka.io/framework/diagnostics"
	"yunka.io/framework/kernel"
	"yunka.io/framework/runtimecomponent"
)

const (
	DefaultHealthPath      = "/healthz"
	DefaultDiagnosticsPath = "/__yunka/diagnostics"
)

// HTTPMiddleware wraps only generated/business HTTP routes. Host-owned health
// and diagnostics endpoints are intentionally outside consumer middleware.
type HTTPMiddleware func(http.Handler) http.Handler

// Runtime contains the process-level resources that generated assembly code
// needs in order to register transports and attach them to core.App lifecycle.
type Runtime struct {
	HTTP              *http.ServeMux
	RPC               grpcgo.ServiceRegistrar
	RuntimeComponents []core.RuntimeComponent
}

// BootstrapFunc bridges the canonical host to consumer-generated typed assembly.
// The returned App remains the only lifecycle owner.
type BootstrapFunc[Applications any] func(context.Context, Runtime) (kernel.BootstrapResult[Applications], error)

// Options contains only process-level hosting facts. Business authentication,
// authorization, repositories and Application factories belong in Bootstrap.
type Options[Applications any] struct {
	HTTPListenAddress string
	GRPCListenAddress string
	HTTPMiddleware    HTTPMiddleware
	GRPCServerOptions []grpcgo.ServerOption
	HealthPath        string
	DiagnosticsPath   string
	Diagnostics       diagnostics.HTTPOptions
	Bootstrap         BootstrapFunc[Applications]
}

// Started exposes the canonical core.App lifecycle owner, the typed generated
// Application set and the resolved transport addresses. It intentionally has no
// Start/Shutdown methods of its own.
type Started[Applications any] struct {
	App          *core.App
	Applications Applications
	HTTPAddress  string
	GRPCAddress  string
}

// Bootstrap creates process transports and delegates typed application assembly
// to the supplied callback. The generated/kernel bootstrap starts core.App, which
// in turn starts and owns the host-created runtime components.
func Bootstrap[Applications any](ctx context.Context, options Options[Applications]) (Started[Applications], error) {
	var zero Started[Applications]
	if ctx == nil {
		ctx = context.Background()
	}
	if options.Bootstrap == nil {
		return zero, errors.New("runtimehost: bootstrap callback is required")
	}
	httpAddress := strings.TrimSpace(options.HTTPListenAddress)
	grpcAddress := strings.TrimSpace(options.GRPCListenAddress)
	if httpAddress == "" {
		return zero, errors.New("runtimehost: HTTP listen address is required")
	}
	if grpcAddress == "" {
		return zero, errors.New("runtimehost: gRPC listen address is required")
	}
	healthPath := normalizePath(options.HealthPath, DefaultHealthPath)
	diagnosticsPath := normalizePath(options.DiagnosticsPath, DefaultDiagnosticsPath)
	if healthPath == diagnosticsPath {
		return zero, errors.New("runtimehost: health and diagnostics paths must be distinct")
	}

	httpListener, err := net.Listen("tcp", httpAddress)
	if err != nil {
		return zero, fmt.Errorf("runtimehost: HTTP listen: %w", err)
	}
	grpcListener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		_ = httpListener.Close()
		return zero, fmt.Errorf("runtimehost: gRPC listen: %w", err)
	}
	ownedByApp := false
	defer func() {
		if !ownedByApp {
			_ = httpListener.Close()
			_ = grpcListener.Close()
		}
	}()

	apiMux := http.NewServeMux()
	health := &lateHealth{}
	diagnostic := &lateDiagnostics{}
	rootMux := http.NewServeMux()
	rootMux.Handle("GET "+healthPath, health)
	rootMux.Handle("GET "+diagnosticsPath, diagnostic)
	apiHandler := http.Handler(apiMux)
	if options.HTTPMiddleware != nil {
		apiHandler = options.HTTPMiddleware(apiHandler)
		if apiHandler == nil {
			return zero, errors.New("runtimehost: HTTP middleware returned nil handler")
		}
	}
	rootMux.Handle("/", apiHandler)

	httpServer := &http.Server{Handler: rootMux, ReadHeaderTimeout: 5 * time.Second}
	grpcServer := grpcgo.NewServer(options.GRPCServerOptions...)
	httpComponent, err := runtimecomponent.HTTP(runtimecomponent.HTTPOptions{
		Name: "http-server", Server: httpServer, Listener: httpListener,
	})
	if err != nil {
		return zero, err
	}
	grpcComponent, err := runtimecomponent.GRPC(runtimecomponent.GRPCOptions{
		Name: "grpc-server", Server: grpcServer, Listener: grpcListener,
	})
	if err != nil {
		return zero, err
	}

	result, err := options.Bootstrap(ctx, Runtime{
		HTTP:              apiMux,
		RPC:               grpcServer,
		RuntimeComponents: []core.RuntimeComponent{httpComponent, grpcComponent},
	})
	if err != nil {
		return zero, fmt.Errorf("runtimehost: bootstrap application: %w", err)
	}
	if result.App == nil {
		return zero, errors.New("runtimehost: bootstrap returned nil App")
	}
	ownedByApp = true
	health.set(result.App)
	if err := diagnostic.set(result.App, options.Diagnostics); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), core.DefaultShutdownTimeout)
		defer cancel()
		return zero, errors.Join(fmt.Errorf("runtimehost: diagnostics: %w", err), result.App.Shutdown(shutdownCtx))
	}

	return Started[Applications]{
		App:          result.App,
		Applications: result.Applications,
		HTTPAddress:  httpListener.Addr().String(),
		GRPCAddress:  grpcListener.Addr().String(),
	}, nil
}

func normalizePath(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if !strings.HasPrefix(value, "/") {
		return "/" + value
	}
	return value
}

type lateHealth struct {
	mu  sync.RWMutex
	app *core.App
}

func (current *lateHealth) set(app *core.App) {
	current.mu.Lock()
	current.app = app
	current.mu.Unlock()
}

func (current *lateHealth) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	current.mu.RLock()
	app := current.app
	current.mu.RUnlock()
	writer.Header().Set("Cache-Control", "no-store")
	if app == nil || !app.Health(request.Context()).Ready {
		http.Error(writer, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = writer.Write([]byte(`{"status":"ok"}`))
}

type lateDiagnostics struct {
	mu      sync.RWMutex
	handler http.Handler
}

func (current *lateDiagnostics) set(app *core.App, options diagnostics.HTTPOptions) error {
	collector, err := diagnostics.New(app)
	if err != nil {
		return err
	}
	handler, err := diagnostics.NewHTTPHandler(collector, options)
	if err != nil {
		return err
	}
	current.mu.Lock()
	current.handler = handler
	current.mu.Unlock()
	return nil
}

func (current *lateDiagnostics) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	current.mu.RLock()
	handler := current.handler
	current.mu.RUnlock()
	if handler == nil {
		http.Error(writer, "diagnostics unavailable", http.StatusServiceUnavailable)
		return
	}
	handler.ServeHTTP(writer, request)
}
