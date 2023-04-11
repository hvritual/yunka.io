package array

import (
	"reflect"
	"testing"
)

/**
 * @BelongProject yunka
 * @BelongPackage util
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/10/26 1:27 下午
 * @Version V1.0
 */

type Test struct {
	A int
}

func (t Test) GetA() int {
	return t.A
}

type T interface {
	GetA() int
}

func TestSlice(t *testing.T) {

	data := Slice([]Test{
		{0},
		{1},
		{2},
	}, reflect.TypeOf([]T(nil)))

	t.Log(data.([]T))
	assertT := data.([]T)
	for _, _t := range assertT {
		t.Log(_t.(Test))
	}

}