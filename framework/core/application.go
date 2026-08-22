package core

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/BurntSushi/toml"
	"yunka.io/framework/core/eventBus"
	"yunka.io/pkg/logExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage infras
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 3:11 下午
 * @Version V1.0
 */

type App struct {
	globalLogger logExt.Logger
	rhTree       *RouterHandleTree
	prepares     []func(Initiator)

	moduleMu    sync.RWMutex
	modules     map[string]Module
	moduleOrder []string

	lifecycleMu sync.Mutex
	state       atomic.Uint32

	// TODO 等待设计接口
	eventBus eventBus.EventBus

	// C6 keeps only an opaque composition reference for source compatibility.
	// The value must be a typed grpc client, ClientConnInterface, or typed
	// factory. It owns no string dispatch, generated handler map, or transport.
	rpcMu          sync.RWMutex
	rpcClient      interface{}
	rpcServerCount int
}

func init() {
	app = new(App)
	app.modules = make(map[string]Module)
	app.rhTree = NewHandleTree()
	app.setState(AppStateNew)
}

func GetApp() *App {
	return app
}

func (app *App) RegisterLogger(lg logExt.Logger) {
	app.globalLogger = lg
}

// Debug
// 接收应用信号进行调试模式处理
func (app *App) Debug() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
			syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
		defer signal.Stop(c)
		for s := range c {
			switch s {
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

func (app *App) Logger() logExt.Logger {
	return app.globalLogger
}

func (app *App) InitConfFile(cfgPath string) error {
	filePath, err := filepath.Abs(cfgPath)
	if err != nil {
		return err
	}
	bs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	return app.InitConfContent(string(bs))
}

func (app *App) InitConfContent(cfg string) error {
	_, err := toml.Decode(cfg, &globalConf)
	return err
}

func (app *App) GetHandleTree() *RouterHandleTree {
	return app.rhTree
}

func (app *App) Run(run func()) {
	app.setState(AppStateInitializing)

	for _, i := range prepares {
		i(globalConf)
	}
	for _, i := range initiators {
		i(app)
	}
	app.rhTree.Walk(func(string, Handle) bool { return false })

	if err := app.Start(context.Background()); err != nil {
		app.setState(AppStateFailed)
		panic(err)
	}
	defer app.Stop()
	run()
}

func (app *App) GetModule(modName string) Module {
	app.moduleMu.RLock()
	defer app.moduleMu.RUnlock()
	return app.modules[modName]
}

// AppRegisterRpc preserves the historical composition call while storing only
// typed grpc-go objects. The old invoke client/server interfaces no longer
// exist. C7 removes this global compatibility holder.
func (app *App) AppRegisterRpc(client interface{}, servers ...interface{}) *App {
	globalAppOnce.Do(func() {
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

func (app *App) RegisterModule(mod Module) {
	if mod == nil {
		return
	}
	app.moduleMu.Lock()
	defer app.moduleMu.Unlock()

	name := mod.Name()
	if _, exists := app.modules[name]; !exists {
		app.moduleOrder = append(app.moduleOrder, name)
	}
	app.modules[name] = mod
}

func (app *App) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	if err := app.Shutdown(ctx); err != nil && app.globalLogger != nil {
		app.globalLogger.Error("application shutdown: ", err)
	}
}
