package explain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

func TestBuildKnownDiagnosticTextUsesCanonicalDefinition(t *testing.T) {
	result, err := Build(" yunka-dx-contract-002 ", "text")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit=%d output=%s", result.ExitCode, result.Output)
	}
	for _, want := range []string{
		diagnostic.CodeContractDrift,
		"stage:    contract",
		"meaning:  generated contract artifacts are stale",
		"action:   Regenerate: yunka generate",
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("output missing %q:\n%s", want, result.Output)
		}
	}
}

func TestBuildKnownDiagnosticJSONIsDeterministic(t *testing.T) {
	first, err := Build(diagnostic.CodeProjectResolve, "json")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(diagnostic.CodeProjectResolve, "JSON")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("json output is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
	var value envelope
	if err := json.Unmarshal([]byte(first.Output), &value); err != nil {
		t.Fatal(err)
	}
	if !value.OK || value.Code != diagnostic.CodeProjectResolve || value.Definition == nil {
		t.Fatalf("envelope=%#v", value)
	}
	if value.Definition.Stage != "project" || len(value.Definition.Actions) != 1 {
		t.Fatalf("definition=%#v", value.Definition)
	}
}

func TestBuildUnknownDiagnosticNeverFuzzyGuesses(t *testing.T) {
	for _, code := range []string{
		"YUNKA-DX-CONTRACT-02",
		"CONTRACT-002",
		"YUNKA-DX-CONTRACT-002-extra",
	} {
		result, err := Build(code, "text")
		if err != nil {
			t.Fatal(err)
		}
		if result.ExitCode != 1 || !strings.Contains(result.Output, "unknown diagnostic code:") {
			t.Fatalf("code=%q result=%#v", code, result)
		}
		if strings.Contains(result.Output, "generated contract artifacts are stale") {
			t.Fatalf("unknown code %q received invented/fuzzy meaning: %s", code, result.Output)
		}
	}
}

func TestBuildUnknownDiagnosticJSONIsMachineReadable(t *testing.T) {
	result, err := Build(" yunka-dx-future-001 ", "json")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("exit=%d output=%s", result.ExitCode, result.Output)
	}
	var value envelope
	if err := json.Unmarshal([]byte(result.Output), &value); err != nil {
		t.Fatal(err)
	}
	if value.OK || value.Code != "YUNKA-DX-FUTURE-001" || value.Error != "unknown diagnostic code" || value.Definition != nil {
		t.Fatalf("envelope=%#v", value)
	}
}

func TestBuildUnsupportedFormatUsesCanonicalCLIFormatDiagnostic(t *testing.T) {
	result, err := Build(diagnostic.CodeContractDrift, "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 || !strings.Contains(result.Output, diagnostic.CodeUnsupportedOutputFormat) {
		t.Fatalf("result=%#v", result)
	}
}

func TestBuildMissingCodeIsUsageFailure(t *testing.T) {
	result, err := Build("   ", "text")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 || result.Output != "explain: diagnostic code is required\n" {
		t.Fatalf("result=%#v", result)
	}
}
