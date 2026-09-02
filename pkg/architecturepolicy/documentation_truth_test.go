package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentationTruthOwnership(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(content)
	}

	agents := read("AGENTS.md")
	memory := read("PROJECT_MEMORY.md")
	status := read("docs/STATUS.md")
	governance := read("docs/DOCUMENTATION_GOVERNANCE.md")
	readme := read("README.md")
	c10 := read("docs/waves/C10-runtime-assembly-framework-productization.md")
	c11 := read("docs/waves/C11-developer-experience-productization.md")

	for _, required := range []string{
		"Read `docs/STATUS.md` completely",
		"Current framework/wave/release/pressure state belongs only in `docs/STATUS.md`",
	} {
		if !strings.Contains(agents, required) {
			t.Errorf("AGENTS.md is missing documentation-truth rule %q", required)
		}
	}

	for _, required := range []string{
		"Document class: **DECISION**",
		"Current delivery/status authority: [`docs/STATUS.md`](docs/STATUS.md)",
		"Current open/deferred/proven pressure state belongs in `docs/STATUS.md`, not in this file.",
	} {
		if !strings.Contains(memory, required) {
			t.Errorf("PROJECT_MEMORY.md is missing durable-memory rule %q", required)
		}
	}
	for _, forbidden := range []string{
		"Repository visibility:",
		"Current `main`",
		"current `main`",
		"current main@",
		"Current Pressure truth",
		"current private personal repository",
	} {
		if strings.Contains(memory, forbidden) {
			t.Errorf("PROJECT_MEMORY.md contains volatile current-state fact %q", forbidden)
		}
	}

	for _, required := range []string{
		"Document class: **STATUS**",
		"## Current framework state",
		"## Current pressure frontier",
		"## Known deferred limitations",
	} {
		if !strings.Contains(status, required) {
			t.Errorf("docs/STATUS.md is missing status authority marker %q", required)
		}
	}

	for _, required := range []string{
		"One fact, one current owner.",
		"Status is centralized.",
		"Durable memory is not a task tracker",
		"HISTORICAL",
		"EVIDENCE",
	} {
		if !strings.Contains(governance, required) {
			t.Errorf("documentation governance is missing rule %q", required)
		}
	}

	for _, historicalDoc := range []struct {
		name    string
		content string
	}{
		{"C10 roadmap", c10},
		{"C11 roadmap", c11},
	} {
		if !strings.Contains(historicalDoc.content, "Document class: **HISTORICAL**") {
			t.Errorf("%s is not explicitly classified HISTORICAL", historicalDoc.name)
		}
		if !strings.Contains(historicalDoc.content, "Current status authority: [`docs/STATUS.md`](../STATUS.md)") {
			t.Errorf("%s does not point to the current status authority", historicalDoc.name)
		}
		if !strings.Contains(historicalDoc.content, "**not** current status truth") {
			t.Errorf("%s does not protect preserved planning prose from being read as current truth", historicalDoc.name)
		}
	}

	for _, required := range []string{
		"[`docs/STATUS.md`](docs/STATUS.md)",
		"[`docs/DOCUMENTATION_GOVERNANCE.md`](docs/DOCUMENTATION_GOVERNANCE.md)",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README.md is missing current documentation authority link %q", required)
		}
	}

	for _, forbidden := range []string{
		"the isolated legacy RPC generator remain compatibility artifacts",
		"without replacing the legacy RPC generator",
		"app/cmd/rpc/pb/` source remains as `legacy-api`",
		"old XR generator, legacy invoke transport, and generated memory dispatcher are still scheduled for atomic deletion",
	} {
		if strings.Contains(readme, forbidden) {
			t.Errorf("README.md still presents completed RPC migration as current: %q", forbidden)
		}
	}
}
