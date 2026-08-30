package dxoutput

import (
	"encoding/json"
	"strings"
	"testing"

	"yunka.io/app/cmd/projectflow"
)

func TestBuildSuccessJSONIsDeterministicAndRootFree(t *testing.T) {
	report := projectflow.Report{
		Root: "/tmp/host-specific-root",
		Stages: []projectflow.Stage{
			{Name: "contract", Status: "ok", Detail: "services=1"},
			{Name: "modules", Status: "skipped", Detail: "no generated modules"},
		},
	}
	first, err := Build("yunka check", "json", report, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build("yunka check", "json", report, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("json output is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first.ExitCode != 0 {
		t.Fatalf("exit=%d", first.ExitCode)
	}
	if strings.Contains(first.Output, "/tmp/host-specific-root") {
		t.Fatalf("json leaked report root: %s", first.Output)
	}
	var envelope struct {
		SchemaVersion int                 `json:"schemaVersion"`
		Command       string              `json:"command"`
		OK            bool                `json:"ok"`
		Stages        []projectflow.Stage `json:"stages"`
	}
	if err := json.Unmarshal([]byte(first.Output), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != 1 || envelope.Command != "yunka check" || !envelope.OK || len(envelope.Stages) != 2 {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestBuildProjectFailureJSONHasStableCodeAndNoAbsoluteRoot(t *testing.T) {
	root := t.TempDir()
	_, workflowErr := projectflow.Check(nil, projectflow.Options{Root: root})
	if workflowErr == nil {
		t.Fatal("expected project failure")
	}
	result, err := Build("yunka check", "json", projectflow.Report{}, workflowErr)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("exit=%d", result.ExitCode)
	}
	if !strings.Contains(result.Output, `"code": "YUNKA-DX-PROJECT-001"`) {
		t.Fatalf("missing stable code: %s", result.Output)
	}
	if strings.Contains(result.Output, root) {
		t.Fatalf("json leaked temp root: %s", result.Output)
	}
	if strings.Contains(result.Output, "timestamp") {
		t.Fatalf("json unexpectedly contains timestamp: %s", result.Output)
	}
}

func TestBuildRejectsUnsupportedFormatWithStableCLIError(t *testing.T) {
	result, err := Build("yunka check", "yaml", projectflow.Report{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit=%d", result.ExitCode)
	}
	if !strings.Contains(result.Output, "YUNKA-DX-DEV-001") {
		t.Fatalf("output=%q", result.Output)
	}
}
