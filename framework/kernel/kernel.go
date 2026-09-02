package kernel

import (
	"errors"

	"github.com/hvritual/yunka.io/framework/core"
	"github.com/hvritual/yunka.io/framework/core/eventBus"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	"github.com/hvritual/yunka.io/framework/platform"
	"github.com/hvritual/yunka.io/pkg/logExt"
)

type Options struct {
	Platform          *platform.Provider
	Catalog           *modulecatalog.Catalog
	ContextFactory    modulecatalog.ContextFactory
	Config            modulecatalog.ConfigProvider
	Logger            logExt.Logger
	Databases         modulecatalog.DatabaseProvider
	EventBus          eventBus.EventBus
	RPC               modulecatalog.RPCProvider
	RuntimeComponents []core.RuntimeComponent
	RuntimeInventory  core.RuntimeInventory
}

// New builds an isolated App. A nil Catalog uses the process-default static
// descriptor catalog populated by blank-import autoload packages. The catalog
// stores descriptors only; all runtime state belongs to the returned App.
func New(options Options) (*core.App, error) {
	catalog := options.Catalog
	if catalog == nil {
		catalog = modulecatalog.Default()
	}
	if options.Platform != nil {
		if options.ContextFactory != nil || options.Config != nil || options.Logger != nil ||
			options.Databases != nil || options.EventBus != nil || options.RPC != nil {
			return nil, errors.New("kernel: Platform cannot be combined with direct capability options or ContextFactory")
		}
		options.ContextFactory = options.Platform
		options.Config = options.Platform.Config()
		options.Logger = options.Platform.Logger()
		options.EventBus = options.Platform.EventBus()
	}
	return core.NewApp(core.AppOptions{
		Config: options.Config, Logger: options.Logger, Databases: options.Databases,
		EventBus: options.EventBus, RPC: options.RPC,
		Catalog: catalog, ContextFactory: options.ContextFactory,
		RuntimeComponents: options.RuntimeComponents, RuntimeInventory: options.RuntimeInventory,
	})
}

func MustNew(options Options) *core.App {
	application, err := New(options)
	if err != nil {
		panic(err)
	}
	return application
}
