package logExt

import (
	sls "github.com/aliyun/aliyun-log-go-sdk"
)

type Trace interface {
	Set(string)
	Get() string
}

type aliTraceLog struct {
	*aliLogStore
	traceId string
}

func NewTraceLogger(store *sls.LogStore, traceId string, opts ...LogStoreOption) Logger {
	var w = newAliLogStore(store, opts...)

	return &aliTraceLog{w, traceId}
}

func (l *aliTraceLog) Set(str string) {
	l.traceId = str
}
func (l *aliTraceLog) Get() string {
	return l.traceId
}

func (l *aliTraceLog) put(level Type, body map[string]string) {
	l.syncChan <- struct{}{}
	body["trace_id"] = l.traceId
	go l.goPut(level, body)
}
