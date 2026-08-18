package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLoadShedderRejectsConcurrencyAndAdapts(t *testing.T) {
	now := time.Unix(100, 0)
	shedder := NewLoadShedder(LoadShedConfig{Enabled: true, MinLimit: 1, MaxLimit: 4, InitialLimit: 2, TargetLatency: 100 * time.Millisecond, IncreaseEvery: 1, DecreaseFactor: 0.5})
	shedder.now = func() time.Time { return now }
	_, ok1 := shedder.acquire(context.Background())
	_, ok2 := shedder.acquire(context.Background())
	_, ok3 := shedder.acquire(context.Background())
	if !ok1 || !ok2 || ok3 {
		t.Fatalf("acquire=%v,%v,%v", ok1, ok2, ok3)
	}
	// Release both slow calls; the limit should decrease but never below MinLimit.
	now = now.Add(200 * time.Millisecond)
	shedder.release(time.Unix(100, 0), nil)
	shedder.release(time.Unix(100, 0), nil)
	if snapshot := shedder.Snapshot(); snapshot.Limit != 1 || snapshot.InFlight != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	// A fast success can increase the adaptive limit again.
	start, ok := shedder.acquire(context.Background())
	if !ok {
		t.Fatal("expected probe")
	}
	now = now.Add(10 * time.Millisecond)
	shedder.release(start, nil)
	if snapshot := shedder.Snapshot(); snapshot.Limit != 2 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestLoadShedderRejectsInsufficientDeadlineBudget(t *testing.T) {
	shedder := NewLoadShedder(LoadShedConfig{Enabled: true, MinimumBudget: 50 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := shedder.Execute(ctx, func(context.Context) error { return errors.New("should not run") })
	if !errors.Is(err, ErrLoadShed) {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadShedderReleasesAdmissionOnPanic(t *testing.T) {
	shedder := NewLoadShedder(LoadShedConfig{Enabled: true, MinLimit: 1, MaxLimit: 1, InitialLimit: 1})
	func() {
		defer func() { _ = recover() }()
		_ = shedder.Execute(context.Background(), func(context.Context) error { panic("boom") })
	}()
	if snapshot := shedder.Snapshot(); snapshot.InFlight != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestLoadShedderConcurrentAdmissionNeverExceedsLimit(t *testing.T) {
	const limit = 4
	shedder := NewLoadShedder(LoadShedConfig{Enabled: true, MinLimit: limit, MaxLimit: limit, InitialLimit: limit, TargetLatency: time.Hour})
	started := make(chan struct{}, 32)
	release := make(chan struct{})
	done := make(chan struct{}, 32)
	for index := 0; index < 32; index++ {
		go func() {
			_ = shedder.Execute(context.Background(), func(context.Context) error {
				started <- struct{}{}
				<-release
				return nil
			})
			done <- struct{}{}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	if got := len(started); got != limit {
		t.Fatalf("admitted=%d want=%d", got, limit)
	}
	close(release)
	for index := 0; index < 32; index++ {
		<-done
	}
	if snapshot := shedder.Snapshot(); snapshot.InFlight != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}
