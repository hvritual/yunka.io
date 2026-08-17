package array

import (
	"fmt"
	"reflect"
)

/**
 * @BelongProject yunka
 * @BelongPackage util
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/10/23 5:52 下午
 * @Version V1.0
 */

func Slice(slice interface{}, newSliceType reflect.Type) interface{} {
	sv := reflect.ValueOf(slice)
	if sv.Kind() != reflect.Slice {
		panic(fmt.Sprintf("Slice called with non-slice value of type %T", slice))
	}
	if newSliceType.Kind() != reflect.Slice {
		panic(fmt.Sprintf("Slice called with non-slice type of type %T", newSliceType))
	}

	sliceLen := sv.Len()
	newSlice := reflect.MakeSlice(newSliceType,
		sliceLen,
		sliceLen)

	for i := 0; i < sliceLen; i++ {
		newSlice.Index(i).Set(sv.Index(i))
	}

	return newSlice.Interface()
}
