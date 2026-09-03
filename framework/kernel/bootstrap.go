package kernel

import (
	"context"
	"errors"
	"fmt"

	"github.com/hvritual/yunka.io/framework/core"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
)

// BootstrapOptions describes structural assembly sequencing only. Build and
// BuildWithCapabilities are mutually exclusive typed construction callbacks;
// Bootstrap does not inspect packages, discover services, or own business
// execution semantics.
type BootstrapOptions[Applications any] struct {
	Kernel                Options
	Build                 func() (Applications, error)
	BuildWithCapabilities func(modulecatalog.CapabilitySet) (Applications, error)
	Register              func(Applications) error
}

// BootstrapResult exposes the already-existing core.App lifecycle owner plus
// the typed Application set. It intentionally has no Start/Shutdown methods of
// its own, so lifecycle ownership cannot drift away from core.App.
type BootstrapResult[Applications any] struct {
	App          *core.App
	Applications Applications
}

// Bootstrap closes structural assembly through the existing App lifecycle:
// construct App/modules -> snapshot typed module capability exports -> build
// typed Applications -> register explicit transports -> start App. Capability
// resolution therefore exists only during construction; business runtime code
// receives typed dependencies and does not retain a service locator.
func Bootstrap[Applications any](ctx context.Context, options BootstrapOptions[Applications]) (BootstrapResult[Applications], error) {
	var zero BootstrapResult[Applications]
	if (options.Build == nil) == (options.BuildWithCapabilities == nil) {
		return zero, errors.New("kernel: exactly one bootstrap build callback is required")
	}
	if options.Register == nil {
		return zero, errors.New("kernel: bootstrap register callback is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	app, capabilities, err := newWithCapabilities(options.Kernel)
	if err != nil {
		return zero, fmt.Errorf("kernel: bootstrap application: %w", err)
	}
	var applications Applications
	if options.BuildWithCapabilities != nil {
		applications, err = options.BuildWithCapabilities(capabilities)
	} else {
		applications, err = options.Build()
	}
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
