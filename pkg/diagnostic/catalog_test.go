package diagnostic

import (
	"strings"
	"testing"
)

func TestDefinitionCatalogIsDeterministicAndValid(t *testing.T) {
	definitions := Definitions()
	if len(definitions) != 29 {
		t.Fatalf("definitions=%d want 29", len(definitions))
	}
	previous := ""
	seen := map[string]bool{}
	for _, definition := range definitions {
		if seen[definition.Code] {
			t.Fatalf("duplicate definition %s", definition.Code)
		}
		seen[definition.Code] = true
		if previous != "" && definition.Code <= previous {
			t.Fatalf("catalog is not sorted: %s before %s", previous, definition.Code)
		}
		previous = definition.Code
		if strings.TrimSpace(definition.Stage) == "" || strings.TrimSpace(definition.Meaning) == "" {
			t.Fatalf("incomplete definition: %#v", definition)
		}
		if _, err := Normalize([]Diagnostic{definition.Diagnostic(SeverityInfo)}); err != nil {
			t.Fatalf("invalid definition %s: %v", definition.Code, err)
		}
	}
	for _, code := range []string{CodeChangeOperation, CodeChangeIntent, CodeChangeEvidence} {
		definition, ok := LookupDefinition(code)
		if !ok || definition.Stage != "change-planning" {
			t.Fatalf("change definition %s=%#v ok=%v", code, definition, ok)
		}
	}
	for _, code := range []string{CodeScaffoldRequest, CodeScaffoldSource, CodeScaffoldOwnership, CodeScaffoldConflict} {
		definition, ok := LookupDefinition(code)
		if !ok || definition.Stage != "structural-scaffold" {
			t.Fatalf("scaffold definition %s=%#v ok=%v", code, definition, ok)
		}
	}
	definition, ok := LookupDefinition(CodeRuntimeFailure)
	if !ok || definition.Stage != "runtime-supervision" {
		t.Fatalf("runtime definition=%#v ok=%v", definition, ok)
	}
}

func TestLookupDefinitionNormalizesOnlyTrimAndCase(t *testing.T) {
	definition, ok := LookupDefinition("  yunka-dx-contract-002  ")
	if !ok || definition.Code != CodeContractDrift {
		t.Fatalf("definition=%#v ok=%v", definition, ok)
	}
	for _, query := range []string{
		"YUNKA-DX-CONTRACT-02",
		"YUNKA-DX-CONTRACT-002-extra",
		"CONTRACT-002",
		"YUNKA DX CONTRACT 002",
	} {
		if _, ok := LookupDefinition(query); ok {
			t.Fatalf("catalog fuzzy-matched %q", query)
		}
	}
}

func TestDefinitionReturnsIndependentActionCopies(t *testing.T) {
	first := MustDefinition(CodeContractDrift)
	second := MustDefinition(CodeContractDrift)
	if len(first.Actions) != 1 || len(second.Actions) != 1 {
		t.Fatalf("actions first=%#v second=%#v", first.Actions, second.Actions)
	}
	first.Actions[0].Value = "mutated"
	if second.Actions[0].Value != "yunka generate" {
		t.Fatalf("catalog action leaked mutation: %#v", second.Actions)
	}
}
