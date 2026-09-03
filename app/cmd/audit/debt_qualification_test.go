package audit

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/auditcore"
)

func TestBuildWithBaseClassifiesDebtWithoutMutatingRepository(t *testing.T) {
	root := t.TempDir()
	writeAuditProjectFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	manifest := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files: []contract.File{
			{Name: "tenant.proto", Domain: &contract.DomainDeclaration{Name: "tenant"}},
			{Name: "device.proto", Domain: &contract.DomainDeclaration{Name: "device"}},
		},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"), "syntax = \"proto3\";\n")
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "generated", contract.ManifestFilename), string(append(manifestBytes, '\n')))
	servicePath := filepath.Join(root, "internal", "tenant", "application", "service.go")
	writeAuditProjectFile(t, servicePath, `package application

import (
	"example.com/demo/internal/device/ports"
	"github.com/hvritual/yunka.io/framework/platform"
)

var _ = platform.Provider{}
`)
	gitAudit(t, root, "init")
	gitAudit(t, root, "config", "user.email", "audit@example.invalid")
	gitAudit(t, root, "config", "user.name", "Yunka Audit Test")
	gitAudit(t, root, "add", ".")
	gitAudit(t, root, "commit", "-m", "baseline")

	// Preserve one proven violation, remove one, and introduce one.
	writeAuditProjectFile(t, servicePath, `package application

import (
	"github.com/hvritual/yunka.io/framework/platform"
	"github.com/hvritual/yunka.io/gateway/authz"
)

var _ = platform.Provider{}
var _ authz.Authorizer
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
		t.Fatalf("debt audit mutated repository: before=%s after=%s", before, after)
	}
	if report.Debt == nil {
		t.Fatal("missing debt delta")
	}
	if len(report.Debt.Existing) != 1 || report.Debt.Existing[0].Rule != auditcore.RulePlatformProviderBypass {
		t.Fatalf("existing=%#v", report.Debt.Existing)
	}
	if len(report.Debt.New) != 1 || report.Debt.New[0].Rule != auditcore.RuleAuthorizationBypass {
		t.Fatalf("new=%#v", report.Debt.New)
	}
	if len(report.Debt.Fixed) != 1 || report.Debt.Fixed[0].Rule != auditcore.RuleCrossDomainRepositoryBypass {
		t.Fatalf("fixed=%#v", report.Debt.Fixed)
	}
	first, err := Render(report, "agent-json")
	if err != nil {
		t.Fatal(err)
	}
	secondReport, err := BuildWithBase(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(secondReport, "agent-json")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("debt audit output is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	for _, expected := range []string{`"existing"`, `"new"`, `"fixed"`, auditcore.RuleAuthorizationBypass} {
		if !strings.Contains(first, expected) {
			t.Fatalf("agent-json missing %q:\n%s", expected, first)
		}
	}
}

func TestBuildWithBaseFailsClosedAcrossModuleIdentityChange(t *testing.T) {
	root := t.TempDir()
	writeAuditProjectFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"), "syntax = \"proto3\";\n")
	manifest := contract.Manifest{SchemaVersion: contract.ManifestVersion, Files: []contract.File{{Name: "tenant.proto", Domain: &contract.DomainDeclaration{Name: "tenant"}}}}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "generated", contract.ManifestFilename), string(append(manifestBytes, '\n')))
	writeAuditProjectFile(t, filepath.Join(root, "internal", "tenant", "application", "service.go"), "package application\n")
	gitAudit(t, root, "init")
	gitAudit(t, root, "config", "user.email", "audit@example.invalid")
	gitAudit(t, root, "config", "user.name", "Yunka Audit Test")
	gitAudit(t, root, "add", ".")
	gitAudit(t, root, "commit", "-m", "baseline")

	writeAuditProjectFile(t, filepath.Join(root, "go.mod"), "module example.com/renamed\n\ngo 1.25.0\n")
	_, err = BuildWithBase(root, "HEAD")
	if err == nil || !strings.Contains(err.Error(), "baseline module") {
		t.Fatalf("expected module-identity failure, got %v", err)
	}
}

func gitAudit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
