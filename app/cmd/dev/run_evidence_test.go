package dev

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yunka.io/pkg/devruntime"
)

func TestRuntimeEvidenceReadyComesFromCanonicalReport(t *testing.T) {
	root := t.TempDir()
	plan := devruntime.Plan{
		Processes: []devruntime.Process{{Name: "api", Command: []string{"api", "--token", "super-secret"}}},
		Runtime: &devruntime.RuntimeConfig{
			Application: "example",
			StatePath:   ".yunka/runtime-state.json",
			GraphPath:   ".yunka/runtime-graph.json",
		},
	}
	var evidence bytes.Buffer
	run := func(context.Context, devruntime.Plan, devruntime.RunOptions) error {
		return devruntime.WriteRuntimeReport(root, plan.Runtime.StatePath, devruntime.RuntimeReport{
			SchemaVersion: devruntime.RuntimeReportSchemaVersion,
			Application:   "example",
			State:         devruntime.RuntimeRunRunning,
			Plan:          []string{"api"},
			Processes: []devruntime.ProcessRuntimeReport{{
				Name: "api", State: devruntime.ProcessReady, Ready: true,
			}},
			StartedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:01Z",
		})
	}
	if err := runWithEvidenceOptions(context.Background(), plan, root, &evidence, io.Discard, io.Discard, run, devruntime.LoadRuntimeReport, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	got := evidence.String()
	for _, want := range []string{
		"DEV plan processes=1 names=api\n",
		"DEV evidence state=.yunka/runtime-state.json graph=.yunka/runtime-graph.json\n",
		"DEV READY process=api\n",
		"DEV READY application=example\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "super-secret") || strings.Contains(got, "--token") {
		t.Fatalf("evidence leaked command arguments:\n%s", got)
	}
	if strings.Contains(got, filepath.ToSlash(root)) {
		t.Fatalf("evidence leaked absolute project root:\n%s", got)
	}
}

func TestRuntimeEvidenceFailureUsesExistingStatusAndDoctor(t *testing.T) {
	root := t.TempDir()
	plan := devruntime.Plan{
		Processes: []devruntime.Process{{Name: "api", Command: []string{"api"}}},
		Runtime: &devruntime.RuntimeConfig{
			Application: "example",
			StatePath:   ".yunka/runtime-state.json",
			GraphPath:   ".yunka/runtime-graph.json",
		},
		Closure: true,
	}
	var evidence bytes.Buffer
	run := func(context.Context, devruntime.Plan, devruntime.RunOptions) error {
		if err := devruntime.WriteRuntimeReport(root, plan.Runtime.StatePath, devruntime.RuntimeReport{
			SchemaVersion: devruntime.RuntimeReportSchemaVersion,
			Application:   "example",
			State:         devruntime.RuntimeRunFailed,
			Reason:        "token=super-secret",
			Plan:          []string{"api"},
			Processes: []devruntime.ProcessRuntimeReport{{
				Name: "api", State: devruntime.ProcessFailed, Error: "token=super-secret",
			}},
			StartedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:01Z",
			FinishedAt: "2026-01-01T00:00:01Z",
		}); err != nil {
			return err
		}
		return errors.New("token=super-secret")
	}
	err := runWithEvidenceOptions(context.Background(), plan, root, &evidence, io.Discard, io.Discard, run, devruntime.LoadRuntimeReport, time.Millisecond)
	if err == nil {
		t.Fatal("expected runner error")
	}
	got := evidence.String()
	for _, want := range []string{
		"DEV FAILED process=api\n",
		"DEV FAILED application=example\n",
		"DEV inspect: yunka dev status --closure --state .yunka/runtime-state.json\n",
		"DEV next: yunka doctor\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "super-secret") || strings.Contains(got, "token=") {
		t.Fatalf("failure UX leaked runtime error details:\n%s", got)
	}
}

func TestLegacyDevRuntimeDoesNotInventReadyEvidence(t *testing.T) {
	plan := devruntime.Plan{Processes: []devruntime.Process{{Name: "legacy", Command: []string{"legacy"}}}}
	var evidence bytes.Buffer
	called := 0
	run := func(context.Context, devruntime.Plan, devruntime.RunOptions) error {
		called++
		return nil
	}
	if err := runWithEvidenceOptions(context.Background(), plan, t.TempDir(), &evidence, io.Discard, io.Discard, run, nil, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("run calls=%d want 1", called)
	}
	got := evidence.String()
	if !strings.Contains(got, "DEV evidence runtime-report=disabled\n") {
		t.Fatalf("missing legacy evidence marker:\n%s", got)
	}
	if strings.Contains(got, "DEV READY") {
		t.Fatalf("legacy path invented Ready evidence without canonical runtime report:\n%s", got)
	}
}

func TestRuntimeEvidenceSuppressesDuplicateCanonicalStates(t *testing.T) {
	var evidence bytes.Buffer
	observed := runtimeEvidenceState{processStates: map[string]devruntime.ProcessState{}}
	report := devruntime.RuntimeReport{
		SchemaVersion: devruntime.RuntimeReportSchemaVersion,
		Application:   "example",
		State:         devruntime.RuntimeRunRunning,
		Processes: []devruntime.ProcessRuntimeReport{{Name: "api", State: devruntime.ProcessReady, Ready: true}},
	}
	renderRuntimeEvidence(&evidence, report, &observed)
	renderRuntimeEvidence(&evidence, report, &observed)
	if got := strings.Count(evidence.String(), "DEV READY process=api"); got != 1 {
		t.Fatalf("process Ready rendered %d times:\n%s", got, evidence.String())
	}
	if got := strings.Count(evidence.String(), "DEV READY application=example"); got != 1 {
		t.Fatalf("application Ready rendered %d times:\n%s", got, evidence.String())
	}
}

func TestRuntimeEvidenceReadFailureCannotChangeCanonicalRunnerError(t *testing.T) {
	canonicalErr := errors.New("canonical-runner-error")
	plan := devruntime.Plan{
		Processes: []devruntime.Process{{Name: "api", Command: []string{"api"}}},
		Runtime: &devruntime.RuntimeConfig{
			Application: "example",
			StatePath:   ".yunka/runtime-state.json",
			GraphPath:   ".yunka/runtime-graph.json",
		},
	}
	var evidence bytes.Buffer
	err := runWithEvidenceOptions(
		context.Background(),
		plan,
		t.TempDir(),
		&evidence,
		io.Discard,
		io.Discard,
		func(context.Context, devruntime.Plan, devruntime.RunOptions) error { return canonicalErr },
		func(string, string) (devruntime.RuntimeReport, error) {
			return devruntime.RuntimeReport{}, errors.New("report unavailable")
		},
		time.Millisecond,
	)
	if !errors.Is(err, canonicalErr) {
		t.Fatalf("returned error=%v want canonical runner error", err)
	}
	got := evidence.String()
	if !strings.Contains(got, "DEV FAILED runtime\n") || !strings.Contains(got, "DEV next: yunka doctor\n") {
		t.Fatalf("missing fallback evidence guidance:\n%s", got)
	}
	if strings.Contains(got, "DEV inspect:") {
		t.Fatalf("status guidance was emitted without a readable canonical report:\n%s", got)
	}
}
