package module

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"yunka.io/framework/core"
	"yunka.io/pkg/conf"
)

/**
 * @BelongProject
 * @BelongPackage module
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/5 9:24 下午
 * @Version V1.0
 */

var (
	_ core.Module = (*module)(nil)
)

func NewModule(name string, appHook func(mod core.Module)) *module {
	mod := &module{
		name:         name,
		infras:       make(map[reflect.Type]*sync.Pool),
		repos:        make(map[reflect.Type]*sync.Pool),
		singleInfras: &sync.Map{},
		srvContainer: make(map[string]*serviceContainer),
	}
	core.RegisterPrepare(func(cnf conf.Map) {
		if appHook != nil {
			appHook(mod)
		}
	})

	core.RegisterInitiator(func(app *core.App) {
		app.RegisterModule(mod)
		for _, fn := range mod.inits {
			err := fn(mod)
			if err != nil {
				fmt.Println("err :", err)
				os.Exit(-1)
			}
		}
	})

	return mod
}

type module struct {
	name         string
	srvContainer map[string]*serviceContainer
	infras       map[reflect.Type]*sync.Pool
	singleInfras *sync.Map
	repos        map[reflect.Type]*sync.Pool
	inits        []core.ModuleInit
}

func (mod *module) Stop() {
	// Pooled module resources are request-scoped and do not own a process-wide
	// lifecycle. Concrete infrastructures remain responsible for closing any
	// external resources they create.
}

func (mod *module) Init(f core.ModuleInit) {
	mod.inits = append(mod.inits, f)
}

func (mod *module) Name() string {
	return mod.name
}

func (mod *module) bind(f interface{},
	getm func() map[reflect.Type]*sync.Pool) error {
	iType, err := parsePoolFunc(f)
	if err != nil {
		return err
	}
	mContainer := getm()
	if mContainer == nil {
		return errors.New(fmt.Sprintf("not found %v", iType))
	}
	_, ok := mContainer[iType]
	if ok {
		return errors.New(fmt.Sprintf("info :%v exist", iType))
	}

	mContainer[iType] = &sync.Pool{
		New: func() interface{} {
			values := reflect.ValueOf(f).Call([]reflect.Value{})
			infraObj := values[0].Interface()
			return infraObj
		},
	}
	return nil
}

func (mod *module) poolObjectIsRuntime(pool *sync.Pool) ([]core.RuntimeInit, interface{}, bool) {
	infra := pool.Get()
	if infra == nil {
		return nil, nil, false
	}
	if _, ok := infra.(core.RuntimeInit); ok {
		return []core.RuntimeInit{infra.(core.RuntimeInit)}, infra, true
	}
	return nil, infra, true
}
