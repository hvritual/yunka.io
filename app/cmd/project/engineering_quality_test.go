package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEngineeringQualityBaselineFreshInstallIsDeterministicAndIdempotent(t *testing.T) {
	root := t.TempDir()
	writeProjectTestFile(t, filepath.Join(root, "go.mod"), "module example.com/quality\n\ngo 1.25.0\n")

	first, err := EnsureEngineeringQualityBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if first.Baseline != EngineeringQualityBaselineRelativePath || first.PolicyVersion != EngineeringQualityPolicyVersion || first.PolicyIdentity != engineeringQualityPolicyIdentity() {
		t.Fatalf("install report=%#v", first)
	}
	paths := []string{
		EngineeringQualityBaselineRelativePath,
		EngineeringQualityRulesRelativePath,
		".yunka/engineering-quality.json",
		EngineeringQualityInstructionsPath,
	}
	before := map[string]string{}
	for _, relative := range paths {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		before[relative] = string(contents)
	}
	baseline, err := decodeEngineeringQualityBaseline([]byte(before[EngineeringQualityBaselineRelativePath]))
	if err != nil {
		t.Fatal(err)
	}
	if baseline.PolicyIdentity != engineeringQualityPolicyIdentity() || baseline.Provenance != EngineeringQualityProvenance || baseline.UpgradePolicy != EngineeringQualityUpgradePolicy {
		t.Fatalf("baseline=%#v", baseline)
	}
	for _, forbidden := range []string{"docs/STATUS.md", "#199", "#198", "AG-06", "current task"} {
		if strings.Contains(before[EngineeringQualityRulesRelativePath], forbidden) || strings.Contains(before[EngineeringQualityBaselineRelativePath], forbidden) {
			t.Fatalf("consumer baseline leaked framework history %q", forbidden)
		}
	}

	second, err := EnsureEngineeringQualityBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if second.UpgradeRequired != "" {
		t.Fatalf("unexpected upgrade request: %s", second.UpgradeRequired)
	}
	for _, relative := range paths {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if string(contents) != before[relative] {
			t.Fatalf("second init mutated %s", relative)
		}
	}
}

func TestEngineeringQualityBaselinePreservesConsumerOwnedConfigurationAndRuleEdits(t *testing.T) {
	root := t.TempDir()
	writeProjectTestFile(t, filepath.Join(root, "go.mod"), "module example.com/existing\n\ngo 1.25.0\n")
	customPolicy := "{\n  \"schemaVersion\": 1,\n  \"blockingRules\": []\n}\n"
	customInstructions := "# Existing repository instructions\n"
	writeProjectTestFile(t, filepath.Join(root, ".yunka", "engineering-quality.json"), customPolicy)
	writeProjectTestFile(t, filepath.Join(root, EngineeringQualityInstructionsPath), customInstructions)

	if _, err := EnsureEngineeringQualityBaseline(root); err != nil {
		t.Fatal(err)
	}
	assertProjectFileEquals(t, filepath.Join(root, ".yunka", "engineering-quality.json"), customPolicy)
	assertProjectFileEquals(t, filepath.Join(root, EngineeringQualityInstructionsPath), customInstructions)

	customRules := "# Project-reviewed engineering quality rules\n"
	writeProjectTestFile(t, filepath.Join(root, EngineeringQualityRulesRelativePath), customRules)
	report, err := EnsureEngineeringQualityBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.UpgradeRequired == "" || !strings.Contains(report.UpgradeRequired, "explicit reconciliation") {
		t.Fatalf("expected explicit reconciliation, got %#v", report)
	}
	assertProjectFileEquals(t, filepath.Join(root, EngineeringQualityRulesRelativePath), customRules)
	assertProjectFileEquals(t, filepath.Join(root, ".yunka", "engineering-quality.json"), customPolicy)
	assertProjectFileEquals(t, filepath.Join(root, EngineeringQualityInstructionsPath), customInstructions)
}

func TestEngineeringQualityBaselineOldVersionRequiresExplicitUpgradeWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	writeProjectTestFile(t, filepath.Join(root, "go.mod"), "module example.com/old\n\ngo 1.25.0\n")
	old := `{
  "schemaVersion": 1,
  "policyVersion": "yunka.engineering-quality/v0",
  "policyIdentity": "old-policy-identity",
  "provenance": "reviewed-local-baseline",
  "rulesPath": ".yunka/ENGINEERING_QUALITY.md",
  "canonicalRulesSha256": "old",
  "enforcementPolicyPath": ".yunka/engineering-quality.json",
  "projectInstructionsPath": "AGENTS.md",
  "upgradePolicy": "explicit-review-required"
}
`
	path := filepath.Join(root, EngineeringQualityBaselineRelativePath)
	writeProjectTestFile(t, path, old)
	report, err := EnsureEngineeringQualityBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.UpgradeRequired == "" || !strings.Contains(report.UpgradeRequired, "requires explicit review") {
		t.Fatalf("expected explicit upgrade requirement: %#v", report)
	}
	assertProjectFileEquals(t, path, old)
	if _, err := os.Stat(filepath.Join(root, EngineeringQualityRulesRelativePath)); !os.IsNotExist(err) {
		t.Fatalf("old baseline should not be silently upgraded; rules stat err=%v", err)
	}
}

func TestEngineeringQualityBaselineWithoutGoModuleIsExplicitlyDeferred(t *testing.T) {
	report, err := EnsureEngineeringQualityBaseline(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if report.Baseline != "" || !strings.Contains(report.Skipped, "go.mod") {
		t.Fatalf("report=%#v", report)
	}
}

func assertProjectFileEquals(t *testing.T, path, expected string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != expected {
		t.Fatalf("%s changed:\n%s", path, contents)
	}
}
