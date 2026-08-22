package core

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/BurntSushi/toml"
	"yunka.io/framework/core/eventBus"
	"yunka.io/framework/core/modulecatalog"
	"yunka.io/pkg/logExt"
)

type App struct {
	globalLogger logExt.Logger
	rhTree       *RouterHandleTree
	prepares     []func(Initiator)

	moduleMu    sync.RWMutex
	modules     map[string]Module
	moduleOrder []string

	compositionMu      sync.RWMutex
	compositionModules []modulecatalog.Instance
	compositionFactory modulecatalog.ContextFactory
	legacyComposition  bool

	lifecycleMu sync.Mutex
	state       atomic.Uint32

	eventBus eventBus.EventBus

	// C6 keeps only an opaque composition reference for source compatibility.
	// The value must be a typed grpc client, ClientConnInterface, or typed
	// factory. It owns no string dispatch, generated handler map, or transport.
	rpcMu          sync.RWMutex
	rpcOnce        sync.Once
	rpcClient      interface{}
	rpcServerCount int
}

func init() {
	var err error
	app, err = NewApp(AppOptions{})
	if err != nil {
		panic(err)
	}
	app.legacyComposition = true
}

func GetApp() *App { return app }

func (app *App) RegisterLogger(logger logExt.Logger) { app.globalLogger = logger }

func (app *App) Debug() {
	go func() {
		channel := make(chan os.Signal, 1)
		signal.Notify(channel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
			syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
		defer signal.Stop(channel)
		for current := range channel {
			switch current {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				app.Stop()
				return
			case syscall.SIGUSR1:
			case syscall.SIGUSR2:
			default:
			}
		}
	}()
}

func (app *App) Logger() logExt.Logger { return app.globalLogger }

func (app *App) InitConfFile(configPath string) error {
	filePath, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	return app.InitConfContent(string(content))
}

func (app *App) InitConfContent(config string) error {
	if app == nil || !app.legacyComposition {
		return errors.New("core: legacy global configuration is available only on the default App")
	}
	_, err := toml.Decode(config, &globalConf)
	return err
}

func (app *App) GetHandleTree() *RouterHandleTree { return app.rhTree }

func (app *App) Run(run func()) {
	app.setState(AppStateInitializing)
	if app.legacyComposition {
		for _, prepare := range prepares {
			prepare(globalConf)
		}
		for _, initiator := range initiators {
			initiator(app)
		}
	}
	app.rhTree.Walk(func(string, Handle) bool { return false })
	if err := app.Start(context.Background()); err != nil {
		app.setState(AppStateFailed)
		panic(err)
	}
	defer app.Stop()
	run()
}

func (app *App) GetModule(moduleName string) Module {
	app.moduleMu.RLock()
	defer app.moduleMu.RUnlock()
	return app.modules[moduleName]
}

// AppRegisterRpc preserves the historical composition call while storing only
// typed grpc-go objects. C7 removes this global compatibility holder.
func (app *App) AppRegisterRpc(client interface{}, servers ...interface{}) *App {
	app.rpcOnce.Do(func() {
		app.rpcMu.Lock()
		defer app.rpcMu.Unlock()
		app.rpcClient = client
		app.rpcServerCount = len(servers)
	})
	return app
}

func GetClient() interface{} {
	app.rpcMu.RLock()
	defer app.rpcMu.RUnlock()
	return app.rpcClient
}

func (app *App) rpcInventory() (bool, int) {
	if app == nil {
		return false, 0
	}
	app.rpcMu.RLock()
	defer app.rpcMu.RUnlock()
	return app.rpcClient != nil, app.rpcServerCount
}

func (app *App) RegisterModule(module Module) {
	if module == nil {
		return
	}
	app.moduleMu.Lock()
	defer app.moduleMu.Unlock()
	name := module.Name()
	if _, exists := app.modules[name]; !exists {
		app.moduleOrder = append(app.moduleOrder, name)
	}
	app.modules[name] = module
}

func (app *App) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil && app.globalLogger != nil {
		app.globalLogger.Error("application shutdown: ", err)
	}
}
