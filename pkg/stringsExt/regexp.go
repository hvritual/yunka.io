package stringsExt

import "regexp"

/**
 * @BelongProject yunka
 * @BelongPackage stringsExt
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/29 8:09 下午
 * @Version V1.0
 */

const (
	regular = "^((13[0-9])|(14[5|7|8])|(15([0-3]|[5-9]))|(16[5|6])|(17([0-3]|[5-8]))|(18[0-9]))\\d{8}$"
	pattern = "\\d+"
)

var (
	reg = regexp.MustCompile(regular)
)

func IsPhone(phone string) bool {
	return reg.MatchString(phone)
}
