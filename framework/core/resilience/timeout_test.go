package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"yunka.io/framework/core/middleware"
)

func TestTimeoutBudgetRejectsInsufficientParentBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	called := false
	err := middleware.New(TimeoutBudget(TimeoutBudgetConfig{Reserve: 15 * time.Millisecond, Minimum: 10 * time.Millisecond})).Handle(ctx, func(context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrTimeoutBudgetExceeded) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestTimeoutBudgetCreatesChildDeadline(t *testing.T) {
	var got time.Duration
	err := middleware.New(TimeoutBudget(TimeoutBudgetConfig{Timeout: 50 * time.Millisecond})).Handle(context.Background(), func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("missing deadline")
		}
		got = time.Until(deadline)
		return nil
	})
	if err != nil || got <= 0 || got > 60*time.Millisecond {
		t.Fatalf("err=%v remaining=%v", err, got)
	}
}
