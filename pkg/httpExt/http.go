package httpExt

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/buger/jsonparser"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"time"
	"yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject mqttProxy
 * @BelongPackage device
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/4/29 8:59 下午
 * @Version V1.0
 */

const (
	BasicJsonCode = `code`
	BasicJsonMsg  = `msg`
	BasicJsonData = `data`
	//dnsServerAddress = "8.8.8.8:53"
)

func dnsPatch() {
	http.DefaultTransport = &http.Transport{
		//DialContext: (&net.Dialer{
		//	Timeout:   10 * time.Second,
		//	KeepAlive: 10 * time.Second,
		//	DualStack: true,
		//	Resolver: &net.Resolver{
		//		PreferGo: true,
		//		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		//			d := net.Dialer{}
		//			return d.DialContext(ctx, "udp", dnsServerAddress)
		//		},
		//	},
		//}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}

func init() {
	dnsPatch()
}

type HttpBaseMsg struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func GetJSON(path string, params map[string]string, response interface{}) error {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if params != nil {
		path = path + `?` + p.Encode()
	}

	req, _ := http.NewRequest("GET", path, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.New("network is unreachable")
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	strValue, _ := jsonparser.GetString(body, BasicJsonCode)
	if strValue != `0` {
		value, _ := jsonparser.GetString(body, BasicJsonMsg)
		return errors.New(value)
	}

	value, dataType, _, err := jsonparser.Get(body, BasicJsonData)
	switch dataType {
	case jsonparser.String:

		d := reflect.ValueOf(response).Elem() // d refers to the variable x
		val := d.Addr().Interface().(*string)
		*val = stringsExt.SliceToString(value)
		return nil
	}
	if response == nil {
		return nil
	}
	err = json.Unmarshal(value, &response)
	if err != nil {
		return err
	}
	return nil
}
func GetID(path string, params map[string]string, headers map[string]string) (string, error) {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if params != nil {
		path = path + `?` + p.Encode()
	}

	req, _ := http.NewRequest("GET", path, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ``, errors.New("network is unreachable")
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	strValue, _ := jsonparser.GetString(body, BasicJsonCode)
	if strValue != `0` {
		value, _ := jsonparser.GetString(body, BasicJsonMsg)
		return ``, errors.New(value)
	}

	value, dataType, _, err := jsonparser.Get(body, BasicJsonData)
	switch dataType {
	case jsonparser.String:
		return stringsExt.SliceToString(value), nil
	}
	return ``, nil
}

func GetDirect(path string, params map[string]string, data interface{}) error {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if params != nil {
		path = path + `?` + p.Encode()
	}

	req, _ := http.NewRequest("GET", path, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.New("network is unreachable")
	}

	defer res.Body.Close()

	return json.NewDecoder(res.Body).Decode(data)
}

func Get(path string, params map[string]string) ([]byte, error) {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if params != nil {
		path = path + `?` + p.Encode()
	}

	req, _ := http.NewRequest("GET", path, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("network is unreachable")
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	strValue, _ := jsonparser.GetString(body, BasicJsonCode)
	if strValue != `0` {
		value, _ := jsonparser.GetString(body, BasicJsonMsg)
		return nil, errors.New(value)
	}

	value, _, _, err := jsonparser.Get(body, BasicJsonData)
	return value, err
}

func PostJSON(path string, headers, params map[string]string, data interface{}) ([]byte, error) {
	bys, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	body, _ := PostJSONDirect(path, headers, params, bys)

	strValue, _ := jsonparser.GetString(body, BasicJsonCode)
	if strValue != `0` {
		value, _ := jsonparser.GetString(body, BasicJsonMsg)
		return nil, errors.New(value)
	}

	value, _, _, err := jsonparser.Get(body, BasicJsonData)
	return value, err
}

func PostJSONDirect(path string, headers, params map[string]string, bys []byte) ([]byte, error) {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if params != nil {
		path = path + `?` + p.Encode()
	}
	var buf io.Reader
	if len(bys) != 0 {
		buf = bytes.NewBuffer(bys)
	}

	req, _ := http.NewRequest(http.MethodPost, path, buf)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("accept", "application/json")
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("network is unreachable")
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	return body, err
}

func Put(path string, params map[string]string) ([]byte, error) {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if params != nil {
		path = path + `?` + p.Encode()
	}

	req, _ := http.NewRequest(http.MethodPut, path, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("network is unreachable")
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	strValue, err := jsonparser.GetString(body, BasicJsonCode)
	if err != nil {
		return nil, err
	}
	if strValue != `0` {
		value, _ := jsonparser.GetString(body, BasicJsonMsg)
		return nil, errors.New(value)
	}

	value, _, _, err := jsonparser.Get(body, BasicJsonData)
	return value, err
}

func PostJSONObject(path string, params map[string]string, obj interface{}) ([]byte, error) {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if params != nil {
		path = path + `?` + p.Encode()
	}
	var buf io.Reader
	if obj != nil {
		bys, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(bys)
	}

	req, _ := http.NewRequest(http.MethodPost, path, buf)

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("network is unreachable")
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	strValue, _ := jsonparser.GetString(body, BasicJsonCode)
	if strValue != `0` {
		value, _ := jsonparser.GetString(body, BasicJsonMsg)
		return nil, errors.New(value)
	}

	value, _, _, err := jsonparser.Get(body, BasicJsonData)
	return value, err
}
