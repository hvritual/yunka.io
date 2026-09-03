package dev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hvritual/yunka.io/pkg/devruntime"
	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

func TestRuntimeEventStreamProjectsCanonicalReadyDiagnosticsAndClosure(t *testing.T) {
	plan := devruntime.Plan{
		Processes: []devruntime.Process{{Name: "worker"}, {Name: "api"}},
		Runtime: &devruntime.RuntimeConfig{Application: "example", StatePath: ".yunka/runtime-state.json", GraphPath: ".yunka/runtime-graph.json"},
		Closure: true,
	}
	report := devruntime.RuntimeReport{
		SchemaVersion: devruntime.RuntimeReportSchemaVersion,
		Application:   "example",
		State:         devruntime.RuntimeRunRunning,
		Plan:          []string{"worker", "api"},
		Processes: []devruntime.ProcessRuntimeReport{
			{Name: "worker", State: devruntime.ProcessRunning},
			{Name: "api", State: devruntime.ProcessReady, Ready: true, Diagnostics: &devruntime.RuntimeCoreSummary{State: "running", HealthState: "healthy", Live: true, Ready: true, RouteCount: 3}},
		},
		StartedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:01Z",
	}
	var output bytes.Buffer
	stream := newRuntimeEventStream(&output)
	if err := stream.plan(plan); err != nil {
		t.Fatal(err)
	}
	if err := stream.evidence(plan); err != nil {
		t.Fatal(err)
	}
	observed := runtimeEventState{processStates: map[string]devruntime.ProcessState{}}
	if err := stream.report(plan, report, &observed); err != nil {
		t.Fatal(err)
	}
	events := decodeRuntimeEvents(t, output.String())
	wantTypes := []string{eventPlanResolved, eventEvidenceConfigured, eventProcessState, eventProcessState, eventApplicationState, eventClosureComplete}
	if len(events) != len(wantTypes) {
		t.Fatalf("events=%d want %d\n%s", len(events), len(wantTypes), output.String())
	}
	for index, want := range wantTypes {
		if events[index].Type != want || events[index].Sequence != uint64(index+1) {
			t.Fatalf("event[%d]=%#v want type=%s sequence=%d", index, events[index], want, index+1)
		}
	}
	// Process events are sorted by stable process identity even if the report order differs.
	if events[2].Process != "api" || events[2].State != string(devruntime.ProcessReady) || events[2].Ready == nil || !*events[2].Ready {
		t.Fatalf("api event=%#v", events[2])
	}
	if events[2].Diagnostics == nil || events[2].Diagnostics.HealthState != "healthy" || !events[2].Diagnostics.Ready {
		t.Fatalf("diagnostics=%#v", events[2].Diagnostics)
	}
	if events[3].Process != "worker" || events[3].State != string(devruntime.ProcessRunning) {
		t.Fatalf("worker event=%#v", events[3])
	}
	if events[4].State != string(devruntime.RuntimeRunRunning) || events[5].State != "complete" {
		t.Fatalf("application/closure events=%#v %#v", events[4], events[5])
	}
	if strings.Contains(output.String(), "command") || strings.Contains(output.String(), "argv") {
		t.Fatalf("event stream exposed command metadata:\n%s", output.String())
	}
}

func TestRuntimeEventStreamFailureUsesStableAgentDiagnosticWithoutRawError(t *testing.T) {
	root := t.TempDir()
	plan := devruntime.Plan{
		Processes: []devruntime.Process{{Name: "api", Command: []string{"api", "--token", "super-secret"}}},
		Runtime:   &devruntime.RuntimeConfig{Application: "example", StatePath: ".yunka/runtime-state.json", GraphPath: ".yunka/runtime-graph.json"},
		Closure:   true,
	}
	var events bytes.Buffer
	canonicalErr := errors.New("token=super-secret")
	run := func(context.Context, devruntime.Plan, devruntime.RunOptions) error {
		if err := devruntime.WriteRuntimeReport(root, plan.Runtime.StatePath, devruntime.RuntimeReport{
			SchemaVersion: devruntime.RuntimeReportSchemaVersion,
			Application:   "example",
			State:         devruntime.RuntimeRunFailed,
			Reason:        "token=super-secret",
			Plan:          []string{"api"},
			Processes:     []devruntime.ProcessRuntimeReport{{Name: "api", State: devruntime.ProcessFailed, Error: "token=super-secret"}},
			StartedAt:     "2026-01-01T00:00:00Z",
			UpdatedAt:     "2026-01-01T00:00:01Z",
			FinishedAt:    "2026-01-01T00:00:01Z",
		}); err != nil {
			return err
		}
		return canonicalErr
	}
	err := runWithEventStreamOptions(context.Background(), plan, root, &events, io.Discard, run, devruntime.LoadRuntimeReport, time.Millisecond)
	if !errors.Is(err, canonicalErr) {
		t.Fatalf("error=%v want canonical error", err)
	}
	text := events.String()
	if strings.Contains(text, "super-secret") || strings.Contains(text, "token=") || strings.Contains(text, "--token") {
		t.Fatalf("event stream leaked runtime secret/details:\n%s", text)
	}
	decoded := decodeRuntimeEvents(t, text)
	last := decoded[len(decoded)-1]
	if last.Type != eventRuntimeDiagnostic || last.Diagnostic == nil || last.Diagnostic.Code != diagnostic.CodeRuntimeFailure {
		t.Fatalf("last event=%#v", last)
	}
	if last.Diagnostic.Stage != "runtime-supervision" || last.Diagnostic.Retry == nil || last.Diagnostic.Retry.Value != "yunka dev" {
		t.Fatalf("diagnostic=%#v", last.Diagnostic)
	}
	var status, doctor bool
	for _, action := range last.Diagnostic.Remediation {
		if strings.HasPrefix(action.Value, "yunka dev status --closure --state .yunka/runtime-state.json") {
			status = true
		}
		if action.Value == "yunka doctor" {
			doctor = true
		}
	}
	if !status || !doctor {
		t.Fatalf("runtime remediation missing status/doctor: %#v", last.Diagnostic.Remediation)
	}
}

