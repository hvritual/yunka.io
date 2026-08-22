package rpcbridge

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProviderFuncNilFailsClosed(t *testing.T) {
	var provider ProviderFunc[string]
	_, _, err := provider.Acquire(context.Background())
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("Acquire error = %v, want ErrProviderUnavailable", err)
	}
}

func TestStaticProvider(t *testing.T) {
	provider := Static("service")
	service, release, err := provider.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if service != "service" {
		t.Fatalf("service = %q", service)
	}
	if err := release(nil); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestOnceReleaseExecutesExactlyOnce(t *testing.T) {
	var calls atomic.Int32
	want := errors.New("call failed")
	release := Once(func(callErr error) error {
		calls.Add(1)
		if !errors.Is(callErr, want) {
			t.Fatalf("call error = %v", callErr)
		}
		return errors.New("release failed")
	})

	first := release(want)
	second := release(nil)
	if calls.Load() != 1 {
		t.Fatalf("release calls = %d", calls.Load())
	}
	if first == nil || first.Error() != second.Error() {
		t.Fatalf("release results differ: %v / %v", first, second)
	}
}

func TestSafeReleaseContainsPanic(t *testing.T) {
	callErr := errors.New("call failed")
	err := SafeRelease(func(error) error {
		panic("secret cleanup detail")
	}, callErr)
	if err == nil || !strings.Contains(err.Error(), "release panicked") {
		t.Fatalf("SafeRelease error = %v", err)
	}
	if !errors.Is(err, callErr) {
		t.Fatalf("SafeRelease lost call error: %v", err)
	}
}
