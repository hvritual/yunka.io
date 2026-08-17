package httpExt

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/buger/jsonparser"
	"io"
	"net/http"
	"net/url"
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

	if len(params) > 0 {
		path = path + `?` + p.Encode()
	}

	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	body, err := do(req)
	if err != nil {
		return err
	}

	strValue, _ := jsonparser.GetString(body, BasicJsonCode)
	if strValue != `0` {
		value, _ := jsonparser.GetString(body, BasicJsonMsg)
		return errors.New(value)
	}

	value, dataType, _, err := jsonparser.Get(body, BasicJsonData)
	if err != nil {
		return err
	}
	if response == nil {
		return nil
	}
	switch dataType {
	case jsonparser.String:
		val, ok := response.(*string)
		if !ok {
			return errors.New("string response requires *string destination")
		}
		*val = stringsExt.SliceToString(value)
		return nil
	}
	err = json.Unmarshal(value, response)
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

	if len(params) > 0 {
		path = path + `?` + p.Encode()
	}

	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	body, err := do(req)
	if err != nil {
		return ``, err
	}

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

	if len(params) > 0 {
		path = path + `?` + p.Encode()
	}

	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	body, err := do(req)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, data)
}

func Get(path string, params map[string]string) ([]byte, error) {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if len(params) > 0 {
		path = path + `?` + p.Encode()
	}

	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	body, err := do(req)
	if err != nil {
		return nil, err
	}

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

	body, err := PostJSONDirect(path, headers, params, bys)
	if err != nil {
		return nil, err
	}

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

	if len(params) > 0 {
		path = path + `?` + p.Encode()
	}
	var buf io.Reader
	if len(bys) != 0 {
		buf = bytes.NewBuffer(bys)
	}

	req, err := http.NewRequest(http.MethodPost, path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Add("content-type", "application/json")
	req.Header.Add("accept", "application/json")
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	return do(req)
}

func Put(path string, params map[string]string) ([]byte, error) {
	var p = url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}

	if len(params) > 0 {
		path = path + `?` + p.Encode()
	}

	req, err := http.NewRequest(http.MethodPut, path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	body, err := do(req)
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

	if len(params) > 0 {
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

	req, err := http.NewRequest(http.MethodPost, path, buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	body, err := do(req)
	if err != nil {
		return nil, err
	}

	strValue, _ := jsonparser.GetString(body, BasicJsonCode)
	if strValue != `0` {
		value, _ := jsonparser.GetString(body, BasicJsonMsg)
		return nil, errors.New(value)
	}

	value, _, _, err := jsonparser.Get(body, BasicJsonData)
	return value, err
}
