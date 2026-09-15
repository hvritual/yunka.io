package advisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/advisorcore"
)

func TestSemanticReviewCommandIsRegistered(t *testing.T) {
	command := Command()
	found := false
	for _, subcommand := range command.Subcommands {
		if subcommand.Name == "semantic" {
			found = true
			if len(subcommand.Subcommands) != 3 {
				t.Fatalf("semantic subcommands=%d want 3", len(subcommand.Subcommands))
			}
		}
	}
	if !found {
		t.Fatal("advisor semantic command is not registered")
	}
}

func TestSemanticReviewHumanProjectionExplainsFindingWithoutConversation(t *testing.T) {
	attestation := advisorcore.SemanticReviewAttestation{
		SchemaVersion:  advisorcore.SemanticReviewSchemaVersion,
		Authority:      advisorcore.AuthorityAdvisoryOnly,
		RequestDigest:  strings.Repeat("1", 64),
		ResponseDigest: strings.Repeat("2", 64),
		SourceIdentity: strings.Repeat("3", 64),
		Result:         advisorcore.SemanticReviewResultValid,
		Findings: []advisorcore.SemanticFinding{{
			ID:                     "semantic-example",
			Path:                   "internal/tenant/application/service.go",
			SymbolOrScope:          "Service",
			Category:               advisorcore.SemanticCategoryAmbiguousResponsibility,
			Severity:               advisorcore.SemanticSeverityHigh,
			Reason:                 "The service owns unrelated lifecycle and quota decisions.",
			RecommendedAction:      advisorcore.SemanticRecommendedAction{Kind: advisorcore.SemanticActionDiscussDesign, Detail: "Review whether quota policy needs a separate owner."},
			BehaviorChangeRequired: false,
			SourceIdentity:         strings.Repeat("3", 64),
		}},
	}
	output, err := RenderSemanticAttestation(attestation, "text")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"ambiguous_responsibility",
		"internal/tenant/application/service.go",
		"Service",
		"unrelated lifecycle and quota decisions",
		"discuss_design",
		"behavior-change-required: false",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("human projection missing %q:\n%s", expected, output)
		}
	}
}

func TestSemanticReviewSourceReaderUsesExactRegularUTF8Files(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "tenant", "domain", "tenant.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package domain\n\ntype Tenant struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sources, err := readSemanticSources(root, []string{"internal/tenant/domain/tenant.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].Path != "internal/tenant/domain/tenant.go" || !strings.Contains(sources[0].Content, "type Tenant struct") {
		t.Fatalf("sources=%#v", sources)
	}
	if _, err := readSemanticSources(root, []string{"../outside.go"}); err == nil {
		t.Fatal("escaping source path was accepted")
	}
	if _, err := readSemanticSources(root, []string{"internal/tenant/domain/tenant.go", "internal/tenant/domain/tenant.go"}); err == nil {
		t.Fatal("duplicate source path was accepted")
	}
}
