package selector

import (
	"context"
	"math"
	"time"

	"github.com/hvritual/yunka.io/pkg/registry"
)

// AdaptiveMode controls the stateful node choice algorithm.
type AdaptiveMode string

const (
	AdaptiveP2C          AdaptiveMode = "p2c"
	AdaptiveEWMA         AdaptiveMode = "ewma"
	AdaptiveLeastRequest AdaptiveMode = "least_request"
)

// FeedbackClassifier maps a completed call error into passive-health feedback.
type FeedbackClassifier func(error) Outcome

// OutlierOptions controls passive ejection. MaxEjectionPercent prevents a
// small service from ejecting every node at once.
type OutlierOptions struct {
	ConsecutiveFailures int
	BaseEjectionTime    time.Duration
	MaxEjectionTime     time.Duration
	MaxEjectionPercent  int
	FailClosed          bool
}

// AdaptiveOptions controls W5 stateful selection.
type AdaptiveOptions struct {
	Enabled        bool
	Mode           AdaptiveMode
	EWMAAlpha      float64
	InitialLatency time.Duration
	FailurePenalty float64
	Outlier        OutlierOptions
	Classify       FeedbackClassifier
	Seed           int64
	Now            func() time.Time
}

type Options struct {
	Registry registry.Registry
	Strategy Strategy
	Adaptive AdaptiveOptions

	Context context.Context
}

type SelectOptions struct {
	Filters      []Filter
	Strategy     Strategy
	AdaptiveMode *AdaptiveMode

	Context context.Context
}

type Option func(*Options)
type SelectOption func(*SelectOptions)

func Registry(r registry.Registry) Option {
	return func(o *Options) { o.Registry = r }
}

func SetStrategy(fn Strategy) Option {
	return func(o *Options) { o.Strategy = fn }
}

// EnableAdaptive opts a selector into W5 stateful selection. NewSelector keeps
// the legacy Random default; NewAdaptiveSelector enables P2C by default.
func EnableAdaptive(mode AdaptiveMode) Option {
	return func(o *Options) {
		o.Adaptive.Enabled = true
		o.Adaptive.Mode = mode
	}
}

// WithAdaptiveConfig replaces adaptive configuration. Zero fields are filled
// with safe defaults during selector construction/Init.
func WithAdaptiveConfig(config AdaptiveOptions) Option {
	return func(o *Options) { o.Adaptive = config }
}

func WithFilter(fn ...Filter) SelectOption {
	return func(o *SelectOptions) { o.Filters = append(o.Filters, fn...) }
}

func WithStrategy(fn Strategy) SelectOption {
	return func(o *SelectOptions) { o.Strategy = fn }
}

// WithAdaptiveMode overrides the configured adaptive mode for one Select/Pick.
func WithAdaptiveMode(mode AdaptiveMode) SelectOption {
	return func(o *SelectOptions) { o.AdaptiveMode = &mode }
}

func defaultAdaptiveOptions() AdaptiveOptions {
	return AdaptiveOptions{
		Mode:           AdaptiveP2C,
		EWMAAlpha:      0.2,
		InitialLatency: 100 * time.Millisecond,
		FailurePenalty: 1,
		Outlier: OutlierOptions{
			ConsecutiveFailures: 5,
			BaseEjectionTime:    30 * time.Second,
			MaxEjectionTime:     5 * time.Minute,
			MaxEjectionPercent:  50,
		},
		Classify: defaultFeedbackClassifier,
		Now:      time.Now,
	}
}

func normalizeAdaptive(config AdaptiveOptions) AdaptiveOptions {
	defaults := defaultAdaptiveOptions()
	if config.Mode == "" {
		config.Mode = defaults.Mode
	}
	if config.EWMAAlpha <= 0 || config.EWMAAlpha > 1 || math.IsNaN(config.EWMAAlpha) {
		config.EWMAAlpha = defaults.EWMAAlpha
	}
	if config.InitialLatency <= 0 {
		config.InitialLatency = defaults.InitialLatency
	}
	if config.FailurePenalty < 0 || math.IsNaN(config.FailurePenalty) {
		config.FailurePenalty = defaults.FailurePenalty
	}
	if config.Outlier.ConsecutiveFailures <= 0 {
		config.Outlier.ConsecutiveFailures = defaults.Outlier.ConsecutiveFailures
	}
	if config.Outlier.BaseEjectionTime <= 0 {
		config.Outlier.BaseEjectionTime = defaults.Outlier.BaseEjectionTime
	}
	if config.Outlier.MaxEjectionTime <= 0 {
		config.Outlier.MaxEjectionTime = defaults.Outlier.MaxEjectionTime
	}
	if config.Outlier.MaxEjectionTime < config.Outlier.BaseEjectionTime {
		config.Outlier.MaxEjectionTime = config.Outlier.BaseEjectionTime
	}
	if config.Outlier.MaxEjectionPercent <= 0 || config.Outlier.MaxEjectionPercent > 100 {
		config.Outlier.MaxEjectionPercent = defaults.Outlier.MaxEjectionPercent
	}
	if config.Classify == nil {
		config.Classify = defaults.Classify
	}
	if config.Now == nil {
		config.Now = defaults.Now
	}
	return config
}
