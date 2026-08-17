package core

import (
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"yunka.io/framework/core/eventBus"
	"yunka.io/pkg/invoke"
	"yunka.io/pkg/logExt"
	"yunka.io/pkg/threading"
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
	modules      map[string]Module
	// TODO 等待设计接口
	eventBus eventBus.EventBus

	srvs []invoke.RpcServer
	clt  invoke.RpcClient
}

func init() {
	app = new(App)
	app.modules = make(map[string]Module)
	app.rhTree = NewHandleTree()

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
		c := make(chan os.Signal)
		//监听指定信号 ctrl+c kill
		signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
			syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
		for {
			for s := range c {
				switch s {
				case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
					return
				case syscall.SIGUSR1:

				case syscall.SIGUSR2:
				default:
				}
			}
		}
	}()
}

// Logger .
func (app *App) Logger() logExt.Logger {
	return app.globalLogger
}

// InitConfFile load conf by file
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

// InitConfContent load conf by content
func (app *App) InitConfContent(cfg string) error {
	_, err := toml.Decode(cfg, &globalConf)
	return err
}

func (app *App) GetHandleTree() *RouterHandleTree {
	return app.rhTree
}

// Run app
func (app *App) Run(run func()) {

	for _, i := range prepares {
		i(globalConf)
	}

	for _, i := range initiators {
		i(app)
	}
	app.rhTree.Walk(func(s string, v Handle) bool {
		//Log().Debug(s)
		return false
	})
	run()
}

func (app *App) GetModule(modName string) Module {
	return app.modules[modName]
}

func RegisterServer(srvName string, service interface{}) error {
	for _, srv := range app.srvs {
		if err := srv.RegisterServer(srvName, service); err != nil {
			return err
		}
	}
	return nil
}

// AppRegisterRpc 注册api rpc相关信息
func (app *App) AppRegisterRpc(client invoke.RpcClient, srvs ...invoke.RpcServer) *App {
	globalAppOnce.Do(func() {
		app.clt = client
		app.srvs = srvs
	})
	return app
}

func GetClient() invoke.RpcClient {
	return app.clt
}

func (app *App) RegisterModule(mod Module) {
	app.modules[mod.Name()] = mod
}

func (app *App) Stop() {
	for _, mod := range app.modules {
		threading.RunSafe(func() {
			mod.Stop()
		})
	}
}
