package core

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/hvritual/yunka.io/framework/core/eventBus"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	"github.com/hvritual/yunka.io/pkg/logExt"
)

// App is one isolated typed application instance. Runtime configuration,
// capabilities and modules are explicit per-App values; no package-global App
// or service locator exists.
type App struct {
	globalLogger logExt.Logger
	rhTree       *RouterHandleTree

	compositionMu      sync.RWMutex
	compositionModules []modulecatalog.Instance
	compositionFactory modulecatalog.ContextFactory

	runtimeComponents []RuntimeComponent
	runtimeInventory  RuntimeInventory

	lifecycleMu sync.Mutex
	state       atomic.Uint32

	eventBus eventBus.EventBus
}

func (app *App) RegisterLogger(logger logExt.Logger) { app.globalLogger = logger }
func (app *App) Logger() logExt.Logger               { return app.globalLogger }
func (app *App) GetHandleTree() *RouterHandleTree    { return app.rhTree }

func (app *App) Debug() {
	go func() {
		channel := make(chan os.Signal, 1)
		signal.Notify(channel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		defer signal.Stop(channel)
		for current := range channel {
			switch current {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				app.Stop()
				return
			}
		}
	}()
}

func (app *App) Run(run func()) {
	if app == nil {
		panic("core: nil application")
	}
	app.setState(AppStateInitializing)
	if app.rhTree != nil {
		app.rhTree.Walk(func(string, Handle) bool { return false })
	}
	if err := app.Start(context.Background()); err != nil {
		app.setState(AppStateFailed)
		panic(err)
	}
	defer app.Stop()
	if run != nil {
		run()
	}
}

func (app *App) Stop() {
	if app == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil && app.globalLogger != nil {
		app.globalLogger.Error("application shutdown: ", err)
	}
}
