package core

import (
	"context"
	"errors"
	"fmt"
	"io"
)

func (app *App) State() AppState {
	return AppState(app.state.Load())
}

func (app *App) setState(state AppState) {
	app.state.Store(uint32(state))
}

func (app *App) moduleSnapshot() []Module {
	app.moduleMu.RLock()
	defer app.moduleMu.RUnlock()

	modules := make([]Module, 0, len(app.moduleOrder))
	for _, name := range app.moduleOrder {
		if mod, ok := app.modules[name]; ok {
			modules = append(modules, mod)
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
	modules := app.moduleSnapshot()

	for _, mod := range modules {
		starter, ok := mod.(Startable)
		if !ok {
			continue
		}
		if err := safeLifecycleCall("start module "+mod.Name(), func() error {
			return starter.Start(ctx)
		}); err != nil {
			cleanupErr := errors.Join(app.shutdownComponents(ctx, modules)...)
			app.setState(AppStateFailed)
			return errors.Join(fmt.Errorf("start module %s: %w", mod.Name(), err), cleanupErr)
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
	modules := app.moduleSnapshot()
	errs := app.shutdownComponents(ctx, modules)
	app.setState(AppStateStopped)
	return errors.Join(errs...)
}

func (app *App) shutdownComponents(ctx context.Context, modules []Module) []error {
	var errs []error

	// Typed RPC clients and servers are explicitly owned by composition/ingress
	// components. App closes only resources it directly owns.
	if app.eventBus != nil {
		if err := safeLifecycleCall("close event bus", app.eventBus.Close); err != nil {
			errs = append(errs, err)
		}
	}

	// Module dependencies are closed after ingress has drained, in reverse
	// registration order.
	for i := len(modules) - 1; i >= 0; i-- {
		if err := shutdownModule(ctx, modules[i]); err != nil {
			errs = append(errs, fmt.Errorf("shutdown module %s: %w", modules[i].Name(), err))
		}
	}
	return errs
}

func (app *App) Health(ctx context.Context) HealthReport {
	if ctx == nil {
		ctx = context.Background()
	}

	state := app.State()
	report := HealthReport{
		State: state.String(),
		Live:  state != AppStateStopped && state != AppStateFailed,
		Ready: state == AppStateReady,
	}
	if state == AppStateStopped || state == AppStateFailed {
		return report
	}

	for _, mod := range app.moduleSnapshot() {
		checker, ok := mod.(HealthChecker)
		if !ok {
			continue
		}

		check := HealthCheck{Name: "module." + mod.Name(), Status: HealthStatusHealthy}
		err := safeLifecycleCall("health module "+mod.Name(), func() error {
			return checker.Health(ctx)
		})
		if err != nil {
			check.Status = HealthStatusUnhealthy
			check.Error = err.Error()
			report.Ready = false
		}
		report.Checks = append(report.Checks, check)
	}

	return report
}

func shutdownModule(ctx context.Context, mod Module) error {
	if shutdowner, ok := mod.(Shutdowner); ok {
		return safeLifecycleCall("shutdown module "+mod.Name(), func() error {
			return shutdowner.Shutdown(ctx)
		})
	}
	return safeLifecycleCall("stop module "+mod.Name(), func() error {
		mod.Stop()
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
	return safeLifecycleCall("start resource", func() error {
		return starter.Start(ctx)
	})
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
	if err := safeLifecycleCall("health "+name, func() error {
		return checker.Health(ctx)
	}); err != nil {
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
		return safeLifecycleCall("shutdown resource", func() error {
			return shutdowner.Shutdown(ctx)
		})
	}
	if closer, ok := resource.(io.Closer); ok {
		return safeLifecycleCall("close resource", closer.Close)
	}
	return nil
}
