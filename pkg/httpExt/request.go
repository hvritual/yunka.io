package httpExt

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

/**
* @Description: TODO
* @date 2019-08-02
* @version V1.0
 */
var (
	client = http.Client{}
)

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

	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	bys, err := ioutil.ReadAll(resp.Body)
	if resp.Body != nil {
		resp.Body.Close()
	}
	return bys, err
}
