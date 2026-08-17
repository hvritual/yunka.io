package conf

import (
	"github.com/BurntSushi/toml"
	"reflect"
)

/**
 * @BelongProject yunka
 * @BelongPackage conf
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 10:48 上午
 * @Version V1.0
 */

var (
	_ toml.Unmarshaler = (Map)(nil)
)

type Val struct {
	isInit bool
	Type   reflect.Type
	Value  interface{}
}

type Map map[string]*Val

func (m Map) RegisterType(name string, val interface{}) {
	m[name] = &Val{
		Type: reflect.TypeOf(val),
		// 构建指向数据的指针
		Value: reflect.New(reflect.TypeOf(val)).Interface(),
	}
}

func GetConf[T any](m Map, name string, dt T) T {
	v, ok := m[name]
	if ok {
		return v.Value.(T)
	}
	return dt
}
