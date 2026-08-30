package runtimecomponent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	grpcgo "google.golang.org/grpc"
	"yunka.io/framework/core"
)

type GRPCOptions struct {
	Name     string
	Server   *grpcgo.Server
	Listener net.Listener
}

func GRPC(options GRPCOptions) (core.RuntimeComponent, error) {
	name := strings.TrimSpace(options.Name)
	if name == "" {
		return core.RuntimeComponent{}, errors.New("runtimecomponent: gRPC component name is required")
	}
	if options.Server == nil {
		return core.RuntimeComponent{}, errors.New("runtimecomponent: gRPC server is required")
	}
	if options.Listener == nil {
		return core.RuntimeComponent{}, errors.New("runtimecomponent: gRPC listener is required")
	}
	runtime := &grpcRuntime{name: name, server: options.Server, listener: options.Listener}
	return core.RuntimeComponent{Name: name, StartFunc: runtime.start, HealthFunc: runtime.health, ShutdownFunc: runtime.shutdown}, nil
}

type grpcRuntime struct {
	name     string
	server   *grpcgo.Server
	listener net.Listener

	mu       sync.RWMutex
	started  bool
	stopping bool
	exited   bool
	serveErr error
	done     chan struct{}
}

func (runtime *grpcRuntime) start(context.Context) error {
	runtime.mu.Lock()
	if runtime.started {
		runtime.mu.Unlock()
		return nil
	}
	if runtime.stopping {
		runtime.mu.Unlock()
		return fmt.Errorf("runtimecomponent: gRPC %s is stopping", runtime.name)
	}
	runtime.started = true
	runtime.done = make(chan struct{})
	runtime.mu.Unlock()

	go func() {
		err := runtime.server.Serve(runtime.listener)
		if errors.Is(err, grpcgo.ErrServerStopped) || errors.Is(err, net.ErrClosed) {
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

func (runtime *grpcRuntime) health(context.Context) error {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	if !runtime.started {
		return fmt.Errorf("runtimecomponent: gRPC %s has not started", runtime.name)
	}
	if runtime.exited && !runtime.stopping {
		if runtime.serveErr != nil {
			return fmt.Errorf("runtimecomponent: gRPC %s exited: %w", runtime.name, runtime.serveErr)
		}
		return fmt.Errorf("runtimecomponent: gRPC %s exited unexpectedly", runtime.name)
	}
	return nil
}

func (runtime *grpcRuntime) shutdown(ctx context.Context) error {
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

	if !started {
		runtime.server.Stop()
		if err := runtime.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("runtimecomponent: close gRPC %s listener: %w", runtime.name, err)
		}
		return nil
	}

	gracefulDone := make(chan struct{})
	go func() {
		runtime.server.GracefulStop()
		close(gracefulDone)
	}()

	select {
	case <-gracefulDone:
		var failures []error
		if err := runtime.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			failures = append(failures, fmt.Errorf("runtimecomponent: close gRPC %s listener: %w", runtime.name, err))
		}
		if err := waitRuntimeDone(ctx, done); err != nil {
			failures = append(failures, err)
		}
		return errors.Join(failures...)
	case <-ctx.Done():
		runtime.server.Stop()
		_ = runtime.listener.Close()
		return ctx.Err()
	}
}
