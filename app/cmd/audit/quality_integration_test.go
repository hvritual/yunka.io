package audit

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/auditcore"
)

func TestBuildWithBaseClassifiesDeclaredBlockingQualityDebt(t *testing.T) {
	root := t.TempDir()
	writeAuditProjectFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"), "syntax = \"proto3\";\n")
	manifest := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files: []contract.File{{Name: "tenant.proto", Domain: &contract.DomainDeclaration{Name: "tenant"}}},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "generated", contract.ManifestFilename), string(append(manifestBytes, '\n')))
	writeAuditProjectFile(t, filepath.Join(root, ".yunka", "engineering-quality.json"), `{
  "schemaVersion": 1,
  "limits": {"maxFileLines": 8},
  "blockingRules": ["AUDIT-SIZE-001"]
}
`)
	servicePath := filepath.Join(root, "internal", "tenant", "application", "service.go")
	writeAuditProjectFile(t, servicePath, `// Package application owns tenant use-case orchestration.
package application

func Work() {}
`)
	gitAudit(t, root, "init")
	gitAudit(t, root, "config", "user.email", "audit@example.invalid")
	gitAudit(t, root, "config", "user.name", "Yunka Audit Test")
	gitAudit(t, root, "add", ".")
	gitAudit(t, root, "commit", "-m", "baseline")

	writeAuditProjectFile(t, servicePath, `// Package application owns tenant use-case orchestration.
package application

func Work(v int) int {
	if v > 0 {
		v++
	}
	if v > 10 {
		v--
	}
	return v
}
`)
	before, err := auditTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildWithBase(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	after, err := auditTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("quality audit mutated project: before=%s after=%s", before, after)
	}
	blocking := auditcore.BlockingNewFindings(report)
	if len(blocking) != 1 || blocking[0].Rule != auditcore.RuleFileLineLimit {
		t.Fatalf("blocking=%#v debt=%#v", blocking, report.Debt)
	}
	if report.Debt == nil || len(report.Debt.New) != 1 || len(report.Debt.Existing) != 0 || len(report.Debt.Fixed) != 0 {
		t.Fatalf("debt=%#v", report.Debt)
	}
	text, err := Render(report, "text")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "blocking_new=1") || !strings.Contains(text, "NEW BLOCKING AUDIT-SIZE-001") {
		t.Fatalf("text output missing blocking debt:\n%s", text)
	}
	agent, err := Render(report, "agent-json")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"qualityPolicy"`, `"blocking": true`, auditcore.RuleFileLineLimit} {
		if !strings.Contains(agent, expected) {
			t.Fatalf("agent output missing %q:\n%s", expected, agent)
		}
	}
}
