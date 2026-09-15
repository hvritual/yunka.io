package change

import (
	"path/filepath"
	"testing"

	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/projectflow"
)

func TestQualityMigrationOwnershipFixtureDiagnostic(t *testing.T) {
	root := prepareQualityMigrationFixture(t)
	mustWrite(t, filepath.Join(root, "internal", "tenant", "domain", "model.go"), "package domain\n\ntype Tenant struct{}\n")

	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: root})
	if err != nil {
		t.Fatalf("describe project: %v", err)
	}
	if descriptor.GeneratedGoRoot != "internal" {
		t.Fatalf("generatedGoRoot=%q descriptor=%#v", descriptor.GeneratedGoRoot, descriptor)
	}
	report, err := ownership.Build(root, []string{
		"internal/tenant/domain/model.go",
		"internal/tenant/domain/tenant.go",
	})
	if err != nil {
		t.Fatalf("ownership build: %v; descriptor=%#v", err, descriptor)
	}
	if len(report.Decisions) != 2 {
		t.Fatalf("ownership decisions=%#v", report.Decisions)
	}
	for _, decision := range report.Decisions {
		if !decision.SafeAutoEdit {
			t.Fatalf("ownership rejected %s: owner=%s mutation=%s reason=%s descriptor=%#v", decision.Path, decision.Owner, decision.Mutation, decision.Reason, descriptor)
		}
	}
}
