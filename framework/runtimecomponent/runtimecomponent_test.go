package runtimecomponent

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
	"yunka.io/framework/core"
)

func TestHTTPComponentRunsThroughCoreAppLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	component, err := HTTP(HTTPOptions{Name: "http", Server: &http.Server{Handler: mux}, Listener: listener})
	if err != nil {
		t.Fatal(err)
	}
	app, err := core.NewApp(core.AppOptions{RuntimeComponents: []core.RuntimeComponent{component}, RuntimeInventory: core.RuntimeInventory{Routes: []string{"/ready"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + listener.Addr().String() + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d want=%d", response.StatusCode, http.StatusNoContent)
	}
	if report := app.Health(context.Background()); !report.Ready {
		t.Fatalf("HTTP runtime did not report ready: %#v", report)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCComponentRunsThroughCoreAppLifecycle(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpcgo.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	component, err := GRPC(GRPCOptions{Name: "grpc", Server: server, Listener: listener})
	if err != nil {
		t.Fatal(err)
	}
	app, err := core.NewApp(core.AppOptions{RuntimeComponents: []core.RuntimeComponent{component}, RuntimeInventory: core.RuntimeInventory{RPCServerCount: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	connection, err := grpcgo.DialContext(ctx, "bufnet",
		grpcgo.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpcgo.WithTransportCredentials(insecure.NewCredentials()),
		grpcgo.WithBlock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	response, err := healthpb.NewHealthClient(connection).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health status=%s", response.Status)
	}
	if report := app.Health(context.Background()); !report.Ready {
		t.Fatalf("gRPC runtime did not report ready: %#v", report)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestTransportComponentsRequireExplicitServersListenersAndNames(t *testing.T) {
	listener := bufconn.Listen(1024)
	defer listener.Close()
	if _, err := HTTP(HTTPOptions{}); err == nil {
		t.Fatal("HTTP accepted missing explicit runtime inputs")
	}
	if _, err := GRPC(GRPCOptions{}); err == nil {
		t.Fatal("gRPC accepted missing explicit runtime inputs")
	}
	if _, err := GRPC(GRPCOptions{Name: "grpc", Server: grpcgo.NewServer(), Listener: listener}); err != nil {
		t.Fatalf("valid explicit gRPC runtime rejected: %v", err)
	}
}
