package logExt

import (
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/golang/protobuf/proto"
	"os"
	"time"
)

type aliLogStore struct {
	l        *sls.LogStore
	syncChan chan struct{}
	chanSize int
	source   string
}

type LogStoreOption func(l *aliLogStore)

func WithSource(source string) LogStoreOption {
	return func(l *aliLogStore) {
		l.source = source
	}
}

func WithSize(size int) LogStoreOption {
	return func(l *aliLogStore) {
		l.chanSize = size
	}
}

func NewAliLogger(store *sls.LogStore, opts ...LogStoreOption) Logger {
	return newAliLogStore(store, opts...)
}

func newAliLogStore(store *sls.LogStore, opts ...LogStoreOption) *aliLogStore {
	var w = &aliLogStore{
		l:        store,
		chanSize: 10,
		source:   "default",
	}

	for i := 0; i < len(opts); i++ {
		opts[i](w)
	}

	w.syncChan = make(chan struct{}, w.chanSize)

	return w
}

func (l *aliLogStore) Print(v ...interface{})   { l.print(infoLevel, v...) }
func (l *aliLogStore) Println(v ...interface{}) { l.print(infoLevel, v...) }
func (l *aliLogStore) Fatal(v ...interface{})   { l.print(errorLevel, v...); os.Exit(1) }
func (l *aliLogStore) Fatalf(format string, args ...interface{}) {
	l.printF(errorLevel, format, args...)
	os.Exit(1)
}
func (l *aliLogStore) Error(v ...interface{}) { l.print(errorLevel, v...) }
func (l *aliLogStore) Errorf(format string, args ...interface{}) {
	l.printF(errorLevel, format, args...)
}
func (l *aliLogStore) Warn(v ...interface{})                    { l.print(warnLevel, v...) }
func (l *aliLogStore) Warnf(format string, args ...interface{}) { l.printF(warnLevel, format, args...) }
func (l *aliLogStore) Info(v ...interface{})                    { l.print(infoLevel, v...) }
func (l *aliLogStore) Infof(format string, args ...interface{}) { l.printF(infoLevel, format, args...) }
func (l *aliLogStore) Debug(v ...interface{})                   { l.print(debugLevel, v...) }
func (l *aliLogStore) Debugf(format string, args ...interface{}) {
	l.printF(debugLevel, format, args...)
}

func (l *aliLogStore) print(level Type, v ...interface{}) {
	l.put(level, map[string]string{
		"content": fmt.Sprintf("%v", v...),
	})
}

func (l *aliLogStore) printF(level Type, format string, args ...interface{}) {
	l.put(level, map[string]string{
		"content": fmt.Sprintf(format, args...),
	})
}

func (l *aliLogStore) put(level Type, body map[string]string) {
	l.syncChan <- struct{}{}
	go l.goPut(level, body)
}

func (l *aliLogStore) goPut(level Type, body map[string]string) {
	var contents = make([]*sls.LogContent, 0)

	for key, val := range body {
		contents = append(contents, &sls.LogContent{
			Key:   proto.String(key),
			Value: proto.String(val),
		})
	}

	var now = uint32(time.Now().Unix())

	var log = &sls.Log{
		Time:     &now,
		Contents: contents,
	}

	var topic = "info"
	switch level {
	case debugLevel:
		topic = "debug"
	case infoLevel:
		topic = "info"
	case warnLevel:
		topic = "warn"
	case errorLevel:
		topic = "error"
	}

	err := l.l.PutLogs(&sls.LogGroup{
		Logs:   []*sls.Log{log},
		Topic:  proto.String(topic),
		Source: proto.String(l.source),
	})
	if err != nil {
		fmt.Println(err)
	}

	<-l.syncChan
}
