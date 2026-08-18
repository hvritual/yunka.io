package resilience

import (
	"context"
	"sync"
	"time"

	"yunka.io/framework/core/middleware"
)

type RateLimitConfig struct {
	Enabled bool
	Rate    float64 // tokens per second
	Burst   int
	Cost    func(context.Context) float64
}

type RateLimitSnapshot struct {
	Tokens float64
	Burst  int
	Rate   float64
}

type TokenBucket struct {
	config RateLimitConfig
	mu     sync.Mutex
	tokens float64
	last   time.Time
	now    func() time.Time
}

func NewTokenBucket(config RateLimitConfig) *TokenBucket {
	if config.Burst <= 0 {
		config.Burst = 1
	}
	if config.Cost == nil {
		config.Cost = func(context.Context) float64 { return 1 }
	}
	now := time.Now()
	return &TokenBucket{config: config, tokens: float64(config.Burst), last: now, now: time.Now}
}

func (bucket *TokenBucket) Allow(ctx context.Context) bool {
	if bucket == nil || !bucket.config.Enabled || bucket.config.Rate <= 0 {
		return true
	}
	cost := bucket.config.Cost(ctx)
	if cost <= 0 {
		return true
	}
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	now := bucket.now()
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens += elapsed * bucket.config.Rate
		if bucket.tokens > float64(bucket.config.Burst) {
			bucket.tokens = float64(bucket.config.Burst)
		}
		bucket.last = now
	}
	if bucket.tokens < cost {
		return false
	}
	bucket.tokens -= cost
	return true
}

func (bucket *TokenBucket) Snapshot() RateLimitSnapshot {
	if bucket == nil {
		return RateLimitSnapshot{}
	}
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	return RateLimitSnapshot{Tokens: bucket.tokens, Burst: bucket.config.Burst, Rate: bucket.config.Rate}
}

type RateLimiterGroup struct {
	config  RateLimitConfig
	mu      sync.Mutex
	buckets map[string]*TokenBucket
}

func NewRateLimiterGroup(config RateLimitConfig) *RateLimiterGroup {
	return &RateLimiterGroup{config: config, buckets: make(map[string]*TokenBucket)}
}

func (group *RateLimiterGroup) Bucket(key string) *TokenBucket {
	if group == nil {
		return nil
	}
	group.mu.Lock()
	defer group.mu.Unlock()
	bucket := group.buckets[key]
	if bucket == nil {
		bucket = NewTokenBucket(group.config)
		group.buckets[key] = bucket
	}
	return bucket
}

func (group *RateLimiterGroup) Middleware(keyFn KeyFunc) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context) error {
			key := resolveKey(ctx, keyFn)
			bucket := group.Bucket(key)
			if bucket != nil && !bucket.Allow(ctx) {
				return reject("rate-limit", key, ErrRateLimited)
			}
			return next(ctx)
		}
	}
}
