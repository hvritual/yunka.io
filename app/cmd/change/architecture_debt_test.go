package change

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/auditcore"
)

func TestArchitectureDebtProofRejectsNewProvenDebt(t *testing.T) {
	root := t.TempDir()
	servicePath := prepareT5AuditProject(t, root, "package application\n")
	baseSHA := commitT5AuditBaseline(t, root)

	mustWrite(t, servicePath, `package application

import "github.com/hvritual/yunka.io/gateway/authz"

var _ authz.Authorizer
`)
	debt, err := collectArchitectureDebt(root, baseSHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(debt.Existing) != 0 || len(debt.New) != 1 || len(debt.Fixed) != 0 {
		t.Fatalf("debt=%#v", debt)
	}
	if debt.New[0].Rule != auditcore.RuleAuthorizationBypass {
		t.Fatalf("new debt=%#v", debt.New)
	}

	attestation := ChangeAttestation{}
	recordArchitectureDebt(&attestation, debt)
	gate, ok := architectureDebtGate(attestation.Gates)
	if !ok || gate.Status != "fail" || !strings.Contains(gate.Detail, "new=1") {
		t.Fatalf("gate=%#v gates=%#v", gate, attestation.Gates)
	}
	if len(attestation.Diagnostics) != 1 || attestation.Diagnostics[0].Stage != "architecture-debt" {
		t.Fatalf("diagnostics=%#v", attestation.Diagnostics)
	}
	if attestation.ArchitectureDebt == nil || len(attestation.ArchitectureDebt.New) != 1 {
		t.Fatalf("architectureDebt=%#v", attestation.ArchitectureDebt)
	}
}

func TestArchitectureDebtProofKeepsExistingDebtNonblocking(t *testing.T) {
	root := t.TempDir()
	prepareT5AuditProject(t, root, `package application

import "github.com/hvritual/yunka.io/framework/platform"

var _ = platform.Provider{}
`)
	baseSHA := commitT5AuditBaseline(t, root)

	debt, err := collectArchitectureDebt(root, baseSHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(debt.Existing) != 1 || len(debt.New) != 0 || len(debt.Fixed) != 0 {
		t.Fatalf("debt=%#v", debt)
	}
	if debt.Existing[0].Rule != auditcore.RulePlatformProviderBypass {
		t.Fatalf("existing debt=%#v", debt.Existing)
	}

	attestation := ChangeAttestation{}
	recordArchitectureDebt(&attestation, debt)
	gate, ok := architectureDebtGate(attestation.Gates)
	if !ok || gate.Status != "pass" || !strings.Contains(gate.Detail, "existing=1") {
		t.Fatalf("gate=%#v gates=%#v", gate, attestation.Gates)
	}
	if len(attestation.Diagnostics) != 0 {
		t.Fatalf("existing debt must not become newly blocking: %#v", attestation.Diagnostics)
	}
}

func TestArchitectureDebtProofAcceptsFixedDebtWithoutPromotingHistoricalDebt(t *testing.T) {
	root := t.TempDir()
	servicePath := prepareT5AuditProject(t, root, `package application

import "github.com/hvritual/yunka.io/framework/platform"

var _ = platform.Provider{}
`)
	baseSHA := commitT5AuditBaseline(t, root)
	mustWrite(t, servicePath, "package application\n")

	debt, err := collectArchitectureDebt(root, baseSHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(debt.New) != 0 || len(debt.Fixed) != 1 || debt.Fixed[0].Rule != auditcore.RulePlatformProviderBypass {
		t.Fatalf("debt=%#v", debt)
	}

	attestation := ChangeAttestation{}
	recordArchitectureDebt(&attestation, debt)
	gate, ok := architectureDebtGate(attestation.Gates)
	if !ok || gate.Status != "pass" || !strings.Contains(gate.Detail, "fixed=1") {
		t.Fatalf("gate=%#v gates=%#v", gate, attestation.Gates)
	}
	if len(attestation.Diagnostics) != 0 {
		t.Fatalf("fixed debt must not create blocking diagnostics: %#v", attestation.Diagnostics)
	}
}

func TestChangeAttestationV2RendersArchitectureDebt(t *testing.T) {
	debt := auditcore.DebtDelta{
		BaseRef:  "abc123",
		BaseSHA:  "abc123",
		Existing: []auditcore.Finding{},
		New:      []auditcore.Finding{},
		Fixed:    []auditcore.Finding{},
	}
	attestation := ChangeAttestation{
		SchemaVersion: ChangeAttestationSchemaVersion,
		BaseSHA:       "abc123",
		HeadSHA:       "def456",
		OperationID:   "tenant.suspend",
		Gates:         []GateResult{},
		Diagnostics:   nil,
		Conformant:    true,
	}
	recordArchitectureDebt(&attestation, debt)
	first, err := RenderChangeAttestation(attestation, DefaultChangeAttestationPath, FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderChangeAttestation(attestation, DefaultChangeAttestationPath, FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("change attestation v2 output is not deterministic")
	}
	for _, expected := range []string{`"schemaVersion": 2`, `"architectureDebt"`, `"architecture-debt"`} {
		if !strings.Contains(first, expected) {
			t.Fatalf("attestation missing %s:\n%s", expected, first)
		}
	}
}

func prepareT5AuditProject(t *testing.T, root, service string) string {
	t.Helper()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	mustWrite(t, filepath.Join(root, "contracts", "proto", "tenant.proto"), "syntax = \"proto3\";\n")
	manifest := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files: []contract.File{{
			Name:   "tenant.proto",
			Domain: &contract.DomainDeclaration{Name: "tenant"},
		}},
	}
	contents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "contracts", "generated", contract.ManifestFilename), string(append(contents, '\n')))
	servicePath := filepath.Join(root, "internal", "tenant", "application", "service.go")
	mustWrite(t, servicePath, service)
	return servicePath
}

func commitT5AuditBaseline(t *testing.T, root string) string {
	t.Helper()
	gitT5(t, root, "init")
	gitT5(t, root, "config", "user.email", "t5@example.invalid")
	gitT5(t, root, "config", "user.name", "Yunka T5 Test")
	gitT5(t, root, "add", ".")
	gitT5(t, root, "commit", "-m", "baseline")
	return gitT5Output(t, root, "rev-parse", "HEAD")
}

func gitT5(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func gitT5Output(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output))
}

func architectureDebtGate(gates []GateResult) (GateResult, bool) {
	for _, gate := range gates {
		if gate.Name == "architecture-debt" {
			return gate, true
		}
	}
	return GateResult{}, false
}
