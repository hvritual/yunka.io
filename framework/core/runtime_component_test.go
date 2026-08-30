package core

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func runtimeComponentRecorder(name string, events *[]string, mu *sync.Mutex, startErr, healthErr error) RuntimeComponent {
	record := func(value string) {
		mu.Lock()
		*events = append(*events, value)
		mu.Unlock()
	}
	return RuntimeComponent{
		Name: name,
		StartFunc: func(context.Context) error {
			record("start:" + name)
			return startErr
		},
		HealthFunc: func(context.Context) error {
			record("health:" + name)
			return healthErr
		},
		ShutdownFunc: func(context.Context) error {
			record("shutdown:" + name)
			return nil
		},
	}
}

func TestAppRuntimeComponentsAreDeterministicHealthyAndReverseShutdown(t *testing.T) {
	var events []string
	mu := &sync.Mutex{}
	app, err := NewApp(AppOptions{
		RuntimeComponents: []RuntimeComponent{
			runtimeComponentRecorder("http", &events, mu, nil, nil),
			runtimeComponentRecorder("grpc", &events, mu, nil, nil),
		},
		RuntimeInventory: RuntimeInventory{
			Routes: []string{"/v1/z", "/v1/a", "/v1/z"}, RPCClientConfigured: true, RPCServerCount: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	report := app.Health(context.Background())
	if !report.Live || !report.Ready || len(report.Checks) != 2 {
		t.Fatalf("unexpected health report: %#v", report)
	}
	diagnostics := app.Diagnostics(context.Background())
	if got, want := diagnostics.Routes, []string{"/v1/a", "/v1/z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("routes=%v want=%v", got, want)
	}
	if diagnostics.Runtime.RouteCount != 2 || !diagnostics.Runtime.RPCClientConfigured || diagnostics.Runtime.RPCServerCount != 1 {
		t.Fatalf("unexpected runtime inventory: %#v", diagnostics.Runtime)
	}
	if len(diagnostics.Components) != 2 || diagnostics.Components[0].Name != "grpc" || diagnostics.Components[1].Name != "http" {
		t.Fatalf("unexpected component diagnostics: %#v", diagnostics.Components)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantEvents := []string{
		"start:grpc", "start:http",
		"health:grpc", "health:http",
		"health:grpc", "health:http",
		"shutdown:http", "shutdown:grpc",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events=%#v want=%#v", events, wantEvents)
	}
}

func TestAppRuntimeComponentHealthBlocksReadiness(t *testing.T) {
	var events []string
	mu := &sync.Mutex{}
	app, err := NewApp(AppOptions{RuntimeComponents: []RuntimeComponent{
		runtimeComponentRecorder("grpc", &events, mu, nil, errors.New("not serving")),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	report := app.Health(context.Background())
	if report.Ready || len(report.Checks) != 1 || report.Checks[0].Status != HealthStatusUnhealthy {
		t.Fatalf("unhealthy runtime component did not block readiness: %#v", report)
	}
}

func TestAppRuntimeComponentStartFailureRollsBackComponents(t *testing.T) {
	var events []string
	mu := &sync.Mutex{}
	app, err := NewApp(AppOptions{RuntimeComponents: []RuntimeComponent{
		runtimeComponentRecorder("a", &events, mu, nil, nil),
		runtimeComponentRecorder("b", &events, mu, errors.New("bind failed"), nil),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(context.Background()); err == nil {
		t.Fatal("Start() error=nil, want failure")
	}
	if app.State() != AppStateFailed {
		t.Fatalf("state=%s want failed", app.State())
	}
	want := []string{"start:a", "start:b", "shutdown:b", "shutdown:a"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%#v want=%#v", events, want)
	}
}

func TestNewAppRejectsInvalidRuntimeComponentContract(t *testing.T) {
	valid := RuntimeComponent{Name: "http", StartFunc: func(context.Context) error { return nil }, ShutdownFunc: func(context.Context) error { return nil }}
	cases := []AppOptions{
		{RuntimeComponents: []RuntimeComponent{{Name: "", StartFunc: valid.StartFunc, ShutdownFunc: valid.ShutdownFunc}}},
		{RuntimeComponents: []RuntimeComponent{{Name: "http", ShutdownFunc: valid.ShutdownFunc}}},
		{RuntimeComponents: []RuntimeComponent{{Name: "http", StartFunc: valid.StartFunc}}},
		{RuntimeComponents: []RuntimeComponent{valid, valid}},
		{RuntimeInventory: RuntimeInventory{RPCServerCount: -1}},
	}
	for index, options := range cases {
		if _, err := NewApp(options); err == nil {
			t.Fatalf("case %d accepted invalid runtime contract", index)
		}
	}
}
