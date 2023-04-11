package core

import (
	"sync"
	"yunka.io/pkg/conf"
	"yunka.io/pkg/logExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage application
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 4:26 下午
 * @Version V1.0
 */

var (
	globalAppOnce sync.Once
	app           *App
	globalConf    = make(conf.Map)
	initiators    []Initiator
	prepares      []Prepare
)

// register module conf type
func RegisterConfType(name string, dType interface{}) {
	globalConf.RegisterType(name, dType)
}

// get conf
func GetConf(name string) interface{} {
	v, ok := globalConf[name]
	if ok {
		return v.Value
	}
	return nil
}

// get logger
func Log() logExt.Logger {
	return app.Logger()
}
