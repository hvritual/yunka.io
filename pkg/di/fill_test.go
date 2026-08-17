package di

import "testing"

/**
 * @BelongProject yunka
 * @BelongPackage di
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/11 5:18 下午
 * @Version V1.0
 */

type TestStruct struct {
	A int
	B string
	C float32
	D float64
}

type TestStruct2 struct {
	A string
	B int
	C float32
	D float64
}

func TestFillValue(t *testing.T) {
	t0 := struct {
		A int
		B string
		C float32
		D float64
	}{
		1, `1`, 1, 1,
	}

	t1 := TestStruct{}

	FillValue(t0, &t1)
	t2 := TestStruct2{}
	FillValue(t0, &t2)

	t.Log(t1, t2)
}
