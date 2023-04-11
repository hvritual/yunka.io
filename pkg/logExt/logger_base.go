package logExt

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
)

type (
	// Level log level
	Level int
	// Type log type
	Type int
)

const (
	fatalLevel = Type(0x1)
	errorLevel = Type(0x2)
	warnLevel  = Type(0x4)
	infoLevel  = Type(0x8)
	debugLevel = Type(0x10)
)

type baseLogger struct {
	_log  *log.Logger
	level Level
	deep  int
}

func (l baseLogger) Print(v ...interface{}) {
	l.Debug(v...)
}

func (l baseLogger) Println(v ...interface{}) {
	l.Debug(v...)
}

func (l baseLogger) Fatal(v ...interface{}) {
	l.log(fatalLevel, v...)
	os.Exit(-1)
}

func (l baseLogger) Fatalf(format string, args ...interface{}) {
	l.logf(fatalLevel, format, args...)
	os.Exit(-1)
}

func (l baseLogger) Error(v ...interface{}) {
	l.log(errorLevel, v...)
}

func (l baseLogger) Errorf(format string, args ...interface{}) {
	l.logf(errorLevel, format, args...)
}

func (l baseLogger) Warn(v ...interface{}) {
	l.log(warnLevel, v...)
}

func (l baseLogger) Warnf(format string, args ...interface{}) {
	l.logf(warnLevel, format, args...)
}

func (l baseLogger) Info(v ...interface{}) {
	l.log(infoLevel, v...)
}

func (l baseLogger) Infof(format string, args ...interface{}) {
	l.logf(infoLevel, format, args...)
}

func (l baseLogger) Debug(v ...interface{}) {
	l.log(debugLevel, v...)
}

func (l baseLogger) Debugf(format string, args ...interface{}) {
	l.logf(debugLevel, format, args...)
}

func (l *baseLogger) log(t Type, v ...interface{}) {
	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		return
	}

	s := fmt.Sprintf("method:%s, fileName:%s:%d, info:%s\n",
		runtime.FuncForPC(pc).Name(),
		path.Base(file),
		line,
		fmt.Sprint(v...))
	l._log.Output(3, s)

}

func (l *baseLogger) logf(t Type, format string, v ...interface{}) {
	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		return
	}

	s := fmt.Sprintf("method:%s, fileName:%s:%d, info:%s\n",
		runtime.FuncForPC(pc).Name(),
		path.Base(file),
		line,
		fmt.Sprintf(format, v...))
	l._log.Output(3, s)
}

func NewBaseLogger() Logger {
	return &baseLogger{
		_log: log.New(os.Stderr, `[yunka] `, log.LstdFlags|log.Lshortfile),
		deep: 4,
	}
}
