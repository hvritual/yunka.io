package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectGeneratedArtifactsUsesCanonicalRendererEvidence(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistence, "coffee_machine.go", `package persistence

type CoffeeMachinePO struct { Serial string }
`)
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(root, "internal", "device")
	issues, err := InspectGeneratedArtifacts(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("clean generated state produced issues: %#v", issues)
	}

	generatedPath := firstDomainGeneratedGoFile(t, domainRoot)
	if err := os.WriteFile(generatedPath, []byte("package domain\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	issues, err = InspectGeneratedArtifacts(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	assertGeneratedIssue(t, issues, GeneratedOwnershipConflict)

	if err := Regenerate(domainRoot); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(generatedPath, append(contents, []byte("\n// drift\n")...), 0o640); err != nil {
		t.Fatal(err)
	}
	issues, err = InspectGeneratedArtifacts(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	assertGeneratedIssue(t, issues, GeneratedArtifactDrift)

	if err := Regenerate(domainRoot); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(domainRoot, "domain", "zz_yunka_stale_gen.go")
	if err := os.MkdirAll(filepath.Dir(stale), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte(generatedDomainMarker+"\npackage domain\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	issues, err = InspectGeneratedArtifacts(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	assertGeneratedIssue(t, issues, GeneratedStaleArtifact)
}

func firstDomainGeneratedGoFile(t *testing.T, root string) string {
	t.Helper()
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(string(contents), generatedDomainMarker) && found == "" {
			found = path
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Fatal("generated Domain fixture has no generated Go file")
	}
	return found
}

func assertGeneratedIssue(t *testing.T, issues []GeneratedArtifactIssue, kind GeneratedArtifactIssueKind) {
	t.Helper()
	for _, issue := range issues {
		if issue.Kind == kind {
			if issue.Path == "" || issue.Domain == "" || issue.Reason == "" {
				t.Fatalf("incomplete generated issue: %#v", issue)
			}
			return
		}
	}
	t.Fatalf("missing generated issue %s: %#v", kind, issues)
}
