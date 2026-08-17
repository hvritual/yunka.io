package httpExt

import (
	"fmt"
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
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	return fn(io.NopCloser(io.LimitReader(resp.Body, maxResponseBytes)), resp.ContentLength)
}
