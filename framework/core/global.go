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
	store         = make(map[string]any)
	storeLock     sync.Mutex
	initiators    []Initiator
	prepares      []Prepare
)

// register module conf type
func RegisterConfType(name string, dType interface{}) {
	globalConf.RegisterType(name, dType)
}

func SetItem[T any](key string, v T) {
	storeLock.Lock()
	defer storeLock.Unlock()
	store[key] = v
}

func GetItem[T any](key string, dt T) T {
	storeLock.Lock()
	defer storeLock.Unlock()
	v, ok := store[key]
	if !ok {
		return dt
	}
	return v.(T)
}

const dbSyncKey = `DBSync`

func SetDbSync(v bool) {
	SetItem[bool](dbSyncKey, v)
}

func IsDbSync() bool {
	return GetItem[bool](dbSyncKey, false)
}

func GetConfV2[T any](name string, dt T) T {
	return conf.GetConf[T](globalConf, name, dt)
}

// get logger
func Log() logExt.Logger {
	return app.Logger()
}
