package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const defaultInstrumentationName = "github.com/hvritual/yunka.io/framework"

// Config defines process-level telemetry identity. Export destinations are
// intentionally configured through standard OTEL_* environment variables so
// applications do not depend on an SLS-specific SDK contract.
type Config struct {
	ServiceName         string
	ServiceVersion      string
	InstanceID          string
	Environment         string
	Region              string
	InstrumentationName string
	LogOutput           io.Writer
	LogLevel            slog.Leveler
	IncludeIdentity     bool
	InstallGlobal       bool
	ResourceAttributes  map[string]string
}

type Provider struct {
	config Config
	logger *slog.Logger

	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter

	requestCount      metric.Int64Counter
	requestErrors     metric.Int64Counter
	requestDuration   metric.Float64Histogram
	resilienceRejects metric.Int64Counter
	runtimeEventCount metric.Int64Counter
	circuitState      metric.Int64Gauge
	circuitFailures   metric.Int64Gauge
	rateTokens        metric.Float64Gauge
	loadLimit         metric.Int64Gauge
	loadInFlight      metric.Int64Gauge

	mu       sync.RWMutex
	shutdown bool
}

func New(ctx context.Context, config Config) (*Provider, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	config.ServiceName = strings.TrimSpace(config.ServiceName)
	if config.ServiceName == "" {
		return nil, errors.New("observability: service name is required")
	}
	if config.InstrumentationName == "" {
		config.InstrumentationName = defaultInstrumentationName
	}
	if config.LogOutput == nil {
		config.LogOutput = os.Stdout
	}
	if config.LogLevel == nil {
		level := new(slog.LevelVar)
		level.Set(slog.LevelInfo)
		config.LogLevel = level
	}

	res := resource.NewSchemaless(resourceAttributes(config)...)

	spanExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("observability: configure trace exporter: %w", err)
	}
	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		_ = spanExporter.Shutdown(ctx)
		return nil, fmt.Errorf("observability: configure metric exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(res),
	)

	provider := &Provider{
		config:         config,
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		tracer:         tracerProvider.Tracer(config.InstrumentationName),
		meter:          meterProvider.Meter(config.InstrumentationName),
	}
	provider.logger = newJSONLogger(config)
	if err := provider.initInstruments(); err != nil {
		_ = provider.Shutdown(ctx)
		return nil, err
	}

	if config.InstallGlobal {
		otel.SetTextMapPropagator(defaultPropagator)
		otel.SetTracerProvider(tracerProvider)
		otel.SetMeterProvider(meterProvider)
	}
	return provider, nil
}

func (provider *Provider) initInstruments() error {
	var err error
	provider.requestCount, err = provider.meter.Int64Counter(
		"yunka.request.total",
		metric.WithDescription("Total completed yunka operations."),
	)
	if err != nil {
		return fmt.Errorf("observability: create request counter: %w", err)
	}
	provider.requestErrors, err = provider.meter.Int64Counter(
		"yunka.request.error.total",
		metric.WithDescription("Total yunka operations completed with an error."),
	)
	if err != nil {
		return fmt.Errorf("observability: create request error counter: %w", err)
	}
	provider.requestDuration, err = provider.meter.Float64Histogram(
		"yunka.request.duration",
		metric.WithDescription("Yunka operation duration in seconds."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("observability: create request duration histogram: %w", err)
	}
	provider.resilienceRejects, err = provider.meter.Int64Counter(
		"yunka.resilience.rejection.total",
		metric.WithDescription("Total resilience policy rejections."),
	)
	if err != nil {
		return fmt.Errorf("observability: create resilience counter: %w", err)
	}
	provider.runtimeEventCount, err = provider.meter.Int64Counter(
		"yunka.runtime.event.total",
		metric.WithDescription("Total runtime operational events emitted by yunka."),
	)
	if err != nil {
		return fmt.Errorf("observability: create runtime event counter: %w", err)
	}
	provider.circuitState, err = provider.meter.Int64Gauge(
		"yunka.resilience.circuit.state",
		metric.WithDescription("Circuit state: 0 closed, 1 half-open, 2 open."),
	)
	if err != nil {
		return fmt.Errorf("observability: create circuit state gauge: %w", err)
	}
	provider.circuitFailures, err = provider.meter.Int64Gauge(
		"yunka.resilience.circuit.failures",
		metric.WithDescription("Current circuit breaker failure count."),
	)
	if err != nil {
		return fmt.Errorf("observability: create circuit failures gauge: %w", err)
	}
	provider.rateTokens, err = provider.meter.Float64Gauge(
		"yunka.resilience.rate.tokens",
		metric.WithDescription("Current token bucket balance."),
	)
	if err != nil {
		return fmt.Errorf("observability: create rate token gauge: %w", err)
	}
	provider.loadLimit, err = provider.meter.Int64Gauge(
		"yunka.resilience.load.limit",
		metric.WithDescription("Current adaptive load-shedding concurrency limit."),
	)
	if err != nil {
		return fmt.Errorf("observability: create load limit gauge: %w", err)
	}
	provider.loadInFlight, err = provider.meter.Int64Gauge(
		"yunka.resilience.load.in_flight",
		metric.WithDescription("Current admitted in-flight calls."),
	)
	if err != nil {
		return fmt.Errorf("observability: create load in-flight gauge: %w", err)
	}
	return nil
}

func resourceAttributes(config Config) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", config.ServiceName),
		attribute.String("cloud.provider", "alibaba_cloud"),
	}
	if config.ServiceVersion != "" {
		attrs = append(attrs, attribute.String("service.version", config.ServiceVersion))
	}
	if config.InstanceID != "" {
		attrs = append(attrs, attribute.String("service.instance.id", config.InstanceID))
	}
	if config.Environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment.name", config.Environment))
	}
	if config.Region != "" {
		attrs = append(attrs, attribute.String("cloud.region", config.Region))
	}
	for key, value := range config.ResourceAttributes {
		key = strings.TrimSpace(key)
		if key == "" || reservedResourceAttribute(key) {
			continue
		}
		attrs = append(attrs, attribute.String(key, value))
	}
	return attrs
}

func reservedResourceAttribute(key string) bool {
	switch key {
	case "service.name", "service.version", "service.instance.id",
		"deployment.environment.name", "cloud.provider", "cloud.region":
		return true
	default:
		return false
	}
}

func (provider *Provider) Logger() *slog.Logger {
	if provider == nil || provider.logger == nil {
		return slog.Default()
	}
	return provider.logger
}

func (provider *Provider) ForceFlush(ctx context.Context) error {
	if provider == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return errors.Join(
		provider.tracerProvider.ForceFlush(ctx),
		provider.meterProvider.ForceFlush(ctx),
	)
}

func (provider *Provider) Shutdown(ctx context.Context) error {
	if provider == nil {
		return nil
	}
	provider.mu.Lock()
	if provider.shutdown {
		provider.mu.Unlock()
		return nil
	}
	provider.shutdown = true
	provider.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	return errors.Join(
		provider.meterProvider.Shutdown(ctx),
		provider.tracerProvider.Shutdown(ctx),
	)
}

func (provider *Provider) IsShutdown() bool {
	if provider == nil {
		return true
	}
	provider.mu.RLock()
	defer provider.mu.RUnlock()
	return provider.shutdown
}
