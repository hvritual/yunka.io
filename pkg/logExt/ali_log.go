package logExt

import (
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"log"
	"yunka.io/pkg/aliLogStore"
)

type aliLog struct {
	conf *aliLogStore.Config
}

func NewAliLog(conf *aliLogStore.Config) (Logger, error) {
	var l = &aliLog{
		conf: conf,
	}
	err := l.client(infoLevel).BuildIndex(&aliLogBody{})
	if err != nil {
		if e, ok := err.(*sls.Error); ok && e.Code == "IndexAlreadyExist" {
		} else {
			return nil, err
		}
	}
	return l, nil
}

func (l *aliLog) Print(v ...interface{})   { l.print(infoLevel, v...) }
func (l *aliLog) Println(v ...interface{}) { l.print(infoLevel, v...) }
func (l *aliLog) Fatal(v ...interface{})   { l.print(errorLevel, v...); log.Fatal(v...) }
func (l *aliLog) Fatalf(format string, args ...interface{}) {
	l.printF(errorLevel, format, args...)
	log.Fatalf(format, args...)
}
func (l *aliLog) Error(v ...interface{})                    { l.print(errorLevel, v...) }
func (l *aliLog) Errorf(format string, args ...interface{}) { l.printF(errorLevel, format, args...) }
func (l *aliLog) Warn(v ...interface{})                     { l.print(warnLevel, v...) }
func (l *aliLog) Warnf(format string, args ...interface{})  { l.printF(warnLevel, format, args...) }
func (l *aliLog) Info(v ...interface{})                     { l.print(infoLevel, v...) }
func (l *aliLog) Infof(format string, args ...interface{})  { l.printF(infoLevel, format, args...) }
func (l *aliLog) Debug(v ...interface{})                    { l.print(debugLevel, v...) }
func (l *aliLog) Debugf(format string, args ...interface{}) { l.printF(debugLevel, format, args...) }

func (l *aliLog) print(level Type, v ...interface{}) {
	l.put(level, &aliLogBody{
		content: fmt.Sprintf("%v", v...),
	})
}

func (l *aliLog) printF(level Type, format string, args ...interface{}) {
	l.put(level, &aliLogBody{
		content: fmt.Sprintf(format, args...),
	})
}

var syncChan = make(chan struct{}, 10)

func (l *aliLog) put(level Type, body *aliLogBody) {
	syncChan <- struct{}{}
	go l.goPut(level, body)
	//_ = l.client(level).Put(body)
}

func (l *aliLog) goPut(level Type, body *aliLogBody) {
	l.client(level).Put(body)
	<-syncChan
}

func (l *aliLog) client(level Type) *aliLogStore.Log {
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
	l.conf.Topic = topic
	return aliLogStore.NewAliLogStore(l.conf)
}
