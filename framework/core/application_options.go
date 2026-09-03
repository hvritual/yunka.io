package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hvritual/yunka.io/framework/core/eventBus"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	"github.com/hvritual/yunka.io/pkg/logExt"
)

type AppOptions struct {
	Config            modulecatalog.ConfigProvider
	Logger            logExt.Logger
	Databases         modulecatalog.DatabaseProvider
	EventBus          eventBus.EventBus
	RPC               modulecatalog.RPCProvider
	Catalog           *modulecatalog.Catalog
	ContextFactory    modulecatalog.ContextFactory
	RuntimeComponents []RuntimeComponent
	RuntimeInventory  RuntimeInventory
}

func NewApp(options AppOptions) (*App, error) {
	application, _, err := NewAppWithCapabilities(options)
	return application, err
}

// NewAppWithCapabilities constructs one isolated App and returns the immutable
// capability snapshot exported by its composed modules. The snapshot is meant
// for bootstrap-time typed application construction; the App does not expose a
// runtime service-locator API.
func NewAppWithCapabilities(options AppOptions) (*App, modulecatalog.CapabilitySet, error) {
	components, err := normalizeRuntimeComponents(options.RuntimeComponents)
	if err != nil {
		return nil, modulecatalog.EmptyCapabilitySet(), err
	}
	inventory, err := normalizeRuntimeInventory(options.RuntimeInventory)
	if err != nil {
		return nil, modulecatalog.EmptyCapabilitySet(), err
	}
	application := &App{
		globalLogger:      options.Logger,
		eventBus:          options.EventBus,
		rhTree:            NewHandleTree(),
		runtimeComponents: components,
		runtimeInventory:  inventory,
	}
	application.setState(AppStateNew)
	if options.Catalog == nil {
		return application, modulecatalog.EmptyCapabilitySet(), nil
	}
	plan, err := options.Catalog.Seal()
	if err != nil {
		return nil, modulecatalog.EmptyCapabilitySet(), err
	}
	factory := options.ContextFactory
	if factory == nil && len(plan.Descriptors) > 0 {
		factory = modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{
			Config: options.Config, Logger: options.Logger, Databases: options.Databases,
			EventBus: options.EventBus, RPC: options.RPC,
		})
	}
	if factory != nil {
		application.compositionFactory = factory
		if err := factory.Prepare(plan.Requirements()); err != nil {
			return nil, modulecatalog.EmptyCapabilitySet(), application.compositionBuildError(fmt.Errorf("core: prepare module capabilities: %w", err))
		}
	}
	for _, descriptor := range plan.Descriptors {
		if descriptor.Build == nil {
			continue
		}
		if factory == nil {
			return nil, modulecatalog.EmptyCapabilitySet(), application.compositionBuildError(fmt.Errorf("core: module %s requires a context factory", descriptor.Name))
		}
		buildContext, err := factory.ForModule(descriptor)
		if err != nil {
			return nil, modulecatalog.EmptyCapabilitySet(), application.compositionBuildError(fmt.Errorf("core: module %s context: %w", descriptor.Name, err))
		}
		instance, err := descriptor.Build(buildContext)
		if err != nil {
			return nil, modulecatalog.EmptyCapabilitySet(), application.compositionBuildError(fmt.Errorf("core: build module %s: %w", descriptor.Name, err))
		}
		if instance == nil {
			return nil, modulecatalog.EmptyCapabilitySet(), application.compositionBuildError(fmt.Errorf("core: build module %s returned nil instance", descriptor.Name))
		}
		if strings.TrimSpace(instance.Name()) != descriptor.Name {
			cause := fmt.Errorf("core: built module name %q does not match descriptor %q", instance.Name(), descriptor.Name)
			return nil, modulecatalog.EmptyCapabilitySet(), application.compositionBuildError(errors.Join(cause, shutdownComposedInstance(context.Background(), instance)))
		}
		application.registerComposedModule(instance)
	}
	capabilities, err := modulecatalog.CollectCapabilities(plan.Descriptors, application.composedModuleSnapshot())
	if err != nil {
		return nil, modulecatalog.EmptyCapabilitySet(), application.compositionBuildError(fmt.Errorf("core: collect module capability exports: %w", err))
	}
	return application, capabilities, nil
}

func (app *App) compositionBuildError(cause error) error {
	var eventBusErr error
	if app != nil && app.eventBus != nil {
		eventBusErr = safeLifecycleCall("close event bus after composition failure", app.eventBus.Close)
	}
	return errors.Join(
		cause,
		shutdownComposedInstances(context.Background(), app.composedModuleSnapshot()),
		shutdownCompositionFactory(context.Background(), app.compositionFactory),
		eventBusErr,
	)
}

func shutdownCompositionFactory(ctx context.Context, factory modulecatalog.ContextFactory) error {
	if factory == nil {
		return nil
	}
	if shutdowner, ok := factory.(Shutdowner); ok {
		return safeLifecycleCall("shutdown composition capabilities", func() error { return shutdowner.Shutdown(ctx) })
	}
	if closer, ok := factory.(io.Closer); ok {
		return safeLifecycleCall("close composition capabilities", closer.Close)
	}
	return nil
}

func (app *App) registerComposedModule(instance modulecatalog.Instance) {
	if app == nil || instance == nil {
		return
	}
	app.compositionMu.Lock()
	app.compositionModules = append(app.compositionModules, instance)
	app.compositionMu.Unlock()
}

func (app *App) composedModuleSnapshot() []modulecatalog.Instance {
	if app == nil {
		return nil
	}
	app.compositionMu.RLock()
	defer app.compositionMu.RUnlock()
	return append([]modulecatalog.Instance(nil), app.compositionModules...)
}

func shutdownComposedInstances(ctx context.Context, instances []modulecatalog.Instance) error {
	var failures []error
	for index := len(instances) - 1; index >= 0; index-- {
		if err := shutdownComposedInstance(ctx, instances[index]); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func shutdownComposedInstance(ctx context.Context, instance modulecatalog.Instance) error {
	if instance == nil {
		return nil
	}
	if shutdowner, ok := instance.(Shutdowner); ok {
		return safeLifecycleCall("shutdown composed module "+instance.Name(), func() error { return shutdowner.Shutdown(ctx) })
	}
	if closer, ok := instance.(io.Closer); ok {
		return safeLifecycleCall("close composed module "+instance.Name(), closer.Close)
	}
	return nil
}
