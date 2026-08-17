package logExt

import (
	"fmt"
	"testing"
)

/**
 * @BelongProject yunka
 * @BelongPackage logExt
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/9 11:11 下午
 * @Version V1.0
 */

func Test_trackLogger_log(t *testing.T) {
	fmt.Println(fmt.Sprintf("\x1b[0;32;48mrequest ID:%s\x1b[0m%s", "testPrintColor", "test"))
}