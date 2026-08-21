// Package log preserves the narrow historical go-kit logging API required by
// aliyun-log-go-sdk while delegating all behavior to the split go-kit/log module.
package log

import (
	"io"
	"time"

	split "github.com/go-kit/log"
)

type Logger = split.Logger
type Valuer = split.Valuer
type SwapLogger = split.SwapLogger

var (
	ErrMissingValue     = split.ErrMissingValue
	DefaultTimestamp    = split.DefaultTimestamp
	DefaultTimestampUTC = split.DefaultTimestampUTC
	DefaultCaller       = split.DefaultCaller
)

func With(logger Logger, keyvals ...interface{}) Logger { return split.With(logger, keyvals...) }
func WithPrefix(logger Logger, keyvals ...interface{}) Logger {
	return split.WithPrefix(logger, keyvals...)
}
func WithSuffix(logger Logger, keyvals ...interface{}) Logger {
	return split.WithSuffix(logger, keyvals...)
}
func NewLogfmtLogger(writer io.Writer) Logger  { return split.NewLogfmtLogger(writer) }
func NewJSONLogger(writer io.Writer) Logger    { return split.NewJSONLogger(writer) }
func NewNopLogger() Logger                     { return split.NewNopLogger() }
func NewSyncWriter(writer io.Writer) io.Writer { return split.NewSyncWriter(writer) }
func NewSyncLogger(logger Logger) Logger       { return split.NewSyncLogger(logger) }
func Timestamp(now func() time.Time) Valuer    { return split.Timestamp(now) }
func TimestampFormat(now func() time.Time, layout string) Valuer {
	return split.TimestampFormat(now, layout)
}
func Caller(depth int) Valuer { return split.Caller(depth) }
