package applicationgraph

import (
	"context"
	"errors"
	"testing"

	graph "github.com/hvritual/yunka.io/pkg/applicationgraph"
)

type panicSource struct{}

func (panicSource) Name() string                                { return "panic" }
func (panicSource) Apply(context.Context, *graph.Builder) error { panic("boom") }

type errorSource struct{}

func (errorSource) Name() string                                { return "error" }
func (errorSource) Apply(context.Context, *graph.Builder) error { return errors.New("boom") }

func TestCompileRejectsDuplicateSources(t *testing.T) {
	source := sourceFunc{name: "same", apply: func(context.Context, *graph.Builder) error { return nil }}
	if _, err := Compile(context.Background(), source, source); err == nil {
		t.Fatal("expected duplicate source error")
	}
}

func TestCompileContainsSourceFailure(t *testing.T) {
	if _, err := Compile(context.Background(), panicSource{}); err == nil {
		t.Fatal("expected panic error")
	}
	if _, err := Compile(context.Background(), errorSource{}); err == nil {
		t.Fatal("expected source error")
	}
}
