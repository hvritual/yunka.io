package transaction

import (
	"errors"
	"fmt"
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
		err := t.Begin(param)
		if err != nil {
			for i := len(trans) - 1; i >= 0; i-- {
				err = errors.Join(err, trans[i].Rollback())
			}
			return err
		}
		trans = append(trans, t)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("transaction panic: %v", recovered)
			for i := len(trans) - 1; i >= 0; i-- {
				err = errors.Join(err, trans[i].Rollback())
			}
			return
		}
		if err != nil {
			for i := len(trans) - 1; i >= 0; i-- {
				err = errors.Join(err, trans[i].Rollback())
			}
			return
		}
		for _, t := range trans {
			err = errors.Join(err, t.Commit())
		}
	}()

	return fn()

}
