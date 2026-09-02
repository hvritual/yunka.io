package platform

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
)

type testConnection struct{}

func (testConnection) Invoke(context.Context, string, any, any, ...grpc.CallOption) error { return nil }
func (testConnection) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

type testInstance string

func (instance testInstance) Name() string { return string(instance) }

type recordingLogger struct {
	mu     sync.Mutex
	values []string
}

func (logger *recordingLogger) record(values ...interface{}) {
	logger.mu.Lock()
	logger.values = append(logger.values, strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(joinValues(values)), "\n")))
	logger.mu.Unlock()
}

func joinValues(values []interface{}) string {
	var builder strings.Builder
	for _, value := range values {
		builder.WriteString(toString(value))
	}
	return builder.String()
}

func toString(value interface{}) string {
	if value == nil {
		return "<nil>"
	}
	if text, ok := value.(string); ok {
		return text
	}
	return "value"
}

func (logger *recordingLogger) Print(values ...interface{})   { logger.record(values...) }
func (logger *recordingLogger) Println(values ...interface{}) { logger.record(values...) }
func (logger *recordingLogger) Fatal(values ...interface{})   { logger.record(values...) }
func (logger *recordingLogger) Fatalf(format string, values ...interface{}) {
	logger.record(format)
}
func (logger *recordingLogger) Error(values ...interface{}) { logger.record(values...) }
func (logger *recordingLogger) Errorf(format string, values ...interface{}) {
	logger.record(format)
}
func (logger *recordingLogger) Warn(values ...interface{}) { logger.record(values...) }
func (logger *recordingLogger) Warnf(format string, values ...interface{}) {
	logger.record(format)
}
func (logger *recordingLogger) Info(values ...interface{}) { logger.record(values...) }
func (logger *recordingLogger) Infof(format string, values ...interface{}) {
	logger.record(format)
}
func (logger *recordingLogger) Debug(values ...interface{}) { logger.record(values...) }
func (logger *recordingLogger) Debugf(format string, values ...interface{}) {
	logger.record(format)
}

func (logger *recordingLogger) snapshot() []string {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	return append([]string(nil), logger.values...)
}

type eventRecorder struct {
	mu     sync.Mutex
	events []string
}

func (recorder *eventRecorder) add(event string) {
	recorder.mu.Lock()
	recorder.events = append(recorder.events, event)
	recorder.mu.Unlock()
}

func (recorder *eventRecorder) snapshot() []string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]string(nil), recorder.events...)
}

