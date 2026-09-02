package runtimehost

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"yunka.io/framework/core/modulecatalog"
	"yunka.io/framework/kernel"
)

func TestBootstrapOwnsTransportLifecycleHealthAndDiagnostics(t *testing.T) {
	started, err := Bootstrap(context.Background(), Options[string]{
		HTTPListenAddress: "127.0.0.1:0",
		GRPCListenAddress: "127.0.0.1:0",
		HTTPMiddleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("X-Hosted-By", "yunka")
				next.ServeHTTP(writer, request)
			})
		},
		Bootstrap: func(ctx context.Context, runtime Runtime) (kernel.BootstrapResult[string], error) {
			if runtime.HTTP == nil || runtime.RPC == nil {
				t.Fatal("runtime transports are required")
			}
			if len(runtime.RuntimeComponents) != 2 {
				t.Fatalf("runtime components = %d, want 2", len(runtime.RuntimeComponents))
			}
			runtime.HTTP.HandleFunc("GET /v1/ping", func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte("pong"))
			})
			return kernel.Bootstrap(ctx, kernel.BootstrapOptions[string]{
				Kernel: kernel.Options{Catalog: modulecatalog.New(), RuntimeComponents: runtime.RuntimeComponents},
				Build: func() (string, error) { return "applications", nil },
				Register: func(string) error { return nil },
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := started.App.Shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	}()
	if started.App == nil || started.Applications != "applications" {
		t.Fatalf("unexpected started result: %#v", started)
	}
	if started.HTTPAddress == "" || started.GRPCAddress == "" {
		t.Fatalf("resolved addresses are required: %#v", started)
	}

	response, err := http.Get("http://" + started.HTTPAddress + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("health status=%d body=%q", response.StatusCode, body)
	}

	response, err = http.Get("http://" + started.HTTPAddress + "/__yunka/diagnostics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"schemaVersion":1`) {
		t.Fatalf("diagnostics status=%d body=%q", response.StatusCode, body)
	}

	response, err = http.Get("http://" + started.HTTPAddress + "/v1/ping")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "pong" {
		t.Fatalf("api status=%d body=%q", response.StatusCode, body)
	}
	if response.Header.Get("X-Hosted-By") != "yunka" {
		t.Fatalf("consumer API middleware was not applied")
	}
}

func TestBootstrapRejectsInvalidProcessFacts(t *testing.T) {
	bootstrap := func(context.Context, Runtime) (kernel.BootstrapResult[struct{}], error) {
		return kernel.BootstrapResult[struct{}]{}, nil
	}
	for name, options := range map[string]Options[struct{}]{
		"missing bootstrap": {HTTPListenAddress: "127.0.0.1:0", GRPCListenAddress: "127.0.0.1:0"},
		"missing http":      {GRPCListenAddress: "127.0.0.1:0", Bootstrap: bootstrap},
		"missing grpc":      {HTTPListenAddress: "127.0.0.1:0", Bootstrap: bootstrap},
		"duplicate paths":   {HTTPListenAddress: "127.0.0.1:0", GRPCListenAddress: "127.0.0.1:0", HealthPath: "/ready", DiagnosticsPath: "/ready", Bootstrap: bootstrap},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Bootstrap(context.Background(), options); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
