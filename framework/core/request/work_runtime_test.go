package request

import (
	"context"
	"errors"
	"testing"
)

func TestWorkRuntimeDelegatesContextCancellation(t *testing.T) {
	runtime := NewWorkRuntime()
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), "key", "value"))
	runtime.SetContext(ctx)
	cancel()
	<-runtime.Done()
	if !errors.Is(runtime.Err(), context.Canceled) {
		t.Fatalf("got %v, want context canceled", runtime.Err())
	}
	if got := runtime.Value("key"); got != "value" {
		t.Fatalf("got value %v", got)
	}
}

func TestFinishRequestReturnsHookErrors(t *testing.T) {
	runtime := NewWorkRuntime()
	want := errors.New("hook failed")
	runtime.BindFinishHook(func(error) error { return want })
	if err := runtime.FinishRequest(nil); !errors.Is(err, want) {
		t.Fatalf("got %v, want hook error", err)
	}
}
