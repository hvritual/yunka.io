package kernel

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"gorm.io/gorm"
	"yunka.io/framework/core/modulecatalog"
	"yunka.io/framework/platform"
	"yunka.io/pkg/logExt"
)

type kernelInstance string

func (instance kernelInstance) Name() string { return string(instance) }

func TestNewUsesSuppliedIsolatedCatalog(t *testing.T) {
	catalog := modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{Name: "orders", Build: func(ctx modulecatalog.BuildContext) (modulecatalog.Instance, error) {
		return kernelInstance(ctx.Descriptor().Name), nil
	}})
	first, err := New(Options{Catalog: catalog, ContextFactory: modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{})})
	if err != nil {
		t.Fatal(err)
	}
	secondCatalog := modulecatalog.New()
	secondCatalog.MustRegister(modulecatalog.Descriptor{Name: "devices", Build: func(ctx modulecatalog.BuildContext) (modulecatalog.Instance, error) {
		return kernelInstance(ctx.Descriptor().Name), nil
	}})
	second, err := New(Options{Catalog: secondCatalog, ContextFactory: modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{})})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("kernel returned shared application")
	}
}

func TestNewUsesAppOwnedPlatformProvider(t *testing.T) {
	var events []string
	var mu sync.Mutex
	database := &gorm.DB{}
	provider, err := platform.New(platform.Options{
		Logger: logExt.NewBaseLogger(),
		Databases: map[string]platform.DatabaseFactory{
			"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
				return platform.DatabaseResource{
					Database: database,
					StartFunc: func(context.Context) error {
						mu.Lock()
						events = append(events, "platform:start")
						mu.Unlock()
						return nil
					},
					ShutdownFunc: func(context.Context) error {
						mu.Lock()
						events = append(events, "platform:shutdown")
						mu.Unlock()
						return nil
					},
				}, nil
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	catalog := modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{
		Name: "orders",
		Requirements: modulecatalog.Requirements{
			Logger:    true,
			Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}},
		},
		Build: func(ctx modulecatalog.BuildContext) (modulecatalog.Instance, error) {
			resolved, err := ctx.Databases().GORM("primary")
			if err != nil {
				return nil, err
			}
			if resolved != database || ctx.Logger() == nil {
				return nil, errors.New("platform capabilities were not injected")
			}
			return kernelInstance("orders"), nil
		},
	})
	application, err := New(Options{Catalog: catalog, Platform: provider})
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(events, []string{"platform:start", "platform:shutdown"}) {
		t.Fatalf("events=%v", events)
	}
}

func TestNewRejectsMixedPlatformAndDirectCapabilities(t *testing.T) {
	provider, err := platform.New(platform.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{Catalog: modulecatalog.New(), Platform: provider, Logger: logExt.NewBaseLogger()}); err == nil {
		t.Fatal("mixed platform/direct configuration accepted")
	}
}
