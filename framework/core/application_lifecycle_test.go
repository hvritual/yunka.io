package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"yunka.io/framework/core/request"
)

type lifecycleTestModule struct {
	name      string
	events    *[]string
	startErr  error
	healthErr error
}

func (m *lifecycleTestModule) Name() string                                        { return m.name }
func (m *lifecycleTestModule) Init(ModuleInit)                                     {}
func (m *lifecycleTestModule) BindInfra(bool, interface{}) error                   { return nil }
func (m *lifecycleTestModule) BindService(Service, interface{}) error              { return nil }
func (m *lifecycleTestModule) BindRepository(interface{}) error                    { return nil }
func (m *lifecycleTestModule) GetService(string, request.Runtime) (Service, error) { return nil, nil }
func (m *lifecycleTestModule) PutService(Service)                                  {}
func (m *lifecycleTestModule) GetRepo(request.Runtime, reflect.Type) Repository    { return nil }
func (m *lifecycleTestModule) PutRepo(reflect.Type, Repository)                    {}
func (m *lifecycleTestModule) GetInfra(request.Runtime, reflect.Type) interface{}  { return nil }
func (m *lifecycleTestModule) PutInfra(reflect.Type, interface{})                  {}
func (m *lifecycleTestModule) Stop()                                               { *m.events = append(*m.events, "stop:"+m.name) }

func (m *lifecycleTestModule) Start(context.Context) error {
	*m.events = append(*m.events, "start:"+m.name)
	return m.startErr
}

func (m *lifecycleTestModule) Shutdown(context.Context) error {
	*m.events = append(*m.events, "shutdown:"+m.name)
	return nil
}

func (m *lifecycleTestModule) Health(context.Context) error {
	*m.events = append(*m.events, "health:"+m.name)
	return m.healthErr
}

func TestAppLifecycleOrderAndHealth(t *testing.T) {
	events := make([]string, 0)
	app := &App{modules: make(map[string]Module), rhTree: NewHandleTree()}
	app.RegisterModule(&lifecycleTestModule{name: "first", events: &events})
	app.RegisterModule(&lifecycleTestModule{name: "second", events: &events})

	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if app.State() != AppStateReady {
		t.Fatalf("State() = %s, want ready", app.State())
	}

	report := app.Health(context.Background())
	if !report.Live || !report.Ready {
		t.Fatalf("Health() = %+v, want live and ready", report)
	}
	if len(report.Checks) != 2 {
		t.Fatalf("Health() checks = %d, want 2", len(report.Checks))
	}

	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}

	want := []string{
		"start:first",
		"start:second",
		"health:first",
		"health:second",
		"shutdown:second",
		"shutdown:first",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestAppStartFailureRollsBackStartedModules(t *testing.T) {
	events := make([]string, 0)
	app := &App{modules: make(map[string]Module), rhTree: NewHandleTree()}
	app.RegisterModule(&lifecycleTestModule{name: "first", events: &events})
	app.RegisterModule(&lifecycleTestModule{name: "second", events: &events, startErr: errors.New("boom")})

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
