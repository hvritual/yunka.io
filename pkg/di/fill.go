package di

import (
	"fmt"
	"reflect"
)

/**
 * @BelongProject
 * @BelongPackage di
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/11 5:13 下午
 * @Version V1.0
 */

func buildMap(tDst reflect.Type, vDst reflect.Value, vMap map[string]reflect.Value) {
	tk := tDst.Kind()
	if tk <= reflect.Float64 || tk == reflect.String {
		vMap[tDst.Name()] = vDst
		return
	}

	if tk == reflect.Struct {
		for i := 0; i < tDst.NumField(); i++ {
			field := tDst.Field(i)
			k := field.Type.Kind()
			if field.Anonymous {
				buildMap(field.Type, vDst.Field(i), vMap)
			} else {
				if k <= reflect.Float64 || k == reflect.String {
					vMap[field.Name] = vDst.Field(i)
				}
			}

		}
	}

}

func setVal(tSrc reflect.Type, vSrc reflect.Value, vMap map[string]reflect.Value) {
	tk := tSrc.Kind()
	if tk <= reflect.Float64 || tk == reflect.String {
		v, ok := vMap[tSrc.Name()]
		if ok {
			if v.Kind() == v.Kind() && v.Type() == v.Type() {
				v.Set(vSrc)
			}
		}
		return
	}
	if tk != reflect.Struct {
		return
	}

	for i := 0; i < tSrc.NumField(); i++ {
		field := tSrc.Field(i)
		if field.Anonymous {
			setVal(field.Type, vSrc.Field(i), vMap)
		} else {
			v, ok := vMap[field.Name]
			if ok {
				if vSrc.Field(i).Kind() == v.Kind() && vSrc.Field(i).Type() == v.Type() {
					v.Set(vSrc.Field(i))
				}
			}
		}
	}
}

func FillValue(src, dst interface{}) {

	tSrc := reflect.TypeOf(src)
	tDst := reflect.TypeOf(dst)
	tsk := tSrc.Kind()
	if tsk != reflect.Struct || tDst.Kind() != reflect.Ptr {
		fmt.Println("error:", tSrc.Kind(), tDst)
		return
	}

	vDst := reflect.ValueOf(dst).Elem()
	tDst = reflect.TypeOf(vDst.Interface())
	vSrc := reflect.ValueOf(src)
	vMap := make(map[string]reflect.Value)

	buildMap(tDst, vDst, vMap)
	setVal(tSrc, vSrc, vMap)
}
