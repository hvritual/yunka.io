package core

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"yunka.io/framework/core/eventBus"
	"yunka.io/framework/core/modulecatalog"
	"yunka.io/pkg/logExt"
)

type composedLifecycleModule struct {
	name   string
	events *[]string
	mu     *sync.Mutex
}

func (module *composedLifecycleModule) Name() string { return module.name }
func (module *composedLifecycleModule) Start(context.Context) error {
	module.mu.Lock()
	*module.events = append(*module.events, "start:"+module.name)
	module.mu.Unlock()
	return nil
}
func (module *composedLifecycleModule) Shutdown(context.Context) error {
	module.mu.Lock()
	*module.events = append(*module.events, "shutdown:"+module.name)
	module.mu.Unlock()
	return nil
}
func (module *composedLifecycleModule) Health(context.Context) error { return nil }

func TestNewAppBuildsTypedCatalogInDeterministicLifecycleOrder(t *testing.T) {
	var mu sync.Mutex
	events := make([]string, 0)
	catalog := modulecatalog.New()
	for _, descriptor := range []modulecatalog.Descriptor{
		{Name: "api", DependsOn: []string{"db"}, Build: func(ctx modulecatalog.BuildContext) (modulecatalog.Instance, error) {
			return &composedLifecycleModule{name: ctx.Descriptor().Name, events: &events, mu: &mu}, nil
		}},
		{Name: "db", Build: func(ctx modulecatalog.BuildContext) (modulecatalog.Instance, error) {
			return &composedLifecycleModule{name: ctx.Descriptor().Name, events: &events, mu: &mu}, nil
		}},
	} {
		if err := catalog.Register(descriptor); err != nil {
			t.Fatal(err)
		}
	}
	application, err := NewApp(AppOptions{Catalog: catalog, ContextFactory: modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{})})
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if report := application.Health(context.Background()); !report.Ready || len(report.Checks) != 2 {
		t.Fatalf("health=%+v", report)
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := events, []string{"start:db", "start:api", "shutdown:api", "shutdown:db"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("events=%v want=%v", got, want)
	}
	diagnostics := application.Diagnostics(context.Background())
	if len(diagnostics.Modules) != 2 || diagnostics.Modules[0].Composition != "typed" || diagnostics.Modules[1].Composition != "typed" {
		t.Fatalf("diagnostics=%+v", diagnostics.Modules)
	}
}

func TestNewAppInstancesDoNotShareRuntimeState(t *testing.T) {
	buildCatalog := func() *modulecatalog.Catalog {
		catalog := modulecatalog.New()
		catalog.MustRegister(modulecatalog.Descriptor{Name: "orders", Build: func(ctx modulecatalog.BuildContext) (modulecatalog.Instance, error) {
			events := make([]string, 0)
			return &composedLifecycleModule{name: ctx.Descriptor().Name, events: &events, mu: &sync.Mutex{}}, nil
		}})
		return catalog
	}
	first, err := NewApp(AppOptions{Catalog: buildCatalog(), ContextFactory: modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{})})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewApp(AppOptions{Catalog: buildCatalog(), ContextFactory: modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{})})
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first.composedModuleSnapshot()[0] == second.composedModuleSnapshot()[0] {
		t.Fatal("independent applications share runtime object")
	}
	if err := first.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if first.State() != AppStateReady || second.State() != AppStateNew {
		t.Fatalf("first=%s second=%s", first.State(), second.State())
	}
	if err := first.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if second.State() != AppStateNew {
		t.Fatalf("second state changed to %s", second.State())
	}
}

func TestNewAppCreatesDefaultStaticFactoryAndRejectsMissingCapabilities(t *testing.T) {
	catalog := modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{Name: "orders", Build: func(context modulecatalog.BuildContext) (modulecatalog.Instance, error) {
		return composedLifecycleName(context.Descriptor().Name), nil
	}})
	if _, err := NewApp(AppOptions{Catalog: catalog}); err != nil {
		t.Fatalf("zero-requirement module: %v", err)
	}

	catalog = modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{Name: "orders", Requirements: modulecatalog.Requirements{Logger: true}, Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) {
		return composedLifecycleName("orders"), nil
	}})
	if _, err := NewApp(AppOptions{Catalog: catalog}); err == nil || !strings.Contains(err.Error(), "logger") {
		t.Fatalf("missing logger err=%v", err)
	}
}

func TestNewAppRejectsMismatchedInstance(t *testing.T) {
	catalog := modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{Name: "orders", Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) {
		return composedLifecycleName("wrong"), nil
	}})
	if _, err := NewApp(AppOptions{Catalog: catalog, ContextFactory: modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{})}); err == nil {
		t.Fatal("mismatched instance name accepted")
	}
}

type composedLifecycleName string

func (name composedLifecycleName) Name() string { return string(name) }

type preparingFactory struct {
	prepared bool
	order    *[]string
}

func (factory *preparingFactory) Prepare(requirements modulecatalog.RequirementSet) error {
	if !reflect.DeepEqual(requirements.Modules, []string{"orders"}) {
		return errors.New("unexpected requirements")
	}
	factory.prepared = true
	*factory.order = append(*factory.order, "prepare")
	return nil
}

func (factory *preparingFactory) ForModule(descriptor modulecatalog.Descriptor) (modulecatalog.BuildContext, error) {
	if !factory.prepared {
		return nil, errors.New("ForModule called before Prepare")
	}
	*factory.order = append(*factory.order, "context:"+descriptor.Name)
	return preparedContext{descriptor: descriptor}, nil
}

type preparedContext struct{ descriptor modulecatalog.Descriptor }

func (context preparedContext) Descriptor() modulecatalog.Descriptor { return context.descriptor }
func (preparedContext) Config() modulecatalog.ConfigProvider         { return nil }
func (preparedContext) Logger() logExt.Logger                        { return nil }
func (preparedContext) Databases() modulecatalog.DatabaseProvider    { return nil }
func (preparedContext) EventBus() eventBus.EventBus                  { return nil }
func (preparedContext) RPC() modulecatalog.RPCProvider               { return nil }

func TestNewAppPreparesAggregateBeforeBuild(t *testing.T) {
	order := make([]string, 0)
	factory := &preparingFactory{order: &order}
	catalog := modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{Name: "orders", Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) {
		order = append(order, "build:orders")
		return composedLifecycleName("orders"), nil
	}})
	if _, err := NewApp(AppOptions{Catalog: catalog, ContextFactory: factory}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"prepare", "context:orders", "build:orders"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order=%v want=%v", order, want)
	}
}

func TestNewAppInstancesHaveNoPackageGlobalRuntime(t *testing.T) {
	first, err := NewApp(AppOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewApp(AppOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("NewApp returned shared application")
	}
	if first.State() != AppStateNew || second.State() != AppStateNew {
		t.Fatalf("unexpected states first=%s second=%s", first.State(), second.State())
	}
	if err := first.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if first.State() != AppStateReady || second.State() != AppStateNew {
		t.Fatalf("application state leaked first=%s second=%s", first.State(), second.State())
	}
	if err := first.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
