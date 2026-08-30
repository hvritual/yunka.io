package diagnostic

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderJSONIsDeterministicAndOrdered(t *testing.T) {
	input := []Diagnostic{
		{
			Code:     "YUNKA-DX-MODULE-002",
			Severity: SeverityWarning,
			Stage:    "module",
			Summary:  "module warning",
		},
		{
			Code:     "YUNKA-DX-ASSEMBLY-001",
			Severity: SeverityError,
			Stage:    "assembly",
			Summary:  "assembly generated artifacts are stale",
			Location: &Location{Path: "contracts/generated/assembly-plan.json"},
			Actions:  []Action{{Kind: ActionCommand, Value: "yunka generate"}},
		},
	}
	first, err := RenderJSON("check", input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderJSON("check", []Diagnostic{input[1], input[0]})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("diagnostic JSON is not deterministic:\n%s\n---\n%s", first, second)
	}
	text := string(first)
	if !strings.Contains(text, `"schemaVersion": 1`) || !strings.Contains(text, `"ok": false`) {
		t.Fatalf("unexpected envelope: %s", text)
	}
	assembly := strings.Index(text, "YUNKA-DX-ASSEMBLY-001")
	module := strings.Index(text, "YUNKA-DX-MODULE-002")
	if assembly < 0 || module < 0 || assembly > module {
		t.Fatalf("errors must sort before warnings: %s", text)
	}
}

func TestRenderTextUsesSameNormalizedIdentity(t *testing.T) {
	text, err := RenderText([]Diagnostic{{
		Code:     "YUNKA-DX-PROJECT-001",
		Severity: SeverityError,
		Stage:    "project",
		Summary:  "project profile path is invalid",
		Detail:   "configured contract root does not exist",
		Location: &Location{Path: ".yunka/project.json", Ref: "workflow.contract.protoRoot"},
		Actions:  []Action{{Kind: ActionEdit, Label: "edit profile", Value: ".yunka/project.json"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"ERROR YUNKA-DX-PROJECT-001",
		"stage:    project",
		"location: .yunka/project.json",
		"detail:   configured contract root does not exist",
		"action:   edit profile: .yunka/project.json",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("text output missing %q:\n%s", expected, text)
		}
	}
}

func TestDiagnosticRejectsVolatileAbsoluteLocation(t *testing.T) {
	_, err := RenderJSON("check", []Diagnostic{{
		Code:     "YUNKA-DX-CONTRACT-001",
		Severity: SeverityError,
		Stage:    "contract",
		Summary:  "contract compile failed",
		Location: &Location{Path: "/tmp/private/project/contracts/proto/device.proto"},
	}})
	if err == nil || !strings.Contains(err.Error(), "project-relative") {
		t.Fatalf("expected absolute path rejection, got %v", err)
	}
}

func TestDiagnosticRejectsUnknownCodeAndActionKind(t *testing.T) {
	_, err := Normalize([]Diagnostic{{
		Code:     "CONTRACT-1",
		Severity: SeverityError,
		Stage:    "contract",
		Summary:  "bad code",
	}})
	if err == nil {
		t.Fatal("expected invalid code failure")
	}
	_, err = Normalize([]Diagnostic{{
		Code:     "YUNKA-DX-CONTRACT-001",
		Severity: SeverityError,
		Stage:    "contract",
		Summary:  "bad action",
		Actions:  []Action{{Kind: "execute", Value: "rm -rf /"}},
	}})
	if err == nil {
		t.Fatal("expected unsupported action kind failure")
	}
}

func TestNewEnvelopeOKWithoutErrors(t *testing.T) {
	envelope, err := NewEnvelope("doctor", []Diagnostic{{
		Code:     "YUNKA-DX-TOOLCHAIN-001",
		Severity: SeverityWarning,
		Stage:    "toolchain",
		Summary:  "optional tool is missing",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !envelope.OK {
		t.Fatal("warning-only envelope must remain ok")
	}
}
