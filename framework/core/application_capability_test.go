package core

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
)

type appCapability interface {
	ID() string
}

type appCapabilityValue struct{ id string }

func (value *appCapabilityValue) ID() string { return value.id }

type appCapabilityModule struct {
	name  string
	value appCapability
	key   modulecatalog.CapabilityKey[appCapability]
}

func (module *appCapabilityModule) Name() string { return module.name }
func (module *appCapabilityModule) ExportCapabilities() []modulecatalog.CapabilityExport {
	return []modulecatalog.CapabilityExport{modulecatalog.ExportCapability(module.key, module.value)}
}

func TestNewAppWithCapabilitiesKeepsModuleExportsAppIsolated(t *testing.T) {
	var sequence atomic.Int32
	key := modulecatalog.MustCapabilityKey[appCapability]("test.cache", "example.com/contracts/testcache", "Cache")
	newCatalog := func() *modulecatalog.Catalog {
		catalog := modulecatalog.New()
		catalog.MustRegister(modulecatalog.Descriptor{
			Name:     "cache-provider",
			Provides: []modulecatalog.CapabilityContract{key.Contract()},
			Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) {
				id := sequence.Add(1)
				return &appCapabilityModule{name: "cache-provider", key: key, value: &appCapabilityValue{id: fmt.Sprintf("app-%d", id)}}, nil
			},
		})
		return catalog
	}
	firstApp, firstSet, err := NewAppWithCapabilities(AppOptions{Catalog: newCatalog()})
	if err != nil {
		t.Fatal(err)
	}
	defer firstApp.Shutdown(context.Background())
	secondApp, secondSet, err := NewAppWithCapabilities(AppOptions{Catalog: newCatalog()})
	if err != nil {
		t.Fatal(err)
	}
	defer secondApp.Shutdown(context.Background())
	first, err := modulecatalog.ResolveCapability(firstSet, key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := modulecatalog.ResolveCapability(secondSet, key)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID() == second.ID() || first == second {
		t.Fatalf("capabilities leaked across Apps: first=%s second=%s", first.ID(), second.ID())
	}
}
