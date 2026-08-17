package cryptoExt

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"strings"
)

func MD5(datas ...string) string {
	var buffer bytes.Buffer
	for _, data := range datas {
		buffer.WriteString(data)
	}
	return MD5Byte(buffer.Bytes())
}

func MD5Byte(bys []byte) string {
	h := md5.New()

	h.Write(bys)
	return strings.ToLower(hex.EncodeToString(h.Sum(nil)))
}
