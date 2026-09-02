package resilience

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

// RetryConfig is intentionally fail-safe: retries require MaxAttempts > 1,
// an idempotency decision, and a retryable-error decision.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      float64
	Idempotent  func(context.Context) bool
	Retryable   func(error) bool
}

type retryAttemptContextKey struct{}

// AttemptFrom returns the 1-based retry attempt for instrumentation.
func AttemptFrom(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	attempt, _ := ctx.Value(retryAttemptContextKey{}).(int)
	return attempt
}

// Retry re-executes downstream middleware only for explicitly idempotent and
// retryable calls. The same parent deadline budget is shared by all attempts.
func Retry(config RetryConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context) error {
			if config.MaxAttempts <= 1 || config.Retryable == nil {
				return next(ctx)
			}
			idempotent := config.Idempotent
			if idempotent == nil {
				idempotent = IdempotentFromMetadata
			}
			if !idempotent(ctx) {
				return next(ctx)
			}

			multiplier := config.Multiplier
			if multiplier < 1 {
				multiplier = 2
			}
			var lastErr error
			for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				attemptCtx := context.WithValue(ctx, retryAttemptContextKey{}, attempt)
				lastErr = next(attemptCtx)
				if lastErr == nil {
					return nil
				}
				if attempt == config.MaxAttempts || !config.Retryable(lastErr) {
					return lastErr
				}

				delay := retryDelay(config, multiplier, attempt)
				if delay <= 0 {
					continue
				}
				timer := time.NewTimer(delay)
				select {
				case <-timer.C:
				case <-ctx.Done():
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					return ctx.Err()
				}
			}
			return lastErr
		}
	}
}

func retryDelay(config RetryConfig, multiplier float64, attempt int) time.Duration {
	if config.BaseDelay <= 0 {
		return 0
	}
	delay := float64(config.BaseDelay)
	for index := 1; index < attempt; index++ {
		delay *= multiplier
		if config.MaxDelay > 0 && time.Duration(delay) >= config.MaxDelay {
			delay = float64(config.MaxDelay)
			break
		}
	}
	if config.Jitter > 0 {
		jitter := config.Jitter
		if jitter > 1 {
			jitter = 1
		}
		factor := 1 - jitter + rand.Float64()*(2*jitter)
		delay *= factor
	}
	if delay < 0 {
		return 0
	}
	return time.Duration(delay)
}

// IdempotentFromMetadata recognizes the explicit runtime attribute
// resilience.idempotent=true. No method name is assumed safe automatically.
func IdempotentFromMetadata(ctx context.Context) bool {
	metadata, ok := runtimecontext.MetadataFrom(ctx)
	if !ok || metadata.Attributes == nil {
		return false
	}
	return metadata.Attributes["resilience.idempotent"] == "true"
}

func IdempotentOperations(operations ...string) func(context.Context) bool {
	allowed := make(map[string]struct{}, len(operations))
	for _, operation := range operations {
		allowed[operation] = struct{}{}
	}
	return func(ctx context.Context) bool {
		_, ok := allowed[OperationKey(ctx)]
		return ok
	}
}
