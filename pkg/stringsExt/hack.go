package stringsExt

import (
	"reflect"
	"regexp"
	"strings"
	"unsafe"
)

/**
* @Description: TODO
* @date 2019-07-16
* @version V1.0
 */
// SliceToString slice to string with out data copy
func SliceToString(b []byte) (s string) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pstring.Data = pbytes.Data
	pstring.Len = pbytes.Len
	return
}

// StringToSlice string to slice with out data copy
func StringToSlice(s string) (b []byte) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pbytes.Data = pstring.Data
	pbytes.Len = pstring.Len
	pbytes.Cap = pstring.Len
	return
}

var (
	regChar = regexp.MustCompile(`[\W|_]{1,}`)
)

func StringStrip(val string) string {
	if val == `` {
		return val
	}
	return regChar.ReplaceAllString(strings.TrimSpace(val), ``)
}