func TestProviderPreparesOnceAndOwnsDeterministicLifecycle(t *testing.T) {
	recorder := &eventRecorder{}
	logger := &recordingLogger{}
	var databaseCalls, rpcCalls int
	provider, err := New(Options{
		Logger: logger,
		Databases: map[string]DatabaseFactory{
			"primary": DatabaseFactoryFunc(func(context.Context, string) (DatabaseResource, error) {
				databaseCalls++
				return DatabaseResource{
					Database:     &gorm.DB{},
					StartFunc:    func(context.Context) error { recorder.add("start:database:primary"); return nil },
					HealthFunc:   func(context.Context) error { recorder.add("health:database:primary"); return nil },
					ShutdownFunc: func(context.Context) error { recorder.add("shutdown:database:primary"); return nil },
				}, nil
			}),
		},
		RPC: map[string]RPCFactory{
			"gateway": RPCFactoryFunc(func(context.Context, string) (RPCResource, error) {
				rpcCalls++
				return RPCResource{
					Connection:   testConnection{},
					StartFunc:    func(context.Context) error { recorder.add("start:rpc:gateway"); return nil },
					HealthFunc:   func(context.Context) error { recorder.add("health:rpc:gateway"); return nil },
					ShutdownFunc: func(context.Context) error { recorder.add("shutdown:rpc:gateway"); return nil },
				}, nil
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	requirements := modulecatalog.RequirementSet{
		Modules:   []string{"orders"},
		Logger:    true,
		Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}},
		RPC:       []modulecatalog.RPCRequirement{{Name: "gateway"}},
	}
	if err := provider.Prepare(requirements); err != nil {
		t.Fatal(err)
	}
	if err := provider.Prepare(requirements); err != nil {
		t.Fatal(err)
	}
	if databaseCalls != 1 || rpcCalls != 1 {
		t.Fatalf("factory calls database=%d rpc=%d", databaseCalls, rpcCalls)
	}
	if got, want := provider.ResourceKeys(), []string{"database:primary", "rpc:gateway"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys=%v want=%v", got, want)
	}
	descriptor := modulecatalog.Descriptor{
		Name: "orders",
		Requirements: modulecatalog.Requirements{
			Logger:    true,
			Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}},
			RPC:       []modulecatalog.RPCRequirement{{Name: "gateway"}},
		},
		Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) { return testInstance("orders"), nil },
	}
	buildContext, err := provider.ForModule(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildContext.Databases().GORM("primary"); err != nil {
		t.Fatal(err)
	}
	if _, err := buildContext.RPC().Connection("gateway"); err != nil {
		t.Fatal(err)
	}
	buildContext.Logger().Info("ready")
	if values := logger.snapshot(); len(values) != 1 || !strings.Contains(values[0], "module=orders") {
		t.Fatalf("logger values=%v", values)
	}
	if err := provider.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := provider.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantEvents := []string{
		"start:database:primary", "start:rpc:gateway",
		"health:database:primary", "health:rpc:gateway",
		"shutdown:rpc:gateway", "shutdown:database:primary",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("events=%v want=%v", got, wantEvents)
	}
}

func TestProviderPrepareFailureCleansOpenedResources(t *testing.T) {
	recorder := &eventRecorder{}
	provider, err := New(Options{Databases: map[string]DatabaseFactory{
		"primary": DatabaseFactoryFunc(func(context.Context, string) (DatabaseResource, error) {
			return DatabaseResource{
				Database:     &gorm.DB{},
				ShutdownFunc: func(context.Context) error { recorder.add("shutdown:database:primary"); return nil },
			}, nil
		}),
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = provider.Prepare(modulecatalog.RequirementSet{
		Modules:   []string{"orders"},
		Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}},
		RPC:       []modulecatalog.RPCRequirement{{Name: "missing"}},
	})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error=%v", err)
	}
	if got, want := recorder.snapshot(), []string{"shutdown:database:primary"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("events=%v want=%v", got, want)
	}
}

func TestProviderStartFailureCleansAllResourcesInReverse(t *testing.T) {
	recorder := &eventRecorder{}
	provider, err := New(Options{
		Databases: map[string]DatabaseFactory{
			"primary": DatabaseFactoryFunc(func(context.Context, string) (DatabaseResource, error) {
				return DatabaseResource{
					Database:     &gorm.DB{},
					StartFunc:    func(context.Context) error { recorder.add("start:database"); return nil },
					ShutdownFunc: func(context.Context) error { recorder.add("shutdown:database"); return nil },
				}, nil
			}),
		},
		RPC: map[string]RPCFactory{
			"gateway": RPCFactoryFunc(func(context.Context, string) (RPCResource, error) {
				return RPCResource{
					Connection:   testConnection{},
					StartFunc:    func(context.Context) error { recorder.add("start:rpc"); return errors.New("unavailable") },
					ShutdownFunc: func(context.Context) error { recorder.add("shutdown:rpc"); return nil },
				}, nil
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Prepare(modulecatalog.RequirementSet{
		Modules:   []string{"orders"},
		Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}},
		RPC:       []modulecatalog.RPCRequirement{{Name: "gateway"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := provider.Start(context.Background()); err == nil {
		t.Fatal("start failure was ignored")
	}
	want := []string{"start:database", "start:rpc", "shutdown:rpc", "shutdown:database"}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events=%v want=%v", got, want)
	}
}

func TestProviderInstancesAreIsolated(t *testing.T) {
	build := func(database *gorm.DB) *Provider {
		provider, err := New(Options{Databases: map[string]DatabaseFactory{
			"primary": DatabaseFactoryFunc(func(context.Context, string) (DatabaseResource, error) {
				return BorrowedDatabase(database), nil
			}),
		}})
		if err != nil {
			t.Fatal(err)
		}
		if err := provider.Prepare(modulecatalog.RequirementSet{
			Modules: []string{"orders"}, Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}},
		}); err != nil {
			t.Fatal(err)
		}
		return provider
	}
	firstDB := &gorm.DB{}
	secondDB := &gorm.DB{}
	first := build(firstDB)
	second := build(secondDB)
	descriptor := modulecatalog.Descriptor{
		Name: "orders", Requirements: modulecatalog.Requirements{Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}}},
		Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) { return testInstance("orders"), nil },
	}
	firstContext, err := first.ForModule(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	secondContext, err := second.ForModule(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	firstResolved, _ := firstContext.Databases().GORM("primary")
	secondResolved, _ := secondContext.Databases().GORM("primary")
	if firstResolved != firstDB || secondResolved != secondDB || firstResolved == secondResolved {
		t.Fatal("platform providers shared database state")
	}
}

func TestProviderRejectsShutdownWhileStartIsInProgress(t *testing.T) {
	startEntered := make(chan struct{})
	allowStart := make(chan struct{})
	shutdowns := 0
	provider, err := New(Options{Databases: map[string]DatabaseFactory{
		"primary": DatabaseFactoryFunc(func(context.Context, string) (DatabaseResource, error) {
			return DatabaseResource{
				Database: &gorm.DB{},
				StartFunc: func(context.Context) error {
					close(startEntered)
					<-allowStart
					return nil
				},
				ShutdownFunc: func(context.Context) error {
					shutdowns++
					return nil
				},
			}, nil
		}),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Prepare(modulecatalog.RequirementSet{
		Modules: []string{"orders"}, Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}},
	}); err != nil {
		t.Fatal(err)
	}
	startResult := make(chan error, 1)
	go func() { startResult <- provider.Start(context.Background()) }()
	<-startEntered
	if err := provider.Shutdown(context.Background()); err == nil || !strings.Contains(err.Error(), "start is in progress") {
		t.Fatalf("shutdown during start error=%v", err)
	}
	if shutdowns != 0 {
		t.Fatalf("resource was shut down while start was active: %d", shutdowns)
	}
	close(allowStart)
	if err := <-startResult; err != nil {
		t.Fatal(err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if shutdowns != 1 {
		t.Fatalf("shutdown calls=%d", shutdowns)
	}
}
