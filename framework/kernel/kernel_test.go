package kernel

import (
	"testing"

	"yunka.io/framework/core/modulecatalog"
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
