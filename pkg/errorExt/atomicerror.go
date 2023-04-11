package errorExt

/**
 * @BelongProject yunka
 * @BelongPackage errorExt
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 4:53 下午
 * @Version V1.0
 */

import "sync/atomic"

type AtomicError struct {
	err atomic.Value // error
}

func (ae *AtomicError) Set(err error) {
	ae.err.Store(err)
}

func (ae *AtomicError) Load() error {
	if v := ae.err.Load(); v != nil {
		return v.(error)
	}
	return nil
}