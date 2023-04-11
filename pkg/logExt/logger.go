package logExt

/**
 * @BelongProject yunka
 * @BelongPackage application
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 3:43 下午
 * @Version V1.0
 */

type Logger interface {
	Print(v ...interface{})
	Println(v ...interface{})
	Fatal(v ...interface{})
	Fatalf(format string, args ...interface{})
	Error(v ...interface{})
	Errorf(format string, args ...interface{})
	Warn(v ...interface{})
	Warnf(format string, args ...interface{})
	Info(v ...interface{})
	Infof(format string, args ...interface{})
	Debug(v ...interface{})
	Debugf(format string, args ...interface{})
}

var (
	defaultLog = NewBaseLogger()
)

func Print(v ...interface{}) {
	defaultLog.Print(v...)
}
func Println(v ...interface{}) {
	defaultLog.Println(v...)
}

func Fatal(v ...interface{}) {
	defaultLog.Fatal(v...)
}

func Fatalf(format string, args ...interface{}) {
	defaultLog.Fatalf(format, args...)
}

func Error(v ...interface{}) {
	defaultLog.Error(v...)
}

func Errorf(format string, args ...interface{}) {
	defaultLog.Errorf(format, args...)
}

func Warn(v ...interface{}) {
	defaultLog.Fatal(v...)
}

func Warnf(format string, args ...interface{}) {
	defaultLog.Warnf(format, args...)
}

func Info(v ...interface{}) {
	defaultLog.Fatal(v...)
}

func Infof(format string, args ...interface{}) {
	defaultLog.Infof(format, args...)
}

func Debug(v ...interface{}) {
	defaultLog.Debug(v...)
}

func Debugf(format string, args ...interface{}) {
	defaultLog.Debugf(format, args...)
}
