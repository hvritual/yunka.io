package kernel

import (
	"context"
	"errors"
	"fmt"

	"yunka.io/framework/core"
)

// BootstrapOptions describes structural assembly sequencing only. The Build and
// Register callbacks are generated/typed by callers; Bootstrap does not inspect
// packages, discover services, or own business execution semantics.
type BootstrapOptions[Applications any] struct {
	Kernel   Options
	Build    func() (Applications, error)
	Register func(Applications) error
}

// BootstrapResult exposes the already-existing core.App lifecycle owner plus
// the typed Application set. It intentionally has no Start/Shutdown methods of
// its own, so lifecycle ownership cannot drift away from core.App.
type BootstrapResult[Applications any] struct {
	App          *core.App
	Applications Applications
}

// Bootstrap closes structural assembly through the existing App lifecycle:
// construct App -> build typed Applications -> register explicit transports ->
// start App. Failures before Start clean up the constructed App; Start itself
// already performs fail-closed rollback of App-owned resources.
func Bootstrap[Applications any](ctx context.Context, options BootstrapOptions[Applications]) (BootstrapResult[Applications], error) {
	var zero BootstrapResult[Applications]
	if options.Build == nil {
		return zero, errors.New("kernel: bootstrap build callback is required")
	}
	if options.Register == nil {
		return zero, errors.New("kernel: bootstrap register callback is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	app, err := New(options.Kernel)
	if err != nil {
		return zero, fmt.Errorf("kernel: bootstrap application: %w", err)
	}
	applications, err := options.Build()
	if err != nil {
		return zero, errors.Join(fmt.Errorf("kernel: bootstrap applications: %w", err), shutdownBeforeStart(app))
	}
	if err := options.Register(applications); err != nil {
		return zero, errors.Join(fmt.Errorf("kernel: bootstrap transports: %w", err), shutdownBeforeStart(app))
	}
	if err := app.Start(ctx); err != nil {
		return zero, fmt.Errorf("kernel: bootstrap start: %w", err)
	}
	return BootstrapResult[Applications]{App: app, Applications: applications}, nil
}

func shutdownBeforeStart(app *core.App) error {
	if app == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultShutdownTimeout)
	defer cancel()
	return app.Shutdown(ctx)
}
