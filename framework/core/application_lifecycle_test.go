package core

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"yunka.io/framework/core/modulecatalog"
)

type lifecycleTestModule struct {
	name      string
	events    *[]string
	mu        *sync.Mutex
	startErr  error
	healthErr error
}

func (m *lifecycleTestModule) Name() string { return m.name }
func (m *lifecycleTestModule) record(event string) {
	m.mu.Lock()
	*m.events = append(*m.events, event)
	m.mu.Unlock()
}
func (m *lifecycleTestModule) Start(context.Context) error {
	m.record("start:" + m.name)
	return m.startErr
}
func (m *lifecycleTestModule) Shutdown(context.Context) error {
	m.record("shutdown:" + m.name)
	return nil
}
func (m *lifecycleTestModule) Health(context.Context) error {
	m.record("health:" + m.name)
	return m.healthErr
}

func lifecycleApplication(t *testing.T, modules ...*lifecycleTestModule) *App {
	t.Helper()
	catalog := modulecatalog.New()
	for _, module := range modules {
		module := module
		catalog.MustRegister(modulecatalog.Descriptor{
			Name:  module.name,
			Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) { return module, nil },
		})
	}
	app, err := NewApp(AppOptions{Catalog: catalog, ContextFactory: modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{})})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestAppLifecycleOrderAndHealth(t *testing.T) {
	events := make([]string, 0)
	mu := &sync.Mutex{}
	app := lifecycleApplication(t,
		&lifecycleTestModule{name: "first", events: &events, mu: mu},
		&lifecycleTestModule{name: "second", events: &events, mu: mu},
	)

	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if app.State() != AppStateReady {
		t.Fatalf("State() = %s, want ready", app.State())
	}
	report := app.Health(context.Background())
	if !report.Live || !report.Ready || len(report.Checks) != 2 {
		t.Fatalf("Health() = %+v", report)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}
	want := []string{"start:first", "start:second", "health:first", "health:second", "shutdown:second", "shutdown:first"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestAppStartFailureRollsBackTypedModules(t *testing.T) {
	events := make([]string, 0)
	mu := &sync.Mutex{}
	app := lifecycleApplication(t,
		&lifecycleTestModule{name: "first", events: &events, mu: mu},
		&lifecycleTestModule{name: "second", events: &events, mu: mu, startErr: errors.New("boom")},
	)
	if err := app.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil, want failure")
	}
	if app.State() != AppStateFailed {
		t.Fatalf("State() = %s, want failed", app.State())
	}
	want := []string{"start:first", "start:second", "shutdown:second", "shutdown:first"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}
