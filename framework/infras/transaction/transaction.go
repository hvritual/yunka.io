package transaction

import (
	"errors"
)

/**
 * @BelongProject yunka
 * @BelongPackage transaction
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/12 4:26 下午
 * @Version V1.0
 */
type Transaction interface {
	Begin(interface{}) error

	Rollback() error

	Commit() error
}

func OpenTransaction(param interface{}, fn func() error, t ...interface{}) (err error) {
	trans := []Transaction(nil)
	for _, r := range t {
		t, ok := r.(Transaction)
		if !ok {
			return errors.New("assert Transaction error")
		}
		trans = append(trans, t)
		err := t.Begin(param)
		if err != nil {
			return err
		}
	}

	defer func() {
		_err := recover()
		if _err != nil {
			for _, t := range trans {
				err = t.Rollback()
			}
		}
	}()

	err = fn()
	if err != nil {
		for _, t := range trans {
			t.Rollback()
		}
	} else {
		for _, t := range trans {
			t.Commit()

		}
	}
	return err

}
