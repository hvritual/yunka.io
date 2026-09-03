package diagnostic

import (
	"bytes"
	"strings"
	"testing"
)

func TestAgentEnvelopeProjectsCauseTargetRemediationAndRetry(t *testing.T) {
	input := []Diagnostic{{
		Code:     CodeContractDrift,
		Severity: SeverityError,
		Stage:    "contract",
		Summary:  "generated contract artifacts are stale",
		Detail:   "manifest.json differs from canonical contract source",
		Location: &Location{Path: "contracts/generated/manifest.json"},
		Actions:  []Action{{Kind: ActionCommand, Label: "Regenerate", Value: "yunka generate"}},
	}}
	envelope, err := NewAgentEnvelope("yunka check", input, false)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != AgentSchemaVersion || envelope.OK || len(envelope.Diagnostics) != 1 {
		t.Fatalf("envelope=%#v", envelope)
	}
	item := envelope.Diagnostics[0]
	if item.Cause.Summary != input[0].Summary || item.Cause.Detail != input[0].Detail {
		t.Fatalf("cause=%#v", item.Cause)
	}
	if item.Target == nil || item.Target.Path != "contracts/generated/manifest.json" {
		t.Fatalf("target=%#v", item.Target)
	}
	if len(item.Remediation) != 1 || item.Remediation[0].Value != "yunka generate" {
		t.Fatalf("remediation=%#v", item.Remediation)
	}
	if item.Retry == nil || item.Retry.Kind != ActionCommand || item.Retry.Value != "yunka check" {
		t.Fatalf("retry=%#v", item.Retry)
	}
}

func TestRenderAgentJSONIsDeterministicAndDoesNotMutateSource(t *testing.T) {
	input := []Diagnostic{
		{
			Code:     "YUNKA-DX-MODULE-002",
			Severity: SeverityWarning,
			Stage:    "module",
			Summary:  "module warning",
		},
		{
			Code:     CodeAssemblyDrift,
			Severity: SeverityError,
			Stage:    "assembly",
			Summary:  "assembly drift",
			Location: &Location{Path: "contracts/generated/assembly-plan.json"},
		},
	}
	first, err := RenderAgentJSON("yunka check", input, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderAgentJSON("yunka check", []Diagnostic{input[1], input[0]}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("agent JSON is not deterministic:\n%s\n---\n%s", first, second)
	}
	if input[1].Location == nil || input[1].Location.Path != "contracts/generated/assembly-plan.json" {
		t.Fatalf("source diagnostic mutated: %#v", input[1])
	}
	text := string(first)
	for _, expected := range []string{`"cause"`, `"target"`, `"remediation"`, `"retry"`, `"yunka check"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("agent JSON missing %s:\n%s", expected, text)
		}
	}
}

func TestAgentEnvelopeOmitsRetryForSuccessfulCommand(t *testing.T) {
	envelope, err := NewAgentEnvelope("yunka doctor", []Diagnostic{{
		Code:     CodeDoctorGitStatus,
		Severity: SeverityWarning,
		Stage:    "developer-environment",
		Summary:  "worktree has changes",
	}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Diagnostics) != 1 || envelope.Diagnostics[0].Retry != nil {
		t.Fatalf("successful command must not advertise retry: %#v", envelope)
	}
}
