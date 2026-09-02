package core

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
)

func (app *App) State() AppState         { return AppState(app.state.Load()) }
func (app *App) setState(state AppState) { app.state.Store(uint32(state)) }

func (app *App) Start(ctx context.Context) error {
	if app == nil {
		return errors.New("core: nil application")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	app.lifecycleMu.Lock()
	defer app.lifecycleMu.Unlock()
	switch app.State() {
	case AppStateReady:
		return nil
	case AppStateStopping, AppStateStopped, AppStateFailed:
		return fmt.Errorf("application cannot start from state %s", app.State())
	}
	app.setState(AppStateStarting)
	modules := app.composedModuleSnapshot()
	components := app.runtimeComponentSnapshot()
	if starter, ok := app.compositionFactory.(Startable); ok {
		if err := safeLifecycleCall("start composition capabilities", func() error { return starter.Start(ctx) }); err != nil {
			cleanupErr := errors.Join(app.shutdownComponents(ctx, modules)...)
			app.setState(AppStateFailed)
			return errors.Join(fmt.Errorf("start composition capabilities: %w", err), cleanupErr)
		}
	}
	for _, module := range modules {
		starter, ok := module.(Startable)
		if !ok {
			continue
		}
		if err := safeLifecycleCall("start module "+module.Name(), func() error { return starter.Start(ctx) }); err != nil {
			cleanupErr := errors.Join(app.shutdownComponents(ctx, modules)...)
			app.setState(AppStateFailed)
			return errors.Join(fmt.Errorf("start module %s: %w", module.Name(), err), cleanupErr)
		}
	}
	for index, component := range components {
		if err := safeLifecycleCall("start runtime component "+component.Name, func() error {
			return component.StartFunc(ctx)
		}); err != nil {
			componentCleanup := errors.Join(shutdownRuntimeComponents(ctx, components[:index+1])...)
			compositionCleanup := errors.Join(app.shutdownComponents(ctx, modules)...)
			app.setState(AppStateFailed)
			return errors.Join(fmt.Errorf("start runtime component %s: %w", component.Name, err), componentCleanup, compositionCleanup)
		}
	}
	app.setState(AppStateReady)
	return nil
}

func (app *App) Shutdown(ctx context.Context) error {
	if app == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	app.lifecycleMu.Lock()
	defer app.lifecycleMu.Unlock()
	if app.State() == AppStateStopped {
		return nil
	}
	app.setState(AppStateStopping)
	var errs []error
	errs = append(errs, shutdownRuntimeComponents(ctx, app.runtimeComponentSnapshot())...)
	errs = append(errs, app.shutdownComponents(ctx, app.composedModuleSnapshot())...)
	app.setState(AppStateStopped)
	return errors.Join(errs...)
}

func (app *App) shutdownComponents(ctx context.Context, modules []modulecatalog.Instance) []error {
	var errs []error
	for index := len(modules) - 1; index >= 0; index-- {
		if err := shutdownComposedInstance(ctx, modules[index]); err != nil {
			errs = append(errs, fmt.Errorf("shutdown module %s: %w", modules[index].Name(), err))
		}
	}
	if err := shutdownCompositionFactory(ctx, app.compositionFactory); err != nil {
		errs = append(errs, fmt.Errorf("shutdown composition capabilities: %w", err))
	}
	if app.eventBus != nil {
		if err := safeLifecycleCall("close event bus", app.eventBus.Close); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (app *App) Health(ctx context.Context) HealthReport {
	if ctx == nil {
		ctx = context.Background()
	}
	state := app.State()
	report := HealthReport{State: state.String(), Live: state != AppStateStopped && state != AppStateFailed, Ready: state == AppStateReady}
	if state == AppStateStopped || state == AppStateFailed {
		return report
	}
	if checker, ok := app.compositionFactory.(HealthChecker); ok {
		check := HealthCheck{Name: "composition.capabilities", Status: HealthStatusHealthy}
		if err := safeLifecycleCall("health composition capabilities", func() error { return checker.Health(ctx) }); err != nil {
			check.Status, check.Error, report.Ready = HealthStatusUnhealthy, err.Error(), false
		}
		report.Checks = append(report.Checks, check)
	}
	for _, module := range app.composedModuleSnapshot() {
		checker, ok := module.(HealthChecker)
		if !ok {
			continue
		}
		check := HealthCheck{Name: "module." + module.Name(), Status: HealthStatusHealthy}
		if err := safeLifecycleCall("health module "+module.Name(), func() error { return checker.Health(ctx) }); err != nil {
			check.Status, check.Error, report.Ready = HealthStatusUnhealthy, err.Error(), false
		}
		report.Checks = append(report.Checks, check)
	}
	for _, component := range app.runtimeComponentSnapshot() {
		if component.HealthFunc == nil {
			continue
		}
		check := HealthCheck{Name: "runtime." + component.Name, Status: HealthStatusHealthy}
		if err := safeLifecycleCall("health runtime component "+component.Name, func() error {
			return component.HealthFunc(ctx)
		}); err != nil {
			check.Status, check.Error, report.Ready = HealthStatusUnhealthy, err.Error(), false
		}
		report.Checks = append(report.Checks, check)
	}
	return report
}

func startResource(ctx context.Context, resource any) error {
	if starter, ok := resource.(Startable); ok {
		return safeLifecycleCall("start resource", func() error { return starter.Start(ctx) })
	}
	return nil
}

func healthCheckResource(ctx context.Context, name string, resource any) (HealthCheck, bool) {
	checker, ok := resource.(HealthChecker)
	if !ok {
		return HealthCheck{}, false
	}
	check := HealthCheck{Name: name, Status: HealthStatusHealthy}
	if err := safeLifecycleCall("health "+name, func() error { return checker.Health(ctx) }); err != nil {
		check.Status, check.Error = HealthStatusUnhealthy, err.Error()
	}
	return check, true
}

func shutdownResource(ctx context.Context, resource any) error {
	if shutdowner, ok := resource.(Shutdowner); ok {
		return safeLifecycleCall("shutdown resource", func() error { return shutdowner.Shutdown(ctx) })
	}
	if closer, ok := resource.(io.Closer); ok {
		return safeLifecycleCall("close resource", closer.Close)
	}
	return nil
}
