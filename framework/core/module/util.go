package module

import (
	"errors"
	"reflect"
)

/**
 * @BelongProject
 * @BelongPackage core
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/5 1:20 下午
 * @Version V1.0
 */

func getAccessObj(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr || reflectValue.Kind() == reflect.Interface {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

func allFields(dest interface{}, call func(reflect.Value, reflect.Type)) {
	obj := getAccessObj(reflect.ValueOf(dest))

	objType := obj.Type()

	if objType.Kind() != reflect.Struct {
		return
	}

	for idx := 0; idx < obj.NumField(); idx++ {
		if objType.Field(idx).Anonymous {
			if !obj.Field(idx).CanAddr() {
				allFields(obj.Field(idx).Addr().Interface(), call)
			} else {
				call(obj.Field(idx), objType.Field(idx).Type)
			}
			continue
		}
		val := obj.Field(idx)
		call(val, val.Type())
	}
	return
}

// allFieldsFromValue
func allFieldsFromValue(val reflect.Value, call func(reflect.Value)) {
	destVal := getAccessObj(val)
	destType := destVal.Type()
	if destType.Kind() != reflect.Struct && destType.Kind() != reflect.Interface {
		return
	}
	for index := 0; index < destVal.NumField(); index++ {
		if destType.Field(index).Anonymous {
			allFieldsFromValue(destVal.Field(index).Addr(), call)
			continue
		}
		val := destVal.Field(index)
		call(val)
	}
}

func parsePoolFunc(f interface{}) (outType reflect.Type, e error) {
	ftype := reflect.TypeOf(f)
	if ftype.Kind() != reflect.Func {
		e = errors.New("It's not a func")
		return
	}
	if ftype.NumOut() != 1 {
		e = errors.New("Return must be an object pointer")
		return
	}
	outType = ftype.Out(0)
	if outType.Kind() != reflect.Ptr {
		e = errors.New("Return must be an object pointer")
		return
	}
	return
}
