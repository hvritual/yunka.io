package stringsExt

import (
	"strings"
	"unicode"
)

/**
 * @BelongProject yunka
 * @BelongPackage stringsExt
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 10:47 上午
 * @Version V1.0
 */

// 驼峰式写法转为下划线写法
func UnderscoreName(name string) string {
	buffer := strings.Builder{}
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i != 0 {
				buffer.WriteString(`_`)
			}
			buffer.WriteRune(unicode.ToLower(r))
		} else {
			buffer.WriteRune(r)
		}
	}

	return buffer.String()
}

func Ucfirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}

func Lcfirst(str string) string {
	for i, v := range str {
		return string(unicode.ToLower(v)) + str[i+1:]
	}
	return ""
}

// 下划线写法转为驼峰写法
func CamelName(name string) string {
	name = strings.Replace(name, "_", " ", -1)
	name = strings.Title(name)
	return strings.Replace(name, " ", "", -1)
}

func CamelNameFlag(name, flag string) string {
	name = strings.Replace(name, flag, " ", -1)
	name = strings.Title(name)
	return strings.Replace(name, " ", "", -1)
}

func SearchString(slice []string, s string) int {
	for i, v := range slice {
		if s == v {
			return i
		}
	}

	return -1
}
