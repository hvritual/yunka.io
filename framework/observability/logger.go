package observability

import (
	"context"
	"log/slog"
	"sort"
)

func newJSONLogger(config Config) *slog.Logger {
	handler := slog.NewJSONHandler(config.LogOutput, &slog.HandlerOptions{Level: config.LogLevel})
	logger := slog.New(handler)
	args := []any{
		"service.name", config.ServiceName,
		"cloud.provider", "alibaba_cloud",
	}
	if config.ServiceVersion != "" {
		args = append(args, "service.version", config.ServiceVersion)
	}
	if config.InstanceID != "" {
		args = append(args, "service.instance.id", config.InstanceID)
	}
	if config.Environment != "" {
		args = append(args, "deployment.environment.name", config.Environment)
	}
	if config.Region != "" {
		args = append(args, "cloud.region", config.Region)
	}
	keys := make([]string, 0, len(config.ResourceAttributes))
	for key := range config.ResourceAttributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "" || reservedResourceAttribute(key) {
			continue
		}
		args = append(args, key, config.ResourceAttributes[key])
	}
	return logger.With(args...)
}

func (provider *Provider) Debug(ctx context.Context, message string, fields ...any) {
	provider.log(ctx, slog.LevelDebug, message, fields...)
}

func (provider *Provider) Info(ctx context.Context, message string, fields ...any) {
	provider.log(ctx, slog.LevelInfo, message, fields...)
}

func (provider *Provider) Warn(ctx context.Context, message string, fields ...any) {
	provider.log(ctx, slog.LevelWarn, message, fields...)
}

func (provider *Provider) Error(ctx context.Context, message string, fields ...any) {
	provider.log(ctx, slog.LevelError, message, fields...)
}

func (provider *Provider) log(ctx context.Context, level slog.Level, message string, fields ...any) {
	if provider == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	base := slogContextFields(ctx, provider.config.IncludeIdentity)
	base = append(base, fields...)
	provider.Logger().Log(ctx, level, message, base...)
}
