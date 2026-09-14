package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAllRejectsUnmanagedDomainTopology(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "access", "domain", "tenant.go"), "package domain\n")

	count, err := CheckAll(internal)
	if err == nil || !strings.Contains(err.Error(), "UNMANAGED_DOMAIN_TOPOLOGY") {
		t.Fatalf("CheckAll error=%v, want UNMANAGED_DOMAIN_TOPOLOGY", err)
	}
	if count != 0 {
		t.Fatalf("managed count=%d want 0", count)
	}
}

func TestCheckRejectsUnmanagedDomainTopology(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "access", "domain", "tenant.go"), "package domain\n")

	if err := Check(internal); err == nil || !strings.Contains(err.Error(), "UNMANAGED_DOMAIN_TOPOLOGY") {
		t.Fatalf("Check error=%v, want UNMANAGED_DOMAIN_TOPOLOGY", err)
	}
}

func TestRegenerateAllRejectsUnmanagedDomainTopology(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "access", "application", "service.go"), "package application\n")

	count, err := RegenerateAll(internal)
	if err == nil || !strings.Contains(err.Error(), "UNMANAGED_DOMAIN_TOPOLOGY") {
		t.Fatalf("RegenerateAll error=%v, want UNMANAGED_DOMAIN_TOPOLOGY", err)
	}
	if count != 0 {
		t.Fatalf("managed count=%d want 0", count)
	}
}

func TestCoverageExemptionAllowsIntentionalUnmanagedDomain(t *testing.T) {
	root, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "legacy", "domain", "model.go"), "package domain\n")
	writeCoverageTestContract(t, root, CoverageContract{
		SchemaVersion: DomainCoverageSchemaVersion,
		Exemptions: []CoverageExemption{{
			Domain: "legacy",
			Reason: "migration is tracked separately",
			Owner:  "platform",
		}},
	})

	entries, err := ValidateCoverage(internal)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].State != CoverageExempt || entries[0].Domain != "legacy" {
		t.Fatalf("unexpected coverage entries: %#v", entries)
	}
	count, err := CheckAll(internal)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("managed count=%d want 0", count)
	}
}

func TestCoverageClassifiesGeneratedDomainAsManaged(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	if err := Generate(Options{Name: "catalog", Root: internal, Global: true}); err != nil {
		t.Fatal(err)
	}

	entries, err := ValidateCoverage(internal)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].State != CoverageManaged || entries[0].Domain != "catalog" {
		t.Fatalf("unexpected coverage entries: %#v", entries)
	}
	count, err := CheckAll(internal)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("managed count=%d want 1", count)
	}
}

func TestCoverageDoesNotClassifyOrdinaryInternalPackageAsDomain(t *testing.T) {
	_, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "runtime", "cache.go"), "package runtime\n")

	entries, err := ValidateCoverage(internal)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("ordinary internal package classified as domain: %#v", entries)
	}
}

func TestCoverageRejectsExemptionForManagedDomain(t *testing.T) {
	root, internal := newCoverageTestProject(t)
	if err := Generate(Options{Name: "catalog", Root: internal, Global: true}); err != nil {
		t.Fatal(err)
	}
	writeCoverageTestContract(t, root, CoverageContract{
		SchemaVersion: DomainCoverageSchemaVersion,
		Exemptions: []CoverageExemption{{Domain: "catalog", Reason: "invalid overlap"}},
	})

	_, err := ValidateCoverage(internal)
	if err == nil || !strings.Contains(err.Error(), "DOMAIN_COVERAGE_CONFLICT") {
		t.Fatalf("ValidateCoverage error=%v, want DOMAIN_COVERAGE_CONFLICT", err)
	}
}

func TestCoverageRejectsStaleExemption(t *testing.T) {
	root, internal := newCoverageTestProject(t)
	writeCoverageTestContract(t, root, CoverageContract{
		SchemaVersion: DomainCoverageSchemaVersion,
		Exemptions: []CoverageExemption{{Domain: "missing", Reason: "stale declaration"}},
	})

	_, err := ValidateCoverage(internal)
	if err == nil || !strings.Contains(err.Error(), "DOMAIN_EXEMPTION_TARGET_INVALID") {
		t.Fatalf("ValidateCoverage error=%v, want DOMAIN_EXEMPTION_TARGET_INVALID", err)
	}
}

func TestCoverageExemptionRequiresReason(t *testing.T) {
	root, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "legacy", "domain", "model.go"), "package domain\n")
	writeCoverageTestContract(t, root, CoverageContract{
		SchemaVersion: DomainCoverageSchemaVersion,
		Exemptions:    []CoverageExemption{{Domain: "legacy"}},
	})

	_, err := ValidateCoverage(internal)
	if err == nil || !strings.Contains(err.Error(), "requires a reason") {
		t.Fatalf("ValidateCoverage error=%v, want missing-reason rejection", err)
	}
}

func TestCoverageRejectsNonCanonicalExemptionWhitespace(t *testing.T) {
	root, internal := newCoverageTestProject(t)
	writeCoverageTestFile(t, filepath.Join(internal, "legacy", "domain", "model.go"), "package domain\n")
	writeCoverageTestContract(t, root, CoverageContract{
		SchemaVersion: DomainCoverageSchemaVersion,
		Exemptions: []CoverageExemption{{
			Domain: " legacy",
			Reason: "migration is tracked separately",
		}},
	})

	_, err := ValidateCoverage(internal)
	if err == nil || !strings.Contains(err.Error(), "non-canonical whitespace") {
		t.Fatalf("ValidateCoverage error=%v, want non-canonical whitespace rejection", err)
	}
}

func newCoverageTestProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/coverage\n\ngo 1.25\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	internal := filepath.Join(root, "internal")
	if err := os.MkdirAll(internal, 0o750); err != nil {
		t.Fatal(err)
	}
	return root, internal
}

func writeCoverageTestContract(t *testing.T, root string, contract CoverageContract) {
	t.Helper()
	contents, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	contents = append(contents, '\n')
	writeCoverageTestFile(t, filepath.Join(root, filepath.FromSlash(DomainCoverageRelativePath)), string(contents))
}

func writeCoverageTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
}
