package runtimecomponent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/hvritual/yunka.io/framework/core"
)

type HTTPOptions struct {
	Name     string
	Server   *http.Server
	Listener net.Listener
}

func HTTP(options HTTPOptions) (core.RuntimeComponent, error) {
	name := strings.TrimSpace(options.Name)
	if name == "" {
		return core.RuntimeComponent{}, errors.New("runtimecomponent: HTTP component name is required")
	}
	if options.Server == nil {
		return core.RuntimeComponent{}, errors.New("runtimecomponent: HTTP server is required")
	}
	if options.Listener == nil {
		return core.RuntimeComponent{}, errors.New("runtimecomponent: HTTP listener is required")
	}
	runtime := &httpRuntime{name: name, server: options.Server, listener: options.Listener}
	return core.RuntimeComponent{Name: name, StartFunc: runtime.start, HealthFunc: runtime.health, ShutdownFunc: runtime.shutdown}, nil
}

type httpRuntime struct {
	name     string
	server   *http.Server
	listener net.Listener

	mu       sync.RWMutex
	started  bool
	stopping bool
	exited   bool
	serveErr error
	done     chan struct{}
}

func (runtime *httpRuntime) start(context.Context) error {
	runtime.mu.Lock()
	if runtime.started {
		runtime.mu.Unlock()
		return nil
	}
	if runtime.stopping {
		runtime.mu.Unlock()
		return fmt.Errorf("runtimecomponent: HTTP %s is stopping", runtime.name)
	}
	runtime.started = true
	runtime.done = make(chan struct{})
	runtime.mu.Unlock()

	go func() {
		err := runtime.server.Serve(runtime.listener)
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			err = nil
		}
		runtime.mu.Lock()
		runtime.exited = true
		runtime.serveErr = err
		close(runtime.done)
		runtime.mu.Unlock()
	}()
	return nil
}

func (runtime *httpRuntime) health(context.Context) error {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	if !runtime.started {
		return fmt.Errorf("runtimecomponent: HTTP %s has not started", runtime.name)
	}
	if runtime.exited && !runtime.stopping {
		if runtime.serveErr != nil {
			return fmt.Errorf("runtimecomponent: HTTP %s exited: %w", runtime.name, runtime.serveErr)
		}
		return fmt.Errorf("runtimecomponent: HTTP %s exited unexpectedly", runtime.name)
	}
	return nil
}

func (runtime *httpRuntime) shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	runtime.mu.Lock()
	if runtime.stopping {
		done := runtime.done
		runtime.mu.Unlock()
		return waitRuntimeDone(ctx, done)
	}
	runtime.stopping = true
	started := runtime.started
	done := runtime.done
	runtime.mu.Unlock()

	var failures []error
	if started {
		if err := runtime.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failures = append(failures, fmt.Errorf("runtimecomponent: shutdown HTTP %s: %w", runtime.name, err))
		}
	}
	if err := runtime.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		failures = append(failures, fmt.Errorf("runtimecomponent: close HTTP %s listener: %w", runtime.name, err))
	}
	if started {
		if err := waitRuntimeDone(ctx, done); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func waitRuntimeDone(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
