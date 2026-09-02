package resilience

import (
	"context"
	"time"

	"github.com/hvritual/yunka.io/framework/core/middleware"
)

// TimeoutBudgetConfig constrains one logical call while preserving budget for
// its caller. Timeout is the maximum duration allocated to this operation.
type TimeoutBudgetConfig struct {
	Timeout time.Duration
	Reserve time.Duration
	Minimum time.Duration
}

// TimeoutBudget derives a child deadline from the parent budget. A call is
// rejected before execution when the remaining parent budget cannot satisfy
// Reserve + Minimum.
func TimeoutBudget(config TimeoutBudgetConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context) error {
			if ctx == nil {
				ctx = context.Background()
			}

			timeout, constrained, err := availableTimeout(ctx, config)
			if err != nil {
				return reject("timeout", OperationKey(ctx), err)
			}
			if !constrained {
				return next(ctx)
			}

			child, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			err = next(child)
			if err == nil && child.Err() != nil {
				return child.Err()
			}
			return err
		}
	}
}

func availableTimeout(ctx context.Context, config TimeoutBudgetConfig) (time.Duration, bool, error) {
	if config.Reserve < 0 {
		config.Reserve = 0
	}
	if config.Minimum < 0 {
		config.Minimum = 0
	}

	if deadline, ok := ctx.Deadline(); ok {
		available := time.Until(deadline) - config.Reserve
		if available <= 0 || (config.Minimum > 0 && available < config.Minimum) {
			return 0, false, ErrTimeoutBudgetExceeded
		}
		if config.Timeout > 0 && config.Timeout < available {
			available = config.Timeout
		}
		return available, true, nil
	}

	if config.Timeout > 0 {
		if config.Minimum > 0 && config.Timeout < config.Minimum {
			return 0, false, ErrTimeoutBudgetExceeded
		}
		return config.Timeout, true, nil
	}
	return 0, false, nil
}

// RemainingBudget reports the caller-visible remaining deadline budget.
func RemainingBudget(ctx context.Context) (time.Duration, bool) {
	if ctx == nil {
		return 0, false
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, true
}
