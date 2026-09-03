package qualification

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	generatedassembly "example.com/c103qualification/internal/assembly"

	"google.golang.org/grpc"
	"github.com/hvritual/yunka.io/framework/core"
	"github.com/hvritual/yunka.io/framework/operation"
)

func TestGeneratedAssemblyMissingTypedCapabilityFailsBeforeRegistrationOrStart(t *testing.T) {
	provider := qualificationCapabilityPlatform(t)
	mux := http.NewServeMux()
	grpcServer := grpc.NewServer()
	var starts atomic.Int32
	_, err := generatedassembly.Bootstrap(context.Background(), generatedassembly.BootstrapOptions{
		Platform:   provider,
		Factories:  capabilityFactories{probe: newRuntimeProbe()},
		Executor:   operation.NewExecutor(nil),
		Transports: generatedassembly.TransportBindings{HTTP: mux, RPC: grpcServer},
		RuntimeComponents: []core.RuntimeComponent{{
			Name:         "must-not-start",
			StartFunc:    func(context.Context) error { starts.Add(1); return nil },
			ShutdownFunc: func(context.Context) error { return nil },
		}},
	})
	if err == nil || !strings.Contains(err.Error(), `capability "cache.qualification" is not provided`) {
		t.Fatalf("missing capability bootstrap error=%v", err)
	}
	if starts.Load() != 0 {
		t.Fatalf("RuntimeComponent started after capability construction failure: starts=%d", starts.Load())
	}
	requestURL := &url.URL{Path: "/v1/devices:transfer"}
	_, pattern := mux.Handler(&http.Request{Method: http.MethodPost, URL: requestURL})
	if pattern != "" {
		t.Fatalf("HTTP transport registered before capability failure: pattern=%q", pattern)
	}
	if services := grpcServer.GetServiceInfo(); len(services) != 0 {
		t.Fatalf("gRPC transport registered before capability failure: services=%v", services)
	}
}
