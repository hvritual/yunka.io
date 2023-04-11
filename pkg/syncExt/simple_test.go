package syncExt

import (
	"errors"
	"strconv"
	"testing"
)

/**
 * @BelongProject yunka
 * @BelongPackage syncExt
 * @Description:
 *
 * @Copyright 2021 - Powered By 云咖
 * @Author: fworld
 * @Date:  2021/4/12 上午12:19
 * @Version V1.0
 */

func TestMulti(t *testing.T) {
	cErr := func(idx int, isErr bool) error {
		t.Log(idx)
		if isErr {
			return errors.New(strconv.Itoa(idx))
		}
		return nil
	}

	err := Multi(
		func() error { return cErr(0, false) },
		func() error { return cErr(1, true) },
		func() error { return cErr(2, true) })
	t.Log(err)
}