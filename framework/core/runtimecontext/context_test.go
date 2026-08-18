package runtimecontext

import (
	"context"
	"testing"
)

func TestMetadataContextClone(t *testing.T) {
	ctx := WithMetadata(context.Background(), Metadata{
		Transport:  "http",
		Operation:  "GET /devices/:id",
		Attributes: map[string]string{"module": "device"},
	})
	metadata, ok := MetadataFrom(ctx)
	if !ok || metadata.Transport != "http" {
		t.Fatalf("unexpected metadata: %#v ok=%v", metadata, ok)
	}
	metadata.Attributes["module"] = "changed"

	again, _ := MetadataFrom(ctx)
	if again.Attributes["module"] != "device" {
		t.Fatalf("context metadata was mutated: %#v", again)
	}
}

func TestTraceIDContext(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-1")
	if got := TraceIDFrom(ctx); got != "trace-1" {
		t.Fatalf("trace id=%q", got)
	}
}
