package dev

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hvritual/yunka.io/pkg/devruntime"
)

func TestRuntimeEventStreamKeepsChildLogsOutOfJSONLChannel(t *testing.T) {
	plan := devruntime.Plan{Processes: []devruntime.Process{{Name: "legacy", Command: []string{"legacy"}}}}
	var events bytes.Buffer
	var child bytes.Buffer
	run := func(_ context.Context, _ devruntime.Plan, options devruntime.RunOptions) error {
		_, _ = fmt.Fprint(options.Stdout, "child-stdout\n")
		_, _ = fmt.Fprint(options.Stderr, "child-stderr\n")
		return nil
	}
	if err := runWithEventStreamOptions(context.Background(), plan, t.TempDir(), &events, &child, run, nil, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(events.String(), "child-stdout") || strings.Contains(events.String(), "child-stderr") {
		t.Fatalf("child logs polluted JSONL event stream:\n%s", events.String())
	}
	if !strings.Contains(child.String(), "child-stdout") || !strings.Contains(child.String(), "child-stderr") {
		t.Fatalf("child output was not preserved on the process-output channel:\n%s", child.String())
	}
	_ = decodeRuntimeEvents(t, events.String())
}
