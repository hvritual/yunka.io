package threading

/**
 * @BelongProject yunka
 * @BelongPackage threading
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 4:48 下午
 * @Version V1.0
 */

import (
	"bytes"
	"runtime"
	"strconv"
	"yunka.io/pkg/rescue"
)

func GoSafe(fn func()) {
	go RunSafe(fn)
}

// Only for debug, never use it in production
func RoutineId() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	// if error, just return 0
	n, _ := strconv.ParseUint(string(b), 10, 64)

	return n
}

func RunSafe(fn func()) {
	defer rescue.Recover()
	fn()
}