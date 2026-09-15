package change

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/auditcore"
	"yunka.io/app/cmd/projectflow"
	"yunka.io/app/cmd/sourceaudit"
)

func TestQualityMigrationGenericContainerSplitProvesNoneDeltas(t *testing.T) {
	root := prepareQualityMigrationFixture(t)
	modelPath := filepath.Join(root, "internal", "tenant", "domain", "model.go")
	mustWrite(t, modelPath, `package domain

type Tenant struct{ ID string }
type Membership struct{ TenantID string }
type Role struct{ Name string }
type Permission struct{ Code string }
`)
	commitQualityMigrationBaseline(t, root)

	plan, projectRoot, err := BuildQualityMigrationPlan(context.Background(), projectflow.Options{Root: root}, "HEAD", []string{MigrationRecipeGenericContainerSplit}, []string{
		"internal/tenant/domain/model.go",
		"internal/tenant/domain/tenant.go",
		"internal/tenant/domain/membership.go",
		"internal/tenant/domain/role.go",
		"internal/tenant/domain/permission.go",
	}, migrationNarrative("Split unrelated domain concepts into semantic files."))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.BaselineFindings) == 0 || !migrationPlanHasRule(plan, auditcore.RuleGenericContainerCohesion) {
		t.Fatalf("baseline findings=%#v", plan.BaselineFindings)
	}
	if _, err := WriteQualityMigrationPlan(projectRoot, DefaultQualityMigrationPlanPath, plan); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(modelPath); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "internal", "tenant", "domain", "tenant.go"), "package domain\n\ntype Tenant struct{ ID string }\n")
	mustWrite(t, filepath.Join(root, "internal", "tenant", "domain", "membership.go"), "package domain\n\ntype Membership struct{ TenantID string }\n")
	mustWrite(t, filepath.Join(root, "internal", "tenant", "domain", "role.go"), "package domain\n\ntype Role struct{ Name string }\n")
	mustWrite(t, filepath.Join(root, "internal", "tenant", "domain", "permission.go"), "package domain\n\ntype Permission struct{ Code string }\n")
	gitT5(t, root, "add", "-A")
	gitT5(t, root, "commit", "-m", "split domain concepts")

	packet, err := CheckQualityMigration(context.Background(), projectflow.Options{Root: root}, DefaultQualityMigrationPlanPath)
	if err != nil {
		t.Fatal(err)
	}
	if !packet.Conformant {
		t.Fatalf("packet violations=%#v", packet.Violations)
	}
	for name, delta := range map[string]ReviewDelta{
		"behavior": packet.BehaviorChange, "api": packet.PublicAPIChange,
		"persistence": packet.PersistenceChange, "generated": packet.GeneratedCodeChange,
	} {
		if delta.State != ReviewDeltaNone {
			t.Fatalf("%s delta=%#v", name, delta)
		}
	}
	if packet.QualityDebt == nil || packet.QualityDebt.BlockingNew != 0 {
		t.Fatalf("quality debt=%#v", packet.QualityDebt)
	}
	first, err := RenderQualityMigrationReview(packet, FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderQualityMigrationReview(packet, FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !strings.Contains(first, `"behaviorChange"`) || !strings.Contains(first, `"state": "NONE"`) {
		t.Fatalf("non-deterministic or incomplete packet:\n%s", first)
	}
}

func TestQualityMigrationDurableTestRenameFixesHistoricalIdentity(t *testing.T) {
	root := prepareQualityMigrationFixture(t)
	oldPath := filepath.Join(root, "internal", "tenant", "domain", "ce09_replay_test.go")
	mustWrite(t, oldPath, `package domain

import "testing"

func TestCE09ReplayOwnerInvariant(t *testing.T) {}
`)
	commitQualityMigrationBaseline(t, root)
	plan, projectRoot, err := BuildQualityMigrationPlan(context.Background(), projectflow.Options{Root: root}, "HEAD", []string{MigrationRecipeDurableTestRename}, []string{
		"internal/tenant/domain/ce09_replay_test.go",
		"internal/tenant/domain/owner_invariant_test.go",
	}, migrationNarrative("Rename durable regression evidence by the invariant it proves."))
	if err != nil {
		t.Fatal(err)
	}
	if !migrationPlanHasRule(plan, auditcore.RuleHistoricalSourceIdentity) {
		t.Fatalf("baseline findings=%#v", plan.BaselineFindings)
	}
	if _, err := WriteQualityMigrationPlan(projectRoot, DefaultQualityMigrationPlanPath, plan); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "internal", "tenant", "domain", "owner_invariant_test.go"), `package domain

import "testing"

func TestOwnerInvariantReplay(t *testing.T) {}
`)
	gitT5(t, root, "add", "-A")
	gitT5(t, root, "commit", "-m", "rename durable test")
	packet, err := CheckQualityMigration(context.Background(), projectflow.Options{Root: root}, DefaultQualityMigrationPlanPath)
	if err != nil {
		t.Fatal(err)
	}
	if !packet.Conformant || packet.BehaviorChange.State != ReviewDeltaNone || packet.PublicAPIChange.State != ReviewDeltaNone {
		t.Fatalf("packet=%#v", packet)
	}
	if packet.QualityDebt.DeterministicFixed == 0 {
		t.Fatalf("expected fixed historical identity debt: %#v", packet.QualityDebt)
	}
}

