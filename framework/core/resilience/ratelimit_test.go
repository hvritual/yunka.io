package resilience

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucketRefills(t *testing.T) {
	now := time.Unix(100, 0)
	bucket := NewTokenBucket(RateLimitConfig{Enabled: true, Rate: 1, Burst: 1})
	bucket.now = func() time.Time { return now }
	bucket.last = now
	bucket.tokens = 1
	if !bucket.Allow(context.Background()) || bucket.Allow(context.Background()) {
		t.Fatal("unexpected initial allowance")
	}
	now = now.Add(time.Second)
	if !bucket.Allow(context.Background()) {
		t.Fatal("expected refill")
	}
}

func TestRateLimiterGroupIsolatesOperations(t *testing.T) {
	group := NewRateLimiterGroup(RateLimitConfig{Enabled: true, Rate: 0.001, Burst: 1})
	if !group.Bucket("a").Allow(context.Background()) || group.Bucket("a").Allow(context.Background()) {
		t.Fatal("bucket a should consume its burst")
	}
	if !group.Bucket("b").Allow(context.Background()) {
		t.Fatal("bucket b should have an independent burst")
	}
}
