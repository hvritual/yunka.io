package kernel

import (
	"context"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
)

type bootstrapCache interface {
	Name() string
}

type bootstrapCacheValue struct{ name string }

func (cache bootstrapCacheValue) Name() string { return cache.name }

type bootstrapCapabilityModule struct {
	key modulecatalog.CapabilityKey[bootstrapCache]
}

func (module bootstrapCapabilityModule) Name() string { return "cache-provider" }
func (module bootstrapCapabilityModule) ExportCapabilities() []modulecatalog.CapabilityExport {
	return []modulecatalog.CapabilityExport{modulecatalog.ExportCapability(module.key, bootstrapCacheValue{name: "redis"})}
}

func TestBootstrapPassesTypedCapabilitiesOnlyToConstructionCallback(t *testing.T) {
	key := modulecatalog.MustCapabilityKey[bootstrapCache]("cache.default", "example.com/contracts/cache", "Cache")
	catalog := modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{
		Name:     "cache-provider",
		Provides: []modulecatalog.CapabilityContract{key.Contract()},
		Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) {
			return bootstrapCapabilityModule{key: key}, nil
		},
	})
	result, err := Bootstrap(context.Background(), BootstrapOptions[string]{
		Kernel: Options{Catalog: catalog},
		BuildWithCapabilities: func(capabilities modulecatalog.CapabilitySet) (string, error) {
			cache, err := modulecatalog.ResolveCapability(capabilities, key)
			if err != nil {
				return "", err
			}
			return cache.Name(), nil
		},
		Register: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.App.Shutdown(context.Background())
	if result.Applications != "redis" {
		t.Fatalf("applications=%q", result.Applications)
	}
}

func TestBootstrapRejectsAmbiguousBuildCallbacks(t *testing.T) {
	_, err := Bootstrap(context.Background(), BootstrapOptions[string]{
		Kernel:                Options{Catalog: modulecatalog.New()},
		Build:                 func() (string, error) { return "legacy", nil },
		BuildWithCapabilities: func(modulecatalog.CapabilitySet) (string, error) { return "typed", nil },
		Register:              func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("ambiguous build callbacks accepted")
	}
}
