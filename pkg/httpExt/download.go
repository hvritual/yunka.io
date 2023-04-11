package httpExt

import (
	"io"
	"net/http"
)

/**
 * @BelongProject quanxingaopin
 * @BelongPackage netExt
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/4 2:43 下午
 * @Version V1.0
 */

type IOFunc func(reader io.ReadCloser, size int64) error

func HttpDownloadIO(url string, fn IOFunc) error {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	return fn(resp.Body, resp.ContentLength)
}
