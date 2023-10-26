package logExt

import (
	"fmt"
	"log"
	"yunka.io/pkg/aliLogStore"
)

type aliTraceLog struct {
	traceId string
	aliLg   *aliLog
}

func (l *aliTraceLog) Set(str string) {
	l.traceId = str
}
func (l *aliTraceLog) Get() string {
	return l.traceId
}

func (l *aliTraceLog) Print(v ...interface{})   { l.print(infoLevel, v...) }
func (l *aliTraceLog) Println(v ...interface{}) { l.print(infoLevel, v...) }
func (l *aliTraceLog) Fatal(v ...interface{})   { l.print(errorLevel, v...); log.Fatal(v...) }
func (l *aliTraceLog) Fatalf(format string, args ...interface{}) {
	l.printF(errorLevel, format, args...)
	log.Fatalf(format, args...)
}
func (l *aliTraceLog) Error(v ...interface{}) { l.print(errorLevel, v...) }
func (l *aliTraceLog) Errorf(format string, args ...interface{}) {
	l.printF(errorLevel, format, args...)
}
func (l *aliTraceLog) Warn(v ...interface{})                    { l.print(warnLevel, v...) }
func (l *aliTraceLog) Warnf(format string, args ...interface{}) { l.printF(warnLevel, format, args...) }
func (l *aliTraceLog) Info(v ...interface{})                    { l.print(infoLevel, v...) }
func (l *aliTraceLog) Infof(format string, args ...interface{}) { l.printF(infoLevel, format, args...) }
func (l *aliTraceLog) Debug(v ...interface{})                   { l.print(debugLevel, v...) }
func (l *aliTraceLog) Debugf(format string, args ...interface{}) {
	l.printF(debugLevel, format, args...)
}

func (l *aliTraceLog) print(level Type, v ...interface{}) {
	l.put(level, aliLogBody{
		content: fmt.Sprintf("%v", v...),
		traceId: l.traceId,
	})
}

func (l *aliTraceLog) printF(level Type, format string, args ...interface{}) {
	l.put(level, aliLogBody{
		content: fmt.Sprintf(format, args...),
		traceId: l.traceId,
	})
}

func (l *aliTraceLog) put(level Type, body aliLogBody) {
	syncChan <- struct{}{}
	go l.goPut(level, body)
	//_ = l.client(level).Put(body)
}

func (l *aliTraceLog) goPut(level Type, body aliLogBody) {
	l.client(level).Put(body)
	<-syncChan
}

func (l *aliTraceLog) client(level Type) *aliLogStore.Log {
	return l.aliLg.client(level)
}

// ---------------------------------------------------------------------

func Copy(lg Logger) Logger {
	switch lg.(type) {
	case *aliLog:
		return &aliTraceLog{
			"",
			lg.(*aliLog),
		}
	}
	return lg
}
