// Package level preserves the historical go-kit level API required by
// aliyun-log-go-sdk and delegates to github.com/go-kit/log/level.
package level

import (
	compatlog "github.com/go-kit/kit/log"
	split "github.com/go-kit/log/level"
)

type Value = split.Value
type Option = split.Option

var ErrInvalidLevelString = split.ErrInvalidLevelString

func Error(logger compatlog.Logger) compatlog.Logger { return split.Error(logger) }
func Warn(logger compatlog.Logger) compatlog.Logger  { return split.Warn(logger) }
func Info(logger compatlog.Logger) compatlog.Logger  { return split.Info(logger) }
func Debug(logger compatlog.Logger) compatlog.Logger { return split.Debug(logger) }
func NewFilter(next compatlog.Logger, options ...Option) compatlog.Logger {
	return split.NewFilter(next, options...)
}
func Allow(value Value) Option                        { return split.Allow(value) }
func AllowAll() Option                                { return split.AllowAll() }
func AllowDebug() Option                              { return split.AllowDebug() }
func AllowInfo() Option                               { return split.AllowInfo() }
func AllowWarn() Option                               { return split.AllowWarn() }
func AllowError() Option                              { return split.AllowError() }
func AllowNone() Option                               { return split.AllowNone() }
func Parse(value string) (Value, error)               { return split.Parse(value) }
func ParseDefault(value string, fallback Value) Value { return split.ParseDefault(value, fallback) }
func ErrNotAllowed(err error) Option                  { return split.ErrNotAllowed(err) }
func SquelchNoLevel(squelch bool) Option              { return split.SquelchNoLevel(squelch) }
func ErrNoLevel(err error) Option                     { return split.ErrNoLevel(err) }
func NewInjector(next compatlog.Logger, value Value) compatlog.Logger {
	return split.NewInjector(next, value)
}
func Key() interface{}  { return split.Key() }
func ErrorValue() Value { return split.ErrorValue() }
func WarnValue() Value  { return split.WarnValue() }
func InfoValue() Value  { return split.InfoValue() }
func DebugValue() Value { return split.DebugValue() }
