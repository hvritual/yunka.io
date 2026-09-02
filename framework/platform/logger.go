package platform

import "github.com/hvritual/yunka.io/pkg/logExt"

type moduleLogger struct {
	delegate logExt.Logger
	module   string
}

func newModuleLogger(delegate logExt.Logger, module string) logExt.Logger {
	if delegate == nil {
		return nil
	}
	return &moduleLogger{delegate: delegate, module: module}
}

func (logger *moduleLogger) values(values []interface{}) []interface{} {
	result := make([]interface{}, 0, len(values)+1)
	result = append(result, "module="+logger.module+" ")
	return append(result, values...)
}

func (logger *moduleLogger) Print(values ...interface{}) {
	logger.delegate.Print(logger.values(values)...)
}
func (logger *moduleLogger) Println(values ...interface{}) {
	logger.delegate.Println(logger.values(values)...)
}
func (logger *moduleLogger) Fatal(values ...interface{}) {
	logger.delegate.Fatal(logger.values(values)...)
}
func (logger *moduleLogger) Error(values ...interface{}) {
	logger.delegate.Error(logger.values(values)...)
}
func (logger *moduleLogger) Warn(values ...interface{}) {
	logger.delegate.Warn(logger.values(values)...)
}
func (logger *moduleLogger) Info(values ...interface{}) {
	logger.delegate.Info(logger.values(values)...)
}
func (logger *moduleLogger) Debug(values ...interface{}) {
	logger.delegate.Debug(logger.values(values)...)
}
func (logger *moduleLogger) Fatalf(format string, values ...interface{}) {
	logger.delegate.Fatalf("module=%s "+format, append([]interface{}{logger.module}, values...)...)
}
func (logger *moduleLogger) Errorf(format string, values ...interface{}) {
	logger.delegate.Errorf("module=%s "+format, append([]interface{}{logger.module}, values...)...)
}
func (logger *moduleLogger) Warnf(format string, values ...interface{}) {
	logger.delegate.Warnf("module=%s "+format, append([]interface{}{logger.module}, values...)...)
}
func (logger *moduleLogger) Infof(format string, values ...interface{}) {
	logger.delegate.Infof("module=%s "+format, append([]interface{}{logger.module}, values...)...)
}
func (logger *moduleLogger) Debugf(format string, values ...interface{}) {
	logger.delegate.Debugf("module=%s "+format, append([]interface{}{logger.module}, values...)...)
}
