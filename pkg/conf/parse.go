package conf

import (
	"reflect"
	"strings"
	"yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage conf
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 10:47 上午
 * @Version V1.0
 */

func setDstVal(srcVal, dstVal reflect.Value, value interface{}) reflect.Value {

	if !dstVal.CanSet() {
		return dstVal
	}

	sType := srcVal.Type()
	switch dstVal.Type().Kind() {
	case reflect.Bool:
		{

			if sType.Kind() == reflect.Bool {
				dstVal = reflect.ValueOf(reflect.New(dstVal.Type()).Interface())
				dstVal.Elem().SetBool(srcVal.Bool())
				dstVal = dstVal.Elem()
			}
		}
	case
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		{

			if sType.Kind() >= reflect.Int && sType.Kind() <= reflect.Int64 {
				dstVal = reflect.ValueOf(reflect.New(dstVal.Type()).Interface())
				dstVal.Elem().SetInt(srcVal.Int())
				dstVal = dstVal.Elem()
			}
		}
	case
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		{
			if sType.Kind() >= reflect.Uint && sType.Kind() <= reflect.Uint64 {
				dstVal = reflect.ValueOf(reflect.New(dstVal.Type()).Interface())
				dstVal.Elem().SetUint(srcVal.Uint())
				dstVal = dstVal.Elem()
			}
		}

	case
		reflect.Float32,
		reflect.Float64:
		if sType.Kind() >= reflect.Float32 && sType.Kind() <= reflect.Float64 {
			dstVal = reflect.ValueOf(reflect.New(dstVal.Type()).Interface())
			dstVal.Elem().SetFloat(srcVal.Float())
			dstVal = dstVal.Elem()
		}
	case
		reflect.String:
		{
			if sType.Kind() == reflect.String {
				if dstVal.IsZero() {
					dstVal = reflect.ValueOf(reflect.New(srcVal.Type()).Interface())
				}
				dstVal.Elem().Set(srcVal)
				dstVal = dstVal.Elem()
			}

		}
	case reflect.Struct:
		{
			if sType.Kind() == reflect.Struct ||
				sType.Kind() == reflect.Map {
				ele := dstVal
				for i := 0; i < ele.NumField(); i++ {
					p := ele.Type().Field(i)
					m := value.(map[string]interface{})
					tag := p.Tag // a reflect.StructTag
					name := tag.Get("toml")
					var (
						ok  = false
						val interface{}
					)
					if name == "" {
						name = stringsExt.UnderscoreName(p.Name)
						val, ok = m[name]
					} else {
						val, ok = m[name]
					}

					if !ok {
						name = strings.ToUpper(p.Name)
						val, ok = m[name]
						if !ok {
							name = strings.ToLower(p.Name)
							val, ok = m[name]
							if !ok {
								name = stringsExt.CamelName(p.Name)
								val, ok = m[name]
								if !ok {
									name = stringsExt.Lcfirst(stringsExt.CamelName(p.Name))
									val, ok = m[name]
								}
							}
						}
					}

					if ok {
						v := reflect.ValueOf(val)
						if v.Type() == p.Type {
							ele.Field(i).Set(v)
						} else {
							ele.Field(i).Set(setDstVal(v, ele.Field(i), val))
						}
					}

				}
			} else {
				if sType.Kind() == reflect.Interface {
					if srcVal.CanInterface() {
						dstVal = setDstVal(srcVal.Elem(), dstVal, srcVal.Elem().Interface())
					}
				}
			}
		}
	case reflect.Array, reflect.Slice:
		{

			if sType.Kind() == reflect.Array ||
				sType.Kind() == reflect.Slice {
				if srcVal.Len() > 0 {
					vItem := srcVal.Index(0)
					if vItem.CanInterface() {
						vSlice := reflect.MakeSlice(dstVal.Type(), srcVal.Len(), srcVal.Cap())
						for i := 0; i < srcVal.Len(); i++ {
							vItem := srcVal.Index(i)
							vSlice.Index(i).Set(setDstVal(reflect.ValueOf(vItem.Interface()),
								vSlice.Index(i), vItem.Interface()))
						}
						dstVal.Set(vSlice)
					}
				}
			} else {
				if sType.Kind() == reflect.Interface {
					if srcVal.CanInterface() {
						dstVal = setDstVal(srcVal.Elem(), dstVal, srcVal.Elem().Interface())
					}
				}
			}
		}
	case reflect.Interface:
		dstVal.Set(srcVal)
	case reflect.Map:
		tv := reflect.MakeMap(dstVal.Type())
		rvType := reflect.TypeOf(value)

		if rvType.Kind() != reflect.Map {
			return dstVal
		}

		if dstVal.Type().Key().Kind() != rvType.Key().Kind() {
			return dstVal
		}
		mr := srcVal.MapRange()
		for mr.Next() {
			k := mr.Key()
			tvVal := reflect.New(dstVal.Type().Elem()).Elem()
			tvVal = setDstVal(mr.Value(), tvVal, mr.Value().Interface())
			tv.SetMapIndex(k, tvVal)
		}
		dstVal = tv
	}
	return dstVal
}

func (m Map) UnmarshalTOML(obj interface{}) error {

	data, ok := obj.(map[string]interface{})
	if ok {
		for key, conf := range data {
			_t, ok := m[key]
			if ok {
				_t.isInit = true
				rv := reflect.ValueOf(conf)
				// 还原成非指针值
				_t.Value = setDstVal(rv, reflect.ValueOf(_t.Value).Elem(), conf).Interface()
			}
		}
	}

	for _, c := range m {
		if !c.isInit {
			c.Value = reflect.ValueOf(c.Value).Elem().Interface()
		}
	}
	return nil
}
