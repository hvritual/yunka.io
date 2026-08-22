package kernel

import (
	"yunka.io/framework/core"
	"yunka.io/framework/core/eventBus"
	"yunka.io/framework/core/modulecatalog"
	"yunka.io/pkg/logExt"
)

type Options struct {
	Catalog        *modulecatalog.Catalog
	ContextFactory modulecatalog.ContextFactory
	Config         modulecatalog.ConfigProvider
	Logger         logExt.Logger
	Databases      modulecatalog.DatabaseProvider
	EventBus       eventBus.EventBus
	RPC            modulecatalog.RPCProvider
}

// New builds an isolated App. A nil Catalog uses the process-default static
// descriptor catalog populated by blank-import autoload packages. The catalog
// stores descriptors only; all runtime state belongs to the returned App.
func New(options Options) (*core.App, error) {
	catalog := options.Catalog
	if catalog == nil {
		catalog = modulecatalog.Default()
	}
	return core.NewApp(core.AppOptions{
		Config: options.Config, Logger: options.Logger, Databases: options.Databases,
		EventBus: options.EventBus, RPC: options.RPC,
		Catalog: catalog, ContextFactory: options.ContextFactory,
	})
}

func MustNew(options Options) *core.App {
	application, err := New(options)
	if err != nil {
		panic(err)
	}
	return application
}
