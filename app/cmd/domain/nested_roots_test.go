package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNestedDomainExemptionPreservesMixedLayout(t *testing.T) {
	root, internal := newCoverageTestProject(t)
	for _, path := range []string{"legacy/domain/record.go", "modules/interaction/domain/record.go"} {
		writeCoverageTestFile(t, filepath.Join(internal, filepath.FromSlash(path)), "package domain\n")
	}
	writeCoverageTestContract(t, root, CoverageContract{SchemaVersion: 1, Exemptions: []CoverageExemption{
		{Domain: "legacy", Reason: "handwritten rules"}, {Domain: "interaction", Reason: "handwritten delivery facts"},
	}})
	entries, err := ValidateCoverage(internal)
	if err != nil || len(entries) != 2 {
		t.Fatalf("mixed inventory: %v, %v", entries, err)
	}
	if entries[0].Domain != "interaction" || entries[0].Root != filepath.Join(internal, "modules", "interaction") || entries[0].State != CoverageExempt {
		t.Fatalf("nested ownership lost: %#v", entries[0])
	}
	if count, err := CheckAll(internal); err != nil || count != 0 {
		t.Fatalf("handwritten trees became generated: count=%d err=%v", count, err)
	}
}

func TestNestedUnregisteredDomainCannotHideInGroupingDirectory(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "modules", "unregistered", "domain", "record.go"), "package domain\n")
	for name, check := range map[string]func(string) (int, error){"check": CheckAll, "generate": RegenerateAll} {
		if _, err := check(internal); err == nil || !strings.Contains(err.Error(), "UNMANAGED_DOMAIN_TOPOLOGY") {
			t.Fatalf("%s silently ignored nested source: %v", name, err)
		}
	}
}

func TestDuplicateDomainIdentityAcrossLayoutsIsRejected(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	for _, path := range []string{"customer/domain/record.go", "modules/customer/domain/record.go"} {
		writeCoverageTestFile(t, filepath.Join(internal, filepath.FromSlash(path)), "package domain\n")
	}
	if _, err := InspectCoverage(internal); err == nil || !strings.Contains(err.Error(), "DOMAIN_IDENTITY_CONFLICT") {
		t.Fatalf("duplicate identity was accepted: %v", err)
	}
}

func TestNestedManagedDomainSharesGenerationCheckAndAuditOwnership(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	if err := Generate(Options{Name: "catalog", Root: filepath.Join(internal, "modules"), Global: true}); err != nil {
		t.Fatal(err)
	}
	if count, err := RegenerateAll(internal); err != nil || count != 1 {
		t.Fatalf("nested regeneration: count=%d err=%v", count, err)
	}
	if count, err := CheckAll(internal); err != nil || count != 1 {
		t.Fatalf("nested check: count=%d err=%v", count, err)
	}
	paths, err := GeneratedPaths(internal)
	if err != nil || len(paths) == 0 {
		t.Fatalf("nested generated ownership: %v %v", paths, err)
	}
	for _, path := range paths {
		if !strings.HasPrefix(path, "modules/catalog/") {
			t.Fatalf("generated ownership escaped nested root: %s", path)
		}
	}
	generated := firstDomainGeneratedGoFile(t, filepath.Join(internal, "modules", "catalog"))
	writeCoverageTestFile(t, generated, "package domain\n")
	if err := Check(internal); err == nil {
		t.Fatal("nested generated corruption escaped Check")
	}
	issues, err := InspectGeneratedArtifacts(internal)
	if err != nil {
		t.Fatal(err)
	}
	assertGeneratedIssue(t, issues, GeneratedOwnershipConflict)
}

func TestDomainDiscoveryRejectsNestedModuleBoundary(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "modules", "other", "go.mod"), "module example.com/other\n")
	if _, err := InspectCoverage(internal); err == nil || !strings.Contains(err.Error(), "DOMAIN_SOURCE_MODULE") {
		t.Fatalf("nested module accepted: %v", err)
	}
}

func TestDomainDiscoveryRejectsDirectoryAliases(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	if err := os.Symlink(t.TempDir(), filepath.Join(internal, "modules")); err != nil {
		t.Skipf("host cannot create symlink: %v", err)
	}
	if _, err := InspectCoverage(internal); err == nil || !strings.Contains(err.Error(), "DOMAIN_SOURCE_ALIAS") {
		t.Fatalf("directory alias accepted: %v", err)
	}
}
