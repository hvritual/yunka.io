package stringsExt

import "math/rand"

/**
* @Description: TODO
* @date 2019-06-28
* @version V1.0
 */
func RandString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(RandInt(65, 90))
	}
	return string(bytes)
}

func RandInt(min int, max int) int {
	return min + rand.Intn(max-min)
}
