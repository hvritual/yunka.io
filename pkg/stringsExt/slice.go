package stringsExt

import (
	"strings"
)

/**
* @Description: TODO
* @date 2019-06-28
* @version V1.0
 */

func StringBuilder(p ...string) string {
	var b strings.Builder
	l := len(p)
	for i := 0; i < l; i++ {
		b.WriteString(p[i])
	}
	return b.String()
}