func TestLegacyRuntimeEventStreamDoesNotInventReady(t *testing.T) {
	plan := devruntime.Plan{Processes: []devruntime.Process{{Name: "legacy", Command: []string{"legacy"}}}}
	var events bytes.Buffer
	if err := runWithEventStreamOptions(
		context.Background(), plan, t.TempDir(), &events, io.Discard,
		func(context.Context, devruntime.Plan, devruntime.RunOptions) error { return nil }, nil, time.Millisecond,
	); err != nil {
		t.Fatal(err)
	}
	decoded := decodeRuntimeEvents(t, events.String())
	if len(decoded) != 2 || decoded[0].Type != eventPlanResolved || decoded[1].Type != eventEvidenceDisabled {
		t.Fatalf("legacy events=%#v", decoded)
	}
	if strings.Contains(events.String(), eventProcessState) || strings.Contains(events.String(), eventApplicationState) || strings.Contains(events.String(), eventClosureComplete) {
		t.Fatalf("legacy path invented runtime evidence:\n%s", events.String())
	}
}

func TestRuntimeEventStreamSuppressesDuplicateStates(t *testing.T) {
	plan := devruntime.Plan{Processes: []devruntime.Process{{Name: "api"}}, Runtime: &devruntime.RuntimeConfig{Application: "example"}}
	report := devruntime.RuntimeReport{
		SchemaVersion: devruntime.RuntimeReportSchemaVersion,
		Application: "example", State: devruntime.RuntimeRunRunning, Plan: []string{"api"},
		Processes: []devruntime.ProcessRuntimeReport{{Name: "api", State: devruntime.ProcessReady, Ready: true}},
		StartedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:01Z",
	}
	var output bytes.Buffer
	stream := newRuntimeEventStream(&output)
	observed := runtimeEventState{processStates: map[string]devruntime.ProcessState{}}
	if err := stream.report(plan, report, &observed); err != nil {
		t.Fatal(err)
	}
	if err := stream.report(plan, report, &observed); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), `"type":"`+eventProcessState+`"`); got != 1 {
		t.Fatalf("process state events=%d\n%s", got, output.String())
	}
	if got := strings.Count(output.String(), `"type":"`+eventApplicationState+`"`); got != 1 {
		t.Fatalf("application state events=%d\n%s", got, output.String())
	}
}

func TestRuntimeStatusJSONLIncludesValidatedClosure(t *testing.T) {
	plan := devruntime.Plan{
		Processes: []devruntime.Process{{Name: "api"}},
		Runtime:   &devruntime.RuntimeConfig{Application: "example"},
		Closure:   true,
	}
	report := devruntime.RuntimeReport{
		SchemaVersion: devruntime.RuntimeReportSchemaVersion,
		Application: "example", State: devruntime.RuntimeRunRunning, Plan: []string{"api"},
		Processes: []devruntime.ProcessRuntimeReport{{Name: "api", State: devruntime.ProcessReady, Ready: true}},
		StartedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:01Z",
	}
	if err := devruntime.ValidateRuntimeClosure(plan, report); err != nil {
		t.Fatalf("fixture closure invalid: %v", err)
	}
	var output bytes.Buffer
	if err := renderRuntimeStatusEvents(&output, plan, ".yunka/runtime-state.json", report, true); err != nil {
		t.Fatal(err)
	}
	decoded := decodeRuntimeEvents(t, output.String())
	if decoded[0].Type != "runtime.snapshot" || decoded[len(decoded)-1].Type != eventClosureComplete {
		t.Fatalf("status events=%#v", decoded)
	}
}

func decodeRuntimeEvents(t *testing.T, value string) []RuntimeEvent {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(value))
	var events []RuntimeEvent
	for decoder.More() {
		var event RuntimeEvent
		if err := decoder.Decode(&event); err != nil {
			t.Fatalf("decode event: %v\n%s", err, value)
		}
		events = append(events, event)
	}
	if len(events) == 0 {
		t.Fatalf("no events decoded from %q", value)
	}
	return events
}
