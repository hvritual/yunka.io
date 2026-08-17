package httpExt

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

/**
* @Description: TODO
* @date 2019-08-02
* @version V1.0
 */
var (
	client = http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}
)

const maxResponseBytes int64 = 16 << 20

func do(req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := io.LimitReader(resp.Body, maxResponseBytes+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return body, nil
}

func Post(url string, headers map[string]string, data interface{}) ([]byte, error) {
	reader := io.Reader(nil)
	if data != nil {
		bys, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewBuffer(bys)
	}

	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return do(req)
}
