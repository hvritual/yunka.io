package syncExt

/**
 * @BelongProject yunka
 * @BelongPackage syncExt
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 4:47 下午
 * @Version V1.0
 */

import (
	"sync"
)

type DoneChan struct {
	done chan struct{}
	once sync.Once
}

func NewDoneChan() *DoneChan {
	return &DoneChan{
		done: make(chan struct{}),
	}
}

func (dc *DoneChan) Close() {
	dc.once.Do(func() {
		close(dc.done)
	})
}

func (dc *DoneChan) Done() chan struct{} {
	return dc.done
}
