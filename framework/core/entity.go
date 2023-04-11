package core

import "yunka.io/framework/core/request"

/**
 * @BelongProject yunka
 * @BelongPackage core
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/9 8:57 下午
 * @Version V1.0
 */

type PersistObject struct {
	conds map[string]interface{} `json:"-"`
}

func (obj *PersistObject) Conds() map[string]interface{} {
	if obj.conds == nil {
		return make(map[string]interface{})
	}
	result := make(map[string]interface{})
	for k, v := range obj.conds {
		result[k] = v
	}
	obj.conds = nil
	return result
}

func (obj *PersistObject) ConfField(name string, value interface{}) {
	if obj.conds == nil {
		obj.conds = make(map[string]interface{})
	}

	obj.conds[name] = value
	return
}

type Entity interface {
	Identity() string
	GetRuntime() request.Runtime
	Marshal() []byte
}
