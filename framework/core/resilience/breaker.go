package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"yunka.io/framework/core/middleware"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half-open"
)

type CircuitBreakerConfig struct {
	Enabled             bool
	FailureThreshold    int
	SuccessThreshold    int
	OpenTimeout         time.Duration
	HalfOpenMaxRequests int
	IsFailure           func(error) bool
}

type CircuitSnapshot struct {
	State            CircuitState
	Failures         int
	Successes        int
	HalfOpenInFlight int
	OpenedAt         time.Time
}

type CircuitBreaker struct {
	config           CircuitBreakerConfig
	mu               sync.Mutex
	state            CircuitState
	failures         int
	successes        int
	halfOpenInFlight int
	openedAt         time.Time
	now              func() time.Time
}

func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	config = normalizeCircuitConfig(config)
	return &CircuitBreaker{config: config, state: CircuitClosed, now: time.Now}
}

func normalizeCircuitConfig(config CircuitBreakerConfig) CircuitBreakerConfig {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 2
	}
	if config.OpenTimeout <= 0 {
		config.OpenTimeout = 30 * time.Second
	}
	if config.HalfOpenMaxRequests <= 0 {
		config.HalfOpenMaxRequests = 1
	}
	if config.IsFailure == nil {
		config.IsFailure = func(err error) bool {
			return err != nil && !errors.Is(err, context.Canceled)
		}
	}
	return config
}

func (breaker *CircuitBreaker) Execute(ctx context.Context, next middleware.Handler) (err error) {
	if breaker == nil || !breaker.config.Enabled {
		return next(ctx)
	}
	if !breaker.allow() {
		return ErrCircuitOpen
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			breaker.complete(errors.New("downstream panic"))
			panic(recovered)
		}
	}()
	err = next(ctx)
	breaker.complete(err)
	return err
}

func (breaker *CircuitBreaker) allow() bool {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	now := breaker.now()
	if breaker.state == CircuitOpen && now.Sub(breaker.openedAt) >= breaker.config.OpenTimeout {
		breaker.state = CircuitHalfOpen
		breaker.successes = 0
		breaker.halfOpenInFlight = 0
	}
	switch breaker.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		return false
	case CircuitHalfOpen:
		if breaker.halfOpenInFlight >= breaker.config.HalfOpenMaxRequests {
			return false
		}
		breaker.halfOpenInFlight++
		return true
	default:
		return true
	}
}

func (breaker *CircuitBreaker) complete(err error) {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	failed := breaker.config.IsFailure(err)

	switch breaker.state {
	case CircuitClosed:
		if failed {
			breaker.failures++
			breaker.successes = 0
			if breaker.failures >= breaker.config.FailureThreshold {
				breaker.openLocked()
			}
			return
		}
		breaker.failures = 0
	case CircuitHalfOpen:
		if breaker.halfOpenInFlight > 0 {
			breaker.halfOpenInFlight--
		}
		if failed {
			breaker.openLocked()
			return
		}
		breaker.successes++
		if breaker.successes >= breaker.config.SuccessThreshold {
			breaker.state = CircuitClosed
			breaker.failures = 0
			breaker.successes = 0
			breaker.openedAt = time.Time{}
		}
	}
}

func (breaker *CircuitBreaker) openLocked() {
	breaker.state = CircuitOpen
	breaker.openedAt = breaker.now()
	breaker.failures = 0
	breaker.successes = 0
	breaker.halfOpenInFlight = 0
}

func (breaker *CircuitBreaker) Snapshot() CircuitSnapshot {
	if breaker == nil {
		return CircuitSnapshot{State: CircuitClosed}
	}
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	return CircuitSnapshot{
		State:            breaker.state,
		Failures:         breaker.failures,
		Successes:        breaker.successes,
		HalfOpenInFlight: breaker.halfOpenInFlight,
		OpenedAt:         breaker.openedAt,
	}
}

type CircuitBreakerGroup struct {
	config   CircuitBreakerConfig
	mu       sync.Mutex
	breakers map[string]*CircuitBreaker
}

func NewCircuitBreakerGroup(config CircuitBreakerConfig) *CircuitBreakerGroup {
	return &CircuitBreakerGroup{config: normalizeCircuitConfig(config), breakers: make(map[string]*CircuitBreaker)}
}

func (group *CircuitBreakerGroup) Breaker(key string) *CircuitBreaker {
	if group == nil {
		return nil
	}
	group.mu.Lock()
	defer group.mu.Unlock()
	breaker := group.breakers[key]
	if breaker == nil {
		breaker = NewCircuitBreaker(group.config)
		group.breakers[key] = breaker
	}
	return breaker
}

func (group *CircuitBreakerGroup) Middleware(keyFn KeyFunc) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context) error {
			key := resolveKey(ctx, keyFn)
			breaker := group.Breaker(key)
			if breaker == nil || !breaker.config.Enabled {
				return next(ctx)
			}
			err := breaker.Execute(ctx, next)
			if errors.Is(err, ErrCircuitOpen) {
				return reject("circuit", key, ErrCircuitOpen)
			}
			return err
		}
	}
}
