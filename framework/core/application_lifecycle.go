package core

import (
	"context"
	"errors"
	"fmt"
	"io"

	"yunka.io/framework/core/modulecatalog"
)

func (app *App) State() AppState         { return AppState(app.state.Load()) }
func (app *App) setState(state AppState) { app.state.Store(uint32(state)) }

func (app *App) moduleSnapshot() []Module {
	app.moduleMu.RLock()
	defer app.moduleMu.RUnlock()
	modules := make([]Module, 0, len(app.moduleOrder))
	for _, name := range app.moduleOrder {
		if module, ok := app.modules[name]; ok {
			modules = append(modules, module)
		}
	}
	return modules
}

func (app *App) Start(ctx context.Context) error {
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
	legacy := app.moduleSnapshot()
	composed := app.composedModuleSnapshot()
	for _, module := range legacy {
		starter, ok := module.(Startable)
		if !ok {
			continue
		}
		if err := safeLifecycleCall("start module "+module.Name(), func() error { return starter.Start(ctx) }); err != nil {
			cleanupErr := errors.Join(app.shutdownComponents(ctx, legacy, composed)...)
			app.setState(AppStateFailed)
			return errors.Join(fmt.Errorf("start module %s: %w", module.Name(), err), cleanupErr)
		}
	}
	if starter, ok := app.compositionFactory.(Startable); ok {
		if err := safeLifecycleCall("start composition capabilities", func() error { return starter.Start(ctx) }); err != nil {
			cleanupErr := errors.Join(app.shutdownComponents(ctx, legacy, composed)...)
			app.setState(AppStateFailed)
			return errors.Join(fmt.Errorf("start composition capabilities: %w", err), cleanupErr)
		}
	}
	for _, module := range composed {
		starter, ok := module.(Startable)
		if !ok {
			continue
		}
		if err := safeLifecycleCall("start composed module "+module.Name(), func() error { return starter.Start(ctx) }); err != nil {
			cleanupErr := errors.Join(app.shutdownComponents(ctx, legacy, composed)...)
			app.setState(AppStateFailed)
			return errors.Join(fmt.Errorf("start composed module %s: %w", module.Name(), err), cleanupErr)
		}
	}
	app.setState(AppStateReady)
	return nil
}

func (app *App) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	app.lifecycleMu.Lock()
	defer app.lifecycleMu.Unlock()
	if app.State() == AppStateStopped {
		return nil
	}
	app.setState(AppStateStopping)
	errs := app.shutdownComponents(ctx, app.moduleSnapshot(), app.composedModuleSnapshot())
	app.setState(AppStateStopped)
	return errors.Join(errs...)
}

func (app *App) shutdownComponents(ctx context.Context, legacy []Module, composed []modulecatalog.Instance) []error {
	var errs []error
	// Typed modules are shut down first in reverse deterministic catalog order.
	for index := len(composed) - 1; index >= 0; index-- {
		if err := shutdownComposedInstance(ctx, composed[index]); err != nil {
			errs = append(errs, fmt.Errorf("shutdown composed module %s: %w", composed[index].Name(), err))
		}
	}
	if err := shutdownCompositionFactory(ctx, app.compositionFactory); err != nil {
		errs = append(errs, fmt.Errorf("shutdown composition capabilities: %w", err))
	}
	// Preserve the historical event-bus ownership position for the legacy path.
	if app.eventBus != nil {
		if err := safeLifecycleCall("close event bus", app.eventBus.Close); err != nil {
			errs = append(errs, err)
		}
	}
	for index := len(legacy) - 1; index >= 0; index-- {
		if err := shutdownModule(ctx, legacy[index]); err != nil {
			errs = append(errs, fmt.Errorf("shutdown module %s: %w", legacy[index].Name(), err))
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
			check.Status = HealthStatusUnhealthy
			check.Error = err.Error()
			report.Ready = false
		}
		report.Checks = append(report.Checks, check)
	}
	for _, module := range app.moduleSnapshot() {
		checker, ok := module.(HealthChecker)
		if !ok {
			continue
		}
		check := HealthCheck{Name: "module." + module.Name(), Status: HealthStatusHealthy}
		if err := safeLifecycleCall("health module "+module.Name(), func() error { return checker.Health(ctx) }); err != nil {
			check.Status = HealthStatusUnhealthy
			check.Error = err.Error()
			report.Ready = false
		}
		report.Checks = append(report.Checks, check)
	}
	for _, module := range app.composedModuleSnapshot() {
		checker, ok := module.(HealthChecker)
		if !ok {
			continue
		}
		check := HealthCheck{Name: "module." + module.Name(), Status: HealthStatusHealthy}
		if err := safeLifecycleCall("health composed module "+module.Name(), func() error { return checker.Health(ctx) }); err != nil {
			check.Status = HealthStatusUnhealthy
			check.Error = err.Error()
			report.Ready = false
		}
		report.Checks = append(report.Checks, check)
	}
	return report
}

func shutdownModule(ctx context.Context, module Module) error {
	if shutdowner, ok := module.(Shutdowner); ok {
		return safeLifecycleCall("shutdown module "+module.Name(), func() error { return shutdowner.Shutdown(ctx) })
	}
	return safeLifecycleCall("stop module "+module.Name(), func() error {
		module.Stop()
		return nil
	})
}

func startResource(ctx context.Context, resource interface{}) error {
	if resource == nil {
		return nil
	}
	starter, ok := resource.(Startable)
	if !ok {
		return nil
	}
	return safeLifecycleCall("start resource", func() error { return starter.Start(ctx) })
}

func healthCheckResource(ctx context.Context, name string, resource interface{}) (HealthCheck, bool) {
	if resource == nil {
		return HealthCheck{}, false
	}
	checker, ok := resource.(HealthChecker)
	if !ok {
		return HealthCheck{}, false
	}
	check := HealthCheck{Name: name, Status: HealthStatusHealthy}
	if err := safeLifecycleCall("health "+name, func() error { return checker.Health(ctx) }); err != nil {
		check.Status = HealthStatusUnhealthy
		check.Error = err.Error()
	}
	return check, true
}

func shutdownResource(ctx context.Context, resource interface{}) error {
	if resource == nil {
		return nil
	}
	if shutdowner, ok := resource.(Shutdowner); ok {
		return safeLifecycleCall("shutdown resource", func() error { return shutdowner.Shutdown(ctx) })
	}
	if closer, ok := resource.(io.Closer); ok {
		return safeLifecycleCall("close resource", closer.Close)
	}
	return nil
}
