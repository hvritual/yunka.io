package logExt

import (
	"fmt"
	"log"
	"os"
)

type trackLogger struct {
	uuid string
	baseLogger
}

func (l trackLogger) Print(v ...interface{}) {
	l.Debug(v...)
}

func (l trackLogger) Println(v ...interface{}) {
	l.Debug(v...)
}

func (l trackLogger) Fatal(v ...interface{}) {
	l.log(fatalLevel, v...)
	os.Exit(-1)
}

func (l trackLogger) Fatalf(format string, args ...interface{}) {
	l.logf(fatalLevel, format, args...)
	os.Exit(-1)
}

func (l trackLogger) Error(v ...interface{}) {
	l.log(errorLevel, v...)
}

func (l trackLogger) Errorf(format string, args ...interface{}) {
	l.logf(errorLevel, format, args...)
}

func (l trackLogger) Warn(v ...interface{}) {
	l.log(warnLevel, v...)
}

func (l trackLogger) Warnf(format string, args ...interface{}) {
	l.logf(warnLevel, format, args...)
}

func (l trackLogger) Info(v ...interface{}) {
	l.log(infoLevel, v...)
}

func (l trackLogger) Infof(format string, args ...interface{}) {
	l.logf(infoLevel, format, args...)
}

func (l trackLogger) Debug(v ...interface{}) {
	l.log(debugLevel, v...)
}

func (l trackLogger) Debugf(format string, args ...interface{}) {
	l.logf(debugLevel, format, args...)
}

func (l *trackLogger) log(t Type, v ...interface{}) {
	//if l.level|Level(t) != l.level {
	//	return
	//}

	s := fmt.Sprint(v...)
	l._log.Output(l.deep, fmt.Sprintf("[\x1b[0;32;48mrequest ID:%s\x1b[0m] %s",  l.uuid, s))

}

func (l *trackLogger) logf(t Type, format string, v ...interface{}) {
	//if l.level|Level(t) != l.level {
	//	return
	//}
	//if l.level|Level(t) != l.level {
	//	return
	//}

	s := fmt.Sprintf(format, v...)
	l._log.Output(l.deep, s)
}

func NewTrackLogger(uuid string) Logger {
	return &trackLogger{
		uuid: uuid,
		baseLogger: baseLogger{
			_log: log.New(os.Stderr, `[yunka] `, log.LstdFlags|log.Lshortfile),
			deep: 3,
		},
	}
}
