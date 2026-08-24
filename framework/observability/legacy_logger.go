package observability

import (
	"context"
	"fmt"
	"os"

	"yunka.io/pkg/logExt"
)

// LegacyLogger adapts the existing logExt.Logger contract to W4 structured
// logging. Passing the concrete request.Context as ctx is supported: it delegates
// context lookups to its current base context, so trace/operation metadata added
// later in the middleware chain is still visible when the log call executes.
func (provider *Provider) LegacyLogger(ctx context.Context) logExt.Logger {
	if ctx == nil {
		ctx = context.Background()
	}
	return &legacyLogger{provider: provider, ctx: ctx}
}

type legacyLogger struct {
	provider *Provider
	ctx      context.Context
}

func (logger *legacyLogger) Print(values ...interface{})   { logger.info(values...) }
func (logger *legacyLogger) Println(values ...interface{}) { logger.info(values...) }
func (logger *legacyLogger) Fatal(values ...interface{}) {
	logger.err(values...)
	os.Exit(1)
}
func (logger *legacyLogger) Fatalf(format string, args ...interface{}) {
	logger.provider.Error(logger.ctx, fmt.Sprintf(format, args...))
	os.Exit(1)
}
func (logger *legacyLogger) Error(values ...interface{}) { logger.err(values...) }
func (logger *legacyLogger) Errorf(format string, args ...interface{}) {
	logger.provider.Error(logger.ctx, fmt.Sprintf(format, args...))
}
func (logger *legacyLogger) Warn(values ...interface{}) { logger.warn(values...) }
func (logger *legacyLogger) Warnf(format string, args ...interface{}) {
	logger.provider.Warn(logger.ctx, fmt.Sprintf(format, args...))
}
func (logger *legacyLogger) Info(values ...interface{}) { logger.info(values...) }
func (logger *legacyLogger) Infof(format string, args ...interface{}) {
	logger.provider.Info(logger.ctx, fmt.Sprintf(format, args...))
}
func (logger *legacyLogger) Debug(values ...interface{}) {
	logger.provider.Debug(logger.ctx, fmt.Sprint(values...))
}
func (logger *legacyLogger) Debugf(format string, args ...interface{}) {
	logger.provider.Debug(logger.ctx, fmt.Sprintf(format, args...))
}

func (logger *legacyLogger) info(values ...interface{}) {
	logger.provider.Info(logger.ctx, fmt.Sprint(values...))
}
func (logger *legacyLogger) warn(values ...interface{}) {
	logger.provider.Warn(logger.ctx, fmt.Sprint(values...))
}
func (logger *legacyLogger) err(values ...interface{}) {
	logger.provider.Error(logger.ctx, fmt.Sprint(values...))
}
