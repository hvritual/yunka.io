package rescue

/**
 * @BelongProject yunka
 * @BelongPackage rescue
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 4:49 下午
 * @Version V1.0
 */

func Recover(cleanups ...func()) {
	for _, cleanup := range cleanups {
		cleanup()
	}

	if p := recover(); p != nil {
		// TODO
	}
}