func TestQualityMigrationRejectsBehaviorAndScopeDrift(t *testing.T) {
	root := prepareQualityMigrationFixture(t)
	servicePath := filepath.Join(root, "internal", "tenant", "domain", "role.go")
	mustWrite(t, servicePath, "package domain\n\ntype Role struct{ Name string }\n")
	mustWrite(t, filepath.Join(root, "internal", "tenant", "domain", "model.go"), `package domain

type Tenant struct{}
type Membership struct{}
type Permission struct{}
`)
	commitQualityMigrationBaseline(t, root)
	plan, projectRoot, err := BuildQualityMigrationPlan(context.Background(), projectflow.Options{Root: root}, "HEAD", []string{MigrationRecipeGenericContainerSplit}, []string{
		"internal/tenant/domain/model.go",
		"internal/tenant/domain/tenant.go",
		"internal/tenant/domain/membership.go",
		"internal/tenant/domain/permission.go",
	}, migrationNarrative("Split the generic container without behavior changes."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteQualityMigrationPlan(projectRoot, DefaultQualityMigrationPlanPath, plan); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, servicePath, "package domain\n\ntype Role struct{ Name string; Changed bool }\n")
	gitT5(t, root, "add", "-A")
	gitT5(t, root, "commit", "-m", "out of scope behavior drift")
	packet, err := CheckQualityMigration(context.Background(), projectflow.Options{Root: root}, DefaultQualityMigrationPlanPath)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Conformant || len(packet.Violations) == 0 {
		t.Fatalf("expected fail-closed packet: %#v", packet)
	}
}

func prepareQualityMigrationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	mustWrite(t, filepath.Join(root, ".yunka", "project.json"), `{
  "version": 2,
  "database": {"tablePrefix": "demo"},
  "workflow": {
    "contract": {"protoRoot": "contracts/proto", "generated": "contracts/generated"},
    "modules": {"root": "modules"},
    "generatedGo": {"root": "internal"},
    "dev": {"manifest": ".yunka/dev.json"}
  }
}
`)
	mustWrite(t, filepath.Join(root, "contracts", "proto", "tenant.proto"), "syntax = \"proto3\";\npackage tenant.v1;\n")
	manifest := map[string]any{
		"schemaVersion": 1,
		"files": []any{map[string]any{"name": "tenant.proto", "domain": map[string]any{"name": "tenant"}}},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "contracts", "generated", "manifest.json"), string(append(manifestBytes, '\n')))
	mustWrite(t, filepath.Join(root, "internal", "tenant", "domain", "doc.go"), "// Package domain owns tenant identity and access concepts.\npackage domain\n")

	sourcePolicy := sourceaudit.Policy{
		SchemaVersion: sourceaudit.SchemaVersion,
		Profiles: []sourceaudit.Profile{{Name: "linux", GOOS: "linux", GOARCH: "amd64", CGO: false, Tags: []string{}}},
		Modules: []sourceaudit.ModulePolicy{{Path: ".", Workspace: "off", Profiles: []string{"linux"}}},
		Components: []sourceaudit.Component{{Name: "project", Path: ".", Kind: "production", Allow: []string{}, AllowExternal: true}},
		Exclusions: []sourceaudit.Exclusion{}, TestSupportImports: []string{},
	}
	sourceBytes, err := json.MarshalIndent(sourcePolicy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, ".yunka", "source-policy.json"), string(append(sourceBytes, '\n')))
	qualityPolicy := auditcore.QualityPolicy{
		SchemaVersion: auditcore.QualityPolicySchemaVersion,
		BlockingRules: []string{
			auditcore.RuleHistoricalSourceIdentity,
			auditcore.RuleMissingPackageDocumentation,
			auditcore.RuleMissingContractDocumentation,
			auditcore.RuleGeneratedOwnershipMix,
			auditcore.RuleStaleGeneratedArtifact,
			auditcore.RuleGeneratedArtifactDrift,
		},
	}
	qualityBytes, err := json.MarshalIndent(qualityPolicy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, ".yunka", "engineering-quality.json"), string(append(qualityBytes, '\n')))
	return root
}

func commitQualityMigrationBaseline(t *testing.T, root string) {
	t.Helper()
	gitT5(t, root, "init")
	gitT5(t, root, "config", "user.email", "migration@example.invalid")
	gitT5(t, root, "config", "user.name", "Yunka Migration Test")
	gitT5(t, root, "add", ".")
	gitT5(t, root, "commit", "-m", "migration baseline")
}

func migrationNarrative(what string) ReviewNarrative {
	return ReviewNarrative{
		Problem: "Historical source is harder to review than necessary.",
		CurrentConcepts: []string{"historical aggregate"},
		DesiredOwnership: []string{"semantic ownership"},
		Why: "Reduce historical reviewability debt without changing behavior.",
		What: what,
		Boundary: "Behavior, public API, persistence and generated ownership remain unchanged.",
		AffectedInvariants: []string{"behavior-preservation"},
		Risks: []string{"accidental semantic drift"},
		UnresolvedFindings: []string{},
	}
}

func migrationPlanHasRule(plan QualityMigrationPlan, rule string) bool {
	for _, finding := range plan.BaselineFindings {
		if finding.Rule == rule {
			return true
		}
	}
	return false
}
